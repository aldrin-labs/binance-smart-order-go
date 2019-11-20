package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type MongoExchange struct {
	ID     primitive.ObjectID `json:"_id"`
	Symbol string
}
