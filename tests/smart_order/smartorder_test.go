package smart_order

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	//"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

//Conditions: models.MongoStrategyCondition{
//Price: 8010,
//ActivationPrice: 6900,
//TakeProfit:      3,
//StopLoss:        2,
//EntryDeviation:  1.5,
//Pair:       "BTC_USDT",
//EntryOrder: models.MongoEntryPoint{Side: "buy", Price: 7000},
//},

// returns conditions of smart order depending on the scenario
func GetTestSmartOrderStrategy(scenario string) models.MongoStrategy {
	smartOrder := models.MongoStrategy{
		ID:           primitive.ObjectID{},
		Conditions:   models.MongoStrategyCondition{},
		State:        models.MongoStrategyState{Amount: 1000},
		TriggerWhen:  models.TriggerOptions{},
		Expiration:   models.ExpirationSchema{},
		OwnerId:      primitive.ObjectID{},
		Social:       models.MongoSocial{},
		Enabled:      true,
	}

	switch scenario {
	case "entryLong":
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: models.MongoEntryPoint{Side: "buy", Price: 7000},
		}
	case "entryShort":
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: models.MongoEntryPoint{Side: "sell", Price: 7000},
		}
	case "trailingEntryLong":
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: models.MongoEntryPoint{Side: "buy", ActivatePrice: 7000},
		}
	case "trailingEntryShort":
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: models.MongoEntryPoint{Side: "sell", ActivatePrice: 7000},
		}
	case "stopLossMarket":
		smartOrder.State = models.MongoStrategyState{
			State:              "InEntry",
			EntryOrderId:       "",
			TakeProfitOrderIds: "",
			StopLossOrderIds:   "",
			StopLoss:           "",
			TrailingEntryPrice: 0,
			TrailingExitPrices: nil,
			EntryPrice:         7000,
			ExitPrice:          0,
			Amount:             0.05,
			Orders:             nil,
			ExecutedOrders:     nil,
			ExecutedAmount:     0,
			ReachedTargetCount: 0,
			StopLossAt:         0,
			LossableAt:         0,
			ProfitableAt:       0,
			ProfitAt:           0,
		}
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: models.MongoEntryPoint{Side: "buy", ActivatePrice: 7000, Amount: 0.05},
			StopLoss: 5,
			Leverage: 1,
		}
	case "stopLossMarketTimeout":
		smartOrder.State = models.MongoStrategyState{
			State:              "InEntry",
			EntryPrice:         7000,
			Amount:             0.05,
		}
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: models.MongoEntryPoint{Side: "buy", ActivatePrice: 7000, Price: 6999, Amount: 0.05},
			TimeoutWhenLoss: 5,
			//TimeoutLoss: 100,
			StopLoss: 5,
			Leverage: 1,
		}
	case "TakeProfitMarket":
		smartOrder.State = models.MongoStrategyState{
			State:              "InEntry",
			EntryPrice:         7000,
			Amount:             0.05,
		}
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: models.MongoEntryPoint{Side: "buy", Price: 6999, Amount: 0.05},
			ExitLevels: []models.MongoEntryPoint{
				{Price: 7050, Amount: 0.05, Type: 0, OrderType: "market"},
			},
			Leverage: 1,
		}
	case "takeProfit":
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			Leverage: 100,
			EntryOrder: models.MongoEntryPoint{
				Side: "buy",
				ActivatePrice: 6950,
				Amount: 0.05,
				EntryDeviation: 3,
				OrderType: "market",
			},
			ExitLevels: []models.MongoEntryPoint{{
				OrderType: "market",
				Type: 1,
				Price: 5,
				Amount: 100,
			}},
		}
	case "trailingEntryExitLeverage":
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			MarketType: 1,
			Leverage: 100,
			EntryOrder: models.MongoEntryPoint{
				Side: "buy",
				ActivatePrice: 6950,
				Amount: 0.05,
				EntryDeviation: 3,
				OrderType: "market",
			},
			ExitLevels: []models.MongoEntryPoint{{
				OrderType: "market",
				Type: 1,
				ActivatePrice: 5,
				EntryDeviation: 3,
				Amount: 100,
			}},
		}
	case "marketEntryTrailingExitLeverage":
		smartOrder.Conditions = models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			Leverage: 100,
			EntryOrder: models.MongoEntryPoint{
				Side: "buy",
				Amount: 0.05,
				OrderType: "market",
			},
			ExitLevels: []models.MongoEntryPoint{{
				OrderType: "limit",
				Type: 1,
				ActivatePrice: 15,
				EntryDeviation: 10,
				Amount: 100,
			}},
		}
	//case "multiplePriceTargets":
	//	smartOrder.Conditions = models.MongoStrategyCondition{
	//		Pair: "BTC_USDT",
	//		EntryOrder: models.MongoEntryPoint{Side: "buy", Price: 7000, Amount: 0.05},
	//	}
	//	smartOrder.Conditions.TakeProfit = 0
	//	smartOrder.Conditions.ExitLevels = []models.MongoEntryPoint{
	//		{
	//			ActivatePrice:           0,
	//			EntryDeviation:          0,
	//			Price:                   1,
	//			HedgeEntry:              0,
	//			HedgeActivation:         0,
	//			HedgeOppositeActivation: 0,
	//			Type:                    1,
	//		},
	//		{
	//			ActivatePrice:           0,
	//			EntryDeviation:          0,
	//			Price:                   3,
	//			HedgeEntry:              0,
	//			HedgeActivation:         0,
	//			HedgeOppositeActivation: 0,
	//			Type:                    1,
	//		},
	//		{
	//			ActivatePrice:           0,
	//			EntryDeviation:          0,
	//			Price:                   5,
	//			HedgeEntry:              0,
	//			HedgeActivation:         0,
	//			HedgeOppositeActivation: 0,
	//			Type:                    1,
	//		},
	//	}
	}

	return smartOrder
}

