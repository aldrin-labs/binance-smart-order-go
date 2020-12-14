package strategies

import (
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	statsd_client "gitlab.com/crypto_project/core/strategy_service/src/statsd"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetStrategy(cur *mongo.Cursor, df interfaces.IDataFeed, tr trading.ITrading, sm interfaces.IStateMgmt, createOrder interfaces.ICreateRequest) (*Strategy, error) {
	var result models.MongoStrategy
	err := cur.Decode(&result)
	return &Strategy{Model: &result, Datafeed: df, Trading: tr, StateMgmt: sm, Singleton: createOrder}, err
}

type Strategy struct {
	Model           *models.MongoStrategy
	StrategyRuntime interfaces.IStrategyRuntime
	Datafeed        interfaces.IDataFeed
	Trading         trading.ITrading
	StateMgmt       interfaces.IStateMgmt
	Statsd          statsd_client.StatsdClient
	Singleton       interfaces.ICreateRequest
}

func (strategy *Strategy) GetModel() *models.MongoStrategy {
	return strategy.Model
}
func (strategy *Strategy) GetSingleton() interfaces.ICreateRequest {
	return strategy.Singleton
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

func (strategy *Strategy) GetStatsd() statsd_client.StatsdClient {
	return strategy.Statsd
}

// Start starts a runtime for the strategy.
func (strategy *Strategy) Start() {
	switch strategy.Model.Type {
	case 1:
		println("runSmartOrder")
		strategy.StrategyRuntime = RunSmartOrder(strategy, strategy.Datafeed, strategy.Trading, strategy.Statsd, strategy.Model.AccountId)
	case 2:
		println("makerOnly")
		strategy.StrategyRuntime = RunMakerOnlyOrder(strategy, strategy.Datafeed, strategy.Trading, strategy.Model.AccountId)
	default:
		fmt.Println("this type of strategy is not supported yet: ", strategy.Model.ID.String(), strategy.Model.Type)
	}
}

// HotReload updates strategy in runtime to keep consistency with persistent state.
func (strategy *Strategy) HotReload(mongoStrategy models.MongoStrategy) {
	strategy.Model.Enabled = mongoStrategy.Enabled
	strategy.Model.Conditions = mongoStrategy.Conditions
	if mongoStrategy.Enabled == false {
		if strategy.StrategyRuntime != nil {
			strategy.StrategyRuntime.Stop() // stop runtime if disabled by DB, externally
		}
	}
}
