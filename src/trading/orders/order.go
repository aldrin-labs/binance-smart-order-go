package orders

import "gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"

type OrderParams struct {
	StopPrice      float64                        `json:"stopPrice,omitempty" bson:"stopPrice"`
	Type           string                         `json:"type,omitempty" bson:"type"`
	MaxIfNotEnough int                            `json:"maxIfNotEnough,omitempty"`
	Retry          bool                           `json:"retry,omitempty"`
	RetryTimeout   int64                          `json:"retryTimeout,omitempty"`
	RetryCount     int                            `json:"retryCount,omitempty"`
	Update         bool                           `json:"update,omitempty"`
	SmartOrder     *models.MongoStrategyCondition `json:"smartOrder,omitempty"`
}

type Order struct {
	TargetPrice  float64     `json:"targetPrice,omitempty" bson:"targetPrice"`
	Symbol       string      `json:"symbol" bson:"symbol"`
	MarketType   int64       `json:"marketType" bson:"marketType"`
	Side         string      `json:"side"`
	Amount       float64     `json:"amount"`
	Filled       float64     `json:"filled"`
	Average      float64     `json:"average"`
	ReduceOnly   *bool       `json:"reduceOnly,omitempty" bson:"reduceOnly"`
	TimeInForce  string      `json:"timeInForce,omitempty" bson:"timeInForce"`
	Type         string      `json:"type" bson:"type"`
	Price        float64     `json:"price,omitempty" bson:"price"`
	StopPrice    float64     `json:"stopPrice,omitempty" bson:"stopPrice"`
	PositionSide string      `json:"positionSide,omitempty" bson:"positionSide"`
	Params       OrderParams `json:"params,omitempty" bson:"params"`
	PostOnly     *bool       `json:"postOnly,omitempty" bson:"postOnly"`
	Frequency    float64     `json:"frequency" bson:"frequency"`
}
