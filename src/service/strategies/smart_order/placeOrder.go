package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"strings"
	"time"
)

func (sm *SmartOrder) placeOrder(price float64, step string) {
	baseAmount := 0.0
	orderType := "market"
	stopPrice := 0.0
	side := ""
	orderPrice := price

	recursiveCall := false
	reduceOnly := false

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
	if isSpot {
		leverage = 1
	}
	switch step {
	case TrailingEntry:
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
		orderType = prefix + orderType // ok we are in futures and can place order before it happened
		break
	case Stoploss:
		reduceOnly = true
		baseAmount = model.Conditions.EntryOrder.Amount - model.State.ExecutedAmount
		side = "buy"

		isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || model.Conditions.HedgeKeyId != nil
		if isTrailingHedgeOrder {
			return
		}
		if model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		// try exit on timeoutWhenLoss
		if model.Conditions.TimeoutWhenLoss > 0 && price < 0 {
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
					if currentState == Stoploss && model.State.StopLossAt == lastTimestamp {
						sm.placeOrder(price, step)
						model.State.State = End
						sm.StateMgmt.UpdateState(model.ID, model.State)
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
	case TakeProfit:
		prefix := "take-profit-"
		reduceOnly = true
		if sm.SelectedExitTarget >= len(model.Conditions.ExitLevels) {
			return
		}
		target := model.Conditions.ExitLevels[sm.SelectedExitTarget]
		isTrailingTarget := target.ActivatePrice != 0
		isSpotMarketOrder := target.OrderType == "market" && isSpot
		if price == 0 && isTrailingTarget {
			// trailing exit, we cant place exit order now
			return
		}
		if price > 0 && !isSpotMarketOrder {
			return // order was placed before, exit
		}

		// try exit on timeoutIfProfitable
		if model.Conditions.TimeoutIfProfitable > 0 && price < 0 {
			orderType = "market"
			break
		}

		side = oppositeSide
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
		// model.State.ExecutedAmount += amount
		break
	case Canceled:
		{
			currentState, _ := sm.State.State(context.TODO())
			thereIsNoEntryToExit := currentState == WaitForEntry || currentState == TrailingEntry || currentState == End
			if thereIsNoEntryToExit {
				return
			}
			side = oppositeSide
			reduceOnly = true
			baseAmount = model.Conditions.EntryOrder.Amount
			break
		}
	}

	baseAmount = sm.toFixed(baseAmount, sm.QuantityAmountPrecision)
	orderPrice = sm.toFixed(orderPrice, sm.QuantityPricePrecision)

	advancedOrderType := orderType
	if strings.Contains(orderType, "stop") || strings.Contains(orderType, "take-profit") {
		orderType = "stop"
		stopPrice = orderPrice
	}
	for {
		if baseAmount == 0 {
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
				ReduceOnly: reduceOnly,
				StopPrice:  stopPrice,
			},
		}
		if request.KeyParams.Type == "stop" {
			request.KeyParams.Params = trading.OrderParams{
				Type: advancedOrderType,
			}
		}
		if step == TrailingEntry && orderType != "market" && ifShouldCancelPreviousOrder && len(model.State.ExecutedOrders) > 0 {
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
				sm.OrdersMux.Lock()
				sm.OrdersMap[response.Data.OrderId] = true
				sm.OrdersMux.Unlock()
				go sm.waitForOrder(response.Data.Id, step)
			} else {
				println("order 0")
			}
			model.State.Orders = append(model.State.Orders, response.Data.Id)
			sm.StateMgmt.UpdateOrders(model.ID, model.State)
			break
		} else {
			println(response.Status)
			if len(response.Data.Msg) > 0 && step != Canceled && step != End && step != Timeout {
				model.Enabled = false
				model.State.State = Error
				model.State.Msg = response.Data.Msg
				go sm.StateMgmt.UpdateState(model.ID, model.State)

				break
			}
			if response.Status == "OK" {
				break
			}
		}
	}
	canPlaceAnotherOrderForNextTarget := sm.SelectedExitTarget+1 < len(model.Conditions.ExitLevels)
	if recursiveCall && canPlaceAnotherOrderForNextTarget {
		sm.SelectedExitTarget += 1
		sm.placeOrder(price, step)
	}
}
