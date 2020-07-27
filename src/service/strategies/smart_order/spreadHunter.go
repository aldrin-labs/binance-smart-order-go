package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
)

func (sm *SmartOrder) checkSpreadCondition(spread interfaces.SpreadData) bool {
	fee := 0.0012
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

	if sm.checkSpreadCondition(currentSpread) {
		println("place waitForEntry bestBid, close", currentSpread.BestBid, currentSpread.Close)
		sm.PlaceOrder(currentSpread.BestBid, WaitForEntry)
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

	if sm.checkSpreadCondition(currentSpread) {
		sm.PlaceOrder(currentSpread.BestBid, TakeProfit)
	}

	return false
}
