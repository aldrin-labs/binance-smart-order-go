package interfaces

import (
	"gitlab.com/crypto_project/core/strategy_service/src/trading/orders"
)

type ICreateRequest interface {
	CreateOrder(order orders.CreateOrderRequest) orders.OrderResponse
}
