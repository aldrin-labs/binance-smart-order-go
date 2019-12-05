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
}

type IStrategyRuntime interface {
	Stop()
	Start()
}

func GetStrategy(cur *mongo.Cursor, df IDataFeed, tr trading.ITrading) (*Strategy, error) {
	var result models.MongoStrategy
	err := cur.Decode(&result)
	return &Strategy{Model: &result, Datafeed: df, Trading: tr }, err
}

func (strategy *Strategy) Start() {
	switch strategy.Model.StrategyType {
	case 1:
		println("runSmartOrder")
		strategy.StrategyRuntime = RunSmartOrder(strategy, strategy.Datafeed, strategy.Trading, nil)
	default:
		fmt.Println("this type of strategy is not supported yet:", strategy.Model.ID.String(), strategy.Model.StrategyType)
	}
}


func (strategy *Strategy) HotReload(mongoStrategy models.MongoStrategy) {
	strategy.Model = &mongoStrategy
	if mongoStrategy.Enabled == false {
		if strategy.StrategyRuntime != nil {
			strategy.StrategyRuntime.Stop()
		}
	}
}