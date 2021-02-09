package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"go.uber.org/zap"
)

func (sm *SmartOrder) checkSpreadCondition(spread interfaces.SpreadData) bool {
	fee := 0.0012
	if (spread.BestAsk/spread.BestBid - 1) > fee {
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
		sm.Strategy.GetLogger().Info("place waitForEntry",
			zap.Float64("best bid", currentSpread.BestBid),
			zap.Float64("close", currentSpread.Close),
			zap.Float64("amount", sm.Strategy.GetModel().Conditions.EntryOrder.Amount),
		)
		sm.PlaceOrder(currentSpread.BestBid, 0.0, WaitForEntry)
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
		sm.PlaceOrder(currentSpread.BestBid, 0.0, TakeProfit)
	}

	return false
}
