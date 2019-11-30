package testing

import "fmt"

type MockTrading struct {
	CallCount map[string]int
	AmountSum map[string]float64
}

func NewMockedTradingAPI() *MockTrading {
	mockTrading := MockTrading{
		CallCount: map[string]int{},
		AmountSum: map[string]float64{},
	}

	return &mockTrading
}


func (mt *MockTrading) CreateOrder(exchange string, pair string, price float64, amount float64, side string) string {
	//if mt.CallCount[exchange] {
	//	callCount[exchange] = 0
	//}
	//if callCount[side] == nil {
	//	callCount[side] = 0
	//}
	//if callCount[pair] == nil {
	//	callCount[pair] = 0
	//}
	println("create order", exchange, pair, side)
	fmt.Printf("%f\n", amount)
	mt.CallCount[exchange]++
	mt.CallCount[side]++
	mt.CallCount[pair]++
	mt.AmountSum[exchange+pair+side+fmt.Sprintf("%f", price)] += amount
	return "tradeid"
}