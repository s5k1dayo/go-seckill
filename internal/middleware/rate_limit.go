package middleware

import (
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type Rate_Limiter struct {
	limiters map[string]*rate.Limiter
}

func NewRateLimiter() *Rate_Limiter {
	return &Rate_Limiter{limiters: make(map[string]*rate.Limiter)}
}

func (lmt Rate_Limiter) AddNewLimiter(key string) {
	r, _ := strconv.ParseFloat(os.Getenv("LIMIT_R"), 64)
	burst, _ := strconv.Atoi(os.Getenv("LIMIT_BURST"))
	lmt.limiters[key] = rate.NewLimiter(rate.Limit(r), burst)
}

func (lmt Rate_Limiter) LmtMiddleWare(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		a := lmt.limiters[key]
		if a.Allow() {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(429, gin.H{"code": 429, "message": "too many requests"})
	}
}
