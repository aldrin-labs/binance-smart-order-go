package testing

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

func GetTestSmartOrder(scenario string) models.MongoStrategy {
	smartOrder := models.MongoStrategy{
		ID:           primitive.ObjectID{},
		StrategyType: 1,
		Conditions: models.MongoStrategyCondition{
			ActivationPrice: 6900,
			TakeProfit:      3,
			StopLoss:        2,
			EntryDeviation:  1.5,
			Pair:            "BTC_USDT",
			EntryOrder:      trading.Order{Side: "buy"},
		},
		State:       models.MongoStrategyState{Amount: 1000},
		TriggerWhen: models.TriggerOptions{},
		Expiration:  models.ExpirationSchema{},
		OwnerId:     primitive.ObjectID{},
		Social:      models.MongoSocial{},
	}
	if scenario == "TestEntries" {
		smartOrder.Conditions.TakeProfit = 0
		smartOrder.Conditions.ExitLevels = []models.MongoEntryPoint{
			{
				ActivatePrice:           0,
				EntryDeviation:          0,
				Price:                   1,
				HedgeEntry:              0,
				HedgeActivation:         0,
				HedgeOppositeActivation: 0,
				Type:                    1,
			},
			{
				ActivatePrice:           0,
				EntryDeviation:          0,
				Price:                   3,
				HedgeEntry:              0,
				HedgeActivation:         0,
				HedgeOppositeActivation: 0,
				Type:                    1,
			},
			{
				ActivatePrice:           0,
				EntryDeviation:          0,
				Price:                   5,
				HedgeEntry:              0,
				HedgeActivation:         0,
				HedgeOppositeActivation: 0,
				Type:                    1,
			},
		}
	}
	return smartOrder
}

func TestSmartOrderGetInTrailingEntry(t *testing.T) {
	smartOrderModel := GetTestSmartOrder("TestEntry")
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
	}, { // Dont hit entry
		Open:   7005,
		High:   7005,
		Low:    6900,
		Close:  6900,
		Volume: 30,
	}}
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := *NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := mongodb.StateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm) //TODO
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.TrailingEntry)
	if !isInState {
		t.Error("SmartOrder state is not TrailingEntry")
	}
}

func TestSmartOrderGetInEntry(t *testing.T) {
	smartOrderModel := GetTestSmartOrder("TestEntry")
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
	sm := mongodb.StateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm) //TODO
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.InEntry)
	if !isInState {
		t.Error("SmartOrder state is not InEntry")
	}
}

func TestSmartOrderTakeProfit(t *testing.T) {
	smartOrderModel := GetTestSmartOrder("TestEntry")
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
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := mongodb.StateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm) //TODO
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(3000 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.End)
	if !isInState {
		t.Error("SmartOrder state is not InEntry")
	}
}

func TestSmartOrderTakeProfitAllTargets(t *testing.T) {
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
	}, { // Take profit 1 target
		Open:   7505,
		High:   7505,
		Low:    7500,
		Close:  7500,
		Volume: 30,
	}, { // Take profit 2 target
		Open:   7705,
		High:   7705,
		Low:    7700,
		Close:  7700,
		Volume: 30,
	}, { // Take profit 3 target
		Open:   7705,
		High:   7705,
		Low:    7700,
		Close:  7700,
		Volume: 30,
	}, { // Take profit 3 target
		Open:   9905,
		High:   9905,
		Low:    9900,
		Close:  9900,
		Volume: 30,
	}}
	smartOrderModel := GetTestSmartOrder("TestEntries")
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := mongodb.StateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm) //TODO
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(2 * time.Second)
	if tradingApi.CallCount["sell"] != 3 && tradingApi.AmountSum["binanceBTC_USDTsell"] != smartOrder.Strategy.Model.Conditions.EntryOrder.Amount {
		t.Error("SmartOrder didn't reach all 3 targets, but reached", tradingApi.CallCount["sell"], tradingApi.AmountSum["binanceBTC_USDTsell"], smartOrder.Strategy.Model.Conditions.EntryOrder.Amount)
	}
}

func TestSmartOrderStopLoss(t *testing.T) {
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
	}, { // Stop loss
		Open:   6205,
		High:   6205,
		Low:    6200,
		Close:  6200,
		Volume: 30,
	}}
	smartOrderModel := GetTestSmartOrder("TestEntries")
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := mongodb.StateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm) //TODO
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(2 * time.Second)

	if tradingApi.CallCount["sell"] != 1 && tradingApi.AmountSum["binanceBTC_USDTsell"] != smartOrder.Strategy.Model.Conditions.EntryOrder.Amount {
		t.Error("SmartOrder didn't sold everything on stop-loss", tradingApi.CallCount["sell"], tradingApi.AmountSum["binanceBTC_USDTsell"], smartOrder.Strategy.Model.Conditions.EntryOrder.Amount)
	}
}

func TestSmartOrderStopLossAfterTakeFirstProfit(t *testing.T) {
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
	}, { // Take profit SELL 300
		Open:   7405,
		High:   7405,
		Low:    7400,
		Close:  7400,
		Volume: 30,
	}, { // Stop loss SELL 700 ( rest )
		Open:   6005,
		High:   6005,
		Low:    6000,
		Close:  6000,
		Volume: 30,
	}}
	smartOrderModel := GetTestSmartOrder("TestEntries")
	df := NewMockedDataFeed(fakeDataStream)
	tradingApi := NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := mongodb.StateMgmt{}
	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm) //TODO
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(3 * time.Second)
	isInState, _ := smartOrder.State.IsInState(strategies.End)
	if !isInState {
		state, _ := smartOrder.State.State(context.TODO())
		t.Error("SmartOrder state is not End", state)
	}
}
