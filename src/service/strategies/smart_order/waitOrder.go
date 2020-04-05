package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
)

func (sm *SmartOrder) waitForOrder(orderId string, orderStatus string) {
	sm.StatusByOrderId.Store(orderId, orderStatus)
	_ = sm.StateMgmt.SubscribeToOrder(orderId, sm.orderCallback)
}
func (sm *SmartOrder) orderCallback(order *models.MongoOrder) {
	if order == nil || order.OrderId == "" || !(order.Status == "filled" || order.Status == "canceled") {
		return
	}
	sm.OrdersMux.Lock()
	if _, ok := sm.OrdersMap[order.OrderId]; ok {
		delete(sm.OrdersMap, order.OrderId)
	} else {
		sm.OrdersMux.Unlock()
		return
	}
	sm.OrdersMux.Unlock()
	err := sm.State.Fire(CheckExistingOrders, *order)
	if err != nil {
		// println(err.Error())
	}
}
func (sm *SmartOrder) checkExistingOrders(ctx context.Context, args ...interface{}) bool {
	if args == nil {
		return false
	}
	order := args[0].(models.MongoOrder)
	orderId := order.OrderId
	step, ok := sm.StatusByOrderId.Load(orderId)
	if order.Status == "filled" || order.Status == "canceled" {
		sm.StatusByOrderId.Delete(orderId)
	}
	if !ok {
		return false
	}
	orderStatus := order.Status
	model := sm.Strategy.GetModel()
	switch orderStatus {
	case "closed", "filled": // TODO i
		switch step {
		case HedgeLoss:
			model.State.ExecutedAmount += order.Filled
			model.State.ExitPrice = order.Average
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
			if model.State.ExecutedAmount >= model.Conditions.EntryOrder.Amount {
				return true
			}
		case TrailingEntry:
			if model.State.EntryPrice > 0 {
				return false
			}
			model.State.EntryPrice = order.Average
			model.State.State = InEntry
			sm.StateMgmt.UpdateEntryPrice(model.ID, model.State)
			return true
		case WaitForEntry:
			if model.State.EntryPrice > 0 {
				return false
			}
			model.State.EntryPrice = order.Average
			model.State.State = InEntry
			sm.StateMgmt.UpdateEntryPrice(model.ID, model.State)
			return true
		case TakeProfit:
			if order.Filled > 0 {
				model.State.ExecutedAmount += order.Filled
			}
			model.State.ExitPrice = order.Average
			if model.State.ExecutedAmount >= model.Conditions.EntryOrder.Amount {
			} else {
				go sm.placeOrder(0, Stoploss)
			}
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
			if model.State.ExecutedAmount >= model.Conditions.EntryOrder.Amount {
				isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || model.Conditions.HedgeKeyId != nil

				if isTrailingHedgeOrder {
					model.State.State = WaitLossHedge
					sm.StateMgmt.UpdateState(model.ID, model.State)
				}
				return true
			}
		case Stoploss:
			if order.Filled > 0 {
				model.State.ExecutedAmount += order.Filled
			}
			model.State.ExitPrice = order.Average
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
			if model.State.ExecutedAmount >= model.Conditions.EntryOrder.Amount {
				println("go to end by stoploss checkExistingOrders")
				return true
			}
		}
		break
	//case "canceled":
	//	switch step {
	//	case WaitForEntry:
	//		model.State.State = Canceled
	//		return true
	//	case InEntry:
	//		model.State.State = Canceled
	//		return true
	//	}
	//	break
	}
	return false
}
