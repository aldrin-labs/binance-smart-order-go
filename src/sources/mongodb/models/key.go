package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type MongoKey struct {
	ID         primitive.ObjectID  `json:"_id" bson:"_id"`
	ExchangeId *primitive.ObjectID `json:"exchangeId" bson:"exchangeId"`
	HedgeMode  bool                `json:"hedgeMode" bson:"hedgeMode"`
}

type KeyAsset struct {
	KeyId primitive.ObjectID `json:"keyId" bson:"keyId"`
	Free  float64            `json:"free" bson:"free"`
}
