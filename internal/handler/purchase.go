package handler

import (
	//"seckill/internal/db"
	//"seckill/internal/service"
	"net/http"
	"seckill/internal/service"

	"github.com/gin-gonic/gin"
)

func PurchaseHandler(c *gin.Context) {
	var req struct {
		ProductID int64 `json:"productid" binding:"required"`
	}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "输入格式不正确"})
		return
	}
	UserID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取用户信息失败"})
		return
	}
	stock, purerr := service.Purchase(UserID.(int64), req.ProductID)
	if purerr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": purerr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "购买成功", "stock": stock})

}
