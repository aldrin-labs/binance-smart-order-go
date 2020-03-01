package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

func (sm *SmartOrder) checkTrailingHedgeLoss(ctx context.Context, args ...interface{}) bool {
	//isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(HedgeLoss)
	//if ok && isWaitingForOrder.(bool) {
	//	return false
	//}
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
			edgePrice = activatePrice * (1 - hedgeDeviation)
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


func (sm *SmartOrder) hedge() {
	if sm.Strategy.GetModel().Conditions.HedgeKeyId != nil {
		hedgedOrder := sm.ExchangeApi.PlaceHedge(sm.Strategy.GetModel())
		if hedgedOrder.Data.Id != "" {
			objId, _ := primitive.ObjectIDFromHex(hedgedOrder.Data.Id)
			sm.Strategy.GetModel().Conditions.HedgeStrategyId = &objId
			sm.StateMgmt.UpdateConditions(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().Conditions)
		}
	}
}

func (sm *SmartOrder) hedgeCallback(winStrategy *models.MongoStrategy) {
	if winStrategy.State.ExitPrice > 0 {
		err := sm.State.Fire(CheckHedgeLoss, *winStrategy)
		if err != nil {
			// println(err.Error())
		}
	}
}
func (sm *SmartOrder) checkLossHedge(ctx context.Context, args ...interface{}) bool {
	if args == nil {
		return false
	}
	strategy := args[0].(models.MongoStrategy)
	if strategy.State.ExitPrice > 0 {
		sm.Strategy.GetModel().State.HedgeExitPrice = strategy.State.ExitPrice
		sm.Strategy.GetModel().State.State = HedgeLoss
		sm.StateMgmt.UpdateHedgeExitPrice(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)

		return true
	}
	return false
}