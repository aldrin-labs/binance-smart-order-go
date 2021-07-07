package trading

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/logging"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/trading/orders"
	"go.uber.org/zap"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)



type Trading struct {
}

var log interfaces.ILogger

func init() {
	logger, _ := logging.GetZapLogger()
	log = logger.With(zap.String("logger", "trading"))
}

func InitTrading() interfaces.ITrading {
	tr := &Trading{}

	return tr
}

// Request encodes data to JSON, sends it to exchange service and returns decoded response.
func Request(method string, data interface{}) interface{} {
	url := "http://" + os.Getenv("EXCHANGESERVICE") + "/" + method
	log.Info("request", zap.String("url", url), zap.String("data", fmt.Sprintf("%+v", data)))

	var jsonStr, err = json.Marshal(data)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		retryDelay := time.Duration(1 * time.Second)
		log.Error("request not successful",
			zap.String("url", url),
			zap.Error(err),
			zap.Duration("retry in, seconds", retryDelay),
			zap.String("request", fmt.Sprintf("%+v", req)),
			zap.String("data", fmt.Sprintf("%+v", data)),
		)
		time.Sleep(retryDelay)
		return Request(method, data)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	log.Info("response",
		zap.String("status", resp.Status),
		zap.String("header", fmt.Sprintf("%v", resp.Header)),
		zap.String("request body", string(jsonStr)),
		zap.String("response body", string(body)),
	)
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



func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

// CreateOrder requests exchange service to create an order.
func (t *Trading) CreateOrder(order orders.CreateOrderRequest) orders.OrderResponse {
	order.KeyParams.Params.Update = true
	if order.KeyParams.PostOnly != nil && *order.KeyParams.PostOnly == false {
		order.KeyParams.PostOnly = nil
	}
	if order.KeyParams.PostOnly == nil && order.KeyParams.MarketType == 1 && (order.KeyParams.Type == "limit" || order.KeyParams.Params.Type == "stop-limit") {
		order.KeyParams.TimeInForce = "GTC"
	}
	if strings.Contains(order.KeyParams.Type, "market") || strings.Contains(order.KeyParams.Params.Type, "market") {
		// TODO: figure out
		// somehow set price to 0 here or at placeOrder
		// "request body":"{\"keyId\":\"5e9d948f15a68aaf7a6aa55d\",\"keyParams\":{\"symbol\":\"ETH_USDT\",\"marketType\":1,\"side\":\"sell\",\"amount\":0.007,\"filled\":0,\"average\":0,\"reduceOnly\":true,\"timeInForce\":\"GTC\",\"type\":\"stop\",\"stopPrice\":1730.76,\"positionSide\":\"BOTH\",\"params\":{\"type\":\"stop-limit\",\"update\":true},\"frequency\":0}}","response body":"{\"status\":\"ERR\",\"data\":{\"msg\":\"Error code: -1102 Message: A mandatory parameter was not sent, was empty/null, or malformed.\"}}"}
		order.KeyParams.Price = 0.0
		log.Info(
			"set order.KeyParams.Price to zero",
			zap.String("order.KeyParams.Type", order.KeyParams.Type),
			zap.String("order.KeyParams.Params.Type", order.KeyParams.Params.Type),
		)
	}
	if order.KeyParams.ReduceOnly != nil && *order.KeyParams.ReduceOnly == false {
		order.KeyParams.ReduceOnly = nil
	}
	rawResponse := Request("createOrder", order)
	var response orders.OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)

	response.Data.OrderId = string(response.Data.OrderId)
	return response
}

type UpdateLeverageParams struct {
	Leverage float64             `json:"leverage"`
	Symbol   string              `json:"symbol"`
	KeyId    *primitive.ObjectID `json:"keyId"`
}

func (t *Trading) UpdateLeverage(keyId *primitive.ObjectID, leverage float64, symbol string) orders.UpdateLeverageResponse {
	if leverage < 1 {
		leverage = 1
	}

	request := UpdateLeverageParams{
		KeyId:    keyId,
		Leverage: leverage,
		Symbol:   symbol,
	}

	rawResponse := Request("updateLeverage", request)

	var response orders.UpdateLeverageResponse
	_ = mapstructure.Decode(rawResponse, &response)

	return response
}

func (t *Trading) CancelOrder(cancelRequest orders.CancelOrderRequest) orders.OrderResponse {
	rawResponse := Request("cancelOrder", cancelRequest)
	var response orders.OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}

// maybe its not the best place and should be in SM, coz its SM related, not trading
// but i dont care atm sorry not sorry
func (t *Trading) PlaceHedge(parentSmartOrder *models.MongoStrategy) orders.OrderResponse {

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

	createRequest := orders.CreateOrderRequest{
		KeyId: hedgedStrategy.Conditions.AccountId,
		KeyParams: orders.Order{
			Type: "smart",
			Params: orders.OrderParams{
				SmartOrder: hedgedStrategy.Conditions,
			},
		},
	}

	rawResponse := Request("createOrder", createRequest)
	var response orders.OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}

func (t *Trading) Transfer(request orders.TransferRequest) orders.OrderResponse {
	rawResponse := Request("transfer", request)

	var response orders.OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}

func (t *Trading) SetHedgeMode(keyId *primitive.ObjectID, hedgeMode bool) orders.OrderResponse {
	request := orders.HedgeRequest{
		KeyId:     keyId,
		HedgeMode: hedgeMode,
	}
	rawResponse := Request("changePositionMode", request)

	var response orders.OrderResponse
	_ = mapstructure.Decode(rawResponse, &response)
	return response
}
