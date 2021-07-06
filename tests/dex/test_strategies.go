package dex

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	//"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// returns conditions of smart order depending on the scenario
func GetTestSmartOrderStrategy(scenario string) models.MongoStrategy {
	smartOrder := models.MongoStrategy{
		// ID:          &primitive.ObjectID{0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x61, 0x62},
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
	case "simpleEntry":
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
	}

	return smartOrder
}
