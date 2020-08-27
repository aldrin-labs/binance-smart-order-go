package interfaces

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
)

// Strategy object
type IStrategy interface {
	GetModel() *models.MongoStrategy
	GetRuntime() IStrategyRuntime
	GetDatafeed() IDataFeed
	GetTrading() trading.ITrading
	GetStateMgmt() IStateMgmt
	GetSingleton() ICreateRequest
}