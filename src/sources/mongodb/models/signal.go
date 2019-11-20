package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type MongoSignalEvent struct {
	T    int64
	Data interface{}
}

type MongoSignal struct {
	Id          primitive.ObjectID `json:"_id"`
	MonType     MongoSignalType
	Condition   MongoSignalCondition
	TriggerWhen TriggerOptions
	Expiration  ExpirationSchema
	OpenEnded   bool
	Events      []MongoSignalEvent
}

type MongoSignalType struct {
	SigType  string `json:"type"`
	Required interface{}
}

type MongoSignalCondition struct {
	TargetPrice   float64
	Symbol        string
	PortfolioId   primitive.ObjectID
	PercentChange float64
	Price         float64
	Amount        float64
	Spread        float64
	ExchangeId    primitive.ObjectID
	ExchangeIds   []primitive.ObjectID
	Pair          string
}

type TriggerOptions struct {
	TrigType string `json:"type"`
	Period   int64
}

type ExpirationSchema struct {
	expirationTimestamp int64
	openEnded           bool
}
