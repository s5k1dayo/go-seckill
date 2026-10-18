package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"seckill/internal/db"
	"seckill/internal/model"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var rdb = goredis.NewClient(&goredis.Options{
	Addr:     "127.0.0.1:6379",
	Password: "",
	DB:       0,
})

var ctx = context.Background()

func Redis_init(list []model.SeckillActivity) {
	ctxto, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctxto).Err(); err != nil {
		log.Fatal("redis connect fail:", err)
	}
	for _, a := range list {
		count, err := db.CountBought(a.ID)
		if err != nil {
			log.Fatal("get bought data fail:", err)
		}
		key := fmt.Sprintf("stock:%d", a.ID)

		if a.EndTime.After(time.Now()) {
			//库存进redis，过期时间设为无限
			if err := rdb.Set(ctx, key, a.Stock-count, 0).Err(); err != nil {
				log.Fatal("write redis fail:", err)
			}
			//将其余信息作为hash加载进redis
			if err := Redis_loaddetail(&a); err != nil {
				log.Fatal(err)
			}
			list, err := db.CreateBought(a.ID)
			if err != nil {
				log.Fatal("get bought fail", err)
			}
			for _, key := range list {
				if berr := rdb.Set(ctx, key, 1, 0).Err(); berr != nil {
					log.Fatal("create bought redis fail:", berr)
				}
			}
			fmt.Printf("redis预热成功")
			fmt.Printf("已加载活动%d，库存%d \n", a.ID, a.Stock-count)
		}

	}

}

func Redis_loaddetail(a *model.SeckillActivity) error {
	detailkey := fmt.Sprintf("seckill:%d:detail", a.ID)
	if err := rdb.HSet(
		ctx,
		detailkey,
		"product_id", a.ProductID,
		"price", a.Price,
		"start_time", a.StartTime.Unix(),
		"end_time", a.EndTime.Unix()).Err(); err != nil {
		log.Println("write redis fail:", err)
		return errors.New("加载活动详情错误")
	}
	rdb.Expire(ctx, detailkey, time.Duration(20+rand.Intn(40))*time.Second)
	return nil
}

var seckillScript = goredis.NewScript(`
	if redis.call('EXISTS',KEYS[1])==1 then
		return {-1,0}
	end
	local stock = tonumber(redis.call('GET',KEYS[2]) or '0')
	if stock <= 0 then
		return {-2,0}
	end
	local productid = tonumber(redis.call('HGET',KEYS[3],'product_id') or '0')
	if productid==0 then
		return {-3,0}
	end
	redis.call('DECR',KEYS[2])
	redis.call('SET',KEYS[1],1)
	redis.call('EXPIRE',KEYS[1],ARGV[1])
	return {1,productid}
`)

func Redis_seckill(UserID int64, ID int64) (int64, error) {
	stockkey := fmt.Sprintf("stock:%d", ID)
	exists, err := rdb.Exists(ctx, stockkey).Result()
	if err != nil {
		log.Println(err)
		return 0, errors.New("redis获取数据失败")
	}
	if exists == 0 {
		return 0, errors.New("活动不存在")
	}
	boughtkey := fmt.Sprintf("bought:%d:%d", UserID, ID)
	productkey := fmt.Sprintf("seckill:%d:detail", ID)
	result, scerr := seckillScript.Run(ctx, rdb, []string{boughtkey, stockkey, productkey}, 86400).Slice()
	if scerr != nil {
		log.Println(scerr)
		return 0, errors.New("请求处理失败")
	}
	//log.Println(result[0], "\n", result[1])
	switch result[0].(int64) {
	case 1:
		return result[1].(int64), nil
	case -1:
		return 0, errors.New("已购买")
	case -2:
		return 0, errors.New("库存不足")
	case -3:
		if _, err = GetOrLoadDetail(ID); err != nil {
			log.Println("fail to reload redis:", err)
			return 0, errors.New("活动不存在")
		}
		return 0, errors.New("数据重载成功，请重试")
	default:
		return 0, errors.New("未知错误")
	}

}

