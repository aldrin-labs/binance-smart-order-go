package testing

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// should implement IStateMgmt
type MockStateMgmt struct {
}

func (sm *MockStateMgmt) GetPosition(strategyId primitive.ObjectID, symbol string) {

}

func (sm *MockStateMgmt) UpdateConditions(strategyId primitive.ObjectID, state *models.MongoStrategyCondition) {

}

// TODO: should be implemented ?
func (sm *MockStateMgmt) UpdateState(strategyId primitive.ObjectID, state *models.MongoStrategyState) {

}
