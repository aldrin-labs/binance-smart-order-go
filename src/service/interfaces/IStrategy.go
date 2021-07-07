package interfaces

import (
	"github.com/go-redsync/redsync/v4"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.uber.org/zap"
)

// Strategy object
type IStrategy interface {
	GetModel() *models.MongoStrategy
	GetRuntime() IStrategyRuntime
	GetSettlementMutex() *redsync.Mutex
	GetDatafeed() IDataFeed
	GetTrading() trading.ITrading
	GetStateMgmt() IStateMgmt
	GetSingleton() ICreateRequest
	GetStatsd() IStatsClient
	GetLogger() *zap.Logger
}
