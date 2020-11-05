package interfaces

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IStateMgmt interface {
	UpdateConditions(strategyId *primitive.ObjectID, state *models.MongoStrategyCondition)
	UpdateEntryPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState)
	UpdateHedgeExitPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState)
	UpdateState(strategyId *primitive.ObjectID, state *models.MongoStrategyState)
	UpdateOrders(strategyId *primitive.ObjectID, state *models.MongoStrategyState)
	UpdateExecutedAmount(strategyId *primitive.ObjectID, state *models.MongoStrategyState)
	GetPosition(strategyId *primitive.ObjectID, symbol string)
	GetOrder(orderId string) *models.MongoOrder
	GetOrderById(orderId *primitive.ObjectID) *models.MongoOrder
	SubscribeToOrder(orderId string, onOrderStatusUpdate func(order *models.MongoOrder)) error
	SubscribeToHedge(strategyId *primitive.ObjectID, onHedgeExitUpdate func(strategy *models.MongoStrategy)) error
	DisableStrategy(strategyId *primitive.ObjectID)
	EnableStrategy(strategyId *primitive.ObjectID)
	GetMarketPrecision(pair string, marketType int64) (int64, int64)
	AnyActiveStrats(strategy *models.MongoStrategy) bool
	InitOrdersWatch()
	SavePNL(templateStrategyId *primitive.ObjectID, profitAmount float64)
	SaveStrategyConditions(strategy *models.MongoStrategy)
	SaveStrategy(strategy *models.MongoStrategy) *models.MongoStrategy
	CreateStrategy(strategy *models.MongoStrategy) *models.MongoStrategy
	EnableHedgeLossStrategy(strategyId *primitive.ObjectID)
	SaveOrder(order models.MongoOrder, keyId *primitive.ObjectID, marketType int64)
	UpdateStrategyState(strategyId *primitive.ObjectID, state *models.MongoStrategyState)
}
