package smart_order

import (
	"context"
	"fmt"
	"github.com/go-redsync/redsync/v4"
	"log"
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
		Low:    5750,
		Close:  6800,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    5800,
		Close:  6800,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    5940,
		Close:  6810,
		Volume: 30,
	}}

	df := tests.NewMockedDataFeed(fakeDataStream)
	df.WaitForOrderInitializationMillis = 2500

	tradingApi := tests.NewMockedTradingAPI()
	tradingApi.BuyDelay = 100
	tradingApi.SellDelay = 100
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, statsd := tests.GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model:     &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: statsd,
		SettlementMutex: &redsync.Mutex{},
	}
	smartOrder := smart_order.New(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(3 * time.Second)

	// one call with 'sell' and one with 'BTC_USDT' should be done
	buyCallCount, buyOk := tradingApi.CallCount.Load("buy")
	sellCallCount, sellOk := tradingApi.CallCount.Load("sell")
	btcUsdtCallCount, usdtBtcOk := tradingApi.CallCount.Load("BTC_USDT")
	_, cancelOk := tradingApi.CanceledOrdersCount.Load("BTC_USDT")

	if cancelOk || !sellOk || !buyOk || !usdtBtcOk || sellCallCount != 2 || btcUsdtCallCount != 5 || buyCallCount != 3 {
		t.Error("3 Entry orders or 1 SL/FL was not placed")
	} else {
		fmt.Println("Success")
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
		Close:  5990,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5985,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  5980,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  5975,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6970,
		Close:  5970,
		Volume: 30,
	},{ // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6970,
		Close:  5965,
		Volume: 30,
	},
	}
	df := tests.NewMockedDataFeed(fakeDataStream)
	df.WaitForOrderInitializationMillis = 1500
	df.TickDuration = 500 * time.Millisecond
	tradingApi := tests.NewMockedTradingAPI()
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, statsd := tests.GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model:     &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: statsd,
		SettlementMutex: &redsync.Mutex{},
	}
	smartOrder := smart_order.New(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(6 * time.Second)

	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	if isInState {
		log.Print("Multi-Entry was closed by SL")
	} else {
		state, _ := smartOrder.State.State(context.TODO())
		t.Error("State is not End, currentState: ", state)
	}
}

func TestSmartOrderMultiEntryTAP(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("multiEntryPlacingTAP")
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
		Close:  5990,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5980,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5975,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5975,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5975,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  5980,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5990,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  6010,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  6040,
		Volume: 30,
	}}
	df := tests.NewMockedDataFeed(fakeDataStream)
	df.WaitForOrderInitializationMillis = 1500
	df.TickDuration = 500 * time.Millisecond
	tradingApi := tests.NewMockedTradingAPI()
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, statsd := tests.GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model:     &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: statsd,
		SettlementMutex: &redsync.Mutex{},
	}
	smartOrder := smart_order.New(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(8 * time.Second)

	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	sellCallCount, sellOk := tradingApi.CallCount.Load("sell")
	fmt.Println(isInState, sellCallCount.(int))

	if isInState && sellOk && sellCallCount == 4 {
		log.Print("Multi-Entry was closed by TAP")
	} else {
		t.Error("Multi-Entry wasn't closed by TAP or SM placed not 3 TAP and 1 SL orders")
	}
}

func TestSmartOrderMultiEntryClosingAfterFirstTAP(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("multiEntryPlacingClosingAfterFirstTAP")
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
	}, {
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
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  5800,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  5700,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5700,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5700,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5704,
		Volume: 30,
	}}
	df := tests.NewMockedDataFeed(fakeDataStream)
	df.WaitForOrderInitializationMillis = 1500
	tradingApi := tests.NewMockedTradingAPI()
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, statsd := tests.GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model:     &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: statsd,
		SettlementMutex: &redsync.Mutex{},
	}
	tradingApi.BuyDelay = 30
	smartOrder := smart_order.New(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(5000 * time.Millisecond)

	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	sellCallCount, sellOk := tradingApi.CallCount.Load("sell")

	log.Print("sellOk ", sellOk)

	if isInState && sellOk && sellCallCount == 3 {
		log.Print("Multi-Entry was closed by first TAP")
	} else {
		state, _ := smartOrder.State.State(context.TODO())
		t.Error("Without loss order was not placed or SM was not closed by CloseAfterFirstTAP option. sellCallCount ", sellCallCount, " state ", state)
	}
}

func TestSmartOrderMultiEntryClosingByWithoutLoss(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("multiEntryPlacingClosingByWithoutLoss")
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
	}, {
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
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  5700,
		Volume: 30,
	}, { // Hit entry
		Open:   6950,
		High:   7305,
		Low:    6950,
		Close:  5700,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    6800,
		Close:  5705,
		Volume: 30,
	}}
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, statsd := tests.GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model:     &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: statsd,
		SettlementMutex: &redsync.Mutex{},
	}
	tradingApi.BuyDelay = 30
	smartOrder := smart_order.New(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(3000 * time.Millisecond)

	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	sellCallCount, sellOk := tradingApi.CallCount.Load("sell")

	log.Print("sellOk ", sellOk)

	if isInState && sellOk && sellCallCount == 3 {
		log.Print("Multi-Entry was closed by Without Loss")
	} else {
		state, _ := smartOrder.State.State(context.TODO())
		t.Error("Without loss order was not placed or SM was not closed by Without Loss. sellCallCount ", sellCallCount, " state ", state)
	}
}
