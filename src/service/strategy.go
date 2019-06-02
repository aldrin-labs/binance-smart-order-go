package service

import (
    "gitlab.com/crypto_project/core/strategy_service/src/models"
    "gitlab.com/crypto_project/core/strategy_service/src/trading"
    "go.mongodb.org/mongo-driver/mongo"
)

// Strategy object
type Strategy struct {
    model models.StrategyModel
}

func GetStrategy(cur *mongo.Cursor) (*Strategy, error) {
    result := &models.StrategyModel{}
    err := cur.Decode(&result)
    return &Strategy{model:models.StrategyModel{}}, err
}

func (strategy * Strategy) Start() {

}


func (strategy * Strategy) CreateOrder(rawOrder trading.CreateOrderRequest) {
    createdOrder := trading.CreateOrder(rawOrder)

    client := mongodb
}