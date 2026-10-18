# 秒杀系统（Seckill）

一个高并发秒杀后端：**JWT 鉴权 → 令牌桶限流 → Redis Lua 原子预扣 → RabbitMQ 异步落库 → MySQL**，
重点解决秒杀场景下的**超卖、重复下单、瞬时流量冲击、落库前崩溃、账目漂移**等问题。

---

## 技术栈

| 类别 | 选型 |
|---|---|
| 语言 / 框架 | Go · Gin |
| 数据库 | MySQL / MariaDB（`database/sql`） |
| 缓存 / 预扣 | Redis · `go-redis/v9` · Lua 脚本 |
| 消息队列 | RabbitMQ · `amqp091-go`（持久化 + publisher confirm + 手动 ack） |
| 鉴权 | JWT（`golang-jwt/jwt/v5`）+ bcrypt |
| 并发控制 | `singleflight`、`golang.org/x/time/rate` 令牌桶 |
| 工程能力 | pprof、优雅停机、健康检查、库存对账 |

---

## 架构

```
                          ┌──────────────────────────────────────────────┐
   client ─── HTTP ──────▶│              Gin HTTP Server                 │
                          │  middleware: JWT 鉴权 → 令牌桶限流(429)       │
                          └───────┬───────────────────────┬──────────────┘
                                  │                       │
                ┌─────────────────▼──────────┐   ┌────────▼─────────────────────┐
                │ 活动详情（读）              │   │ 秒杀下单（写）                │
                │ Cache-Aside                │   │ 1) Redis Lua 原子预扣         │
                │ · 正缓存                    │   │    判重 + 判库存 + DECR       │
                │ · 空对象缓存 → 防穿透        │   │ 2) RabbitMQ 发布(confirm)     │
                │ · singleflight → 防击穿      │   └────────┬─────────────────────┘
                │ · 随机 TTL → 防雪崩          │            │
                └────────────┬───────────────┘            ▼
                             │                  ┌──────────────────────────┐
                             │                  │ RabbitMQ  order_queue    │
                             │                  │ durable + persistent     │
                             │                  └────────────┬─────────────┘
                             ▼                               ▼
                      ┌─────────────┐              ┌────────────────────────┐
                      │   Redis     │              │ Consumer × N（手动 ack）│
                      │ stock/detail│◀── 失败回补 ──│ 幂等落库（唯一键）      │
                      │ bought      │              └───────────┬────────────┘
                      │ notexist    │                          │
                      └──────┬──────┘                          ▼
                             │                      ┌────────────────────────┐
                             └──── 对账(每 60s) ────▶│ MySQL seckill_orders   │
                                校准 Redis 库存        └────────────────────────┘
```

**下单路径**：
`JWT 鉴权 → 限流 → Redis Lua（原子判重 + 扣库存）→ 发布 MQ（等 confirm）→ 返回 200`
`→ 消费者异步落库（幂等）→ 失败则回补 Redis 库存`

---

## 核心实现

### 1. 防超卖：Redis Lua 原子预扣
把「判断是否已购 + 判断库存 + 扣减 + 标记已购」放进**同一个 Lua 脚本**，Redis 单线程原子执行：

```lua
if EXISTS(bought) then return -1 end       -- 已购买
if stock <= 0     then return -2 end       -- 库存不足
DECR stock
SET bought
```

- 判断与扣减不可分割，**并发下不会超卖**；
- 库存 key 不存在时按 0 处理（fail-closed，宁可不卖）；
- 订单表唯一键 `(user_id, activity_id)` 作为最终幂等兜底。

### 2. 削峰：MQ 异步落库
- HTTP 只负责「预扣 + 投递」，不等待 DB，前台吞吐大幅提升；
- 队列 `durable` + 消息 `Persistent` + **publisher confirm**，保证 broker 接管消息；
- 消费者**手动 ack**、重复键视为幂等成功；
- 发布失败或落库失败 → `Redis_cancel` 回补库存，避免"扣了库存没订单"。

### 3. 崩溃不丢单
预扣成功、落库前进程崩溃时，消息仍在 broker 中；重启后消费者继续落库。
通过 `kill -9` 崩溃测试验证：**新增订单数 == HTTP 200 数**，不会少卖。

### 4. 缓存三防（活动详情读接口）
| 问题 | 手段 |
|---|---|
| 穿透 | 空对象缓存 + 参数校验 |
| 击穿 | `singleflight` 合并同 key 的并发回源 |
| 雪崩 | 详情 TTL 随机抖动 + 启动预热 |

### 5. 库存对账
后台定时用订单台账校验 `Redis 库存 == 活动库存 − 已售`；
只在 **MQ 队列为空（无在途订单）** 时判定，避免把在途延迟误判为漂移，发现持久差异则告警。

### 6. 限流、健康检查与优雅停机
- 令牌桶限流（可配置速率/桶容量），超限返回 `429`；
- `/ping` 为 liveness，`/ready` 为 readiness（检查 MySQL/Redis/MQ 与关机状态）；
- `SIGTERM`：先让 `/ready` 返回 503 摘流量 → HTTP 请求 drain → 停止消费者 → 按序关闭 MQ/DB/Redis。

---

## 压测表现

通过**叠加技术栈**逐步提升并发处理能力（同一台机器，wrk 压测）：

| 阶段 | 技术改动 | 前台 QPS | 落库速率 |
|---|---|---|---|
| 同步写库 | MySQL 事务 + 行锁 | 1398 | 1398 /s |
| Redis 预扣 + MQ 异步 | 削峰、请求不再等 DB | 3905 | ~490 /s |
| 多消费者 worker 池 | 10 个消费者并行落库 | **4232** | **~2151 /s** |

- 前台 QPS：**1398 → 4232（约 3 倍）**；
- 落库吞吐：1398 → ~2151 /s；
- 压测期间队列积压达数万，起到削峰缓冲作用；
- 全程一致性校验通过：**新增订单 == 接受请求 + 在途**。

---

## 运行

```dotenv
# .env
JWT_SECRET_KEY="<32+ 字节随机串>"
DSN="<user>:<pass>@tcp(127.0.0.1:3306)/seckill"
MQ="amqp://<user>:<pass>@localhost:5672/"
LIMIT_R=300          # 令牌桶速率 req/s
LIMIT_BURST=500      # 桶容量
```

```bash
go run main.go
```

- API：`http://127.0.0.1:8080`
- pprof：`http://localhost:6060/debug/pprof/`

> `.env` 含密钥，不要提交到仓库。

---

## 接口

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/ping` | 否 | liveness |
| GET | `/ready` | 否 | readiness |
| POST | `/login` | 否 | 登录，返回 JWT |
| GET | `/api/activity?id=` | 是 | 活动详情（Cache-Aside） |
| POST | `/api/seckill` | 是 | 秒杀下单（Redis 预扣 + MQ） |

鉴权：`Authorization: Bearer <token>`。

---

## 已知限制

- 读缓存目前只覆盖活动详情；
- MQ 未接死信队列，消费失败直接丢弃（库存已回补）；
- publisher confirm 超时存在语义二义性，依赖对账兜底；
- 限流为单机全局桶，按用户/IP 限流未做。

---

## License

仅用于学习与求职展示。
