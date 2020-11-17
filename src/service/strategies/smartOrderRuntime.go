package strategies

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	statsd_client "gitlab.com/crypto_project/core/strategy_service/src/statsd"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"time"
)

type KeyAsset struct {
	KeyId primitive.ObjectID `json:"keyId" bson:"keyId"`
	Free  float64            `json:"free" bson:"free"`
}

func RunSmartOrder(strategy *Strategy, df interfaces.IDataFeed, td trading.ITrading, st statsd_client.StatsdClient, keyId *primitive.ObjectID) interfaces.IStrategyRuntime {
	if strategy.Model.Conditions.Leverage == 0 {
		strategy.Model.Conditions.Leverage = 1
	}
	if strategy.Model.Conditions.MarketType == 0 {
		strategy.Model.Conditions.Leverage = 1
	}
	if keyId == nil || strategy.Model.Conditions.EntryOrder.Type == 1 {
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
		DetermineRelativeEntryAmount(strategy, keyAsset, df)
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
		strategy.Model.State = &models.MongoStrategyState{
			ReceivedProfitAmount: 0,
			ReceivedProfitPercentage: 0,
			State: "",
		}
	}

	strategy.StateMgmt.SaveStrategyConditions(strategy.Model)
	strategy.StateMgmt.UpdateStateAndConditions(strategy.Model.ID, strategy.Model)
	runtime := smart_order.NewSmartOrder(strategy, df, td, st, keyId, strategy.StateMgmt)
	go runtime.Start()

	return runtime
}

func DetermineRelativeEntryAmount(strategy *Strategy, keyAsset KeyAsset, df interfaces.IDataFeed) {
	if strategy.Model.Conditions.EntryOrder.Type == 1 {
		percentageOfBalance := strategy.Model.Conditions.EntryOrder.Amount
		margin := keyAsset.Free / 100 * percentageOfBalance
		attempts := 0
		for {
			if attempts > 10 {
				strategy.Model.State = &models.MongoStrategyState{
					State: smart_order.Error,
					Msg: "currentOHLCVp is nil. Please contact us in telegram",
				}
				break
			}

			if strategy.Model.Conditions.EntryOrder.OrderType == "limit" {
				strategy.Model.Conditions.EntryOrder.Amount = margin * strategy.Model.Conditions.Leverage / strategy.Model.Conditions.EntryOrder.Price
				break
			} else { // market and maker-only
				currentOHLCVp := df.GetPriceForPairAtExchange(strategy.GetModel().Conditions.Pair, "binance", strategy.GetModel().Conditions.MarketType)
				if currentOHLCVp != nil {
					strategy.Model.Conditions.EntryOrder.Amount = margin * strategy.Model.Conditions.Leverage / currentOHLCVp.Close
					break
				} else {
					attempts += 1
					time.Sleep(1 * time.Second)
					continue
				}
			}
		}
	}
}