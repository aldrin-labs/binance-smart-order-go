package orders

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CreateOrderRequest struct {
	KeyId     *primitive.ObjectID `json:"keyId"`
	KeyParams Order               `json:"keyParams"`
}

type CancelOrderRequestParams struct {
	OrderId    string `json:"id"`
	Pair       string `json:"pair"`
	MarketType int64  `json:"marketType"`
}

type CancelOrderRequest struct {
	KeyId     *primitive.ObjectID      `json:"keyId"`
	KeyParams CancelOrderRequestParams `json:"keyParams"`
}

type HedgeRequest struct {
	KeyId     *primitive.ObjectID `json:"keyId"`
	HedgeMode bool                `json:"hedgeMode"`
}

type TransferRequest struct {
	FromKeyId  *primitive.ObjectID `json:"fromKeyId"`
	ToKeyId    *primitive.ObjectID `json:"toKeyId"`
	Symbol     string              `json:"symbol"`
	MarketType int                 `json:"marketType"`
	Amount     float64             `json:"amount"`
}
