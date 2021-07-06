package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"sync"
	"time"
)

// should implement IStateMgmt
type MockStateMgmt struct {
	StateMap      sync.Map
	ConditionsMap sync.Map
	Trading       *MockTrading
	DataFeed      IDataFeed
}

func (sm *MockStateMgmt) UpdateStrategyState(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
}

func (sm *MockStateMgmt) UpdateStateAndConditions(strategyId *primitive.ObjectID, model *models.MongoStrategy) {
	panic("implement me")
}

func (sm *MockStateMgmt) SaveStrategy(strategy *models.MongoStrategy) *models.MongoStrategy {
	panic("implement me")
}

func (sm *MockStateMgmt) SaveOrder(order models.MongoOrder, keyId *primitive.ObjectID, marketType int64) {
	panic("implement me")
}

func (sm *MockStateMgmt) EnableStrategy(strategyId *primitive.ObjectID) {
	return
}

func (sm *MockStateMgmt) CreateStrategy(strategy *models.MongoStrategy) *models.MongoStrategy {
	panic("implement me")
}

func (sm *MockStateMgmt) GetOrderById(orderId *primitive.ObjectID) *models.MongoOrder {
	panic("implement me")
}

func NewMockedStateMgmt(trading *MockTrading, dataFeed IDataFeed) MockStateMgmt {
	stateMgmt := MockStateMgmt{
		Trading:  trading,
		DataFeed: dataFeed,
	}

	return stateMgmt
}

func (sm *MockStateMgmt) GetOrder(orderId string) *models.MongoOrder {
	//panic("implement me")
	return &models.MongoOrder{}
}

func (sm *MockStateMgmt) GetMarketPrecision(pair string, marketType int64) (int64, int64) {
	return 2, 3
}

func (sm *MockStateMgmt) SubscribeToOrder(orderId string, onOrderStatusUpdate func(order *models.MongoOrder)) error {
	//panic("implement me")
	go func() {
		for {
			orderRaw, ok := sm.Trading.OrdersMap.Load(orderId)
			if ok {
				order := orderRaw.(models.MongoOrder)
				delay := sm.Trading.BuyDelay
				if order.Side == "sell" {
					delay = sm.Trading.SellDelay
				}
				if order.Status == "canceled" {
					break
				}
				time.Sleep(time.Duration(delay) * time.Millisecond)
				isStopOrder := strings.Contains(order.Type, "stop")
				isTapOrder := strings.Contains(order.Type, "take")
				currentPrice := sm.DataFeed.GetPriceForPairAtExchange("BTC_USDT", "binance", 1).Close
				if order.Type == "market" ||
					order.Side == "sell" && order.Average <= currentPrice && (order.Type == "limit" || isTapOrder) ||
					order.Side == "buy" && order.Average >= currentPrice && (order.Type == "limit" || isTapOrder) ||
					order.Side == "buy" && order.Average <= currentPrice && isStopOrder ||
					order.Side == "sell" && order.Average >= currentPrice && isStopOrder {
					order.Status = "filled"
					sm.Trading.OrdersMap.Store(orderId, order)
					onOrderStatusUpdate(&order)
					break
				}
			}
		}
	}()
	return nil
}

func (sm *MockStateMgmt) SubscribeToHedge(strategyId *primitive.ObjectID, onHedgeExitUpdate func(strategy *models.MongoStrategy)) error {
	panic("implement me")
}

func (sm *MockStateMgmt) GetPosition(strategyId *primitive.ObjectID, symbol string) {

}

func (sm *MockStateMgmt) UpdateConditions(strategyId *primitive.ObjectID, conditions *models.MongoStrategyCondition) {
	sm.ConditionsMap.Store(strategyId, &conditions)
}

// TODO: should be implemented ?
func (sm *MockStateMgmt) UpdateState(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	sm.StateMap.Store(strategyId, &state)
}

func (sm *MockStateMgmt) UpdateEntryPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	sm.StateMap.Store(strategyId, &state)
}

func (sm *MockStateMgmt) UpdateHedgeExitPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	sm.StateMap.Store(strategyId, &state)
}

func (sm *MockStateMgmt) UpdateExecutedAmount(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	sm.StateMap.Store(strategyId, &state)
}

func (sm *MockStateMgmt) UpdateOrders(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	sm.StateMap.Store(strategyId, &state)
}

func (sm *MockStateMgmt) DisableStrategy(strategyId *primitive.ObjectID) {
	return
}

func (sm *MockStateMgmt) AnyActiveStrats(strategy *models.MongoStrategy) bool {
	panic("implement me")
}

func (sm *MockStateMgmt) InitOrdersWatch() {
	panic("implement me")
}

func (sm *MockStateMgmt) SavePNL(templateStrategyId *primitive.ObjectID, profitAmount float64) {
	panic("implement me")
}

func (sm *MockStateMgmt) SaveStrategyConditions(strategy *models.MongoStrategy) {
	return
}

func (sm *MockStateMgmt) EnableHedgeLossStrategy(strategyId *primitive.ObjectID) {
	return
}
