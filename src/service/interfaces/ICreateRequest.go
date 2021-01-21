package interfaces

import (
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"time"
)

type ICreateRequest interface {
	CreateOrder(order trading.CreateOrderRequest) trading.OrderResponse
	SaveCycleTime(t time.Duration)
}
