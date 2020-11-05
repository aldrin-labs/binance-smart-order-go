package strategies

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
)

type KeyAsset struct {
	KeyId primitive.ObjectID `json:"keyId" bson:"keyId"`
	Free  float64            `json:"free" bson:"free"`
}

func RunSmartOrder(strategy *Strategy, df interfaces.IDataFeed, td trading.ITrading, keyId *primitive.ObjectID) interfaces.IStrategyRuntime {
	if strategy.Model.Conditions.Leverage == 0 {
		strategy.Model.Conditions.Leverage = 1
	}
	if strategy.Model.Conditions.MarketType == 0 {
		strategy.Model.Conditions.Leverage = 1
	}
	if keyId == nil {
		KeyAssets := mongodb.GetCollection("core_key_assets") // TODO: move to statemgmt, avoid any direct dependecies here
		keyAssetId := strategy.Model.Conditions.KeyAssetId.String()
		var request bson.D
		request = bson.D{
			{"_id", strategy.Model.Conditions.KeyAssetId},
		}
		log.Print(keyAssetId)
		ctx := context.Background()
		var keyAsset KeyAsset
		err := KeyAssets.FindOne(ctx, request).Decode(&keyAsset)
		if err != nil {
			log.Print("keyAssetsCursor ", err.Error())
		}
		keyId = &keyAsset.KeyId

		// type 1 for entry point - relative amount
		log.Println("strategy.Model.Conditions.EntryOrder.Type", strategy.Model.Conditions.EntryOrder.Type)
		if strategy.Model.Conditions.EntryOrder.Type == 1 {
			percentageOfBalance := strategy.Model.Conditions.EntryOrder.Amount
			log.Println("percentageOfBalance", percentageOfBalance)
			strategy.Model.Conditions.EntryOrder.Amount = keyAsset.Free / 100 * percentageOfBalance
			log.Println("new strategy.Model.Conditions.EntryOrder.Amount", keyAsset.Free / 100 * percentageOfBalance)
		}
	}

	if strategy.Model.Conditions.MarketType == 1 && !strategy.Model.Conditions.SkipInitialSetup {
		res := td.UpdateLeverage(keyId, strategy.Model.Conditions.Leverage, strategy.Model.Conditions.Pair)
		if res.Status != "OK" {
			strategy.Model.State = &models.MongoStrategyState{
				State: smart_order.Error,
				Msg: res.ErrorMessage,
			}
		}
	}
	if strategy.Model.State == nil {
		strategy.Model.State = &models.MongoStrategyState{}
	}
	strategy.StateMgmt.SaveStrategyConditions(strategy.Model)
	strategy.StateMgmt.UpdateConditions(strategy.Model.ID, strategy.Model.Conditions)
	runtime := smart_order.NewSmartOrder(strategy, df, td, keyId, strategy.StateMgmt)
	go runtime.Start()

	return runtime
}
