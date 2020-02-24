package strategies

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
	if strategy.Model.Conditions.MarketType == 1 {
		go td.UpdateLeverage(keyId, strategy.Model.Conditions.Leverage, strategy.Model.Conditions.Pair)
	}
	runtime := smart_order.NewSmartOrder(strategy, df, td, keyId, strategy.StateMgmt)
	go runtime.Start()

	return runtime
}
