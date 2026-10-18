package handler

import (
	"context"
	"net/http"
	"seckill/internal/db"
	"seckill/internal/mq"
	"seckill/internal/redis"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

var shuttingDown atomic.Bool

func SetShuttingdown() {
	shuttingDown.Store(true)
}

func ReadyHandler(c *gin.Context) {
	if shuttingDown.Load() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavaliable"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	//sql，func Check(ctx context.Context) bool {
	dbok := db.Check(ctx)
	redisok := redis.Check(ctx)
	mqok := mq.Check(ctx)
	if dbok && redisok && mqok {
		c.JSON(http.StatusOK, gin.H{"message": "service aviliable"})
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavaliable"})

}
