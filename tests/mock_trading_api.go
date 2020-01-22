package tests

import (
	"container/list"
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"strconv"
	"sync"
)

type MockTrading struct {
	OrdersMap *sync.Map
	CreatedOrders *list.List
	CanceledOrders *list.List
	CallCount map[string]int
	AmountSum map[string]float64
	Feed *MockDataFeed
	BuyDelay int
	SellDelay int
}

func (mt MockTrading) UpdateLeverage(keyId string, leverage float64) interface{} {
	panic("implement me")
}

func NewMockedTradingAPI() *MockTrading {
	mockTrading := MockTrading{
		CallCount: map[string]int{},
		AmountSum: map[string]float64{},
		CreatedOrders: list.New(),
		CanceledOrders: list.New(),
		OrdersMap: &sync.Map{},
		BuyDelay: 1000, // default 1 sec wait before orders got filled
		SellDelay: 1000, // default 1 sec wait before orders got filled
	}

	return &mockTrading
}

func NewMockedTradingAPIWithMarketAccess(feed *MockDataFeed) *MockTrading {
	mockTrading := MockTrading{
		CallCount: map[string]int{},
		AmountSum: map[string]float64{},
		Feed: feed,
		CreatedOrders: list.New(),
		CanceledOrders: list.New(),
		OrdersMap: &sync.Map{},
	}

	return &mockTrading
}

func (mt MockTrading) CreateOrder(req trading.CreateOrderRequest) trading.OrderResponse {
	fmt.Printf("Create Order Request: %v %f", req, req.KeyParams.Amount)
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
		Status:  "filled",
		OrderId: orderId,
		Average: req.KeyParams.Price,
		Filled:  req.KeyParams.Amount,
		Type: req.KeyParams.Type,
		Side: req.KeyParams.Side,
		Symbol: req.KeyParams.Symbol,
		StopPrice: req.KeyParams.StopPrice,
		ReduceOnly: req.KeyParams.ReduceOnly,
	}
	mt.OrdersMap.Store(orderId, order)
	mt.CreatedOrders.PushBack(order)
	// filled := req.KeyParams.Amount
	//if req.KeyParams.Type != "market" {
	//	filled = 0
	//}
	return trading.OrderResponse{Status: "OK", Data: trading.OrderResponseData{
		Id:      orderId,
		OrderId: orderId,
		Status:  "open",
		Price:   0,
		Average: 0,
		Filled:  0,
	}}
}

func (mt MockTrading) CancelOrder(req trading.CancelOrderRequest) trading.OrderResponse {
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
	response := trading.OrderResponse{
		Status: "OK",
	}
	return response
}
