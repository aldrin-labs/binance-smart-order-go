package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"time"
)

func (sm *SmartOrder) checkSpreadCondition(spread interfaces.SpreadData, orderType string, isEntry bool) bool {
	fee := 0.001 * 2
	// price := spread.BestBid
	// model := sm.Strategy.GetModel()
	// amount := model.Conditions.EntryOrder.Amount

	//if orderType == "limit" {
	//	if isEntry {
	//		price = model.Conditions.EntryOrder.Price
	//	} else {
	//		price = model.Conditions.ExitLevels[sm.SelectedExitTarget].Price
	//	}
	//}
	if (spread.BestAsk / spread.BestBid - 1) > fee {
		return true
	}

	return false
}

func (sm *SmartOrder) checkSpreadEntry(ctx context.Context, args ...interface{}) bool {
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(WaitForEntry)
	if ok && isWaitingForOrder.(bool) {
		return false
	}
	if !sm.Strategy.GetModel().Conditions.EntrySpreadHunter {
		return false
	}
	currentSpread := args[0].(interfaces.SpreadData)
	model := sm.Strategy.GetModel()


	if sm.checkSpreadCondition(currentSpread, model.Conditions.EntryOrder.OrderType, true) {
		if model.Conditions.EntryWaitingTime > 0 {
			time.Sleep(time.Millisecond * time.Duration(model.Conditions.EntryWaitingTime))
		}

		sm.PlaceOrder(currentSpread.BestBid, WaitForEntry)
		return true
	}

	return false
}

func (sm *SmartOrder) checkSpreadTakeProfit(ctx context.Context, args ...interface{}) bool {
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	if ok && isWaitingForOrder.(bool) {
		return false
	}
	if !sm.Strategy.GetModel().Conditions.TakeProfitSpreadHunter {
		return false
	}
	currentSpread := args[0].(interfaces.SpreadData)
	model := sm.Strategy.GetModel()

	if sm.checkSpreadCondition(currentSpread, model.Conditions.ExitLevels[sm.SelectedExitTarget].OrderType, false) {
		if model.Conditions.TakeProfitWaitingTime > 0 {
			time.Sleep(time.Millisecond * time.Duration(model.Conditions.TakeProfitWaitingTime))
		}

		sm.PlaceOrder(currentSpread.BestBid, TakeProfit)
		return true
	}

	return false
}
