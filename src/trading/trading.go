package trading

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strings"
)

type OrderResponseData struct {
	Id string `json:"orderId"`
	OrderId string `json:"orderId"`
	Status string `json:"status"`
	Price float64 `json:"price"`
	Average float64 `json:"average"`
	Filled float64 `json:"filled"`
}

type OrderResponse struct {
	Status string `json:"status"`
	Data OrderResponseData `json:"data"`
}

type ITrading interface {
	CreateOrder(order CreateOrderRequest) OrderResponse
	CancelOrder(params CancelOrderRequest) OrderResponse
	UpdateLeverage(keyId string, leverage float64) interface{}
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
		println(err.Error())

		return Request(method, data)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("request Body:", string(jsonStr))
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
	StopPrice float64 `json:"stopPrice,omitempty" bson:"stopPrice"`
	Type      string  `json:"type,omitempty" bson:"type"`
	MaxIfNotEnough int `json:"maxIfNotEnough,omitempty"`
	Update bool `json:"update,omitempty"`
}

type Order struct {
	TargetPrice float64             `json:"targetPrice,omitempty" bson:"targetPrice"`
	Symbol      string              `json:"symbol" bson:"symbol"`
	MarketType  int64               `json:"marketType" bson:"marketType"`
	Side        string              `json:"side"`
	Amount      float64             `json:"amount"`
	ReduceOnly  bool              	`json:"reduceOnly" bson:"reduceOnly"`
	TimeInForce string              `json:"timeInForce,omitempty" bson:"timeInForce"`
	Type   		string              `json:"type" bson:"type"`
	Price       float64             `json:"price,omitempty" bson:"price"`
	StopPrice float64 `json:"stopPrice,omitempty" bson:"stopPrice"`
	Params      OrderParams         `json:"params,omitempty" bson:"params"`
}

type CreateOrderRequest struct {
	KeyId     *primitive.ObjectID `json:"keyId"`
	KeyParams Order `json:"keyParams"`
}

type CancelOrderRequestParams struct {
	OrderId string `json:"id"`
	Pair string `json:"pair"`
	MarketType int64 `json:"marketType"`
}

type CancelOrderRequest struct {
	KeyId   *primitive.ObjectID `json:"keyId"`
	KeyParams CancelOrderRequestParams `json:"keyParams"`
}
func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num * output)) / output
}

func (t *Trading) CreateOrder(order CreateOrderRequest) OrderResponse {
	order.KeyParams.Params.Update = true
	if order.KeyParams.MarketType == 1 && (order.KeyParams.Type == "limit" || order.KeyParams.Params.Type == "stop-limit") {
		order.KeyParams.TimeInForce = "GTC"
	}
	if strings.Contains(order.KeyParams.Type, "market") || strings.Contains(order.KeyParams.Params.Type, "market")  {
		order.KeyParams.Price = 0.0
	}
	rawResponse := Request("createOrder", order)
	var response OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)

	response.Data.Id = string(response.Data.OrderId)
	return response
}

type UpdateLeverageParams struct {
	Leverage float64 `json:"leverage"`
	KeyId string `json:"keyId"`
}
func (t *Trading) UpdateLeverage(keyId string, leverage float64) interface{} {
	if leverage < 1 {
		leverage = 1
	}

	request := UpdateLeverageParams{
		KeyId: keyId,
		Leverage:leverage,
	}
	return Request("updateLeverage", request)
}

func (t *Trading) CancelOrder(cancelRequest CancelOrderRequest) OrderResponse {
	rawResponse := Request("cancelOrder", cancelRequest)
	var response OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}
