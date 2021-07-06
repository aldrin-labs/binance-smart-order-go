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
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/tests"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSmartOrderTrailingEntryAndThenActivateTrailingWithHighLeverage(t *testing.T) {
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
			Low:    6950,
			Close:  6950,
			Volume: 30,
		}, { // Hit entry 100x leverage
			Open:   6952.5,
			High:   6952.5,
			Low:    6952.5,
			Close:  6952.5,
			Volume: 30,
		}, { // Activate trailing profit
			Open:   6959.5,
			High:   6959.5,
			Low:    6959.5,
			Close:  6959.5,
			Volume: 30,
		}, { // It goes up..
			Open:   6970,
			High:   6970,
			Low:    6970,
			Close:  6990,
			Volume: 30,
		}}
	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryExitLeverage")
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	tradingApi.SellDelay = 30000
	keyId := primitive.NewObjectID()
	//sm := mongodb.StateMgmt{}
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, stats := tests.GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model:     &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: stats,
		SettlementMutex: &redsync.Mutex{},
	}
	smartOrder := smart_order.New(&strategy, df, sm.Trading, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(10 * time.Second)
	isInState, _ := smartOrder.State.IsInState(smart_order.InEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not InEntry (State: " + stateStr + ")")
	}
}

func TestSmartOrderTrailingEntryAndTrailingExitWithHighLeverage(t *testing.T) {
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
			Low:    6950,
			Close:  6950,
			Volume: 30,
		}, { // Hit entry 100x leverage
			Open:   6952.5,
			High:   6952.5,
			Low:    6952.5,
			Close:  6952.5,
			Volume: 30,
		}, { // Activate trailing profit
			Open:   6959.5,
			High:   6959.5,
			Low:    6959.5,
			Close:  6959.5,
			Volume: 30,
		}, { // It goes up..
			Open:   6970,
			High:   6970,
			Low:    6970,
			Close:  6970,
			Volume: 30,
		}, { // It goes up..
			Open:   6967.5,
			High:   6967.5,
			Low:    6967.5,
			Close:  6999.5,
			Volume: 30,
		}, { // Spiked down, ok, up trend is over, we are taking profits now
			Open:   6967.5,
			High:   6967.5,
			Low:    6967.5,
			Close:  6980.5,
			Volume: 30,
		}}
	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryExitLeverage")
	df := tests.NewMockedDataFeed(fakeDataStream)
	df.WaitForOrderInitialization = 3000
	df.WaitBetweenTicks = 1500
	df.CycleLastNEntries = 4
	tradingApi := tests.NewMockedTradingAPI()
	tradingApi.BuyDelay = 300
	tradingApi.SellDelay = 300
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, stats := tests.GetLoggerStatsd()
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
	time.Sleep(25 * time.Second)
	isInState, _ := smartOrder.State.IsInState(smart_order.End)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not End (State: " + stateStr + ")")
	}
}

func TestSmartOrderTrailingEntryAndFollowTrailingMaximumsWithoutEarlyExitWithHighLeverage(t *testing.T) {
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
			Low:    6950,
			Close:  6950,
			Volume: 30,
		}, { // Hit entry 100x leverage
			Open:   6952.5,
			High:   6952.5,
			Low:    6952.5,
			Close:  6952.5,
			Volume: 30,
		}, { // Activate trailing profit
			Open:   6959.5,
			High:   6959.5,
			Low:    6959.5,
			Close:  6959.5,
			Volume: 30,
		}, { // It goes up..
			Open:   6970,
			High:   6970,
			Low:    6970,
			Close:  6970,
			Volume: 30,
		}, { // It goes up..
			Open:   6975,
			High:   6975,
			Low:    6975,
			Close:  6975,
			Volume: 30,
		}, { // It goes up..
			Open:   7170,
			High:   7170,
			Low:    7170,
			Close:  7170,
			Volume: 30,
		},
	}

	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryExitLeverage")
	// inputs:
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	tradingApi.BuyDelay = 100
	tradingApi.SellDelay = 20100
	//sm := mongodb.StateMgmt{}
	sm := tests.NewMockedStateMgmt(tradingApi, df)
	logger, stats := tests.GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: stats,
		SettlementMutex: &redsync.Mutex{},
	}
	keyId := primitive.NewObjectID()
	smartOrder := smart_order.New(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})

	go smartOrder.Start()
	time.Sleep(7 * time.Second)
	// Check if we got in entry, and follow trailing maximum
	isInState, _ := smartOrder.State.IsInState(smart_order.InEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not InEntry (State: " + stateStr + ")")
	}
	// was 6952.09, changed to 6954.59
	expectedEntryPrice := 6954.59
	expectedTrailingExitPrice := 7170.0
	entryPrice := smartOrder.Strategy.GetModel().State.EntryPrice
	if entryPrice != expectedEntryPrice {
		t.Error("SmartOrder entryPrice != " + fmt.Sprintf("%f", entryPrice) + "")
	}

	if len(smartOrder.Strategy.GetModel().State.TrailingExitPrices) == 0 {
		t.Error("No trailing exit price")
		return
	}
	trailingExitPrice := smartOrder.Strategy.GetModel().State.TrailingExitPrices[0]
	if trailingExitPrice != expectedTrailingExitPrice {
		t.Error("SmartOrder trailingExitPrice " + fmt.Sprintf("%f", trailingExitPrice) + " != " + fmt.Sprintf("%f", expectedTrailingExitPrice) + "")
	}

	// check if trailing exit order was placed

	lastCreatedOrder := tradingApi.CreatedOrders.Back().Value.(models.MongoOrder)
	if lastCreatedOrder.Side != "sell" {
		t.Error("Last order is not sell")
	}

	// Check if update trailings and cancel previous orders
	fakeDataStream2 := []interfaces.OHLCV{
		{ // It goes up..
			Open:   7180,
			High:   7180,
			Low:    7180,
			Close:  7180,
			Volume: 30,
		},
		{ // It goes up..
			Open:   7270,
			High:   7270,
			Low:    7270,
			Close:  7270,
			Volume: 30,
		},
		{ // It goes up..
			Open:   7370,
			High:   7370,
			Low:    7370,
			Close:  7370,
			Volume: 30,
		},
	}

	df.AddToFeed(fakeDataStream2)
	time.Sleep(3 * time.Second)
	// test 2
	expectedTrailingExitPrice = 7370.0
	trailingExitPrice = smartOrder.Strategy.GetModel().State.TrailingExitPrices[0]
	if trailingExitPrice != expectedTrailingExitPrice {
		t.Error("SmartOrder trailingExitPrice " + fmt.Sprintf("%f", trailingExitPrice) + " != " + fmt.Sprintf("%f", expectedTrailingExitPrice) + "")
	}
}
