package handler

import (
	"net/http"
	"seckill/internal/redis"
	"seckill/internal/service"
	"time"

	"github.com/gin-gonic/gin"
)

func SeckillHandler(c *gin.Context) {
	var req struct {
		ID int64 `json:"seckillid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "输入格式不正确"})
		return
	}
	UserID, exist := c.Get("userid")
	if !exist {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取用户信息失败"})
		return
	}
	times, terr := redis.GetTime(req.ID)
	if terr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": terr.Error()})
		return
	}
	start_time := times[0]
	end_time := times[1]
	if end_time.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "活动已结束"})
		return
	} else if start_time.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "活动尚未开始"})
		return
	} else {
		if err := service.Seckill(UserID.(int64), req.ID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "购买成功！"})
	}

}
