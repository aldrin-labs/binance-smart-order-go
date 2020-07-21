package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type MongoStrategyUpdateEvent struct {
	FullDocument MongoStrategy `json:"fullDocument" bson:"fullDocument"`
}

type MongoOrderUpdateEvent struct {
	FullDocument MongoOrder `json:"fullDocument" bson:"fullDocument"`
}

type MongoPositionUpdateEvent struct {
	FullDocument MongoPosition `json:"fullDocument" bson:"fullDocument"`
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

type MongoMarketDefaultProperties struct {
	PricePrecision    int64 `json:"pricePrecision" bson:"pricePrecision"`
	QuantityPrecision int64 `json:"quantityPrecision" bson:"quantityPrecision"`
}

type MongoMarketProperties struct {
	Binance MongoMarketDefaultProperties `json:"binance" bson:"binance"`
}

type MongoMarket struct {
	ID         primitive.ObjectID    `json:"_id" bson:"_id"`
	Properties MongoMarketProperties `json:"properties" bson:"properties"`
}

type MongoOrder struct {
	ID         primitive.ObjectID `json:"_id" bson:"_id"`
	Status     string             `json:"status,omitempty" bson:"status"`
	OrderId    string             `json:"id,omitempty" bson:"id"`
	PostOnlyFinalOrderId string   `json:"postOnlyFinalOrderId,omitempty" bson:"postOnlyFinalOrderId"`
	PostOnlyInitialOrderId string `json:"postOnlyInitialOrderId,omitempty" bson:"postOnlyInitialOrderId"`
	Filled     float64            `json:"filled,omitempty" bson:"filled"`
	Average    float64            `json:"average,omitempty" bson:"average"`
	Side       string             `json:"side,omitempty" bson:"side"`
	Type       string             `json:"type,omitempty" bson:"type"`
	Symbol     string             `json:"symbol,omitempty" bson:"symbol"`
	ReduceOnly bool               `json:"reduceOnly,omitempty" bson:"reduceOnly"`
	StopPrice  float64            `json:"stopPrice,omitempty" bson:"stopPrice"`
}
type MongoPosition struct {
	ID          primitive.ObjectID `json:"_id" bson:"_id"`
	KeyId       primitive.ObjectID `json:"keyId" bson:"keyId"`
	EntryPrice  float64            `json:"entryPrice,omitempty" bson:"entryPrice"`
	MarkPrice   float64            `json:"markPrice,omitempty" bson:"markPrice"`
	PositionAmt float64            `json:"positionAmt,omitempty" bson:"positionAmt"`
	Symbol      string             `json:"symbol,omitempty" bson:"symbol"`
	Leverage    float64            `json:"leverage,omitempty" bson:"leverage"`
	UpdatedAt   time.Time          `json:"updatedAt,omitempty" bson:"updatedAt"`
}

type MongoStrategy struct {
	ID              *primitive.ObjectID     `json:"_id" bson:"_id"`
	Type            int64                   `json:"type,omitempty" bson:"type"`
	Enabled         bool                    `json:"enabled,omitempty" bson:"enabled"`
	AccountId       *primitive.ObjectID     `json:"accountId,omitempty" bson:"accountId"`
	Conditions      *MongoStrategyCondition `json:"conditions,omitempty" bson:"conditions"`
	State           *MongoStrategyState     `bson:"state,omitempty"`
	TriggerWhen     TriggerOptions          `bson:"triggerWhen,omitempty"`
	Expiration      ExpirationSchema
	LastUpdate      int64
	SignalIds       []primitive.ObjectID
	OrderIds        []primitive.ObjectID `bson:"orderIds,omitempty"`
	WaitForOrderIds []primitive.ObjectID `bson:"waitForOrderIds,omitempty"`
	OwnerId         primitive.ObjectID
	Social          MongoSocial `bson:"social"` // {sharedWith: [RBAC]}
	CreatedAt   time.Time             `json:"createdAt,omitempty" bson:"createdAt"`
}

type MongoStrategyType struct {
	SigType  string `json:"type"`
	Required interface{}
}

type MongoStrategyState struct {
	State                  string  `json:"state,omitempty" bson:"state"`
	Msg                    string  `json:"msg,omitempty" bson:"msg"`
	EntryOrderId           string  `json:"entryOrderId,omitempty" bson:"entryOrderId"`
	// we save params to understand which was changed
	EntryPointPrice        float64  `json:"entryPointPrice,omitempty" bson:"entryPointPrice"`
	EntryPointType         string   `json:"entryPointType,omitempty" bson:"entryPointType"`
	EntryPointSide         string   `json:"entryPointSide,omitempty" bson:"entryPointSide"`
	EntryPointAmount       float64  `json:"entryPointAmount,omitempty" bson:"entryPointAmount"`
	StopLoss               float64  `json:"stopLoss,omitempty" bson:"stopLoss"`
	StopLossPrice          float64  `json:"stopLossPrice, omitempty" bson:"stopLossPrice"`
	StopLossOrderIds       []string  `json:"stopLossOrderIds,omitempty" bson:"stopLossOrderIds"`
	ForcedLoss             float64  `json:"forcedLoss,omitempty" bson:"forcedLoss"`
	ForcedLossPrice        float64  `json:"forcedLossPrice, omitempty" bson:"forcedLossPrice"`
	ForcedLossOrderIds     []string `json:"forcedLossOrderIds,omitempty" bson:"forcedLossOrderIds"`
	TakeProfit  		   []*MongoEntryPoint `json:"takeProfit,omitempty" bson:"takeProfit"`
	TakeProfitPrice        float64  `json:"takeProfitPrice, omitempty" bson:"takeProfitPrice"`
	TakeProfitHedgePrice   float64  `json:"takeProfitHedgePrice,omitempty" bson:"takeProfitHedgePrice"`
	TakeProfitOrderIds     []string `json:"takeProfitOrderIds,omitempty" bson:"takeProfitOrderIds"`

	TrailingEntryPrice     float64 `json:"trailingEntryPrice,omitempty" bson:"trailingEntryPrice"`
	HedgeExitPrice         float64 `json:"hedgeExitPrice,omitempty" bson:"hedgeExitPrice"`
	TrailingHedgeExitPrice float64 `json:"trailingHedgeExitPrice,omitempty" bson:"trailingHedgeExitPrice"`

	TrailingExitPrice  float64   `json:"trailingExitPrice,omitempty" bson:"trailingExitPrice"`
	TrailingExitPrices []float64 `json:"trailingExitPrices,omitempty" bson:"trailingExitPrices"`
	EntryPrice         float64   `json:"entryPrice,omitempty" bson:"entryPrice"`
	ExitPrice          float64   `json:"exitPrice,omitempty" bson:"exitPrice"`
	Amount             float64   `json:"amount,omitempty" bson:"amount"`
	Orders             []string  `json:"orders,omitempty" bson:"orders"`
	ExecutedOrders     []string  `json:"executedOrders,omitempty" bson:"executedOrders"`
	ExecutedAmount     float64   `json:"executedAmount,omitempty" bson:"executedAmount"`
	ReachedTargetCount int       `json:"reachedTargetCount,omitempty" bson:"reachedTargetCount"`

	TrailingCheckAt int64 `json:"trailingCheckAt,omitempty" bson:"trailingCheckAt"`
	StopLossAt      int64 `json:"stopLossAt,omitempty" bson:"stopLossAt"`
	LossableAt      int64 `json:"lossableAt,omitempty" bson:"lossableAt"`
	ProfitableAt    int64 `json:"profitableAt,omitempty" bson:"profitableAt"`
	ProfitAt        int64 `json:"profitAt,omitempty" bson:"profitAt"`
}

type MongoEntryPoint struct {
	ActivatePrice           float64 `json:"activatePrice,omitempty" bson:"activatePrice"`
	EntryDeviation          float64 `json:"entryDeviation,omitempty" bson:"entryDeviation"`
	Price                   float64 `json:"price,omitempty" bson:"price"`
	Side                    string  `json:"side,omitempty" bson:"side"`
	Amount                  float64 `json:"amount,omitempty" bson:"amount"`
	HedgeEntry              float64 `json:"hedgeEntry,omitempty" bson:"hedgeEntry"`
	HedgeActivation         float64 `json:"hedgeActivation,omitempty" bson:"hedgeActivation"`
	HedgeOppositeActivation float64 `json:"hedgeOppositeActivation,omitempty" bson:"hedgeOppositeActivation"`
	// Type: 0 means absolute price, 1 means price is relative to entry price
	Type      int64  `json:"type,omitempty" bson:"type"`
	OrderType string `json:"orderType,omitempty" bson:"orderType"`
}

type MongoStrategyCondition struct {
	AccountId *primitive.ObjectID `json:"accountId,omitempty" bson:"accountId"`

	Hedging         bool                `json:"hedging,omitempty" bson:"hedging"`
	HedgeMode       bool                `json:"hedgeMode,omitempty" bson:"hedgeMode"`
	HedgeKeyId      *primitive.ObjectID `json:"hedgeKeyId,omitempty" bson:"hedgeKeyId"`
	HedgeStrategyId *primitive.ObjectID `json:"hedgeStrategyId,omitempty" bson:"hedgeStrategyId"`

	TemplateToken          string   `json:"templateToken,omitempty" bson:"templateToken"`
	PositionWasClosed      bool     `json:"positionWasClosed, omitempty" bson:"positionWasClosed"`
	SkipInitialSetup       bool     `json:"skipInitialSetup, omitempty" bson:"skipInitialSetup"`
	CancelIfAnyActive      bool     `json:"cancelIfAnyActive,omitempty" bson:"cancelIfAnyActive"`
	TrailingExitExternal   bool     `json:"trailingExitExternal,omitempty" bson:"trailingExitExternal"`
	TrailingExitPrice  	   float64  `json:"trailingExitPrice,omitempty" bson:"trailingExitPrice"`
	StopLossPrice          float64  `json:"stopLossPrice,omitempty" bson:"stopLossPrice"`
	ForcedLossPrice        float64  `json:"forcedLossPrice,omitempty" bson:"forcedLossPrice"`
	TakeProfitPrice        float64  `json:"takeProfitPrice,omitempty" bson:"takeProfitPrice"`
	TakeProfitHedgePrice   float64  `json:"takeProfitHedgePrice,omitempty" bson:"takeProfitHedgePrice"`
	StopLossExternal       bool     `json:"stopLossExternal,omitempty" bson:"stopLossExternal"`
	TakeProfitExternal     bool     `json:"takeProfitExternal,omitempty" bson:"takeProfitExternal"`
	WithoutLossAfterProfit float64  `json:"withoutLossAfterProfit,omitempty" bson:"withoutLossAfterProfit"`
	EntrySpreadHunter      bool     `json:"entrySpreadHunter,omitempty" bson:"entrySpreadHunter"`
	EntryWaitingTime       int64    `json:"entryWaitingTime,omitempty" bson:"entryWaitingTime"`
	TakeProfitSpreadHunter bool     `json:"takeProfitSpreadHunter,omitempty" bson:"takeProfitSpreadHunter"`
	TakeProfitWaitingTime  int64    `json:"takeProfitWaitingTime,omitempty" bson:"takeProfitWaitingTime"`
	KeyAssetId *primitive.ObjectID `json:"keyAssetId,omitempty" bson:"keyAssetId"`
	Pair       string              `json:"pair,omitempty" bson:"pair"`
	MarketType int64               `json:"marketType,omitempty" bson:"marketType"`
	EntryOrder *MongoEntryPoint    `json:"entryOrder,omitempty" bson:"entryOrder"`

	WaitingEntryTimeout   float64 `json:"waitingEntryTimeout,omitempty" bson:"waitingEntryTimeout"`
	ActivationMoveStep    float64 `json:"activationMoveStep,omitempty" bson:"activationMoveStep"`
	ActivationMoveTimeout float64 `json:"activationMoveTimeout,omitempty" bson:"activationMoveTimeout"`

	TimeoutIfProfitable float64 `json:"timeoutIfProfitable,omitempty" bson:"timeoutIfProfitable"`
	// then take profit after some time
	TimeoutWhenProfit float64 `json:"timeoutWhenProfit,omitempty" bson:"timeoutWhenProfit"` // if position became profitable at takeProfit,
	// then dont exit but wait N seconds and exit, so you may catch pump

	ContinueIfEnded           bool    `json:"continueIfEnded,omitempty" bson:"continueIfEnded"`                     // open opposite position, or place buy if sold, or sell if bought // , if entrypoints specified, trading will be within entrypoints, if not exit on takeProfit or timeout or stoploss
	TimeoutBeforeOpenPosition float64 `json:"timeoutBeforeOpenPosition,omitempty" bson:"timeoutBeforeOpenPosition"` // wait after closing position before opening new one
	ChangeTrendIfLoss         bool    `json:"changeTrendIfLoss,omitempty" bson:"changeTrendIfLoss"`
	ChangeTrendIfProfit       bool    `json:"changeTrendIfProfit,omitempty" bson:"changeTrendIfProfit"`

	MoveStopCloser        bool    `json:"moveStopClose,omitempty" bson:"moveStopCloser"`
	MoveForcedStopAtEntry bool    `json:"moveForcedStopAtEntry,omitempty" bson:"moveForcedStopAtEntry"`
	TimeoutWhenLoss       float64 `json:"timeoutWhenLoss,omitempty" bson:"timeoutWhenLoss"` // wait after hit SL and gives it a chance to grow back
	TimeoutLoss           float64 `json:"timeoutLoss,omitempty" bson:"timeoutLoss"`         // if ROE negative it counts down and if still negative then exit
	StopLoss              float64 `json:"stopLoss,omitempty" bson:"stopLoss"`
	StopLossType          string  `json:"stopLossType,omitempty" bson:"stopLossType"`
	ForcedLoss            float64 `json:"forcedLoss,omitempty" bson:"forcedLoss"`
	HedgeLossDeviation    float64 `json:"hedgeLossDeviation,omitempty" bson:"hedgeLossDeviation"`

	CreatedByTemplate  bool                `json:"createdByTemplate,omitempty" bson:"createdByTemplate"`
	TemplateStrategyId *primitive.ObjectID `json:"templateStrategyId,omitempty" bson:"templateStrategyId"`

	Leverage    float64            `json:"leverage,omitempty" bson:"leverage"`
	EntryLevels []*MongoEntryPoint `json:"entryLevels,omitempty" bson:"entryLevels"`
	ExitLevels  []*MongoEntryPoint `json:"exitLevels,omitempty" bson:"exitLevels"`
}