var cancelscript = goredis.NewScript(`
	if redis.call('EXISTS',KEYS[2])==1 then
		redis.call('INCR',KEYS[1])
		redis.call('DEL',KEYS[2])
		return nil
	end
	`)

func Redis_cancel(UserID int64, ID int64) error {
	stockkey := fmt.Sprintf("stock:%d", ID)
	boughtkey := fmt.Sprintf("bought:%d:%d", UserID, ID)
	_, err := cancelscript.Run(ctx, rdb, []string{stockkey, boughtkey}).Result()
	if err != nil && err != goredis.Nil {
		log.Println(err)
		return errors.New("redis撤销失败，请联系管理员")
	}
	return nil
}

func GetTime(ID int64) ([]time.Time, error) {
	a, err := GetOrLoadDetail(ID)
	if err != nil {
		return nil, err
	}
	var res []time.Time
	res = append(res, a.StartTime)
	res = append(res, a.EndTime)
	return res, nil
}

func readDetailCache(id int64) (*model.SeckillActivity, bool) {
	stockey := fmt.Sprintf("stock:%d", id)
	key := fmt.Sprintf("seckill:%d:detail", id)
	stock, serr := rdb.Get(ctx, stockey).Int()
	if serr != nil {
		log.Println("fail to get stock from redis:", serr)
		return nil, false
	}
	res, err := rdb.HMGet(ctx, key, "product_id", "price", "start_time", "end_time").Result()
	if err != nil || res[0] == nil {
		log.Println("fail to get detail from redis:", err)
		return nil, false
	}
	productID, _ := strconv.ParseInt(res[0].(string), 10, 64)
	price, _ := strconv.ParseFloat(res[1].(string), 64)
	startUnix, _ := strconv.ParseInt(res[2].(string), 10, 64)
	endUnix, _ := strconv.ParseInt(res[3].(string), 10, 64)
	a := model.SeckillActivity{
		ID:        id,
		ProductID: productID,
		Stock:     int64(stock),
		Price:     price,
		StartTime: time.Unix(startUnix, 0).UTC(),
		EndTime:   time.Unix(endUnix, 0).UTC(),
	}
	return &a, true
}

var sfGroup singleflight.Group

func GetOrLoadDetail(id int64) (*model.SeckillActivity, error) {
	a, ok := readDetailCache(id)
	if ok {
		return a, nil
	}
	emptykey := fmt.Sprintf("notexist:%d", id)
	if n, _ := rdb.Get(ctx, emptykey).Int(); n == 1 {
		return nil, db.ErrActivityNotFound
	}
	sfkey := fmt.Sprintf("detail:%d", id)
	v, err, _ := sfGroup.Do(sfkey, func() (any, error) {
		a, ok := readDetailCache(id)
		if ok {
			return a, nil
		}
		aa, err := db.GetDetail(id)
		if err == db.ErrActivityNotFound {
			rerr := rdb.Set(ctx, emptykey, 1, 60*time.Second).Err()
			if rerr != nil {
				log.Println("fail to set notexist redis:", rerr)
			}
			return nil, err
		} else if err != nil {
			return nil, err
		}
		aa.StartTime = aa.StartTime.UTC()
		aa.EndTime = aa.EndTime.UTC()
		_ = Redis_loaddetail(aa)
		key := fmt.Sprintf("stock:%d", id)
		stock, err := rdb.Get(ctx, key).Int()
		if err != nil {
			log.Println("fail to get stock from redis:", err)
			return nil, errors.New("库存加载失败")
		}
		aa.Stock = int64(stock)
		log.Println("回源id=", id)
		return aa, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*model.SeckillActivity), nil
}

func Redis_Getstock(id int64) (int64, error) {
	key := fmt.Sprintf("stock:%d", id)
	stock, err := rdb.Get(ctx, key).Int()
	if err != nil {
		log.Println("fail to get stock from redis", err)
		return 0, errors.New("获取redis库存失败")
	}
	return int64(stock), nil

}

func Quit() {
	err := rdb.Close()
	if err != nil {
		log.Println("fail to close redis :", err)
	}
}

func Check(ctx context.Context) bool {
	return rdb.Ping(ctx).Err() == nil
}
