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

type OrderResponseData struct {
	Id string `json:"id"`
	Status string `json:"status"`
}

type OrderResponse struct {
	Status string `json:"status"`
	Data OrderResponseData `json:"data"`
}

type ITrading interface {
	CreateOrder(order CreateOrderRequest) OrderResponse
	CancelOrder(params CancelOrderRequest) interface{}
}

type Trading struct {
}

func InitTrading() ITrading {
	tr := &Trading{}

	return tr
}

func Request(method string, data interface{}) interface{} {
	url := "http://" + os.Getenv("EXCHANGESERVICE") + "/" + method
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
	var response interface{}
	_ = json.Unmarshal(body, &response)
	return response
}
/*
{"status":"OK","data":{"info":
{"symbol":"BTCUSDT","orderId":878847053,"orderListId":-1,
"clientOrderId":"xLWGEmTb8wdS1dxHo8wJoP","transactTime":1575711104420,
"price":"0.00000000",
"origQty":"0.00200100",
"executedQty":"0.00200100",
"cummulativeQuoteQty":"15.01872561",
"status":"FILLED",
"timeInForce":"GTC",
"type":"MARKET","side":"BUY","fills":[{"price":"7505.61000000","qty":"0.00200100",
"commission":"0.00072150","commissionAsset":"BNB","tradeId":214094126}]},
"id":"878847053","timestamp":1575711104420,"datetime":"2019-12-07T09:31:44.420Z",
"symbol":"BTC_USDT","type":"market","side":"buy","price":7505.61,"amount":0.002001,
"cost":15.01872561,"average":7505.61,"filled":0.002001,"remaining":0,"status":"closed",
"fee":{"cost":0.0007215,"currency":"BNB"},"trades":[{"info":{"price":"7505.61000000",
"qty":"0.00200100","commission":"0.00072150","commissionAsset":"BNB","tradeId":214094126},
"symbol":"BTC/USDT","price":7505.61,"amount":0.002001,"cost":15.01872561,
"fee":{"cost":0.0007215,"currency":"BNB"}}]}}
 */
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
	MaxIfNotEnough int `json:"maxIfNotEnough"`
}

type Order struct {
	TargetPrice float64             `json:"targetPrice" bson:"targetPrice"`
	Symbol      string              `json:"symbol" bson:"symbol"`
	Side        string              `json:"side"`
	Amount      float64             `json:"amount"`
	TimeInForce string              `json:"timeInForce" bson:"timeInForce"`
	Type   		string              `json:"type" bson:"type"`
	Price       float64             `json:"price" bson:"price"`
	Params      OrderParams         `json:"orderParams" bson:"orderParams"`
}

type CreateOrderRequest struct {
	KeyId     *primitive.ObjectID `json:"keyId"`
	KeyParams Order `json:"keyParams"`
}

type CancelOrderRequest struct {
	KeyId   string
	OrderId string
}

func (t *Trading) CreateOrder(order CreateOrderRequest) OrderResponse {
	order.KeyParams.Params.MaxIfNotEnough = 1
	response := Request("createOrder", order).(OrderResponse)
	return response
}

func (t *Trading) CancelOrder(cancelRequest CancelOrderRequest) interface{} {
	return Request("cancelOrder", cancelRequest)
}
