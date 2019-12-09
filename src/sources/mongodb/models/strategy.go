package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MongoStrategyUpdateEvent struct {
	FullDocument MongoStrategy `json:"fullDocument" bson:"fullDocument"`
}

type MongoStrategyEvent struct {
	T    int64
	Data interface{}
}

type MongoT struct {
	T    int64
	Data interface{}
}

type RBAC struct {
	Id          primitive.ObjectID `json:"_id"`
	userId      string
	portfolioId string
	accessLevel int32
}

type MongoSocial struct {
	SharedWith []primitive.ObjectID `bson:"sharedWith"` // [RBAC]
	IsPrivate  bool
}

type MongoStrategy struct {
	ID           primitive.ObjectID `json:"_id" bson:"_id"`
	StrategyType int64              `bson:"strategyType"`
	Enabled      bool				`bson:"enabled"`
	Conditions   MongoStrategyCondition `bson:"conditions"`
	State        MongoStrategyState	`bson:"state"`
	TriggerWhen  TriggerOptions `bson:"triggerWhen"`
	Expiration   ExpirationSchema
	OpenEnded    bool
	LastUpdate   int64
	SignalIds    []primitive.ObjectID
	OrderIds     []primitive.ObjectID `bson:"orderIds"`
	OwnerId      primitive.ObjectID
	Social       MongoSocial `bson:"social"` // {sharedWith: [RBAC]}
}

type MongoStrategyType struct {
	SigType  string `json:"type"`
	Required interface{}
}

type MongoStrategyState struct {
	State              string `json:"state" bson:"state"`
	TrailingEntryPrice float64 `json:"trailingEntryPrice" bson:"trailingEntryPrice"`
	EntryPrice         float64 `json:"entryPrice" bson:"entryPrice"`
	Amount             float64 `json:"amount" bson:"amount"`
	Orders             []string `json:"orders" bson:"orders"`
	ExecutedAmount     float64 `json:"executedAmount" bson:"executedAmount"`
	ReachedTargetCount int `json:"reachedTargetCount" bson:"reachedTargetCount"`
}

type MongoEntryPoint struct {
	ActivatePrice           float64 `json:"activatePrice" bson:"activatePrice"`
	EntryDeviation          float64 `json:"entryDeviation" bson:"entryDeviation"`
	Price                   float64 `json:"price" bson:"price"`
	Side               		string `json:"side" bson:"side"`
	Amount                  float64 `json:"amount" bson:"amount"`
	HedgeEntry              float64 `json:"hedgeEntry" bson:"hedgeEntry"`
	HedgeActivation         float64 `json:"hedgeActivation" bson:"hedgeActivation"`
	HedgeOppositeActivation float64 `json:"hedgeOppositeActivation" bson:"hedgeOppositeActivation"`
	Type                    int64 `json:"type" bson:"type"`
	OrderType               string `json:"orderType" bson:"orderType"`
}

type MongoStrategyCondition struct {
	KeyAssetId			primitive.ObjectID `json:"keyAssetId" bson:"keyAssetId"`
	Pair                string `json:"pair" bson:"pair"`
	MarketType			int64 `json:"marketType" bson:"marketType"`
	EntryOrder			MongoEntryPoint `json:"entryOrder" bson:"entryOrder"`

	PortfolioId         primitive.ObjectID `json:"portfolioId" bson:"portfolioId"`
	PercentChange       float64 `json:"percentChange" bson:"percentChange"`
	Price               float64 `json:"price" bson:"price"`
	ActivationPrice     float64 `json:"activationPrice" bson:"activationPrice"`
	EntryDeviation      float64 `json:"entryDeviation" bson:"entryDeviation"`
	ExchangeId          primitive.ObjectID `json:"exchangeId" bson:"exchangeId"`
	ExchangeIds         []primitive.ObjectID
	ExitDeviation       float64 `json:"exitDeviation" bson:"exitDeviation"`
	TakeProfit          float64 `json:"takeProfit" bson:"takeProfit"`
	TimeoutIfProfitable float64 `json:"timeoutIfProfitable" bson:"timeoutIfProfitable"`
	// then take profit after some time
	TimeoutWhenProfit float64 `json:"timeoutWhenProfit" bson:"timeoutWhenProfit"` // if position became profitable at takeProfit,
	// then dont exit but wait N seconds and exit, so you may catch pump
	ContinueIfEnded float64 `json:"continueIfEnded" bson:"continueIfEnded"` // open opposite position, or place buy if sold, or sell if bought
	// , if entrypoints specified, trading will be within entrypoints, if not exit on takeProfit or timeout or stoploss
	TimeoutBeforeOpenPosition float64 `json:"timeoutBeforeOpenPosition" bson:"timeoutBeforeOpenPosition"` // wait after closing position before opening new one
	ChangeTrendIfLoss         bool `json:"changeTrendIfLoss" bson:"changeTrendIfLoss"`
	ChangeTrendIfProfit       bool `json:"changeTrendIfProfit" bson:"changeTrendIfProfit"`
	StopLoss                  float64 `json:"stopLoss" bson:"stopLoss"`
	TimeoutLoss               float64 `json:"timeoutLoss" bson:"timeoutLoss"`
	ForcedLoss                float64 `json:"forcedLoss" bson:"forcedLoss"`
	Leverage                  float64 `json:"leverage" bson:"leverage"`
	EntryLevels               []MongoEntryPoint `json:"entryLevels" bson:"entryLevels"`
	ExitLevels                []MongoEntryPoint `json:"exitLevels" bson:"exitLevels"`
	TrailingEntries           bool `json:"trailingEntries" bson:"trailingEntries"`
	TrailingExit              bool `json:"trailingExit" bson:"trailingExit"`
}
