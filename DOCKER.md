# Docker 一键部署

本项目提供 `docker-compose.yml`，一条命令即可拉起 **应用 + MySQL + Redis + RabbitMQ** 四个服务。

## 前置

- Docker 24+ 与 Docker Compose v2（`docker compose version` 可用）
- 宿主机 **8080**、**15673** 端口未被占用
- 首次启动需要拉取镜像（`mariadb:11`、`redis:7-alpine`、`rabbitmq:3.13-management`、`golang:1.26-alpine`、`alpine:3.20`）

## 一键启动

```bash
git clone https://github.com/s5k1dayo/go-seckill.git
cd go-seckill

# 强烈建议先设置 JWT 密钥（不设置会使用默认占位值）
export JWT_SECRET_KEY="$(openssl rand -hex 32)"

docker compose up -d --build
```

查看状态：

```bash
docker compose ps
docker compose logs -f app
```

## 服务与端口

| 服务 | 容器名 | 端口 | 说明 |
|---|---|---|---|
| app | seckill-app | **8080** | Go 后端 |
| mysql | seckill-mysql | 仅内部 | MariaDB 11，数据卷 `mysql_data` |
| redis | seckill-redis | 仅内部 | Redis 7，AOF 持久化，数据卷 `redis_data` |
| rabbitmq | seckill-rabbitmq | **15673** → 15672 | 管理台（避免与本机 15672 冲突） |

> MySQL / Redis / RabbitMQ 的端口没有映射到宿主机，只在 compose 内部网络互通，避免和本机已装的服务冲突。
> 需要调试可用 `docker compose exec`（见下文）。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `JWT_SECRET_KEY` | 占位值 | JWT 签名密钥，**生产必须修改** |
| `MYSQL_PASSWORD` | `seckill123` | 应用连接 MySQL 的密码 |
| `MYSQL_ROOT_PASSWORD` | `root123` | MySQL root 密码 |
| `RABBITMQ_USER` / `RABBITMQ_PASSWORD` | `admin` / `admin123` | RabbitMQ 账号 |
| `LIMIT_R` | `300` | 令牌桶速率（req/s） |
| `LIMIT_BURST` | `500` | 令牌桶容量 |

覆盖方式（任选其一）：

```bash
# 1) shell 环境变量
MYSQL_PASSWORD=myPass JWT_SECRET_KEY=xxx docker compose up -d --build

# 2) 项目根目录 .env（Compose 会自动读取；注意该文件不要提交）
echo 'JWT_SECRET_KEY=xxxx' >> .env

# 3) 指定 env 文件
docker compose --env-file .env.docker up -d --build
```

> compose 里应用的 `DSN` / `MQ` 已按容器服务名（`mysql`、`rabbitmq`、`redis`）写好，不需要改成本机地址。

## 初始化数据

首次启动（数据卷为空）时，`docker/mysql-init/01-schema.sql` 会自动执行，创建：

- `user` / `product` / `orders`
- `seckill_activity` / `seckill_orders`
- 演示数据：
  - 账号：`test` / `123456`
  - 商品：`id=1`，库存 100000
  - 秒杀活动：`id=1`，库存 1000，有效期 2020-01-01 ~ 2099-12-31

> 初始化脚本只在**第一次**启动、数据卷为空时执行。改了表结构后想重新初始化：`docker compose down -v`（会删除数据）。

## 验证

```bash
# 健康检查
curl http://127.0.0.1:8080/ping
curl http://127.0.0.1:8080/ready

# 登录拿 token
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"test","password":"123456"}' \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')

# 查看活动详情
curl "http://127.0.0.1:8080/api/activity?id=1" -H "Authorization: Bearer $TOKEN"

# 秒杀下单（活动 id=1）
curl -X POST http://127.0.0.1:8080/api/seckill \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"seckillid":1}'
```

RabbitMQ 管理台：<http://127.0.0.1:15673>（账号见 `RABBITMQ_USER` / `RABBITMQ_PASSWORD`，默认 `admin` / `admin123`）。

## 常用命令

```bash
docker compose ps                     # 状态
docker compose logs -f app            # 应用日志
docker compose logs -f mysql          # MySQL 日志
docker compose restart app            # 重启应用
docker compose down                   # 停止并删除容器（保留数据卷）
docker compose down -v                # 停止并删除容器 + 数据卷（重置数据库/Redis/MQ）

# 进容器调试
docker compose exec mysql mariadb -useckill -pseckill123 seckill
docker compose exec redis redis-cli
docker compose exec rabbitmq rabbitmqctl list_queues
```

## 容器架构

```
                    ┌──────────────────────────────┐
   :8080  ────────▶ │  app (Go / Gin)              │
                    │  JWT · 限流 · Redis Lua 预扣  │
                    │  MQ 发布 · 消费者 · 对账       │
                    └───┬──────────┬──────────┬────┘
                        │          │          │
              ┌─────────▼──┐  ┌────▼─────┐  ┌─▼────────────┐
              │ mysql:3306 │  │redis:6379│  │rabbitmq:5672 │
              │  MariaDB   │  │  缓存/库存│  │  order_queue │
              └────────────┘  └──────────┘  └──────────────┘
                    ▲              ▲               ▲
                    └──────────────┴───────────────┘
                          compose 内部网络
```

## 常见问题

**1. 启动时 app 报数据库连接失败？**
MySQL 首次初始化需要时间，compose 已用 `healthcheck + depends_on` 等待；如果仍失败，看 `docker compose logs mysql`，确认数据卷是否被旧数据占用（`down -v` 重置）。

**2. 8080 / 15673 端口被占用？**
改 `docker-compose.yml` 里的 `ports` 映射，例如 `"18080:8080"`。

**3. 想改 JWT 密钥 / 数据库密码？**
见「环境变量」，改完 `docker compose up -d` 重建 app 即可。

**4. pprof 访问不到？**
代码里 pprof 绑定的是容器内 `localhost:6060`，没有映射到宿主机。需要的话把 `main.go` 改成监听 `:6060` 并在 compose 里加端口映射。

**5. 为什么 MySQL/Redis/RabbitMQ 没有映射端口？**
为了避免和宿主机上已运行的本地服务冲突；需要时用 `docker compose exec` 进入，或自行添加 `ports`。

**6. 生产环境能用吗？**
这是面向演示/学习的编排：请务必修改 JWT 密钥、数据库与 MQ 密码，并按需加 TLS、资源限制、日志采集与监控。
