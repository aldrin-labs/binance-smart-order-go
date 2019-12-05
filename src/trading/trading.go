package trading

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io/ioutil"
	"net/http"
	"os"
)

type ITrading interface {
	CreateOrder(order CreateOrderRequest) interface{}
	CancelOrder(params CancelOrderRequest) interface{}
}

type Trading struct {
}

func InitTrading() ITrading {
	tr := &Trading{}

	return tr
}

func Request(method string, data interface{}) interface{} {
	url := "http://" + os.Getenv("EXCHANGESERVCE") + "/" + method
	fmt.Println("URL:>", url)

	var jsonStr, err = json.Marshal(data)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))
	return body
}

/*
{
	"keyId": "5ca48f82744e09001ac430d5",
	"keyParams": {
    "symbol": "BTC/USDT",
    "type": "limit",
    "side": "buy",
    "amount": 0.026,
    "price": 90
	}
}
*/

type OrderParams struct {
	StopPrice float64 `json:"stopPrice" bson:"stopPrice"`
	Type      string  `json:"type" bson:"type"`
}

type Order struct {
	TargetPrice float64             `json:"targetPrice" bson:"targetPrice"`
	Symbol      string              `json:"symbol" bson:"symbol"`
	Side        string              `json:"side"`
	Amount      float64             `json:"amount"`
	Type   		string              `json:"orderType" bson:"orderType"`
	Price       float64             `json:"price" bson:"price"`
	Params      OrderParams         `json:"orderParams" bson:"orderParams"`
}

type CreateOrderRequest struct {
	KeyId     *primitive.ObjectID
	KeyParams Order
}

type CancelOrderRequest struct {
	KeyId   string
	OrderId string
}

func (t *Trading) CreateOrder(order CreateOrderRequest) interface{} {
	return Request("createOrder", order)
}

func (t *Trading) CancelOrder(cancelRequest CancelOrderRequest) interface{} {
	return Request("cancelOrder", cancelRequest)
}
