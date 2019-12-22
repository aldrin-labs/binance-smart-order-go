package testing

/*
	This file contains test cases for entry in smart order
	for normal and trailing smart orders
*/

import (
	"context"
	"fmt"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

// smart order should transition to InEntry state if currect OHLCV close price is less than condition price
func TestSmartOrderGetInEntryLong(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("entryLong")
	// price dips in the middle
	fakeDataStream := []strategies.OHLCV{{
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
	}}
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.InEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not InEntry(State: " + stateStr + ")")
	}
}

// smart order should wait for entry if price condition is not met
func TestSmartOrderShouldWaitForEntryLong(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("entryLong")
	// price is rising
	fakeDataStream := []strategies.OHLCV{{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  7005,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7200,
		Low:    7050,
		Close:  7100,
		Volume: 30,
	}, {
		Open:   7305,
		High:   7305,
		Low:    7300,
		Close:  7300,
		Volume: 30,
	}}
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.WaitForEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not WaitForEntry(State: " + stateStr + ")")
	}
}

// smart order should transition to InEntry state if currect OHLCV close price is more than condition price
func TestSmartOrderGetInEntryShort(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("entryShort")
	// price rises
	fakeDataStream := []strategies.OHLCV{{
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
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.InEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not InEntry(State: " + stateStr + ")")
	}
}

// smart order should wait for entry if price condition is not met
func TestSmartOrderShouldWaitForEntryShort(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("entryShort")
	// price is falling
	fakeDataStream := []strategies.OHLCV{{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  6900,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7200,
		Low:    7050,
		Close:  6800,
		Volume: 30,
	}, {
		Open:   7305,
		High:   7305,
		Low:    7300,
		Close:  6700,
		Volume: 30,
	}}
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.WaitForEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not WaitForEntry(State: " + stateStr + ")")
	}
}

// smart order should transition to TrailingEntry state if ActivatePrice > 0 AND currect OHLCV close price is less than condition price
func TestSmartOrderGetInTrailingEntryLong(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryLong")
	// price rises
	fakeDataStream := []strategies.OHLCV{{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  6900,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  7001,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  7100,
		Volume: 30,
	}}
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := *NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	//sm := mongodb.StateMgmt{}
	sm := MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.TrailingEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not TrailingEntry (State: " + stateStr + ")")
	}
}

// smart order should wait for entry if price condition is not met
/*func TestSmartOrderShouldWaitForTrailingEntryLong(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryLong")
	// price falls
	fakeDataStream := []strategies.OHLCV{{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  6900,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  6800,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  6700,
		Volume: 30,
	}}
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := *NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	//sm := mongodb.StateMgmt{}
	sm := MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.WaitForEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not WaitForEntry (State: " + stateStr + ")")
	}
}*/

// smart order should transition to TrailingEntry state if ActivatePrice > 0 AND currect OHLCV close price is more than condition price
func TestSmartOrderGetInTrailingEntryShort(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryShort")
	// price falls
	fakeDataStream := []strategies.OHLCV{{
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
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  6800,
		Volume: 30,
	}}
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := *NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.TrailingEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not TrailingEntry (State: " + stateStr + ")")
	}
}

// smart order should wait for entry if price condition is not met
/*func TestSmartOrderShouldWaitForTrailingEntryShort(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("trailingEntryShort")
	// price rises
	fakeDataStream := []strategies.OHLCV{{
		Open:   7100,
		High:   7101,
		Low:    7000,
		Close:  7105,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  7200,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  7300,
		Volume: 30,
	}}
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := *NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := MockStateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.WaitForEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not WaitForEntry (State: " + stateStr + ")")
	}
}*/
