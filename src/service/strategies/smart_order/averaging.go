package smart_order

import (
	"context"
	"fmt"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.uber.org/zap"
	"time"
)

func (sm *SmartOrder) placeMultiEntryOrders(stopLoss bool) {
	// we execute this func again for 1 option
	sm.Strategy.GetLogger().Info("WaitForEntryIds cancel in placeMultiEntryOrders",
		zap.String("WaitForEntryIds", fmt.Sprintf("%v", sm.Strategy.GetModel().State.WaitForEntryIds)),
	)
	go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.WaitForEntryIds)

	model := sm.Strategy.GetModel()
	sm.SelectedEntryTarget = 0
	currentPrice := model.Conditions.EntryLevels[0].Price
	sumAmount := 0.0
	sumTotal := 0.0

	// here we should place all entry orders
	for i, target := range model.Conditions.EntryLevels {
		currentAmount := 0.0

		if target.Type == 0 {
			currentAmount = target.Amount
			currentPrice = target.Price
		} else {
			currentAmount = model.Conditions.EntryOrder.Amount / 100 * target.Amount
			if model.Conditions.EntryOrder.Side == "buy" {
				currentPrice = currentPrice * (100 - target.Price/model.Conditions.Leverage) / 100
			} else {
				currentPrice = currentPrice * (100 + target.Price/model.Conditions.Leverage) / 100
			}
		}

		if i == len(model.Conditions.EntryLevels) - 1 {
			currentAmount = model.Conditions.EntryOrder.Amount - sumAmount
		}
		currentAmount = sm.toFixed(currentAmount, sm.QuantityAmountPrecision, Floor)

		go sm.PlaceOrder(currentPrice, currentAmount, WaitForEntry)

		sumAmount += currentAmount
		sumTotal += currentAmount * currentPrice
	}

	if stopLoss {
		go sm.PlaceOrder(currentPrice, sumAmount, Stoploss)
		if model.Conditions.ForcedLoss > 0 {
			go sm.PlaceOrder(currentPrice, 0.0, "ForcedLoss")
		}
	}

	// TODO, for averaging without placeEntryAfterTAP
	// we should replace stop loss if it's simple avg without placeEntryAfterTAP
	// coz it may affect on existing position by amount > left from entry targets

}

// enterMultiEntry executes once multiEntryOrder got executed
func (sm *SmartOrder) enterMultiEntry(ctx context.Context, args ...interface{}) (stateless.State, error) {
	sm.StopMux.Lock()
	model := sm.Strategy.GetModel()

	// place forced loss, TODO: requires e2e tests
	isWaitingForcedLoss, forcedLossOk := sm.IsWaitingForOrder.Load("ForcedLoss")
	if model.Conditions.ForcedLoss > 0 && (!forcedLossOk || !isWaitingForcedLoss.(bool)) && len(model.State.ForcedLossOrderIds) == 0 {
		sm.IsWaitingForOrder.Store("ForcedLoss", true)
		time.AfterFunc(3*time.Second, func() { sm.PlaceOrder(model.State.EntryPrice, 0.0, "ForcedLoss") })
	}

	//TODO: HACK, state machine should never get here after all entryTargets fired \
	// we should look into this and fix it ASAP; until then averaging might not work as intended
	//if sm.SelectedEntryTarget >= len(model.Conditions.EntryLevels) {
	//	sm.Strategy.GetLogger().Error("SelectedEntry target not in model.Conditions.EntryLevels")
	//	return nil
	//}
	// place BEP
	if model.Conditions.EntryLevels[sm.SelectedEntryTarget].PlaceWithoutLoss {
		sm.PlaceOrder(0, sm.getAveragingEntryAmount(model, sm.SelectedEntryTarget), "WithoutLoss")
	}

	// cancel old TAP, TODO: we are not confident to keep it or remove, requires tests
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	if ok && isWaitingForOrder.(bool) {
		state, _ := sm.State.State(ctx)
		if state == End {
			return InMultiEntry, nil
		}
	}

	sm.IsWaitingForOrder.Store(TakeProfit, true)
	ids := model.State.TakeProfitOrderIds[:]
	sm.TryCancelAllOrdersConsistently(ids)
	sm.PlaceOrder(0, 0.0, TakeProfit)

	sm.SelectedEntryTarget += 1
	sm.StopMux.Unlock()
	return InMultiEntry, nil
}

func (sm *SmartOrder) getAveragingEntryAmount(model *models.MongoStrategy, executedTargets int) float64 {
	baseAmount := 0.0
	for i, target := range model.Conditions.EntryLevels {
		if i <= executedTargets {
			if target.Type == 0 {
				baseAmount += target.Amount
			} else {
				baseAmount += target.Amount * model.Conditions.EntryOrder.Amount / 100
			}
		}
	}
	return baseAmount
}

func (sm *SmartOrder) getLastTargetPrice(model *models.MongoStrategy) float64 {
	currentPrice := 0.0
	for i, target := range model.Conditions.EntryLevels {
		// executed target
		if i <= sm.SelectedEntryTarget {
			currentPrice = model.State.EntryPrice
		} else {
			// target that will be executed (placed already)
			if target.Type == 0 {
				currentPrice = target.Price
			} else {
				if model.Conditions.EntryOrder.Side == "buy" {
					currentPrice = currentPrice * (100 - target.Price/model.Conditions.Leverage) / 100
				} else {
					currentPrice = currentPrice * (100 + target.Price/model.Conditions.Leverage) / 100
				}
			}
		}
	}
	return currentPrice
}
