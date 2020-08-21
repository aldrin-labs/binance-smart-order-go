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

// smart order should create limit order while still in waitingForEntry state if not trailing
func TestSmartOrderMultiEntryPlacing(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("multiEntryPlacing")
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
	tradingApi.BuyDelay = 5000
	tradingApi.SellDelay = 5000
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(1000 * time.Millisecond)

	// one call with 'sell' and one with 'BTC_USDT' should be done
	buyCallCount, buyOk := tradingApi.CallCount.Load("buy")
	sellCallCount, sellOk := tradingApi.CallCount.Load("sell")
	btcUsdtCallCount, usdtBtcOk := tradingApi.CallCount.Load("BTC_USDT")
	if !sellOk || !buyOk || !usdtBtcOk || sellCallCount != 1 || btcUsdtCallCount != 4 || buyCallCount != 3 {
		t.Error("Something went wrong")
	} else {
		fmt.Println("Success! There were " + strconv.Itoa(sellCallCount.(int)) + " trading api calls with sell params, and " + strconv.Itoa(buyCallCount.(int)) + " with buy side")
	}
}

func TestSmartOrderMultiEntryStopLoss(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("multiEntryPlacing")
	// price rises (This has no meaning now, reuse and then remove fake data stream)
	fakeDataStream := []interfaces.OHLCV{{
		Open:   6000,
		High:   7101,
		Low:    6750,
		Close:  6000,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	},{
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	},{
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	},{
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	},{
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	},{
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	},{
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	},{
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	},{
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5900,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  5800,
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
	time.Sleep(15000 * time.Millisecond)

	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	if isInState {
		log.Print("Multi-Entry was closed by SL")
	}
}
