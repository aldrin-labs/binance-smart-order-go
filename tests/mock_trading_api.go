package tests

import (
	"container/list"
	"fmt"
	"strconv"
	"sync"

	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MockTrading struct {
	OrdersMap      *sync.Map
	CreatedOrders  *list.List
	CanceledOrders *list.List
	CallCount      sync.Map
	AmountSum      sync.Map
	Feed           *MockDataFeed
	BuyDelay       int
	SellDelay      int
}

func (mt MockTrading) UpdateLeverage(keyId *primitive.ObjectID, leverage float64, symbol string) interface{} {
	panic("implement me")
}

func NewMockedTradingAPI() *MockTrading {
	mockTrading := MockTrading{
		// CallCount:      map[string]int{},
		// AmountSum:      map[string]float64{},
		CallCount:      sync.Map{},
		AmountSum:      sync.Map{},
		CreatedOrders:  list.New(),
		CanceledOrders: list.New(),
		OrdersMap:      &sync.Map{},
		BuyDelay:       1000, // default 1 sec wait before orders got filled
		SellDelay:      1000, // default 1 sec wait before orders got filled
	}

	return &mockTrading
}

func NewMockedTradingAPIWithMarketAccess(feed *MockDataFeed) *MockTrading {
	mockTrading := MockTrading{
		CallCount:      sync.Map{},
		AmountSum:      sync.Map{},
		Feed:           feed,
		CreatedOrders:  list.New(),
		CanceledOrders: list.New(),
		OrdersMap:      &sync.Map{},
	}

	return &mockTrading
}

func (mt MockTrading) CreateOrder(req trading.CreateOrderRequest) trading.OrderResponse {
	fmt.Printf("Create Order Request: %v %f \n", req, req.KeyParams.Amount)

	callCount, _ := mt.CallCount.LoadOrStore(req.KeyParams.Side, 0)
	mt.CallCount.Store(req.KeyParams.Side, callCount.(int)+1)

	callCount, _ = mt.CallCount.LoadOrStore(req.KeyParams.Symbol, 0)
	mt.CallCount.Store(req.KeyParams.Symbol, callCount.(int)+1)

	//mt.AmountSum[exchange+pair+side+fmt.Sprintf("%f", price)] += amount
	amountSumKey := req.KeyParams.Symbol + req.KeyParams.Side + fmt.Sprintf("%f", req.KeyParams.Price)
	amountSum, _ := mt.AmountSum.LoadOrStore(amountSumKey, 0.0)
	mt.AmountSum.Store(amountSumKey, amountSum.(float64)+req.KeyParams.Amount)

	callCount, _ = mt.CallCount.Load(req.KeyParams.Symbol)
	orderId := req.KeyParams.Symbol + strconv.Itoa(callCount.(int))
	order := models.MongoOrder{
		Status:     "filled",
		OrderId:    orderId,
		Average:    req.KeyParams.Price,
		Filled:     req.KeyParams.Amount,
		Type:       req.KeyParams.Type,
		Side:       req.KeyParams.Side,
		Symbol:     req.KeyParams.Symbol,
		StopPrice:  req.KeyParams.StopPrice,
		ReduceOnly: req.KeyParams.ReduceOnly,
	}
	if order.Average == 0 {
		lent := len(mt.Feed.tickerData)
		index := mt.Feed.currentTick
		if mt.Feed.currentTick >= lent {
			index = lent - 1
		}
		order.Average = mt.Feed.tickerData[index].Close
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
	callCount, _ := mt.CallCount.Load(req.KeyParams.Pair)
	orderId := string(callCount.(int)) + strconv.Itoa(callCount.(int))

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
