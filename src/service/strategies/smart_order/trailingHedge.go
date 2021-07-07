package smart_order

import (
	"context"
	"time"

	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (sm *SmartOrder) checkTrailingHedgeLoss(ctx context.Context, args ...interface{}) bool {
	//isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(HedgeLoss)
	//if ok && isWaitingForOrder.(bool) {
	//	return false
	//}
	if sm.Strategy.GetModel().Conditions.TakeProfitExternal {
		return false
	}
	currentOHLCV := args[0].(interfaces.OHLCV)

	side := sm.Strategy.GetModel().Conditions.EntryOrder.Side
	activatePrice := sm.Strategy.GetModel().State.HedgeExitPrice
	hedgeDeviation := sm.Strategy.GetModel().Conditions.HedgeLossDeviation
	edgePrice := sm.Strategy.GetModel().State.TrailingHedgeExitPrice
	switch side {
	case "buy":
		if edgePrice == 0 {
			edgePrice = activatePrice * (1 - hedgeDeviation)
		}
		if currentOHLCV.Close > edgePrice {
			sm.Strategy.GetModel().State.TrailingHedgeExitPrice = currentOHLCV.Close
			edgePrice = sm.Strategy.GetModel().State.TrailingHedgeExitPrice

			go sm.placeTrailingOrder(edgePrice, time.Now().UnixNano(), 0, side, false, HedgeLoss)
		}
		break
	case "sell":
		if edgePrice == 0 {
			edgePrice = activatePrice * (1 + hedgeDeviation)
		}
		if currentOHLCV.Close < edgePrice {
			sm.Strategy.GetModel().State.TrailingHedgeExitPrice = currentOHLCV.Close
			edgePrice = sm.Strategy.GetModel().State.TrailingHedgeExitPrice

			go sm.placeTrailingOrder(edgePrice, time.Now().UnixNano(), 0, side, false, HedgeLoss)
		}
		break
	}

	return false
}

func (sm *SmartOrder) waitForHedge() {
	_ = sm.StateMgmt.SubscribeToHedge(sm.Strategy.GetModel().Conditions.HedgeStrategyId, sm.hedgeCallback)
}

const waitForSeconds = 5

func (sm *SmartOrder) hedge() {
	if sm.Strategy.GetModel().Conditions.MarketType == 1 && sm.Strategy.GetModel().Conditions.Hedging {
		if !sm.Strategy.GetModel().Conditions.SkipInitialSetup {
			//TODO: look into WHY is it done like that
			sm.ExchangeApi.SetHedgeMode(sm.Strategy.GetModel().AccountId, true)
			time.Sleep(waitForSeconds * time.Second)
		}

		if (sm.Strategy.GetModel().Conditions.HedgeStrategyId == nil || sm.Strategy.GetModel().Conditions.ContinueIfEnded) && sm.Strategy.GetModel().Enabled {
			hedgedOrder := sm.ExchangeApi.PlaceHedge(sm.Strategy.GetModel())
			if hedgedOrder.Data.OrderId != "" {
				objId, _ := primitive.ObjectIDFromHex(hedgedOrder.Data.OrderId)
				sm.Strategy.GetModel().Conditions.HedgeStrategyId = &objId
				sm.StateMgmt.UpdateConditions(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().Conditions)
			}
		}
		return
	}
	if sm.Strategy.GetModel().Conditions.MarketType == 1 && !sm.Strategy.GetModel().Conditions.SkipInitialSetup {
		if sm.Strategy.GetModel().Conditions.HedgeMode {
			sm.ExchangeApi.SetHedgeMode(sm.Strategy.GetModel().AccountId, true)
			time.Sleep(waitForSeconds * time.Second)
			return
		}

		sm.ExchangeApi.SetHedgeMode(sm.Strategy.GetModel().AccountId, false)
		time.Sleep(waitForSeconds * time.Second)
	}
}

func (sm *SmartOrder) hedgeCallback(winStrategy *models.MongoStrategy) {
	if winStrategy.State != nil && winStrategy.State.ExitPrice > 0 {
		err := sm.State.Fire(CheckHedgeLoss, *winStrategy)
		if err != nil {
			// log.Print(err.Error())
		}
	}
}

func (sm *SmartOrder) enterWaitLossHedge(ctx context.Context, args ...interface{}) error {
	// go sm.shareProfits()
	return nil
}

func (sm *SmartOrder) checkLossHedge(ctx context.Context, args ...interface{}) bool {
	if args == nil {
		return false
	}
	strategy := args[0].(models.MongoStrategy)
	if strategy.State.ExitPrice > 0 {
		model := sm.Strategy.GetModel()
		if model.State.ExitPrice == 0 {
			sideCoefficient := 1.0
			fee := 0.04 * 4
			if strategy.Conditions.EntryOrder.Side == "sell" {
				sideCoefficient = -1.0
			}

			winStrategyProfitPercentage := ((strategy.State.ExitPrice/strategy.State.EntryPrice)*100 - 100) * strategy.Conditions.Leverage * sideCoefficient
			winStrategyProfitPercentage = winStrategyProfitPercentage - (fee * model.Conditions.Leverage)

			zeroProfitPrice := model.State.EntryPrice * (1 - winStrategyProfitPercentage/100/model.Conditions.Leverage)
			if model.Conditions.EntryOrder.Side == "sell" {
				zeroProfitPrice = model.State.EntryPrice * (1 + winStrategyProfitPercentage/100/model.Conditions.Leverage)
			}

			sm.StateMgmt.EnableHedgeLossStrategy(model.ID)
			sm.PlaceOrder(-1, 0.0, "WithoutLoss")
			sm.PlaceOrder(zeroProfitPrice, 0.0, "WithoutLoss")

			model.Conditions.TakeProfitExternal = false
			model.State.HedgeExitPrice = strategy.State.ExitPrice
			model.State.State = HedgeLoss
			sm.StateMgmt.UpdateHedgeExitPrice(model.ID, model.State)
		} else {
			model.State.State = End
			go sm.StateMgmt.UpdateState(model.ID, model.State)
		}
		return true
	}
	return false
}

func (sm *SmartOrder) shareProfits() {
	/*
		no sharing for now ;)
		state, _ := sm.State.State(context.Background())
		if state != HedgeLoss {
			model := sm.Strategy.GetModel()
			entryPrice := model.State.EntryPrice
			exitPrice := model.State.ExitPrice
			leverage := model.Conditions.Leverage
			amount := (model.State.ExecutedAmount * model.State.EntryPrice) / leverage
			biggerPrice := exitPrice
			smallerPrice := entryPrice
			if smallerPrice < biggerPrice {
				biggerPrice = entryPrice
				smallerPrice = exitPrice
			}
			profitRatio := (biggerPrice/smallerPrice-1)*leverage
			profitAmount := amount * profitRatio
			profitsToShare := (profitAmount - amount) / 2

			transfer := trading.TransferRequest{
				FromKeyId:  sm.KeyId,
				ToKeyId:    sm.Strategy.GetModel().Conditions.HedgeKeyId,
				Symbol:     "USDT",
				MarketType: 1,
				Amount:     profitsToShare,
			}
			sm.ExchangeApi.Transfer(transfer)
		}*/
}
