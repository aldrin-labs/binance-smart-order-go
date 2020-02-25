package smart_order

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
	tradingApi.SellDelay = 30000
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
	time.Sleep(10000 * time.Millisecond)

	// check that one call with 'sell' and one with 'BTC_USDT' should be done
	if tradingApi.CallCount["sell"] != 4 || tradingApi.CallCount["BTC_USDT"] != 5 {
		t.Error("There were " + strconv.Itoa(tradingApi.CallCount["sell"]) + " trading api calls with sell params and " + strconv.Itoa(tradingApi.CallCount["BTC_USDT"]) + " with BTC_USDT params")
	}

	// check if we are in right state
	isInState, _ := smartOrder.State.IsInState(smart_order.InEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not End (State: " + stateStr + ")")
	}
	fmt.Println("Success! There were " + strconv.Itoa(tradingApi.CallCount["sell"]) + " trading api calls with sell params and " + strconv.Itoa(tradingApi.CallCount["BTC_USDT"]) + " with BTC_USDT params")
}
