package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"seckill/internal/model"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB

func Db_init() {
	dsn := string(os.Getenv("DSN"))
	if len(dsn) == 0 {
		log.Fatal("DSN未设置")
	}
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("连接打开失败:", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal("连接数据库失败：", err)
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(50)
	db.SetConnMaxLifetime(time.Hour)
	fmt.Println("mariadb连接成功")
}

func GetDB() *sql.DB {
	return db
}

func IsDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError //sql错误地址变量
	if errors.As(err, &mysqlErr) { //as函数需要传入地址，将sql错误地址的信息写到变量的地址中
		return mysqlErr.Number == 1062
	}
	return false
}

func GetActivity() ([]model.SeckillActivity, error) {
	rows, err := db.Query("SELECT id,product_id,stock,start_time,end_time,seckill_price FROM seckill_activity ")
	if err != nil {
		log.Println(err.Error())
		return nil, errors.New("获取活动失败")
	}
	defer rows.Close()
	var list []model.SeckillActivity
	for rows.Next() {
		var a model.SeckillActivity
		if err := rows.Scan(&a.ID, &a.ProductID, &a.Stock, &a.StartTime, &a.EndTime, &a.Price); err != nil {
			log.Println(err.Error())
			return nil, errors.New("活动数据解析失败")
		}
		list = append(list, a)
	}
	if err := rows.Err(); err != nil {
		log.Println(err.Error())
		return nil, errors.New("活动数据解析失败")
	}
	fmt.Println("活动数据加载成功")
	return list, nil
}

var ErrActivityNotFound = errors.New("活动不存在")

func GetDetail(ID int64) (*model.SeckillActivity, error) {
	var a model.SeckillActivity
	err := db.QueryRow("SELECT product_id ,stock,seckill_price,start_time,end_time FROM seckill_activity WHERE id=?", ID).Scan(&a.ProductID, &a.Stock, &a.Price, &a.StartTime, &a.EndTime)
	if err == sql.ErrNoRows {
		return nil, ErrActivityNotFound
	} else if err != nil {
		log.Println("fail to load activity detail", err)
		return nil, errors.New("活动数据解析失败")
	}
	a.ID = ID
	return &a, nil
}

func CountBought(ActivityID int64) (int64, error) {
	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM seckill_orders WHERE activity_id = ?", ActivityID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func CreateBought(ActivityID int64) ([]string, error) {
	rows, err := db.Query("SELECT user_id,activity_id FROM seckill_orders WHERE activity_id = ?", ActivityID)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	var res []string
	for rows.Next() {
		var userid int64
		var activityid int64
		rows.Scan(&userid, &activityid)
		key := fmt.Sprintf("bought:%d:%d", userid, activityid)
		res = append(res, key)
	}
	return res, nil
}

func Quit() {
	err := db.Close()
	if err != nil {
		log.Println("fail to close db :", err)
	}
}

func Check(ctx context.Context) bool {
	return db.PingContext(ctx) == nil
}
