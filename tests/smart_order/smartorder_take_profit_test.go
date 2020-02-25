package smart_order

import (
	"context"
	"fmt"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/tests"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strconv"
	"testing"
	"time"
)

// smart order should take profit if condition is met
func TestSmartTakeProfit(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("TakeProfitMarket")
	// price rises
	fakeDataStream := []smart_order.OHLCV{{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  7005,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  7150,
		Volume: 30,
	}}
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi)
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(1450 * time.Millisecond)

	// check that one call with 'sell' and one with 'BTC_USDT' should be done
	if tradingApi.CallCount["sell"] == 0 || tradingApi.CallCount["BTC_USDT"] == 0 {
		t.Error("There were " + strconv.Itoa(tradingApi.CallCount["buy"]) + " trading api calls with buy params and " + strconv.Itoa(tradingApi.CallCount["BTC_USDT"]) + " with BTC_USDT params")
	}

	// check if we are in right state
	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not TakeProfit (State: " + stateStr + ")")
	}
	fmt.Println("Success! There were " + strconv.Itoa(tradingApi.CallCount["sell"]) + " trading api calls with buy params and " + strconv.Itoa(tradingApi.CallCount["BTC_USDT"]) + " with BTC_USDT params")
}

func TestSmartOrderTakeProfit(t *testing.T) {
	fakeDataStream := []smart_order.OHLCV{
		{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  7005,
		Volume: 30,
	}, { // Activation price
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  6900,
		Volume: 30,
	}, { // Hit entry
		Open:   7305,
		High:   7305,
		Low:    7300,
		Close:  7300,
		Volume: 30,
		}, { // Hit entry
			Open:   7305,
			High:   7305,
			Low:    7300,
			Close:  7300,
			Volume: 30,
	}, { // Take profit
		Open:   7705,
		High:   7705,
		Low:    7700,
		Close:  7700,
		Volume: 30,
	}, { // Take profit
		Open:   7705,
		High:   7705,
		Low:    7700,
		Close:  7700,
		Volume: 30,
	}, { // Take profit
		Open:   7705,
		High:   7705,
		Low:    7700,
		Close:  7700,
		Volume: 30,
	}}
	smartOrderModel := GetTestSmartOrderStrategy("takeProfit")
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	//sm := mongodb.StateMgmt{}
	sm := tests.NewMockedStateMgmt(tradingApi)
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(2 * time.Second)
	// TODO: now checking if TakeProfit is triggering, but it stops when sm.exit returns default "End" state
	// TODO: so it should test for TakeProfit state or calls to exchange API or maybe for smart order results?
	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not End (State: " + stateStr + ")")
	}
}

// Smart order can take profit on multiple price targets, not only at one price
func TestSmartOrderTakeProfitAllTargets(t *testing.T) {
	fakeDataStream := []smart_order.OHLCV{{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  7005,
		Volume: 30,
	}, { // Activation price
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  6900,
		Volume: 30,
	}, { // Hit entry
		Open:   7305,
		High:   7305,
		Low:    7300,
		Close:  7300,
		Volume: 30,
	}, { // Take profit 1 target
		Open:   7505,
		High:   7505,
		Low:    7500,
		Close:  7500,
		Volume: 30,
	}, { // Take profit 2 target
		Open:   7705,
		High:   7705,
		Low:    7700,
		Close:  7700,
		Volume: 30,
	}, { // Take profit 3 target
		Open:   7705,
		High:   7705,
		Low:    7700,
		Close:  7700,
		Volume: 30,
	}, { // Take profit 3 target
		Open:   9905,
		High:   9905,
		Low:    9900,
		Close:  9900,
		Volume: 30,
	}}
	smartOrderModel := GetTestSmartOrderStrategy("multiplePriceTargets")
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	//sm := mongodb.StateMgmt{}
	sm := tests.NewMockedStateMgmt(tradingApi)
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm) //TODO
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(2 * time.Second)
	if tradingApi.CallCount["sell"] != 3 && tradingApi.AmountSum["binanceBTC_USDTsell"] != smartOrder.Strategy.Model.Conditions.EntryOrder.Amount {
		t.Error("SmartOrder didn't reach all 3 targets, but reached", tradingApi.CallCount["sell"], tradingApi.AmountSum["binanceBTC_USDTsell"], smartOrder.Strategy.Model.Conditions.EntryOrder.Amount)
	}
}