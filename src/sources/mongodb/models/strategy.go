package models

import "go.mongodb.org/mongo-driver/bson/primitive"

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
	sharedWith []primitive.ObjectID // [RBAC]
	isPrivate  bool
}

type MongoStrategy struct {
	Id          primitive.ObjectID `json:"_id"`
	StrategyType      string `json:"strategyType"`
	Enabled 	bool
	Condition   MongoStrategyCondition
	State		MongoStrategyState
	TriggerWhen TriggerOptions
	Expiration  ExpirationSchema
	OpenEnded   bool
	LastUpdate  int64
	SignalIds   []primitive.ObjectID
	OrderIds    []primitive.ObjectID
	OwnerId     primitive.ObjectID
	Social      MongoSocial // {sharedWith: [RBAC]}
}

type MongoStrategyType struct {
	SigType  string `json:"type"`
	Required interface{}
}

type MongoStrategyState struct {
	State	string
	TrailingEntryPrice float64
	EntryPrice float64
	Amount float64
	ExecutedAmount float64
	ReachedTargetCount int
}

type MongoEntryPoint struct {
	ActivatePrice           float64
	EntryDeviation          float64
	Price                   float64
	Amount                  float64
	HedgeEntry              float64
	HedgeActivation         float64
	HedgeOppositeActivation float64
	Type                    int64
	OrderType               string
}

type MongoStrategyCondition struct {
	TargetPrice         float64
	Symbol              string
	PortfolioId         primitive.ObjectID
	PercentChange       float64
	Price               float64
	ActivationPrice     float64
	EntryDeviation      float64
	Amount              float64
	ExchangeId          primitive.ObjectID
	ExchangeIds         []primitive.ObjectID
	Pair                string
	Side                string
	ExitDeviation       float64
	TakeProfit          float64
	TimeoutIfProfitable float64
	// then take profit after some time
	TimeoutWhenProfit float64 // if position became profitable at takeProfit,
	// then dont exit but wait N seconds and exit, so you may catch pump
	ContinueIfEnded float64 // open opposite position, or place buy if sold, or sell if bought
	// , if entrypoints specified, trading will be within entrypoints, if not exit on takeProfit or timeout or stoploss
	TimeoutBeforeOpenPosition float64 // wait after closing position before opening new one
	ChangeTrendIfLoss         bool
	ChangeTrendIfProfit       bool
	StopLoss                  float64
	TimeoutLoss               float64
	ForcedLoss                float64
	Leverage                  float64
	EntryLevels               []MongoEntryPoint
	ExitLeveles               []MongoEntryPoint
	TrailingEntries           bool
	TrailingExit              bool
}
