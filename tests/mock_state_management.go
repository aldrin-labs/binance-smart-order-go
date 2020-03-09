package tests

import (
	"sync"
	"time"

	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// should implement IStateMgmt
type MockStateMgmt struct {
	StateMap      sync.Map
	ConditionsMap sync.Map
	Trading       *MockTrading
}

func NewMockedStateMgmt(trading *MockTrading) MockStateMgmt {
	stateMgmt := MockStateMgmt{
		Trading: trading,
	}

	return stateMgmt
}

func (sm *MockStateMgmt) GetOrder(orderId string) *models.MongoOrder {
	return &models.MongoOrder{}
}

func (sm *MockStateMgmt) GetMarketPrecision(pair string, marketType int64) (int64, int64) {
	return 2, 3
}

func (sm *MockStateMgmt) SubscribeToOrder(orderId string, onOrderStatusUpdate func(order *models.MongoOrder)) error {
	go func() {
		for {
			orderRaw, ok := sm.Trading.OrdersMap.Load(orderId)
			if ok {
				order := orderRaw.(models.MongoOrder)
				delay := sm.Trading.BuyDelay
				if order.Side == "sell" {
					delay = sm.Trading.SellDelay
				}
				time.Sleep(time.Duration(delay) * time.Millisecond)
				order.Status = "filled"
				onOrderStatusUpdate(&order)
				break
			}
		}
	}()
	return nil
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

func (sm *MockStateMgmt) UpdateExecutedAmount(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	sm.StateMap.Store(strategyId, &state)
}

func (sm *MockStateMgmt) UpdateOrders(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	sm.StateMap.Store(strategyId, &state)
}

func (sm *MockStateMgmt) DisableStrategy(strategyId *primitive.ObjectID) {

}

func (sm *MockStateMgmt) SubscribeToHedge(strategyId *primitive.ObjectID, onHedgeExitUpdate func(strategy *models.MongoStrategy)) error {
	return nil
}

func (sm *MockStateMgmt) UpdateHedgeExitPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {

}

func (sm *MockStateMgmt) AnyActiveStrats(strategy *models.MongoStrategy) bool {
	panic("implement me")
	return false
}
