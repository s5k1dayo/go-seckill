package handler

import (
	"net/http"
	"seckill/internal/model"
	"seckill/internal/redis"

	"github.com/gin-gonic/gin"
)

func ActivityHandler(c *gin.Context) {
	var req struct {
		SeckillID int64 `form:"id" binding:"required"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "输入参数错误"})
		return
	}
	if req.SeckillID < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "输入参数错误"})
		return
	}
	var activity *model.SeckillActivity
	activity, err := redis.GetOrLoadDetail(req.SeckillID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":         activity.ID,
		"productid":  activity.ProductID,
		"stock":      activity.Stock,
		"price":      activity.Price,
		"start_time": activity.StartTime,
		"end_time":   activity.EndTime,
	}})
}
