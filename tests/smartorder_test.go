package testing

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

func GetTestSmartOrder(scenario string) models.MongoStrategy {
	smartOrder := models.MongoStrategy{
		Id:      primitive.ObjectID{},
		MonType: models.MongoStrategyType{},
		Condition: models.MongoStrategyCondition{
			ActivationPrice: 6900,
			TakeProfit:      3,
			StopLoss:        2,
			EntryDeviation:  1.5,
			Amount:          1000.0,
			Pair:            "BTC_USDT",
			Side:            "buy",
		},
		State:       models.MongoStrategyState{},
		TriggerWhen: models.TriggerOptions{},
		Expiration:  models.ExpirationSchema{},
		OwnerId:     primitive.ObjectID{},
		Social:      models.MongoSocial{},
	}
	if scenario == "TestEntries" {
		smartOrder.Condition.TakeProfit = 0
		smartOrder.Condition.ExitLeveles = []models.MongoEntryPoint{
			{
				ActivatePrice:           0,
				EntryDeviation:          0,
				Price:                   1,
				Amount:                  30,
				HedgeEntry:              0,
				HedgeActivation:         0,
				HedgeOppositeActivation: 0,
				Type:                    1,
			},
			{
				ActivatePrice:           0,
				EntryDeviation:          0,
				Price:                   3,
				Amount:                  50,
				HedgeEntry:              0,
				HedgeActivation:         0,
				HedgeOppositeActivation: 0,
				Type:                    1,
			},
			{
				ActivatePrice:           0,
				EntryDeviation:          0,
				Price:                   5,
				Amount:                  20,
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
	tradingApi := NewMockedTradingAPI()
	smartOrder := strategies.NewSmartOrder(&smartOrderModel, df, tradingApi)
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
	smartOrder := strategies.NewSmartOrder(&smartOrderModel, df, tradingApi)
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
	smartOrder := strategies.NewSmartOrder(&smartOrderModel, df, tradingApi)
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
	smartOrder := strategies.NewSmartOrder(&smartOrderModel, df, tradingApi)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(2 * time.Second)
	if tradingApi.CallCount["sell"] != 3 && tradingApi.AmountSum["binanceBTC_USDTsell"] != smartOrder.Model.Condition.Amount {
		t.Error("SmartOrder didn't reach all 3 targets, but reached", tradingApi.CallCount["sell"], tradingApi.AmountSum["binanceBTC_USDTsell"], smartOrder.Model.Condition.Amount)
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
	smartOrder := strategies.NewSmartOrder(&smartOrderModel, df, tradingApi)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()
	time.Sleep(2 * time.Second)

	if tradingApi.CallCount["sell"] != 1 && tradingApi.AmountSum["binanceBTC_USDTsell"] != smartOrder.Model.Condition.Amount {
		t.Error("SmartOrder didn't sold everything on stop-loss", tradingApi.CallCount["sell"], tradingApi.AmountSum["binanceBTC_USDTsell"], smartOrder.Model.Condition.Amount)
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
	smartOrder := strategies.NewSmartOrder(&smartOrderModel, df, tradingApi)
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
