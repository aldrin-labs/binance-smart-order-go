package smart_order

/*
	This file contains test cases stop-loss part of smart orders
*/

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/tests"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// smart order should exit if loss condition is met
func TestSmartExitOnStopMarket(t *testing.T) {
	// price drops
	fakeDataStream := []interfaces.OHLCV{{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  7005,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  6900,
		Volume: 30,
	}, {
		Open:   6905,
		High:   7005,
		Low:    6600,
		Close:  6600,
		Volume: 30,
	}}
	smartOrderModel := GetTestSmartOrderStrategy("stopLossMarket")
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()

	tradingApi.BuyDelay = 1000
	tradingApi.SellDelay = 1000
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
	time.Sleep(5000 * time.Millisecond)

	// check that one call with 'sell' and one with 'BTC_USDT' should be done
	sellCallCount, sellFound := tradingApi.CallCount.Load("sell")
	btcUsdtCallCount, usdtBtcFound := tradingApi.CallCount.Load("BTC_USDT")
	if !sellFound || !usdtBtcFound || sellCallCount == 0 || btcUsdtCallCount == 0 {
		t.Error("There were 0 trading api calls with sell params and 0 with BTC_USDT params")
	} else {
		fmt.Println("Success! There were " + strconv.Itoa(sellCallCount.(int)) + " trading api calls with sell params and " + strconv.Itoa(btcUsdtCallCount.(int)) + " with BTC_USDT params")
	}

	// check if we are in right state
	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not End (State: " + stateStr + ")")
	}
}

// smart order should wait for timeout if set
func TestSmartExitOnStopMarketTimeout(t *testing.T) {
	// price drops
	fakeDataStream := []interfaces.OHLCV{
		{
			Open:   7100,
			High:   7101,
			Low:    7000,
			Close:  7005,
			Volume: 30,
		}, {
			Open:   7005,
			High:   7005,
			Low:    6900,
			Close:  6900,
			Volume: 30,
		}, {
			Open:   6905,
			High:   7005,
			Low:    6600,
			Close:  6600,
			Volume: 30,
		}, {
			Open:   6605,
			High:   6600,
			Low:    6500,
			Close:  6500,
			Volume: 30,
		}, {
			Open:   6605,
			High:   6600,
			Low:    6500,
			Close:  6500,
			Volume: 30,
		}, {
			Open:   6605,
			High:   6600,
			Low:    6500,
			Close:  6500,
			Volume: 30,
		}}
	smartOrderModel := GetTestSmartOrderStrategy("stopLossMarketTimeout")
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
	time.Sleep(3000 * time.Millisecond)

	// check that one call with 'sell' and one with 'BTC_USDT' should be done
	sellCallCount, sellFound := tradingApi.CallCount.Load("sell")
	btcUsdtCallCount, usdtBtcFound := tradingApi.CallCount.Load("BTC_USDT")
	if !sellFound || !usdtBtcFound || sellCallCount == 0 || btcUsdtCallCount == 0 {
		t.Error("There were 0 trading api calls with sell params and 0 with BTC_USDT params")
	} else {
		fmt.Println("Success! There were " + strconv.Itoa(sellCallCount.(int)) + " trading api calls with buy params and " + strconv.Itoa(btcUsdtCallCount.(int)) + " with BTC_USDT params")
	}

	// check if we are in right state
	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not End (State: " + stateStr + ")")
	}
}
