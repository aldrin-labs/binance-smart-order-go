package trading

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrderResponseData struct {
	OrderId string `json:"orderId, string"`
	Msg     string `json:"msg,string"`
	//OrderId string  `json:"orderId, string"`
	Status  string  `json:"status"`
	Type    string  `json:"type"`
	Price   float64 `json:"price"`
	Average float64 `json:"average"`
	Amount  float64 `json:"amount"`
	Filled  float64 `json:"filled"`
	Code    int64   `json:"code" bson:"code"`
}

type OrderResponse struct {
	Status string            `json:"status"`
	Data   OrderResponseData `json:"data"`
}

type UpdateLeverageResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

type TransferRequest struct {
	FromKeyId  *primitive.ObjectID `json:"fromKeyId"`
	ToKeyId    *primitive.ObjectID `json:"toKeyId"`
	Symbol     string              `json:"symbol"`
	MarketType int                 `json:"marketType"`
	Amount     float64             `json:"amount"`
}

type ITrading interface {
	CreateOrder(order CreateOrderRequest) OrderResponse
	CancelOrder(params CancelOrderRequest) OrderResponse
	PlaceHedge(parentSmarOrder *models.MongoStrategy) OrderResponse

	UpdateLeverage(keyId *primitive.ObjectID, leverage float64, symbol string) UpdateLeverageResponse
	Transfer(request TransferRequest) OrderResponse
	SetHedgeMode(keyId *primitive.ObjectID, hedgeMode bool) OrderResponse
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
		log.Print(err.Error())

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
	StopPrice      float64                        `json:"stopPrice,omitempty" bson:"stopPrice"`
	Type           string                         `json:"type,omitempty" bson:"type"`
	MaxIfNotEnough int                            `json:"maxIfNotEnough,omitempty"`
	Retry          bool                           `json:"retry,omitempty"`
	RetryTimeout   int64                          `json:"retryTimeout,omitempty"`
	RetryCount     int                            `json:"retryCount,omitempty"`
	Update         bool                           `json:"update,omitempty"`
	SmartOrder     *models.MongoStrategyCondition `json:"smartOrder,omitempty"`
}

type Order struct {
	TargetPrice  float64     `json:"targetPrice,omitempty" bson:"targetPrice"`
	Symbol       string      `json:"symbol" bson:"symbol"`
	MarketType   int64       `json:"marketType" bson:"marketType"`
	Side         string      `json:"side"`
	Amount       float64     `json:"amount"`
	Filled       float64     `json:"filled"`
	Average      float64     `json:"average"`
	ReduceOnly   *bool       `json:"reduceOnly,omitempty" bson:"reduceOnly"`
	TimeInForce  string      `json:"timeInForce,omitempty" bson:"timeInForce"`
	Type         string      `json:"type" bson:"type"`
	Price        float64     `json:"price,omitempty" bson:"price"`
	StopPrice    float64     `json:"stopPrice,omitempty" bson:"stopPrice"`
	PositionSide string      `json:"positionSide,omitempty" bson:"positionSide"`
	Params       OrderParams `json:"params,omitempty" bson:"params"`
	PostOnly     *bool       `json:"postOnly,omitempty" bson:"postOnly"`
	Frequency    float64     `json:"frequency" bson:"frequency"`
}

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

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

func (t *Trading) CreateOrder(order CreateOrderRequest) OrderResponse {
	order.KeyParams.Params.Update = true
	if order.KeyParams.PostOnly != nil && *order.KeyParams.PostOnly == false {
		order.KeyParams.PostOnly = nil
	}
	if order.KeyParams.PostOnly == nil && order.KeyParams.MarketType == 1 && (order.KeyParams.Type == "limit" || order.KeyParams.Params.Type == "stop-limit") {
		order.KeyParams.TimeInForce = "GTC"
	}
	if strings.Contains(order.KeyParams.Type, "market") || strings.Contains(order.KeyParams.Params.Type, "market") {
		order.KeyParams.Price = 0.0
	}
	if order.KeyParams.ReduceOnly != nil && *order.KeyParams.ReduceOnly == false {
		order.KeyParams.ReduceOnly = nil
	}
	rawResponse := Request("createOrder", order)
	var response OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)

	response.Data.OrderId = string(response.Data.OrderId)
	return response
}

type UpdateLeverageParams struct {
	Leverage float64             `json:"leverage"`
	Symbol   string              `json:"symbol"`
	KeyId    *primitive.ObjectID `json:"keyId"`
}

func (t *Trading) UpdateLeverage(keyId *primitive.ObjectID, leverage float64, symbol string) UpdateLeverageResponse {
	if leverage < 1 {
		leverage = 1
	}

	request := UpdateLeverageParams{
		KeyId:    keyId,
		Leverage: leverage,
		Symbol:   symbol,
	}

	rawResponse := Request("updateLeverage", request)

	var response UpdateLeverageResponse
	_ = mapstructure.Decode(rawResponse, &response)

	return response
}

func (t *Trading) CancelOrder(cancelRequest CancelOrderRequest) OrderResponse {
	rawResponse := Request("cancelOrder", cancelRequest)
	var response OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}

// maybe its not the best place and should be in SM, coz its SM related, not trading
// but i dont care atm sorry not sorry
func (t *Trading) PlaceHedge(parentSmartOrder *models.MongoStrategy) OrderResponse {

	var jsonStr, _ = json.Marshal(parentSmartOrder)
	var hedgedStrategy models.MongoStrategy
	_ = json.Unmarshal(jsonStr, &hedgedStrategy)

	hedgedStrategy.Conditions.HedgeStrategyId = parentSmartOrder.ID
	// dont need it for now i guess
	//accountId, _ := primitive.ObjectIDFromHex(hedgedStrategy.Conditions.HedgeKeyId.Hex())
	hedgedStrategy.Conditions.AccountId = parentSmartOrder.AccountId
	hedgedStrategy.Conditions.Hedging = false
	oppositeSide := hedgedStrategy.Conditions.EntryOrder.Side
	if oppositeSide == "buy" {
		oppositeSide = "sell"
	} else {
		oppositeSide = "buy"
	}
	hedgedStrategy.Conditions.ContinueIfEnded = false
	hedgedStrategy.Conditions.EntryOrder.Side = oppositeSide
	hedgedStrategy.Conditions.TemplateToken = ""

	createRequest := CreateOrderRequest{
		KeyId: hedgedStrategy.Conditions.AccountId,
		KeyParams: Order{
			Type: "smart",
			Params: OrderParams{
				SmartOrder: hedgedStrategy.Conditions,
			},
		},
	}

	rawResponse := Request("createOrder", createRequest)
	var response OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}

func (t *Trading) Transfer(request TransferRequest) OrderResponse {
	rawResponse := Request("transfer", request)

	var response OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}

func (t *Trading) SetHedgeMode(keyId *primitive.ObjectID, hedgeMode bool) OrderResponse {
	request := HedgeRequest{
		KeyId:     keyId,
		HedgeMode: hedgeMode,
	}
	rawResponse := Request("changePositionMode", request)

	var response OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}
