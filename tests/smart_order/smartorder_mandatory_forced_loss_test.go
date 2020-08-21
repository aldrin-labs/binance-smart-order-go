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

// smart order should create limit order while still in waitingForEntry state if not trailing
func TestSmartOrderMandatoryForcedLoss(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("mandatoryForcedLoss")
	// price rises (This has no meaning now, reuse and then remove fake data stream)
	fakeDataStream := []interfaces.OHLCV{{
		Open:   6800,
		High:   7101,
		Low:    6750,
		Close:  6900,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  6900,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  7010,
		Volume: 30,
	}}
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(2000 * time.Millisecond)

	// one call with 'sell' and one with 'BTC_USDT' should be done
	sellCallCount, sellOk := tradingApi.CallCount.Load("sell")
	btcUsdtCallCount, usdtBtcOk := tradingApi.CallCount.Load("BTC_USDT")
	if !sellOk || !usdtBtcOk || sellCallCount != 1 || btcUsdtCallCount != 2 {
		t.Error("Mandatory forced stop was not placed or another close orders was placed")
	} else {
		fmt.Println("Success! There were " + strconv.Itoa(sellCallCount.(int)) + " trading api calls with sell params, that is mandatory forced loss")
	}
}
