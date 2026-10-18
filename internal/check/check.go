package check

import (
	"context"
	"log"
	"seckill/internal/db"
	"seckill/internal/model"
	"seckill/internal/mq"
	"seckill/internal/redis"
	"time"
)

func StartCheck(ctx context.Context, list []model.SeckillActivity) {
	for true {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			for _, a := range list {
				ok, err := CheckJob(&a)
				if ok && err == nil {
					log.Printf("活动%d一致性检验通过", a.ID)
				} else if !ok && err == nil {
					if empty := mq.WaitQueueEmpty(); empty {
						RetryCheck(&a)
						continue
					}
				} else {
					log.Println("fail to check same:", err)
				}
			}
		}

	}
}

func RetryCheck(a *model.SeckillActivity) {
	ok, err := CheckJob(a)
	if err != nil {
		log.Println("fail to check same:", err)
		return
	}
	if ok {
		log.Printf("活动%d一致性检验通过", a.ID)
		return
	} else {
		log.Printf("活动%d一致性检验失败，需处理", a.ID)
		//启用修复程序

	}
}

func CheckJob(a *model.SeckillActivity) (bool, error) {
	id := a.ID
	RedisStock, err := redis.Redis_Getstock(id)
	if err != nil {
		return false, err
	}
	DbBought, derr := db.CountBought(id)
	if derr != nil {
		return false, derr
	}
	if RedisStock == a.Stock-DbBought {
		return true, nil
	}
	return false, nil
}
