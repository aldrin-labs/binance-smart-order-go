package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type MongoAsset struct {
	ID     primitive.ObjectID `json:"_id" bson:"_id"`
	Symbol string             `json:"symbol" bson:"symbol"`
}

type MongoKeyAsset struct {
	ID          primitive.ObjectID     `json:"_id" bson:"_id"`
	Type        int64                  `json:"type" bson:"type"`
	Enabled     bool                   `bson:"enabled"`
	Conditions  MongoStrategyCondition `bson:"conditions"`
	State       MongoStrategyState     `bson:"state"`
	TriggerWhen TriggerOptions         `bson:"triggerWhen"`
	Expiration  ExpirationSchema
	OpenEnded   bool
	LastUpdate  int64
	SignalIds   []primitive.ObjectID
	OrderIds    []primitive.ObjectID `bson:"orderIds"`
	OwnerId     primitive.ObjectID
	Social      MongoSocial `bson:"social"` // {sharedWith: [RBAC]}
}
