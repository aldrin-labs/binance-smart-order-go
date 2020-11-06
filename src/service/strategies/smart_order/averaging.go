package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"log"
	"time"
)

func (sm *SmartOrder) placeMultiEntryOrders(stopLoss bool) {
	// we execute this func again for 1 option
	log.Println("WaitForEntryIds cancel in placeMultiEntryOrders", sm.Strategy.GetModel().State.WaitForEntryIds)
	go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.WaitForEntryIds)

	model := sm.Strategy.GetModel()
	sm.SelectedEntryTarget = 0
	currentPrice := model.Conditions.EntryLevels[0].Price

	sumAmount := 0.0
	sumTotal := 0.0

	// here we should place all entry orders
	for _, target := range model.Conditions.EntryLevels {
		currentAmount := 0.0

		if target.Type == 0 {
			currentAmount = target.Amount
			currentPrice = target.Price
			sm.PlaceOrder(currentPrice, currentAmount, WaitForEntry)

		} else {
			currentAmount = model.Conditions.EntryOrder.Amount / 100 * target.Amount
			if model.Conditions.EntryOrder.Side == "buy" {
				currentPrice = currentPrice * (100 - target.Price/model.Conditions.Leverage) / 100
			} else {
				currentPrice = currentPrice * (100 + target.Price/model.Conditions.Leverage) / 100
			}
			sm.PlaceOrder(currentPrice, currentAmount, WaitForEntry)
		}

		sumAmount += currentAmount
		sumTotal += currentAmount * currentPrice
	}

	// TODO, for averaging without placeEntryAfterTAP
	// we should replace stop loss if it's simple avg without placeEntryAfterTAP
	// coz it may affect on existing position

	if stopLoss {
		sm.PlaceOrder(currentPrice, 0.0, Stoploss)
		if model.Conditions.ForcedLoss > 0 {
			sm.PlaceOrder(currentPrice, 0.0, "ForcedLoss")
		}
	}
}

func (sm *SmartOrder) entryMultiEntry(ctx context.Context, args ...interface{}) error {
	sm.StopMux.Lock()
	log.Print("entryMultiEntry")
	model := sm.Strategy.GetModel()

	isWaitingForcedLoss, forcedLossOk := sm.IsWaitingForOrder.Load("ForcedLoss")
	if model.Conditions.ForcedLoss > 0 && (!forcedLossOk || !isWaitingForcedLoss.(bool)) && len(model.State.ForcedLossOrderIds) == 0 {
		sm.IsWaitingForOrder.Store("ForcedLoss", true)
		time.AfterFunc(3 * time.Second, func() {sm.PlaceOrder(model.State.EntryPrice, 0.0, "ForcedLoss")})
	}

	// place BEP
	if model.Conditions.EntryLevels[sm.SelectedEntryTarget].PlaceWithoutLoss {
		sm.PlaceOrder(0, sm.getAveragingEntryAmount(model), "WithoutLoss")
	}

	// cancel old TAP
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	if ok && isWaitingForOrder.(bool) {
		state, _ := sm.State.State(ctx)
		if state == End {
			return nil
		}
	}

	sm.IsWaitingForOrder.Store(TakeProfit, true)
	ids := model.State.TakeProfitOrderIds[:]
	sm.TryCancelAllOrdersConsistently(ids)
	sm.PlaceOrder(0, 0.0, TakeProfit)

	sm.SelectedEntryTarget += 1
	sm.StopMux.Unlock()
	return nil
}

func (sm *SmartOrder) getAveragingEntryAmount(model *models.MongoStrategy) float64 {
	baseAmount := 0.0
	for i, target := range model.Conditions.EntryLevels {
		if i <= sm.SelectedEntryTarget {
			if target.Type == 0 {
				baseAmount += target.Amount
			} else {
				baseAmount += target.Amount * model.Conditions.EntryOrder.Amount / 100
			}
		}
	}
	return baseAmount
}
