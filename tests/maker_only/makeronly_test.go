package maker_only

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetTestMakerOnlyOrderStrategy(scenario string) models.MongoStrategy {
	makerOnly := models.MongoStrategy{
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
		makerOnly.Conditions.Pair
	}
}
