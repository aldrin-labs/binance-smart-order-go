package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"time"
)

func (sm *SmartOrder) checkSpreadCondition(spread interfaces.SpreadData, orderType string) bool {
	// update this func for correct calculation
	fee := 0.01 * 2

	if orderType == "limit" {
		fee = 0.005 * 2
	}

	if spread.BestAsk - spread.BestBid > fee {
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


	if sm.checkSpreadCondition(currentSpread, model.Conditions.EntryOrder.OrderType) {
		if model.Conditions.EntryWaitingTime > 0 {
			time.Sleep(time.Millisecond * time.Duration(model.Conditions.EntryWaitingTime))
		}

		sm.PlaceOrder(currentSpread.Close, WaitForEntry)
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

	if sm.checkSpreadCondition(currentSpread, model.Conditions.ExitLevels[sm.SelectedExitTarget].OrderType) {
		if model.Conditions.EntryWaitingTime > 0 {
			time.Sleep(time.Millisecond * time.Duration(model.Conditions.TakeProfitWaitingTime))
		}

		sm.PlaceOrder(0, TakeProfit)
		return true
	}

	return false
}
