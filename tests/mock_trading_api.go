package tests

import (
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"strconv"
	"sync"
)

type MockTrading struct {
	OrdersMap sync.Map
	CallCount map[string]int
	AmountSum map[string]float64
	Feed *MockDataFeed
}

func (mt MockTrading) UpdateLeverage(keyId string, leverage float64) interface{} {
	panic("implement me")
}

func NewMockedTradingAPI() *MockTrading {
	mockTrading := MockTrading{
		CallCount: map[string]int{},
		AmountSum: map[string]float64{},
	}

	return &mockTrading
}

func NewMockedTradingAPIWithMarketAccess(feed *MockDataFeed) *MockTrading {
	mockTrading := MockTrading{
		CallCount: map[string]int{},
		AmountSum: map[string]float64{},
		Feed: feed,
	}

	return &mockTrading
}

func (mt MockTrading) CreateOrder(req trading.CreateOrderRequest) trading.OrderResponse {
	fmt.Printf("Create Order Request: %v", req)
	println()
	//if mt.CallCount[exchange] {
	//	callCount[exchange] = 0
	//}
	//if callCount[side] == nil {
	//	callCount[side] = 0
	//}
	//if callCount[pair] == nil {
	//	callCount[pair] = 0
	//}
	//println("create order", exchange, pair, side)
	//println("create order", req.KeyParams.Symbol, req.KeyParams.Side)
	//fmt.Printf("%f\n", req.KeyParams.Amount)
	//mt.CallCount[exchange]++
	mt.CallCount[req.KeyParams.Side]++
	mt.CallCount[req.KeyParams.Symbol]++
	//mt.AmountSum[exchange+pair+side+fmt.Sprintf("%f", price)] += amount
	mt.AmountSum[req.KeyParams.Symbol+req.KeyParams.Side+fmt.Sprintf("%f", req.KeyParams.Price)] += req.KeyParams.Amount
	orderId := req.KeyParams.Symbol + strconv.Itoa(mt.CallCount[req.KeyParams.Symbol])
	order := models.MongoOrder{
		Status:  "open",
		OrderId: orderId,
		Average: req.KeyParams.Price,
		Filled:  req.KeyParams.Amount,
	}
	mt.OrdersMap.Store(orderId, order)

	return trading.OrderResponse{Status: "OK", Data: trading.OrderResponseData{
		Id:      orderId,
		OrderId: orderId,
		Status:  "open",
		Price:   req.KeyParams.Price,
		Average: req.KeyParams.Price,
		Filled:  req.KeyParams.Amount,
	}}
}

func (mt MockTrading) CancelOrder(req trading.CancelOrderRequest) interface{} {
	orderId := string(mt.CallCount[req.KeyParams.Pair]) + strconv.Itoa(mt.CallCount[req.KeyParams.Pair])

	orderRaw, ok := mt.OrdersMap.Load(orderId)
	var order models.MongoOrder

	if !ok {
		order = models.MongoOrder{
			Status:  "canceled",
			OrderId: orderId,
		}
	} else {
		order = orderRaw.(models.MongoOrder)
		order.Status = "canceled"
	}
	mt.OrdersMap.Store(orderId, order)
	return order
}
