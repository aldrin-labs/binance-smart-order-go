package tests

import (
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
)

type MockTrading struct {
	CallCount map[string]int
	AmountSum map[string]float64
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

func (mt MockTrading) CreateOrder(r trading.CreateOrderRequest) trading.OrderResponse {
	fmt.Printf("Create Order Request: %v", r)
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
	//println("create order", r.KeyParams.Symbol, r.KeyParams.Side)
	//fmt.Printf("%f\n", r.KeyParams.Amount)
	//mt.CallCount[exchange]++
	mt.CallCount[r.KeyParams.Side]++
	mt.CallCount[r.KeyParams.Symbol]++
	//mt.AmountSum[exchange+pair+side+fmt.Sprintf("%f", price)] += amount
	mt.AmountSum[r.KeyParams.Symbol+r.KeyParams.Side+fmt.Sprintf("%f", r.KeyParams.Price)] += r.KeyParams.Amount
	return trading.OrderResponse{Status: "OK", Data: trading.OrderResponseData{
		Id: string(mt.CallCount[r.KeyParams.Symbol]),
		OrderId: 0,
		Status:  "closed",
		Price:   r.KeyParams.Price,
		Average: r.KeyParams.Price,
		Filled: r.KeyParams.Amount,
	}}
}

func (mt MockTrading) CancelOrder(r trading.CancelOrderRequest) interface{} {
	return 0
}
