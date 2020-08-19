package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
)

func (sm *SmartOrder) waitForOrder(orderId string, orderStatus string) {
	//println("in wait for order")
	sm.StatusByOrderId.Store(orderId, orderStatus)
	_ = sm.StateMgmt.SubscribeToOrder(orderId, sm.orderCallback)
}
func (sm *SmartOrder) orderCallback(order *models.MongoOrder) {
	//println("order callback in")
	if order == nil || (order.OrderId == "" && order.PostOnlyInitialOrderId == "") {
		return
	}
	currentState, _ := sm.State.State(context.Background())
	model := sm.Strategy.GetModel()
	if !(order.Status == "filled" || order.Status == "canceled")  {
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

	println("before firing CheckExistingOrders")
	err := sm.State.Fire(CheckExistingOrders, *order)
	// when checkExisitingOrders wasn't called
	if (currentState == Stoploss || currentState == End || (currentState == InEntry && model.State.StopLossAt > 0)) && order.Status == "filled" {
		if order.Filled > 0 {
			model.State.ExecutedAmount += order.Filled
		}
		model.State.ExitPrice = order.Average
		calculateAndSavePNL(model, sm.StateMgmt)
		sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
	}
	if err != nil {
		println(err.Error())
	}
}
func (sm *SmartOrder) checkExistingOrders(ctx context.Context, args ...interface{}) bool {
	println("in checkExisitingFunc")
	if args == nil {
		return false
	}
	order := args[0].(models.MongoOrder)
	orderId := order.OrderId
	step, ok := sm.StatusByOrderId.Load(orderId)
	orderStatus := order.Status
	//println("step ok", step, ok, order.OrderId)
	if orderStatus == "filled" || orderStatus == "canceled" && (step == WaitForEntry || step == TrailingEntry) {
		sm.StatusByOrderId.Delete(orderId)
	}
	if !ok {
		return false
	}
	println("orderStatus, step", orderStatus, step.(string))
	model := sm.Strategy.GetModel()
	if order.Type == "post-only" {
		order = *sm.StateMgmt.GetOrder(order.PostOnlyFinalOrderId)
	}
	switch orderStatus {
	case "closed", "filled": // TODO i
		switch step {
		case HedgeLoss:
			model.State.ExecutedAmount += order.Filled
			model.State.ExitPrice = order.Average

			calculateAndSavePNL(model, sm.StateMgmt)
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
			println("model.State.EntryPrice in waitForEntry", model.State.EntryPrice)
			if model.State.EntryPrice > 0 {
				return false
			}
			println("waitoForEntry in waitOrder average", order.Average)
			model.State.EntryPrice = order.Average
			model.State.State = InEntry
			sm.StateMgmt.UpdateEntryPrice(model.ID, model.State)
			return true
		case TakeProfit:
			if order.Filled > 0 {
				model.State.ExecutedAmount += order.Filled
			}
			model.State.ExitPrice = order.Average
			amount := model.Conditions.EntryOrder.Amount
			if model.Conditions.MarketType == 0 {
				amount = amount * 0.99
			}
			println("model.State.ExecutedAmount >= amount", model.State.ExecutedAmount >= amount)
			if model.State.ExecutedAmount >= amount {
			} else {
				go sm.PlaceOrder(0, Stoploss)
			}

			calculateAndSavePNL(model, sm.StateMgmt)
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)

			if model.State.ExecutedAmount >= amount {
				isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || model.Conditions.Hedging == true

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
			amount := model.Conditions.EntryOrder.Amount
			if model.Conditions.MarketType == 0 {
				amount = amount * 0.99
			}
			calculateAndSavePNL(model, sm.StateMgmt)
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
			if model.State.ExecutedAmount >= amount {
				return true
			}
		case "WithoutLoss":
			if order.Filled > 0 {
				model.State.ExecutedAmount += order.Filled
			}
			model.State.ExitPrice = order.Average
			amount := model.Conditions.EntryOrder.Amount
			if model.Conditions.MarketType == 0 {
				amount = amount * 0.99
			}
			calculateAndSavePNL(model, sm.StateMgmt)
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)

			if model.State.ExecutedAmount >= amount {
				model.State.State = End
				sm.StateMgmt.UpdateState(model.ID, model.State)
				return true
			}
		case Canceled:
			if order.Filled > 0 {
				model.State.ExecutedAmount += order.Filled
			}
			model.State.ExitPrice = order.Average
			amount := model.Conditions.EntryOrder.Amount
			if model.Conditions.MarketType == 0 {
				amount = amount * 0.99
			}
			calculateAndSavePNL(model, sm.StateMgmt)
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
			if model.State.ExecutedAmount >= amount {
				model.State.State = End
				sm.StateMgmt.UpdateState(model.ID, model.State)
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

func calculateAndSavePNL(model *models.MongoStrategy, stateMgmt interfaces.IStateMgmt) float64 {

	leverage := model.Conditions.Leverage
	if leverage == 0 {
		leverage = 1.0
	}

	sideCoefficient := 1.0
	side := model.Conditions.EntryOrder.Side
	if side == "sell" {
		sideCoefficient = -1.0
	}

	profitPercentage := ((model.State.ExitPrice/model.State.EntryPrice)*100 - 100) * leverage * sideCoefficient

	profitAmount := (model.State.Amount / leverage) * model.State.EntryPrice * (profitPercentage / 100)

	if model.Conditions.CreatedByTemplate {
		go stateMgmt.SavePNL(model.Conditions.TemplateStrategyId, profitAmount)
	}

	return profitAmount
}
