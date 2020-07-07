package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
)

func (sm *SmartOrder) checkSpreadCondition(spread interfaces.SpreadData, orderType string, isEntry bool) bool {
	fee := 0.001 * 2
	price := spread.BestBid
	model := sm.Strategy.GetModel()
	amount := model.Conditions.EntryOrder.Amount

	//println("spread.BestAsk - spread.BestBid", spread.BestAsk - spread.BestBid)
	//println("fee * price * amount", fee * price * amount)

	if spread.BestAsk - spread.BestBid > fee * price * amount {
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
		sm.PlaceOrder(currentSpread.BestBid, TakeProfit)
		return true
	}

	return false
}
