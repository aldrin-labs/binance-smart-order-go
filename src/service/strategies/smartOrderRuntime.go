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
	"go.uber.org/zap"
	"fmt"
	"time"
)

type KeyAsset struct {
	KeyId primitive.ObjectID `json:"keyId" bson:"keyId"`
	Free  float64            `json:"free" bson:"free"`
}

// RunSmartOrder starts a runtime for the strategy with given interfaces to market data and trading API.
func RunSmartOrder(strategy *Strategy, df interfaces.IDataFeed, td trading.ITrading, st *statsd_client.StatsdClient, keyId *primitive.ObjectID) interfaces.IStrategyRuntime {
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
		strategy.Log.Info("reading key asset document",
			zap.String("keyAssetId", keyAssetId),
		)
		ctx := context.Background()
		var keyAsset KeyAsset
		err := KeyAssets.FindOne(ctx, request).Decode(&keyAsset)
		if err != nil {
			strategy.Log.Error("can't find a key asset",
				zap.String("key asset", fmt.Sprintf("%+v", keyAsset)),
				zap.String("cursor err", err.Error()),
			)
		}
		keyId = &keyAsset.KeyId

		// type 1 for entry point - relative amount
		DetermineRelativeEntryAmount(strategy, keyAsset, df) // TODO(khassanov): call for relative only
	}

	if strategy.Model.Conditions.MarketType == 1 && !strategy.Model.Conditions.SkipInitialSetup {
		res := td.UpdateLeverage(keyId, strategy.Model.Conditions.Leverage, strategy.Model.Conditions.Pair)
		if res.Status != "OK" {
			strategy.Model.State = &models.MongoStrategyState{
				State: smart_order.Error,
				Msg:   res.ErrorMessage,
			}
			strategy.Log.Error("can't update leverage",
				zap.String("trading interface response", res.ErrorMessage),
			)
		}
	}
	if strategy.Model.State == nil {
		strategy.Model.State = &models.MongoStrategyState{
			ReceivedProfitAmount:     0, // TODO(khassanov): remove obvious defaults?
			ReceivedProfitPercentage: 0,
			State:                    "",
		}
	}

	strategy.StateMgmt.SaveStrategyConditions(strategy.Model) // TODO(khassanov): rename this and the following
	strategy.StateMgmt.UpdateStateAndConditions(strategy.Model.ID, strategy.Model)
	runtime := smart_order.NewSmartOrder(strategy, df, td, st, keyId, strategy.StateMgmt)
	strategy.Log.Info("runtime starts")
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
					Msg:   "currentOHLCVp is nil. Please contact us in telegram",
				}
				strategy.Log.Warn("can't calc relative entry",
					zap.Int("attempts", attempts),
				)
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
		strategy.Log.Info("relative amount calculated",
			zap.Float64("amount", strategy.Model.Conditions.EntryOrder.Amount),
		)
	}
}
