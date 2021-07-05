package smart_order

import (
	"context"
	"fmt"
	"github.com/go-redsync/redsync/v4"
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

// smart order should exit if loss condition is met
func TestSmartPlaceStopLossForEachTarget(t *testing.T) {
	// price drops
	fakeDataStream := []interfaces.OHLCV{
		{
			Open:   7100,
			High:   7101,
			Low:    7000,
			Close:  7000,
			Volume: 30,
		}, {
			Open:   7005,
			High:   7005,
			Low:    7000,
			Close:  7000,
			Volume: 30,
		}, {
			Open:   6905,
			High:   7005,
			Low:    7000,
			Close:  7000,
			Volume: 30,
		}}
	smartOrderModel := GetTestSmartOrderStrategy("stopLossMultiTargets")
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	tradingApi.SellDelay = 3000
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, stats := GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model:     &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: stats,
		SettlementMutex: &redsync.Mutex{},
	}
	smartOrder := smart_order.New(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(2000 * time.Millisecond)

	// check that one call with 'sell' and one with 'BTC_USDT' should be done
	sellCallCount, sellCallsFound := tradingApi.CallCount.Load("sell")
	btcUsdtCallCount, btcUsdtCallsFound := tradingApi.CallCount.Load("BTC_USDT")
	if !sellCallsFound || !btcUsdtCallsFound || sellCallCount != 4 || btcUsdtCallCount != 5 {
		if !sellCallsFound {
			sellCallCount = 0
		}
		if !btcUsdtCallsFound {
			btcUsdtCallCount = 0
		}
		t.Error("There were " + strconv.Itoa(sellCallCount.(int)) + " trading api calls with sell params and " + strconv.Itoa(btcUsdtCallCount.(int)) + " with BTC_USDT params")
	} else {
		fmt.Println("Success! There were " + strconv.Itoa(sellCallCount.(int)) + " trading api calls with sell params and " + strconv.Itoa(btcUsdtCallCount.(int)) + " with BTC_USDT params")
	}

	// check if we are in right state
	isInState, _ := smartOrder.State.IsInState(smart_order.InEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not End (State: " + stateStr + ")")
	}
}
