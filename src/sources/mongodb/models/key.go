package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type MongoKey struct {
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	ExchangeId        primitive.ObjectID `json:"exchangeId" bson:"exchangeId"`
	HedgeMode bool               `json:"hedgeMode" bson:"hedgeMode"`
}
