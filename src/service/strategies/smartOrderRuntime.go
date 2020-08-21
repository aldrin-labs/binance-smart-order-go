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
)
type MongoKey struct {
	Hostname string `json:"hostname" bson:"hostname"`
}

type KeyAsset struct {
	KeyId primitive.ObjectID `json:"keyId" bson:"keyId"`
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
		println(keyAssetId)
		ctx := context.Background()
		var keyAsset KeyAsset
		err := KeyAssets.FindOne(ctx, request).Decode(&keyAsset)
		if err != nil {
			println("keyAssetsCursor", err.Error())
		}
		keyId = &keyAsset.KeyId
	}

	Key := mongodb.GetCollection("core_keys")
	var request bson.D
	request = bson.D{
		{"_id", keyId},
	}
	ctx := context.Background()
	var userKey MongoKey
	err := Key.FindOne(ctx, request).Decode(&userKey)
	if err != nil {
		println("keyAssetsCursor", err.Error())
	}
	hostname := userKey.Hostname


	if strategy.Model.Conditions.MarketType == 1 && !strategy.Model.Conditions.SkipInitialSetup {
		go td.UpdateLeverage(keyId, strategy.Model.Conditions.Leverage, strategy.Model.Conditions.Pair, hostname)
	}
	if strategy.Model.State == nil {
		strategy.Model.State = &models.MongoStrategyState{}
	}
	strategy.StateMgmt.SaveStrategyConditions(strategy.Model)
	strategy.StateMgmt.UpdateConditions(strategy.Model.ID, strategy.Model.Conditions)
	runtime := smart_order.NewSmartOrder(strategy, df, td, keyId, strategy.StateMgmt, hostname)
	go runtime.Start()

	return runtime
}
