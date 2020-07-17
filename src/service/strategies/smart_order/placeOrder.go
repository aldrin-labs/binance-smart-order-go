package smart_order

import (
	"context"
	"strings"
	"time"

	"gitlab.com/crypto_project/core/strategy_service/src/trading"
)

func (sm *SmartOrder) PlaceOrder(price float64, step string) {
	baseAmount := 0.0
	orderType := "market"
	stopPrice := 0.0
	frequency := 0.0
	side := ""
	orderPrice := price

	recursiveCall := false
	reduceOnly := false

	attemptsToPlaceOrder := 0
	oppositeSide := "buy"
	model := sm.Strategy.GetModel()
	if model.Conditions.EntryOrder.Side == oppositeSide {
		oppositeSide = "sell"
	}
	prefix := "stop-"
	isFutures := model.Conditions.MarketType == 1
	isSpot := model.Conditions.MarketType == 0
	isTrailingEntry := model.Conditions.EntryOrder.ActivatePrice != 0
	ifShouldCancelPreviousOrder := false
	leverage := model.Conditions.Leverage
	isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || model.Conditions.Hedging == true
	if isSpot {
		leverage = 1
	}
	switch step {
	case TrailingEntry:
		println("trailing entry order placing")
		orderType = model.Conditions.EntryOrder.OrderType // TODO find out to remove duplicate lines with 154 & 164
		isStopOrdersSupport := isFutures || orderType == "limit"
		if isStopOrdersSupport { // we can place stop order, lets place it
			orderType = prefix + model.Conditions.EntryOrder.OrderType
		} else {
			return
		}
		baseAmount = model.Conditions.EntryOrder.Amount

		isNewTrailingMaximum := price == -1
		isTrailingTarget := model.Conditions.EntryOrder.ActivatePrice != 0
		if isNewTrailingMaximum && isTrailingTarget {
			ifShouldCancelPreviousOrder = true
			if model.Conditions.EntryOrder.OrderType == "market" {
				if isFutures {
					orderType = prefix + model.Conditions.EntryOrder.OrderType
				} else {
					return // we cant place stop-market orders on spot so we'll wait for exact price
				}
			}
		} else {
			return
		}
		side = model.Conditions.EntryOrder.Side
		if side == "sell" {
			orderPrice = model.State.TrailingEntryPrice * (1 - model.Conditions.EntryOrder.EntryDeviation/100/model.Conditions.Leverage)
		} else {
			orderPrice = model.State.TrailingEntryPrice * (1 + model.Conditions.EntryOrder.EntryDeviation/100/model.Conditions.Leverage)
		}
		break
	case InEntry:
		isStopOrdersSupport := isFutures || orderType == "limit"
		if !isTrailingEntry || isStopOrdersSupport {
			return // if it wasnt trailing we knew the price and placed order already (limit or market)
			// but if it was trailing with stop-orders support we also already placed order
		} // so here we only place after trailing market order for spot market:
		orderType = model.Conditions.EntryOrder.OrderType
		baseAmount = model.Conditions.EntryOrder.Amount
		side = model.Conditions.EntryOrder.Side
		break
	case WaitForEntry:
		if isTrailingEntry {
			return // do nothing because we dont know entry price, coz didnt hit activation price yet
		}

		orderType = model.Conditions.EntryOrder.OrderType

		side = model.Conditions.EntryOrder.Side
		baseAmount = model.Conditions.EntryOrder.Amount
		println("orderPrice in waitForEntry", orderPrice)
		break
	case HedgeLoss:
		reduceOnly = true
		baseAmount = model.Conditions.EntryOrder.Amount - model.State.ExecutedAmount
		side = "buy"
		if model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		orderType = model.Conditions.StopLossType

		stopLoss := model.Conditions.HedgeLossDeviation
		ifShouldCancelPreviousOrder = true
		if side == "sell" {
			orderPrice = model.State.TrailingHedgeExitPrice * (1 - stopLoss/100/leverage)
		} else {
			orderPrice = model.State.TrailingHedgeExitPrice * (1 + stopLoss/100/leverage)
		}
		if model.Conditions.TakeProfitHedgePrice > 0 {
			orderPrice = model.Conditions.TakeProfitHedgePrice
		}

		orderType = prefix + orderType // ok we are in futures and can place order before it happened
		break
	case Stoploss:
		reduceOnly = true
		baseAmount = model.Conditions.EntryOrder.Amount - model.State.ExecutedAmount
		//if isSpot {
		//	baseAmount = baseAmount * 0.99
		//}

		side = "buy"
		if model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}

		if model.Conditions.StopLossPrice > 0 {
			orderPrice = model.Conditions.StopLossPrice
			if isFutures {
				orderType = prefix + model.Conditions.StopLossType
			} else {
				orderType = model.Conditions.StopLossType
			}
			break
		}

		if isTrailingHedgeOrder {
			return
		}
		// try exit on timeoutWhenLoss
		if model.Conditions.TimeoutWhenLoss > 0 && price < 0 || model.Conditions.StopLossPrice == -1 {
			orderType = "market"
			break
		}

		if model.Conditions.TimeoutLoss == 0 {
			orderType = model.Conditions.StopLossType
			isStopOrdersSupport := isFutures // || orderType == "limit"
			stopLoss := model.Conditions.StopLoss
			if side == "sell" {
				orderPrice = model.State.EntryPrice * (1 - stopLoss/100/leverage)
			} else {
				orderPrice = model.State.EntryPrice * (1 + stopLoss/100/leverage)
			}

			if isSpot {
				if price > 0 {
					break // keep market order
				} else if !isStopOrdersSupport {
					return // it is attempt to place an stop-order but we are on spot
					// we cant place stop orders coz then amount will be locked
				}
			}
			orderType = prefix + orderType // ok we are in futures and can place order before it happened

		} else {
			if price > 0 && model.State.StopLossAt == 0 {
				model.State.StopLossAt = time.Now().Unix()
				go func(lastTimestamp int64) {
					time.Sleep(time.Duration(model.Conditions.TimeoutLoss) * time.Second)
					currentState := sm.Strategy.GetModel().State.State
					//println("currentState", currentState, model.State.StopLossAt, lastTimestamp)
					if currentState == Stoploss && model.State.StopLossAt == lastTimestamp {
						sm.PlaceOrder(price, step)
					} else {
						model.State.StopLossAt = -1
						sm.StateMgmt.UpdateState(model.ID, model.State)
					}
				}(model.State.StopLossAt)
				return
			} else if price > 0 && model.State.StopLossAt > 0 {
				orderType = model.Conditions.StopLossType
				orderPrice = price
			} else {
				return // cant do anything here
			}
		}
		break
	case "ForcedLoss":
		reduceOnly = true
		side = "buy"
		baseAmount = model.Conditions.EntryOrder.Amount
		orderType = "market"

		if model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || model.Conditions.HedgeKeyId != nil
		if isTrailingHedgeOrder {
			return
		}

		if model.Conditions.ForcedLossPrice > 0 {
			orderPrice = model.Conditions.ForcedLossPrice
			if isFutures {
				orderType = prefix + orderType
			}
			break
		}

		isSpotMarketOrder := isSpot
		if isSpotMarketOrder {
			return
		}

		if !isSpot {
			orderType = prefix + orderType
		}

		if side == "sell" {
			orderPrice = model.State.EntryPrice * (1 - model.Conditions.ForcedLoss/100/leverage)
		} else {
			orderPrice = model.State.EntryPrice * (1 + model.Conditions.ForcedLoss/100/leverage)
		}
		break
	case "WithoutLoss":
		// entry price + commission
		reduceOnly = true
		side = "buy"
		baseAmount = model.Conditions.EntryOrder.Amount
		orderType = prefix + "limit"
		fee := 0.12

		if model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}

		// if price 0 then market price == entry price for spot market order
		if isSpot && price != 0 {
			return // we cant place market order on spot at exists before it happened, because there is no stop markets
		}

		if isFutures {
			fee = 0.04
		}

		if model.Conditions.Hedging || model.Conditions.HedgeMode {
			fee = fee * 4
		} else {
			fee = fee * 2
		}

		if fee*model.Conditions.Leverage > model.Conditions.WithoutLossAfterProfit &&
			model.Conditions.WithoutLossAfterProfit > 0 {
			orderType = "take-profit-" + "limit"
		}

		if side == "sell" {
			orderPrice = model.State.EntryPrice * (1 + fee/100)
		} else {
			orderPrice = model.State.EntryPrice * (1 - fee/100)
		}

		if price > 0 {
			orderPrice = price
		}

		currentOHLCVp := sm.DataFeed.GetPriceForPairAtExchange(sm.Strategy.GetModel().Conditions.Pair, sm.ExchangeName, sm.Strategy.GetModel().Conditions.MarketType)
		if currentOHLCVp != nil {
			currentOHLCV := *currentOHLCVp
			if currentOHLCV.Close < orderPrice && sm.Strategy.GetModel().Conditions.EntryOrder.Side == "buy" ||
				currentOHLCV.Close > orderPrice && sm.Strategy.GetModel().Conditions.EntryOrder.Side == "sell" {
				orderType = "take-profit-" + "limit"
			}
		}
		break
	case TakeProfit:
		prefix := "take-profit-"
		reduceOnly = true
		if sm.SelectedExitTarget >= len(model.Conditions.ExitLevels) {
			return
		}
		target := model.Conditions.ExitLevels[sm.SelectedExitTarget]
		isTrailingTarget := target.ActivatePrice != 0
		isSpotMarketOrder := target.OrderType == "market" && isSpot
		baseAmount = model.Conditions.EntryOrder.Amount
		side = oppositeSide
		println("take profit price, orderPrice", price, orderPrice)

		//if model.Conditions.TakeProfitSpreadHunter && price > 0 {
		//	orderType = "maker-only"
		//	if model.Conditions.TakeProfitWaitingTime > 0 {
		//		frequency = float64(model.Conditions.TakeProfitWaitingTime)
		//	}
		//	break
		//}

		if price == 0 && isTrailingTarget {
			// trailing exit, we cant place exit order now
			return
		}
		if price > 0 && !isSpotMarketOrder {
			return // order was placed before, exit
		}

		// try exit on timeoutIfProfitable
		if (model.Conditions.TimeoutIfProfitable > 0 && price < 0) || model.Conditions.TakeProfitPrice == -1 {
			orderType = "market"
			break
		}

		if model.Conditions.TakeProfitPrice > 0 && !isTrailingTarget {
			orderPrice = model.Conditions.TakeProfitPrice
			if isFutures && target.OrderType == "market" {
				orderType = prefix + model.Conditions.ExitLevels[0].OrderType
			} else {
				orderType = model.Conditions.ExitLevels[0].OrderType
			}
			break
		}

		if price == 0 && !isTrailingTarget {
			orderType = target.OrderType
			if target.OrderType == "market" {
				if isFutures {
					orderType = prefix + target.OrderType
					recursiveCall = true
				} else {
					return // we cant place market order on spot at exists before it happened, because there is no stop markets
				}
			} else {
				recursiveCall = true
			}
			switch target.Type {
			case 0:
				orderPrice = target.Price
				break
			case 1:
				if side == "sell" {
					orderPrice = model.State.EntryPrice * (1 + target.Price/100/leverage)
				} else {
					orderPrice = model.State.EntryPrice * (1 - target.Price/100/leverage)
				}
				break
			}
		}
		isNewTrailingMaximum := price == -1
		if isNewTrailingMaximum && isTrailingTarget {
			prefix = "stop-"
			orderType = target.OrderType
			ifShouldCancelPreviousOrder = true
			if isFutures {
				orderType = prefix + target.OrderType
			} else if price == 0 {
				return // we cant place stop-market orders on spot so we'll wait for exact price
			}
			if side == "sell" {
				orderPrice = model.State.TrailingEntryPrice * (1 - target.EntryDeviation/100/leverage)
			} else {
				orderPrice = model.State.TrailingEntryPrice * (1 + target.EntryDeviation/100/leverage)
			}
			if model.Conditions.TakeProfitExternal {
				orderPrice = model.Conditions.TrailingExitPrice
			}
		}
		if sm.SelectedExitTarget < len(model.Conditions.ExitLevels)-1 {
			baseAmount = target.Amount
			if target.Type == 1 {
				if baseAmount == 0 {
					baseAmount = 100
				}
				baseAmount = model.Conditions.EntryOrder.Amount * (baseAmount / 100)
			}
		} else {
			baseAmount = sm.getLastTargetAmount()
		}
		println("take profit price, orderPrice in the end", price, orderPrice)
		// model.State.ExecutedAmount += amount
		break
	case Canceled:
		{
			currentState, _ := sm.State.State(context.TODO())
			thereIsNoEntryToExit := currentState == WaitForEntry || currentState == TrailingEntry || currentState == End || model.State.ExecutedAmount >= model.Conditions.EntryOrder.Amount
			if thereIsNoEntryToExit {
				return
			}
			side = oppositeSide
			reduceOnly = true
			baseAmount = model.Conditions.EntryOrder.Amount
			orderType = "market"
			if isSpot {
				sm.TryCancelAllOrdersConsistently(sm.Strategy.GetModel().State.Orders)
			}
			break
		}
	}
	baseAmount = sm.toFixed(baseAmount, sm.QuantityAmountPrecision)
	println("orderPrice before toFixed", orderPrice)
	orderPrice = sm.toFixed(orderPrice, sm.QuantityPricePrecision)
	println("orderPrice after toFixed", orderPrice)

	advancedOrderType := orderType
	if strings.Contains(orderType, "stop") || strings.Contains(orderType, "take-profit") {
		orderType = "stop"
		stopPrice = orderPrice
	}
	for {
		if baseAmount == 0 || orderType == "limit" && orderPrice == 0 {
			return
		}
		request := trading.CreateOrderRequest{
			KeyId: sm.KeyId,
			KeyParams: trading.Order{
				Symbol:     model.Conditions.Pair,
				MarketType: model.Conditions.MarketType,
				Type:       orderType,
				Side:       side,
				Amount:     baseAmount,
				Price:      orderPrice,
				ReduceOnly: &reduceOnly,
				StopPrice:  stopPrice,
				Frequency: frequency,
			},
		}
		if request.KeyParams.Type == "stop" {
			request.KeyParams.Params = trading.OrderParams{
				Type: advancedOrderType,
			}
		}
		if isSpot {
			request.KeyParams.Params.MaxIfNotEnough = 1
		}
		isSpotTAP := isSpot && step == TakeProfit && model.Conditions.ExitLevels[sm.SelectedExitTarget].ActivatePrice != 0
		if (step == TrailingEntry || isSpotTAP) && orderType != "market" && ifShouldCancelPreviousOrder && len(model.State.ExecutedOrders) > 0 {
			count := len(model.State.ExecutedOrders)
			existingOrderId := model.State.ExecutedOrders[count-1]
			response := sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.CancelOrderRequestParams{
					OrderId:    existingOrderId,
					MarketType: model.Conditions.MarketType,
					Pair:       model.Conditions.Pair,
				},
			})
			if response.Status == "ERR" { // looks like order was already executed or canceled in other thread
				return
			}
		}
		if isTrailingHedgeOrder || model.Conditions.HedgeMode {
			request.KeyParams.ReduceOnly = nil
			if model.Conditions.EntryOrder.Side == "sell" {
				request.KeyParams.PositionSide = "SHORT"
			} else {
				request.KeyParams.PositionSide = "LONG"
			}
		}
		response := sm.ExchangeApi.CreateOrder(request)
		if response.Status == "OK" && response.Data.Id != "0" && response.Data.Id != "" {
			sm.IsWaitingForOrder.Store(step, true)
			if ifShouldCancelPreviousOrder {
				// cancel existing order if there is such ( and its not TrailingEntry )
				if len(model.State.ExecutedOrders) > 0 && step != TrailingEntry {
					count := len(model.State.ExecutedOrders)
					existingOrderId := model.State.ExecutedOrders[count-1]
					sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
						KeyId: sm.KeyId,
						KeyParams: trading.CancelOrderRequestParams{
							OrderId:    existingOrderId,
							MarketType: model.Conditions.MarketType,
							Pair:       model.Conditions.Pair,
						},
					})
				}
				model.State.ExecutedOrders = append(model.State.ExecutedOrders, response.Data.Id)
			}
			if response.Data.Id != "0" {
				println("response.Data.Id", response.Data.Id)
				sm.OrdersMux.Lock()
				sm.OrdersMap[response.Data.Id] = true
				sm.OrdersMux.Unlock()
				println("waitForOrder execute")
				go sm.waitForOrder(response.Data.Id, step)

				// save placed orders id to state SL/TAP
				if step == Stoploss {
					model.State.StopLossOrderIds = append(model.State.StopLossOrderIds, response.Data.Id)
				} else if step == "ForcedLoss" {
					model.State.ForcedLossOrderIds = append(model.State.ForcedLossOrderIds, response.Data.Id)
				} else if step == TakeProfit {
					model.State.TakeProfitOrderIds = append(model.State.TakeProfitOrderIds, response.Data.Id)
				}
			} else {
				println("order 0")
			}
			if step != Canceled {
				model.State.Orders = append(model.State.Orders, response.Data.Id)
				sm.StateMgmt.UpdateOrders(model.ID, model.State)
			}
			break
		} else {
			println(response.Status)
			// need correct message from exchange_service when down
			//if len(response.Data.Msg) > 0 && strings.Contains(response.Data.Msg, "network error") {
			//	time.Sleep(time.Second * 5)
			//	continue
			//}
			if len(response.Data.Msg) > 0 && attemptsToPlaceOrder < 1 && strings.Contains(response.Data.Msg, "Key is processing") {
				attemptsToPlaceOrder += 1
				time.Sleep(time.Minute * 1)
				continue
			}
			if len(response.Data.Msg) > 0 && attemptsToPlaceOrder < 3 && strings.Contains(response.Data.Msg, "position side does not match") {
				attemptsToPlaceOrder += 1
				time.Sleep(time.Second * 5)
				continue
			}
			if len(response.Data.Msg) > 0 && attemptsToPlaceOrder < 3 && strings.Contains(response.Data.Msg, "invalid json") {
				attemptsToPlaceOrder += 1
				time.Sleep(2 * time.Second)
				continue
			}
			if len(response.Data.Msg) > 0 && strings.Contains(response.Data.Msg, "immediately trigger") {
				if step == TrailingEntry {
					orderType = "market"
					stopPrice = 0.0
					ifShouldCancelPreviousOrder = false
					continue
				} else {
					sm.PlaceOrder(0, Canceled)
					break
				}
			}
			if len(response.Data.Msg) > 0 && strings.Contains(response.Data.Msg, "ReduceOnly Order is rejected") {
				model.Enabled = false
				go sm.StateMgmt.UpdateState(model.ID, model.State)
				break
			}
			if len(response.Data.Msg) > 0 {
				model.Enabled = false
				model.State.State = Error
				model.State.Msg = response.Data.Msg
				go sm.StateMgmt.UpdateState(model.ID, model.State)
				break
			}
			if response.Status == "OK" {
				break
			}
			attemptsToPlaceOrder += 1
		}
	}
	canPlaceAnotherOrderForNextTarget := sm.SelectedExitTarget+1 < len(model.Conditions.ExitLevels)
	if recursiveCall && canPlaceAnotherOrderForNextTarget {
		sm.SelectedExitTarget += 1
		sm.PlaceOrder(price, step)
	}
}
