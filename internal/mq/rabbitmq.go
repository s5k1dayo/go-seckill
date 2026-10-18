package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"seckill/internal/db"
	"seckill/internal/redis"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var Conn *amqp.Connection
var Ch *amqp.Channel
var Q amqp.Queue

type ordermsg struct {
	UserID    int64 `json:"user_id"`
	ID        int64 `json:"id"`
	ProductID int64 `json:"product_id"`
}

func Mq_init() {
	var err error
	Conn, err = amqp.Dial(os.Getenv("MQ"))
	if err != nil {
		log.Fatal(err)
	}
	Ch, err = Conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	Q, err = Ch.QueueDeclare(
		"order_queue",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	if err = Ch.Confirm(false); err != nil {
		log.Fatal(err)
	}
	fmt.Println("mq连接成功")
}

func Mq_startworkers(n int) {
	for i := 0; i < n; i++ {
		go worker()
		wg.Add(1)
	}
}

var (
	mu      sync.Mutex
	workers []*amqp.Channel
	wg      sync.WaitGroup
)

func worker() {
	defer wg.Done()
	var err error
	ch, err := Conn.Channel()
	if err != nil {
		log.Println("fail to connect channel:", err)
		return
	}
	defer ch.Close()
	mu.Lock()
	workers = append(workers, ch)
	mu.Unlock()
	err = ch.Qos(
		10, 0, false,
	)
	if err != nil {
		log.Println("fail to set qos:", err)
		return
	}
	Mq_Consume(ch)
}

func Mq_Publish(user_id int64, id int64, product_id int64) error {

	msg := ordermsg{
		UserID:    user_id,
		ID:        id,
		ProductID: product_id,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		log.Println(err)
		return errors.New("消息队列参数错误")
	}
	dc, cerr := Ch.PublishWithDeferredConfirm(
		"",
		Q.Name,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if cerr != nil {
		log.Println(cerr)
		return errors.New("消息队列发送错误")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ok, derr := dc.WaitContext(ctx)
	if derr != nil {
		log.Println(derr)
		return errors.New("消息超时")
	}
	if !ok {
		return errors.New("broker拒绝接收消息")
	}
	return nil
}

func Mq_Consume(ch *amqp.Channel) {
	msgs, err := ch.Consume(
		Q.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	for msg := range msgs {
		var data ordermsg
		if err := json.Unmarshal(msg.Body, &data); err != nil {
			log.Println("mq数据解析失败：", err)
			msg.Nack(false, false)
			continue
		}
		DB := db.GetDB()
		if _, err := DB.Exec("INSERT INTO seckill_orders(user_id,product_id,activity_id) VALUES (?,?,?)", data.UserID, data.ProductID, data.ID); err != nil {
			if db.IsDuplicateKey(err) {
				msg.Ack(false)
				log.Println("落库失败：", err.Error())
				continue
			}
			log.Println("落库失败：", err.Error())
			cerr := redis.Redis_cancel(data.UserID, data.ID)
			if cerr != nil {
				log.Println("redis撤销失败", cerr)
			}
			msg.Nack(false, false)
			continue
		}
		msg.Ack(false)
	}
}

func WaitQueueEmpty() bool {
	deadline := time.Now().Add(time.Second)
	for time.Until(deadline) > 0 {
		q, err := Ch.QueueDeclarePassive(Q.Name, false, false, false, false, nil)
		if err != nil {
			log.Println("fail to get queue from mq", err)
			continue
		}
		num := q.Messages
		if num == 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func Quit() {
	mu.Lock()
	wks := append([]*amqp.Channel(nil), workers...)
	mu.Unlock()
	for _, wk := range wks {
		wk.Close()
	}
	wg.Wait()
	if Ch != nil {
		Ch.Close()
	}
	if Conn != nil {
		Conn.Close()
	}
}

func Check(ctx context.Context) bool {
	ch := make(chan bool, 1)
	go func() {
		ch <- Conn.IsClosed() == false
	}()
	select {
	case res := <-ch:
		return res
	case <-ctx.Done():
		return false

	}
}
