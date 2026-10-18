package model

import (
	"time"
)

type SeckillActivity struct {
	ID        int64
	ProductID int64
	Stock     int64
	Price     float64
	StartTime time.Time
	EndTime   time.Time
}
