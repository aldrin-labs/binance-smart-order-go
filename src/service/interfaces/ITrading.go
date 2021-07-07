package interfaces

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading/orders"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

//type ITrading interface {
//	CreateOrder(exchange string, pair string, price float64, amount float64, side string) string
//}

type ITrading interface {
	CreateOrder(order orders.CreateOrderRequest) orders.OrderResponse
	CancelOrder(params orders.CancelOrderRequest) orders.OrderResponse
	PlaceHedge(parentSmarOrder *models.MongoStrategy) orders.OrderResponse

	UpdateLeverage(keyId *primitive.ObjectID, leverage float64, symbol string) orders.UpdateLeverageResponse
	Transfer(request orders.TransferRequest) orders.OrderResponse
	SetHedgeMode(keyId *primitive.ObjectID, hedgeMode bool) orders.OrderResponse
}