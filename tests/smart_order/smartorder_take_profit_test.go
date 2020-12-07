package smart_order

import (
	"context"
	"fmt"
	"log"
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

// smart order should take profit if condition is met
func TestSmartTakeProfit(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("TakeProfitMarket")
	// price rises
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
		Close:  7150,
		Volume: 30,
	}}
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
		StateMgmt: &sm,
	}
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(1450 * time.Millisecond)

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
		t.Error("SmartOrder state is not TakeProfit (State: " + stateStr + ")")
	}
}

func TestSmartOrderTakeProfit(t *testing.T) {
	fakeDataStream := []interfaces.OHLCV{
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
	keyId := primitive.NewObjectID()
	//sm := mongodb.StateMgmt{}
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
		StateMgmt: &sm,
	}
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
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
	fakeDataStream := []interfaces.OHLCV{{
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
		Open:   8855,
		High:   8855,
		Low:    8855,
		Close:  8855,
		Volume: 30,
	}, { // Take profit 3 target
		Open:   9240,
		High:   9240,
		Low:    9240,
		Close:  9240,
		Volume: 30,
	}, { // Take profit 3 target
		Open:   9240,
		High:   9240,
		Low:    9240,
		Close:  9240,
		Volume: 30,
	}}
	smartOrderModel := GetTestSmartOrderStrategy("multiplePriceTargets")
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
		StateMgmt: &sm,
	}
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm) //TODO
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()

	time.Sleep(2 * time.Second)

	sellCallCount, _ := tradingApi.CallCount.Load("sell")
	amountSold, _ := tradingApi.AmountSum.Load("BTC_USDTsell")
	expectedAmountToSell := smartOrder.Strategy.GetModel().Conditions.EntryOrder.Amount

	if sellCallCount != 3 || amountSold != expectedAmountToSell {
		t.Error("SmartOrder didn't reach all 3 targets, but called sell", sellCallCount, "times for amount", amountSold, "out of", expectedAmountToSell)
	}
}
