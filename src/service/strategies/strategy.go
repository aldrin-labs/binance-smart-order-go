package strategies

import (
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
)


func GetStrategy(cur *mongo.Cursor, df interfaces.IDataFeed, tr trading.ITrading, sm interfaces.IStateMgmt) (*Strategy, error) {
	var result models.MongoStrategy
	err := cur.Decode(&result)
	return &Strategy{Model: &result, Datafeed: df, Trading: tr, StateMgmt: sm }, err
}

// Strategy object
type Strategy struct {
	Model           *models.MongoStrategy
	StrategyRuntime interfaces.IStrategyRuntime
	Datafeed        interfaces.IDataFeed
	Trading         trading.ITrading
	StateMgmt       interfaces.IStateMgmt
}
func (strategy *Strategy) GetModel() *models.MongoStrategy {
	return strategy.Model
}
func (strategy *Strategy) GetRuntime() interfaces.IStrategyRuntime {
	return strategy.StrategyRuntime
}
func (strategy *Strategy) GetDatafeed() interfaces.IDataFeed {
	return strategy.Datafeed
}
func (strategy *Strategy) GetTrading() trading.ITrading {
	return strategy.Trading
}
func (strategy *Strategy) GetStateMgmt() interfaces.IStateMgmt {
	return strategy.StateMgmt
}

func (strategy *Strategy) Start() {
	switch strategy.Model.Type {
	case 1:
		log.Print("runSmartOrder")
		strategy.StrategyRuntime = RunSmartOrder(strategy, strategy.Datafeed, strategy.Trading, nil, )
	default:
		fmt.Println("this type of strategy is not supported yet: ", strategy.Model.ID.String(), strategy.Model.Type)
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