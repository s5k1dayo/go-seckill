package main

import (
	//"net/http"
	//"github.com/gin-gonic/gin"
	//"fmt"
	//"seckill/internal/handler"
	"context"
	"errors"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"seckill/internal/auth"
	"seckill/internal/check"
	"seckill/internal/db"
	"seckill/internal/handler"
	"seckill/internal/mq"
	"seckill/internal/redis"
	"seckill/internal/router"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	go http.ListenAndServe("localhost:6060", nil)
	if err := godotenv.Load(); err != nil {
		log.Fatal(err.Error())
	}
	auth.Jwt_init()
	db.Db_init()
	mq.Mq_init()
	mq.Mq_startworkers(10)
	list, err := db.GetActivity()
	if err != nil {
		log.Fatal(err.Error())
	}
	redis.Redis_init(list)
	checkctx, checkcancel := context.WithCancel(context.Background())
	checkdone := make(chan struct{})
	go func() {
		check.StartCheck(checkctx, list)
		close(checkdone)
	}()
	engine := router.Router()
	srv := http.Server{
		Addr:    ":8080",
		Handler: engine,
	}
	go func() {
		log.Printf("HTTP server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error:%v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	handler.SetShuttingdown()
	signal.Stop(stop)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
		if err := srv.Close(); err != nil {
			log.Printf("Force close failed: %v", err)
		}
	}
	log.Println("HTTP server stopped")

	checkcancel()
	<-checkdone
	mq.Quit()
	db.Quit()
	redis.Quit()
}
