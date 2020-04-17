package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type MongoKey struct {
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	HedgeMode bool               `json:"hedgeMode" bson:"hedgeMode"`
}