//func TestSmartOrderStopLoss(t *testing.T) {
//	fakeDataStream := []strategies.OHLCV{{
//		Open:   7100,
//		High:   7101,
//		Low:    7000,
//		Close:  7005,
//		Volume: 30,
//	}, { // Activation price
//		Open:   7005,
//		High:   7005,
//		Low:    6900,
//		Close:  6900,
//		Volume: 30,
//	}, { // Hit entry
//		Open:   7305,
//		High:   7305,
//		Low:    7300,
//		Close:  7300,
//		Volume: 30,
//	}, { // Stop loss
//		Open:   6205,
//		High:   6205,
//		Low:    6200,
//		Close:  6200,
//		Volume: 30,
//	}}
//	smartOrderModel := GetTestSmartOrderStrategy("TestEntries")
//	df := NewMockedDataFeed(fakeDataStream)
//	tradingApi := NewMockedTradingAPI()
//	strategy := strategies.Strategy{
//		Model: &smartOrderModel,
//	}
//	keyId := primitive.NewObjectID()
//	sm := mongodb.StateMgmt{}
//	smartOrder := strategies.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm) //TODO
//	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
//		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
//	})
//	go smartOrder.Start()
//	time.Sleep(2 * time.Second)
//
//	if tradingApi.CallCount["sell"] != 1 && tradingApi.AmountSum["binanceBTC_USDTsell"] != smartOrder.Strategy.Model.Conditions.EntryOrder.Amount {
//		t.Error("SmartOrder didn't sold everything on stop-loss", tradingApi.CallCount["sell"], tradingApi.AmountSum["binanceBTC_USDTsell"], smartOrder.Strategy.Model.Conditions.EntryOrder.Amount)
//	}
//}

/*func TestSmartOrderStopLossAfterTakeFirstProfit(t *testing.T) {
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
	smartOrderModel := GetTestSmartOrderStrategy("TestEntries")
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
}*/
