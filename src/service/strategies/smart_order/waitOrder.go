package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"log"
)

func (sm *SmartOrder) waitForOrder(orderId string, orderStatus string) {
	//log.Print("in wait for order")
	sm.StatusByOrderId.Store(orderId, orderStatus)
	_ = sm.StateMgmt.SubscribeToOrder(orderId, sm.orderCallback)
}
func (sm *SmartOrder) orderCallback(order *models.MongoOrder) {
	//log.Print("order callback in")
	if order == nil || (order.OrderId == "" && order.PostOnlyInitialOrderId == "") {
		return
	}
	//currentState, _ := sm.State.State(context.Background())
	//model := sm.Strategy.GetModel()
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

	log.Print("before firing CheckExistingOrders")
	err := sm.State.Fire(CheckExistingOrders, *order)
	// when checkExisitingOrders wasn't called
	//if (currentState == Stoploss || currentState == End || (currentState == InEntry && model.State.StopLossAt > 0)) && order.Status == "filled" {
	//	if order.Filled > 0 {
	//		model.State.ExecutedAmount += order.Filled
	//	}
	//	model.State.ExitPrice = order.Average
	//	calculateAndSavePNL(model, sm.StateMgmt, step)
	//	sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
	//}
	if err != nil {
		log.Print(err.Error())
	}
}
func (sm *SmartOrder) checkExistingOrders(ctx context.Context, args ...interface{}) bool {
	log.Print("in checkExisitingFunc")
	if args == nil {
		return false
	}
	order := args[0].(models.MongoOrder)
	orderId := order.OrderId
	step, ok := sm.StatusByOrderId.Load(orderId)
	orderStatus := order.Status
	//log.Print("step ok", step, ok, order.OrderId)
	if orderStatus == "filled" || orderStatus == "canceled" && (step == WaitForEntry || step == TrailingEntry) {
		sm.StatusByOrderId.Delete(orderId)
	}
	if !ok {
		return false
	}
	log.Print("orderStatus, ", orderStatus, " step ", step.(string))
	model := sm.Strategy.GetModel()
	isMultiEntry := len(model.Conditions.EntryLevels) > 0

	if order.Type == "post-only" {
		order = *sm.StateMgmt.GetOrder(order.PostOnlyFinalOrderId)
	}

	switch orderStatus {
	case "closed", "filled": // TODO i
		switch step {
		case HedgeLoss:
			model.State.ExecutedAmount += order.Filled
			model.State.ExitPrice = order.Average

			sm.calculateAndSavePNL(model, step, order.Filled)
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
			model.State.PositionAmount += order.Filled
			sm.StateMgmt.UpdateEntryPrice(model.ID, model.State)
			return true
		case WaitForEntry:
			log.Print("model.State.EntryPrice in waitForEntry ", model.State.EntryPrice)
			if model.State.EntryPrice > 0 && !isMultiEntry {
				return false
			}
			log.Print("waitForEntry in waitOrder average ", order.Average)

			if isMultiEntry {
				model.State.State = InMultiEntry
				// calc average weight
				if model.State.EntryPrice > 0 {
					log.Println("model.State.EntryPrice before: ", model.State.EntryPrice, " order.Filled: ", order.Filled)
					total := model.State.EntryPrice * model.State.PositionAmount + order.Average * order.Filled
					model.State.EntryPrice = total / (model.State.PositionAmount + order.Filled)
					log.Println("model.State.EntryPrice after: ", model.State.EntryPrice)
				} else {
					model.State.EntryPrice = order.Average
				}
			} else { // not avg
				model.State.EntryPrice = order.Average
				model.State.State = InEntry
			}

			model.State.PositionAmount += order.Filled

			sm.StateMgmt.UpdateEntryPrice(model.ID, model.State)
			return true
		case TakeProfit:
			sm.IsWaitingForOrder.Store(TakeProfit, false)
			amount := model.Conditions.EntryOrder.Amount

			model.State.ExitPrice = order.Average
			if order.Filled > 0 {
				model.State.ExecutedAmount += order.Filled
			}
			if model.Conditions.MarketType == 0 {
				amount = amount * 0.99
			}
			log.Print("model.State.ExecutedAmount >= amount ", model.State.ExecutedAmount >= amount, "model.Conditions.PlaceEntryAfterTAP ", model.Conditions.PlaceEntryAfterTAP)
			// here we gonna close SM if CloseStrategyAfterFirstTAP enabled or we executed all entry && TAP orders
			if model.State.ExecutedAmount >= amount || model.Conditions.CloseStrategyAfterFirstTAP {
				model.State.State = End
			} else if model.Conditions.PlaceEntryAfterTAP {
				// place entry orders again
				// set it to 0, place only entry orders
				model.State.ExecutedAmount = 0
				sm.placeMultiEntryOrders(false)
			}

			sm.calculateAndSavePNL(model, step, order.Filled)

			if model.State.ExecutedAmount >= amount || model.Conditions.CloseStrategyAfterFirstTAP {
				isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || model.Conditions.Hedging

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

			sm.calculateAndSavePNL(model, step, order.Filled)
			log.Print("model.State.ExecutedAmount >= amount in SL ", model.State.ExecutedAmount >= amount)
			if model.State.ExecutedAmount >= amount {
				return true
			}
		case "ForcedLoss":
			if order.Filled > 0 {
				model.State.ExecutedAmount += order.Filled
			}
			model.State.ExitPrice = order.Average
			amount := model.Conditions.EntryOrder.Amount
			if model.Conditions.MarketType == 0 {
				amount = amount * 0.99
			}

			sm.calculateAndSavePNL(model, step, order.Filled)
			log.Print("model.State.ExecutedAmount >= amount in ForcedLoss ", model.State.ExecutedAmount >= amount)
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

			sm.calculateAndSavePNL(model, step, order.Filled)
			// close sm if bep executed
			if model.State.ExecutedAmount >= amount || isMultiEntry {
				model.State.State = End
				sm.StateMgmt.UpdateState(model.ID, model.State)
				return true
			}
		case Canceled:
			sm.IsWaitingForOrder.Store(step, false)
			if order.Filled > 0 {
				model.State.ExecutedAmount += order.Filled
			}
			model.State.ExitPrice = order.Average
			amount := model.Conditions.EntryOrder.Amount
			if model.Conditions.MarketType == 0 {
				amount = amount * 0.99
			}

			sm.calculateAndSavePNL(model, step, order.Filled)

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

func (sm *SmartOrder) calculateAndSavePNL(model *models.MongoStrategy,  step interface{}, filledAmount float64) float64 {

	leverage := model.Conditions.Leverage
	if leverage == 0 {
		leverage = 1.0
	}

	sideCoefficient := 1.0
	isMultiEntry := len(model.Conditions.EntryLevels) > 0

	log.Println("PositionAmount ", model.State.PositionAmount, " filledAmount ", filledAmount)
	model.State.PositionAmount -= filledAmount

	amount := filledAmount
	entryPrice := model.State.EntryPrice
	// somehow we execute tap without entry
	if entryPrice == 0 {
		entryPrice = model.State.SavedEntryPrice
	}

	if entryPrice == 0 || model.State.ExitPrice == 0 {
		log.Println("calculateAndSavePNL: entry or exit price 0, entry: ", entryPrice, " exit: ", model.State.ExitPrice)
		return 0
	}

	side := model.Conditions.EntryOrder.Side
	if side == "sell" {
		sideCoefficient = -1.0
	}

	log.Println("model.State.ExitPrice ", model.State.ExitPrice, " model.State.EntryPrice ", entryPrice, " leverage ", leverage)
	profitPercentage := ((model.State.ExitPrice / entryPrice) * 100 - 100) * leverage * sideCoefficient
	log.Println("profitPercentage ", profitPercentage)
	profitAmount := (amount / leverage) * entryPrice * (profitPercentage / 100)
	log.Println("profitAmount ", profitAmount)

	log.Println("before ", model.State.ReceivedProfitPercentage," ", model.State.ReceivedProfitAmount)
	model.State.ReceivedProfitPercentage += profitPercentage
	model.State.ReceivedProfitAmount += profitAmount


	if model.Conditions.CreatedByTemplate {
		go sm.Strategy.GetStateMgmt().SavePNL(model.Conditions.TemplateStrategyId, profitAmount)
	}

	// if we got profit from target from averaging
	if (step == TakeProfit || step == "WithoutLoss") && isMultiEntry {
		model.State.ExitPrice = 0
		model.State.SavedEntryPrice = entryPrice
		model.State.EntryPrice = 0
	}

	sm.Strategy.GetStateMgmt().UpdateStrategyState(model.ID, model.State)

	return profitAmount
}