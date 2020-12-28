package interfaces

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	statsd_client "gitlab.com/crypto_project/core/strategy_service/src/statsd"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.uber.org/zap"
)

// Strategy object
type IStrategy interface {
	GetModel() *models.MongoStrategy
	GetRuntime() IStrategyRuntime
	GetDatafeed() IDataFeed
	GetTrading() trading.ITrading
	GetStateMgmt() IStateMgmt
	GetSingleton() ICreateRequest
	GetStatsd() statsd_client.StatsdClient
	GetLogger() *zap.Logger
}
