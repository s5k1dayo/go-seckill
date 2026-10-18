package router

import (
	"net/http"
	"seckill/internal/handler"
	"seckill/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Router() *gin.Engine {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "hello"})
	})
	r.GET("/ready", handler.ReadyHandler)
	r.POST("/login", handler.LoginHandler)
	auth := r.Group("/api")
	auth.Use(middleware.AuthMiddleware())
	limiter := middleware.NewRateLimiter()
	limiter.AddNewLimiter("seckill")
	{
		auth.POST("/purchase", handler.PurchaseHandler)
		auth.POST("/seckill", limiter.LmtMiddleWare("seckill"), handler.SeckillHandler)
		auth.GET("/activity", handler.ActivityHandler)
	}
	return r
}
