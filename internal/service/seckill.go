package service

import (
	"seckill/internal/mq"
	"seckill/internal/redis"
)

func Seckill(UserID int64, ID int64) error {
	ProductID, err := redis.Redis_seckill(UserID, ID) //调用redis扣库存
	if err != nil {
		return err
	}
	//发送入库消息队列
	err = mq.Mq_Publish(UserID, ID, ProductID)
	if err != nil {
		if cerr := redis.Redis_cancel(UserID, ID); cerr != nil {
			return cerr
		}
		return err
	}
	return nil
}
