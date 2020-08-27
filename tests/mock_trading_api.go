package tests

import (
	"container/list"
	"fmt"
	"log"
	"strconv"
	"sync"

	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MockTrading struct {
	OrdersMap           *sync.Map
	CreatedOrders       *list.List
	CanceledOrders      *list.List
	CanceledOrdersCount *sync.Map
	CallCount           *sync.Map
	AmountSum           *sync.Map
	Feed                *MockDataFeed
	BuyDelay            int
	SellDelay           int
}

func (mt MockTrading) UpdateLeverage(keyId *primitive.ObjectID, leverage float64, symbol string) interface{} {
	panic("implement me")
}

func NewMockedTradingAPI() *MockTrading {
	mockTrading := MockTrading{
		CallCount:           &sync.Map{},
		AmountSum:           &sync.Map{},
		CreatedOrders:       list.New(),
		CanceledOrdersCount: &sync.Map{},
		CanceledOrders:      list.New(),
		OrdersMap:           &sync.Map{},
		BuyDelay:            1000, // default 1 sec wait before orders got filled
		SellDelay:           1000, // default 1 sec wait before orders got filled
	}

	return &mockTrading
}

func NewMockedTradingAPIWithMarketAccess(feed *MockDataFeed) *MockTrading {
	mockTrading := MockTrading{
		CallCount:           &sync.Map{},
		AmountSum:           &sync.Map{},
		Feed:                feed,
		CreatedOrders:       list.New(),
		CanceledOrders:      list.New(),
		OrdersMap:           &sync.Map{},
		CanceledOrdersCount: &sync.Map{},
	}

	return &mockTrading
}

func (mt MockTrading) CreateOrder(req trading.CreateOrderRequest) trading.OrderResponse {
	fmt.Printf("Create Order Request: %v %f \n", req, req.KeyParams.Amount)

	callCount, _ := mt.CallCount.LoadOrStore(req.KeyParams.Side, 0)
	mt.CallCount.Store(req.KeyParams.Side, callCount.(int)+1)

	callCount, _ = mt.CallCount.LoadOrStore(req.KeyParams.Symbol, 0)
	mt.CallCount.Store(req.KeyParams.Symbol, callCount.(int)+1)

	//amountSumKey := req.KeyParams.Symbol + req.KeyParams.Side + fmt.Sprintf("%f", req.KeyParams.Price)
	amountSumKey := req.KeyParams.Symbol + req.KeyParams.Side

	amountSum, _ := mt.AmountSum.LoadOrStore(amountSumKey, 0.0)
	mt.AmountSum.Store(amountSumKey, amountSum.(float64)+req.KeyParams.Amount)

	callCount, _ = mt.CallCount.Load(req.KeyParams.Symbol)
	orderId := req.KeyParams.Symbol + strconv.Itoa(callCount.(int))

	orderType := req.KeyParams.Type

	if req.KeyParams.Params.Type != "" {
		orderType = req.KeyParams.Params.Type
	}
	order := models.MongoOrder{
		Status:     "open",
		OrderId:    orderId,
		Average:    req.KeyParams.Price,
		Filled:     req.KeyParams.Amount,
		Type:       orderType,
		Side:       req.KeyParams.Side,
		Symbol:     req.KeyParams.Symbol,
		StopPrice:  req.KeyParams.StopPrice,
		ReduceOnly: *req.KeyParams.ReduceOnly,
	}
	if order.Average == 0 && mt.Feed != nil {
		lent := len(mt.Feed.tickerData)
		index := mt.Feed.currentTick
		if mt.Feed.currentTick >= lent {
			index = lent - 1
		}
		if index < 0 {
			index = 0
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
		OrderId: orderId,
		OrderId: orderId,
		Status:  "open",
		Price:   0,
		Average: 0,
		Filled:  0,
	}}
}

func (mt MockTrading) CancelOrder(req trading.CancelOrderRequest) trading.OrderResponse {
	fmt.Printf("Cancel Order Request: %v %f \n", req, req.KeyParams.Pair)
	callCount, callOk := mt.CallCount.Load(req.KeyParams.Pair)

	log.Print("callOk ", callOk)
	if !callOk {
		response := trading.OrderResponse{
			Status: "ERR",
		}
		return response
	}

	orderId := req.KeyParams.OrderId
	callCount, _ = mt.CanceledOrdersCount.LoadOrStore(req.KeyParams.Pair, 0)

	orderRaw, ok := mt.OrdersMap.Load(orderId)
	var order models.MongoOrder

	if !ok {
		order = models.MongoOrder{
			Status:  "canceled",
			OrderId: orderId,
		}
	} else {
		order = orderRaw.(models.MongoOrder)
		log.Print("order in cancel", order.Status, order.Side)
		if order.Status == "open" {
			mt.CanceledOrdersCount.Store(req.KeyParams.Pair, callCount.(int)+1)
			order.Status = "canceled"
		}
	}

	mt.OrdersMap.Store(orderId, order)
	response := trading.OrderResponse{
		Status: "OK",
	}
	return response
}

func (mt MockTrading) PlaceHedge(parentSmarOrder *models.MongoStrategy) trading.OrderResponse {
	panic("implement me")
}

func (mt MockTrading) Transfer(request trading.TransferRequest) trading.OrderResponse {
	panic("implement me")
}

func (mt MockTrading) SetHedgeMode(keyId *primitive.ObjectID, hedgeMode bool) trading.OrderResponse {
	response := trading.OrderResponse{
		Status: "OK",
	}
	return response
}
