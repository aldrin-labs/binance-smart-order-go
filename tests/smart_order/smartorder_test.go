package smart_order

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	//"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// returns conditions of smart order depending on the scenario
func GetTestSmartOrderStrategy(scenario string) models.MongoStrategy {
	smartOrder := models.MongoStrategy{
		ID:          &primitive.ObjectID{},
		Conditions:  &models.MongoStrategyCondition{},
		State:       &models.MongoStrategyState{Amount: 1000},
		TriggerWhen: models.TriggerOptions{},
		Expiration:  models.ExpirationSchema{},
		OwnerId:     primitive.ObjectID{},
		Social:      models.MongoSocial{},
		Enabled:     true,
	}

	switch scenario {
	case "entryLong":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: &models.MongoEntryPoint{
				Side:      "buy",
				Price:     7000,
				Amount:    0.001,
				OrderType: "limit",
			},
			ExitLevels: []*models.MongoEntryPoint{
				{
					Type:      1,
					OrderType: "limit",
					Price:     10,
					Amount:    100,
				},
			},
		}
	case "entryShort":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: &models.MongoEntryPoint{
				Side:      "sell",
				Price:     7000,
				OrderType: "limit",
				Amount:    0.001,
			},
			ExitLevels: []*models.MongoEntryPoint{
				{
					Type:      1,
					OrderType: "limit",
					Price:     10,
					Amount:    100,
				},
			},
		}
	case "trailingEntryLong":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: &models.MongoEntryPoint{
				Side:           "buy",
				ActivatePrice:  7000,
				EntryDeviation: 1,
				OrderType:      "limit",
				Amount:         0.001,
			},
			ExitLevels: []*models.MongoEntryPoint{
				{
					Type:      1,
					OrderType: "limit",
					Price:     10,
					Amount:    100,
				},
			},
		}
	case "trailingEntryShort":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: &models.MongoEntryPoint{
				Side:           "sell",
				ActivatePrice:  7000,
				EntryDeviation: 1,
				OrderType:      "limit",
				Amount:         0.001,
			},
			ExitLevels: []*models.MongoEntryPoint{
				{
					Type:      1,
					OrderType: "limit",
					Price:     10,
					Amount:    100,
				},
			},
		}
	case "stopLossMarket":
		smartOrder.State = &models.MongoStrategyState{
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
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair:         "BTC_USDT",
			EntryOrder:   &models.MongoEntryPoint{Side: "buy", ActivatePrice: 7000, Amount: 0.05, OrderType: "limit", EntryDeviation: 1},
			StopLoss:     5,
			Leverage:     1,
			StopLossType: "market",
			ExitLevels: []*models.MongoEntryPoint{
				{
					Type:      1,
					OrderType: "market",
					Price:     10,
					Amount:    100,
				},
			},
		}
	case "stopLossMarketTimeout":
		smartOrder.State = &models.MongoStrategyState{
			State:      "InEntry",
			EntryPrice: 7000,
			Amount:     0.05,
		}
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair:            "BTC_USDT",
			EntryOrder:      &models.MongoEntryPoint{Side: "buy", OrderType: "limit", Price: 7000, Amount: 0.05},
			TimeoutWhenLoss: 5,
			//TimeoutLoss: 100,
			StopLoss:     5,
			StopLossType: "market",
			Leverage:     1,
			ExitLevels: []*models.MongoEntryPoint{
				{
					Type:      1,
					OrderType: "limit",
					Price:     10,
					Amount:    100,
				},
			},
		}
	case "stopLossMarketTimeoutLoss":
		smartOrder.State = &models.MongoStrategyState{
			State:      "InEntry",
			EntryPrice: 7000,
			Amount:     0.05,
		}
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair:            "BTC_USDT",
			EntryOrder:      &models.MongoEntryPoint{Side: "buy", OrderType: "limit", ActivatePrice: 7000, EntryDeviation: 1, Amount: 0.05},
			//TimeoutWhenLoss: 5,
			MarketType: 1,
			TimeoutLoss: 5,
			StopLoss:     2,
			StopLossType: "limit",
			Leverage:     1,
			ExitLevels: []*models.MongoEntryPoint{
				{
					Type:      1,
					OrderType: "limit",
					Price:     10,
					Amount:    100,
				},
			},
		}
	case "TakeProfitMarket":
		smartOrder.State = &models.MongoStrategyState{
			State:      "InEntry",
			EntryPrice: 7000,
			Amount:     0.05,
		}
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair:       "BTC_USDT",
			EntryOrder: &models.MongoEntryPoint{Side: "buy", OrderType: "limit", Price: 6999, Amount: 0.05},
			ExitLevels: []*models.MongoEntryPoint{
				{Price: 7050, Amount: 0.05, Type: 0, OrderType: "market"},
			},
			Leverage: 1,
		}
	case "takeProfit":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair:         "BTC_USDT",
			Leverage:     100,
			StopLossType: "market",
			StopLoss:     5,
			EntryOrder: &models.MongoEntryPoint{
				Side:           "buy",
				ActivatePrice:  6950,
				Amount:         0.05,
				EntryDeviation: 3,
				OrderType:      "market",
			},
			ExitLevels: []*models.MongoEntryPoint{{
				OrderType: "market",
				Type:      1,
				Price:     5,
				Amount:    100,
			}},
		}
	case "trailingEntryExitLeverage":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair:         "BTC_USDT",
			MarketType:   1,
			Leverage:     100,
			StopLossType: "market",
			StopLoss:     10,
			EntryOrder: &models.MongoEntryPoint{
				Side:           "buy",
				ActivatePrice:  6950,
				Amount:         0.05,
				EntryDeviation: 3,
				OrderType:      "market",
			},
			ExitLevels: []*models.MongoEntryPoint{{
				OrderType:      "market",
				Type:           1,
				ActivatePrice:  5,
				EntryDeviation: 3,
				Amount:         100,
			}},
		}
	case "marketEntryTrailingExitLeverage":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair:         "BTC_USDT",
			Leverage:     100,
			StopLoss:     10,
			StopLossType: "limit",
			EntryOrder: &models.MongoEntryPoint{
				Side:      "buy",
				Amount:    0.05,
				OrderType: "market",
			},
			ExitLevels: []*models.MongoEntryPoint{{
				OrderType:      "limit",
				Type:           1,
				ActivatePrice:  15,
				EntryDeviation: 10,
				Amount:         100,
			}},
		}
	case "stopLossMultiTargets":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair:       "BTC_USDT",
			MarketType: 1,
			EntryOrder: &models.MongoEntryPoint{Side: "buy", Price: 6999, Amount: 0.15, OrderType: "market"},
			ExitLevels: []*models.MongoEntryPoint{
				{Price: 10, Amount: 50, Type: 1, OrderType: "limit"},
				{Price: 15, Amount: 25, Type: 1, OrderType: "limit"},
				{Price: 20, Amount: 25, Type: 1, OrderType: "limit"},
			},
			Leverage:     20,
			StopLossType: "limit",
			StopLoss:     10,
		}
	case "multiplePriceTargets":
		smartOrder.Conditions = &models.MongoStrategyCondition{
			Pair: "BTC_USDT",
			EntryOrder: &models.MongoEntryPoint{
				Side:      "buy",
				Price:     7000,
				Amount:    0.01,
				OrderType: "limit",
			},
			ExitLevels: []*models.MongoEntryPoint{
				{Price: 10, Amount: 50, Type: 1, OrderType: "limit"},
				{Price: 15, Amount: 25, Type: 1, OrderType: "limit"},
				{Price: 20, Amount: 25, Type: 1, OrderType: "limit"},
			},
		}
	}

	return smartOrder
}
