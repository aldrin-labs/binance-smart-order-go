package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sync"
)

// should implement IStateMgmt
type MockStateMgmt struct {
	StateMap sync.Map
	ConditionsMap sync.Map
	Trading *MockTrading
}

func NewMockedStateMgmt(trading *MockTrading) MockStateMgmt {
	stateMgmt := MockStateMgmt{
		Trading: trading,
	}

	return stateMgmt
}

func (sm *MockStateMgmt) GetOrder(orderId string) *models.MongoOrder {
	//panic("implement me")
	return &models.MongoOrder{}
}

func (sm *MockStateMgmt) SubscribeToOrder(orderId string, onOrderStatusUpdate func(order *models.MongoOrder)) error {
	//panic("implement me")
	go func() {
		for {
			orderRaw, ok := sm.Trading.OrdersMap.Load(orderId)
			if ok {
				order := orderRaw.(models.MongoOrder)
				onOrderStatusUpdate(&order)
				break
			}
		}
	}()
	return nil
}

func (sm *MockStateMgmt) GetPosition(strategyId primitive.ObjectID, symbol string) {

}

func (sm *MockStateMgmt) UpdateConditions(strategyId primitive.ObjectID, conditions *models.MongoStrategyCondition) {
	sm.ConditionsMap.Store(strategyId, &conditions)
}

// TODO: should be implemented ?
func (sm *MockStateMgmt) UpdateState(strategyId primitive.ObjectID, state *models.MongoStrategyState) {
	sm.StateMap.Store(strategyId, &state)
}

func (sm *MockStateMgmt) DisableStrategy(strategyId primitive.ObjectID) {

}
