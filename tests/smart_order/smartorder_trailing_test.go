package smart_order

import (
	"context"
	"fmt"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/tests"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)


func TestSmartOrderTrailingEntryAndThenActivateTrailingWithHighLeverage(t *testing.T) {
	fakeDataStream := []strategies.OHLCV{
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
		}}
	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryExitLeverage")
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	//sm := mongodb.StateMgmt{}
	sm := tests.MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(3 * time.Second)
	isInState, _ := smartOrder.State.IsInState(strategies.InEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not InEntry (State: " + stateStr + ")")
	}
}

func TestSmartOrderTrailingEntryAndTrailingExitWithHighLeverage(t *testing.T) {
	fakeDataStream := []strategies.OHLCV{
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
		}, { // Spiked down, ok, up trend is over, we are taking profits now
			Open:   6967.5,
			High:   6967.5,
			Low:    6967.5,
			Close:  6967.5,
			Volume: 30,
		}}
	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryExitLeverage")
	df := tests.NewMockedDataFeed(fakeDataStream)
	tradingApi := tests.NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	//sm := mongodb.StateMgmt{}
	sm := tests.MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(6 * time.Second)
	isInState, _ := smartOrder.State.IsInState(strategies.End)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not End (State: " + stateStr + ")")
	}
}