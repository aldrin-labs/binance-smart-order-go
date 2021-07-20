package strategies

import (
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"time"
)

// RunSmartOrder starts a runtime for the strategy with given interfaces to market data and trading API.
func RunSmartOrder(strategy *Strategy, df interfaces.IDataFeed, td interfaces.ITrading, st interfaces.IStatsClient, keyId *primitive.ObjectID) interfaces.IStrategyRuntime {
	strategy.Log.Info("entry")
	if strategy.Model.Conditions.Leverage == 0 {
		strategy.Model.Conditions.Leverage = 1
	}
	if strategy.Model.Conditions.MarketType == 0 {
		strategy.Model.Conditions.Leverage = 1
	}
	if keyId == nil || strategy.Model.Conditions.EntryOrder.Type == 1 {
		keyAsset, err := strategy.StateMgmt.GetKeyAsset("core_key_assets", strategy.Model.Conditions.KeyAssetId)

		if err != nil {
			strategy.Log.Error("can't find a key asset",
				zap.String("key asset", fmt.Sprintf("%+v", keyAsset)),
				zap.String("cursor err", err.Error()),
			)
		}

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
	strategy.Log.Info("instantiate runtime")
	runtime := smart_order.New(strategy, df, td, st, keyId, strategy.StateMgmt)
	strategy.Log.Info("start runtime")
	go runtime.Start()

	return runtime
}

func DetermineRelativeEntryAmount(strategy *Strategy, keyAsset models.KeyAsset, df interfaces.IDataFeed) {
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
				currentOHLCVp := df.GetPriceForPairAtExchange(strategy.GetModel().Conditions.Pair, strategy.GetModel().Conditions.Exchange, strategy.GetModel().Conditions.MarketType)
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
