package strategies

import (
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/mongo"
)

// Strategy object
type Strategy struct {
	Model           *models.MongoStrategy
	StrategyRuntime IStrategyRuntime
	Datafeed        IDataFeed
	Trading         trading.ITrading
	StateMgmt 		IStateMgmt
}

type IStrategyRuntime interface {
	Stop()
	Start()
}

func GetStrategy(cur *mongo.Cursor, df IDataFeed, tr trading.ITrading, sm IStateMgmt) (*Strategy, error) {
	var result models.MongoStrategy
	err := cur.Decode(&result)
	return &Strategy{Model: &result, Datafeed: df, Trading: tr, StateMgmt: sm }, err
}

func (strategy *Strategy) Start() {
	switch strategy.Model.Type {
	case 1:
		println("runSmartOrder")
		strategy.StrategyRuntime = RunSmartOrder(strategy, strategy.Datafeed, strategy.Trading, nil, )
	default:
		fmt.Println("this type of strategy is not supported yet:", strategy.Model.ID.String(), strategy.Model.Type)
	}
}


func (strategy *Strategy) HotReload(mongoStrategy models.MongoStrategy) {
	strategy.Model.Enabled = mongoStrategy.Enabled
	strategy.Model.Conditions = mongoStrategy.Conditions
	if mongoStrategy.Enabled == false {
		if strategy.StrategyRuntime != nil {
			strategy.StrategyRuntime.Stop()
		}
	}
}