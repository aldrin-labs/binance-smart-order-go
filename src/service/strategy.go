package service

import (
    "gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
    "gitlab.com/crypto_project/core/strategy_service/src/trading"
    "go.mongodb.org/mongo-driver/mongo"
)

// Strategy object
type Strategy struct {
    model models.MongoStrategy
}

func GetStrategy(cur *mongo.Cursor) (*Strategy, error) {
    result := &models.MongoStrategy{}
    err := cur.Decode(&result)
    return &Strategy{model:models.MongoStrategy{}}, err
}

func (strategy * Strategy) Start() {

}


func (strategy * Strategy) CreateOrder(rawOrder trading.CreateOrderRequest) {
    trading.CreateOrder(rawOrder)
}