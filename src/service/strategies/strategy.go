package strategies

import (
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	statsd_client "gitlab.com/crypto_project/core/strategy_service/src/statsd"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// GetStrategy instantiates strategy with resources created and given.
func GetStrategy(cur *mongo.Cursor, df interfaces.IDataFeed, tr trading.ITrading, sm interfaces.IStateMgmt, createOrder interfaces.ICreateRequest) (*Strategy, error) {
	var model models.MongoStrategy
	err := cur.Decode(&model)
	if err != nil {
		return &Strategy{}, err
	}
	loggerConfig := zap.NewProductionConfig()
	logPath := fmt.Sprintf("/var/lib/strategy_service/strategy-%q.log", model.ID.Hex())
	loggerConfig.OutputPaths = []string{logPath}
	logger, err := loggerConfig.Build()
	if err != nil {
		return &Strategy{}, err // don't create a silent strategy, TODO(khassanov): don't fail here, add an alert
	}
	return &Strategy{Model: &model, Datafeed: df, Trading: tr, StateMgmt: sm, Singleton: createOrder, Log: logger}, nil
}

// A Strategy describes user defined rules to order trades and what are interfaces to execute these rules.
type Strategy struct {
	Model           *models.MongoStrategy
	StrategyRuntime interfaces.IStrategyRuntime
	Datafeed        interfaces.IDataFeed
	Trading         trading.ITrading
	StateMgmt       interfaces.IStateMgmt
	Statsd          statsd_client.StatsdClient
	Singleton       interfaces.ICreateRequest
	Log				*zap.Logger
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

// ID returns unique identifier the strategy holds.
func (strategy *Strategy) ID() string {
	return fmt.Sprintf("%q", strategy.Model.ID.Hex())
}

// Start starts a runtime for the strategy.
func (strategy *Strategy) Start() {
	switch strategy.Model.Type {
	case 1:
		strategy.Log.Info("running smart order",
			zap.String("id", strategy.ID()),
			zap.Int64("type", strategy.Model.Type),
		)
		strategy.StrategyRuntime = RunSmartOrder(strategy, strategy.Datafeed, strategy.Trading, strategy.Statsd, strategy.Model.AccountId)
		strategy.Statsd.Inc("strategy.sm_runtime_start")
	case 2:
		strategy.Log.Info("running maker only order",
			zap.String("id", strategy.ID()),
			zap.Int64("type", strategy.Model.Type),
		)
		strategy.StrategyRuntime = RunMakerOnlyOrder(strategy, strategy.Datafeed, strategy.Trading, strategy.Model.AccountId)
		strategy.Statsd.Inc("strategy.mo_runtime_start")
	default:
		strategy.Log.Warn("strategy type not supported",
			zap.String("id", strategy.ID()),
			zap.Int64("type", strategy.Model.Type),
		)
	}
}

// HotReload updates strategy in runtime to keep consistency with persistent state.
func (strategy *Strategy) HotReload(mongoStrategy models.MongoStrategy) {
	strategy.Log.Info("hot reloading",
		zap.String("id", strategy.ID()),
	)
	strategy.Model.Enabled = mongoStrategy.Enabled
	strategy.Model.Conditions = mongoStrategy.Conditions
	if mongoStrategy.Enabled == false {
		if strategy.StrategyRuntime != nil {
			strategy.StrategyRuntime.Stop() // stop runtime if disabled by DB, externally
		}
	}
	strategy.Statsd.Inc("strategy.hot_reload")
}
