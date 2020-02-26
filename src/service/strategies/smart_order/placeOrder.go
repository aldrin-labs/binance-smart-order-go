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
	if sm.Strategy.GetModel().Conditions.EntryOrder.Side == oppositeSide {
		oppositeSide = "sell"
	}
	prefix := "stop-"
	isFutures := sm.Strategy.GetModel().Conditions.MarketType == 1
	isSpot := sm.Strategy.GetModel().Conditions.MarketType == 0
	isTrailingEntry := sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice > 0
	ifShouldCancelPreviousOrder := false
	leverage := sm.Strategy.GetModel().Conditions.Leverage
	if isSpot {
		leverage = 1
	}
	switch step {
	case TrailingEntry:
		orderType = sm.Strategy.GetModel().Conditions.EntryOrder.OrderType // TODO find out to remove duplicate lines with 154 & 164
		isStopOrdersSupport := isFutures || orderType == "limit"
		if isStopOrdersSupport { // we can place stop order, lets place it
			orderType = prefix + sm.Strategy.GetModel().Conditions.EntryOrder.OrderType
		} else {
			return
		}
		baseAmount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount

		isNewTrailingMaximum := price == -1
		isTrailingTarget := sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice > 0
		if isNewTrailingMaximum && isTrailingTarget {
			ifShouldCancelPreviousOrder = true
			if sm.Strategy.GetModel().Conditions.EntryOrder.OrderType == "market" {
				if isFutures {
					orderType = prefix + sm.Strategy.GetModel().Conditions.EntryOrder.OrderType
				} else {
					return // we cant place stop-market orders on spot so we'll wait for exact price
				}
			}
		} else {
			return
		}
		side = sm.Strategy.GetModel().Conditions.EntryOrder.Side
		if side == "sell" {
			orderPrice = sm.Strategy.GetModel().State.TrailingEntryPrice * (1 - sm.Strategy.GetModel().Conditions.EntryOrder.EntryDeviation/100/sm.Strategy.GetModel().Conditions.Leverage)
		} else {
			orderPrice = sm.Strategy.GetModel().State.TrailingEntryPrice * (1 + sm.Strategy.GetModel().Conditions.EntryOrder.EntryDeviation/100/sm.Strategy.GetModel().Conditions.Leverage)
		}
		break
	case InEntry:
		isStopOrdersSupport := isFutures || orderType == "limit"
		if !isTrailingEntry || isStopOrdersSupport {
			return // if it wasnt trailing we knew the price and placed order already (limit or market)
			// but if it was trailing with stop-orders support we also already placed order
		} // so here we only place after trailing market order for spot market:
		orderType = sm.Strategy.GetModel().Conditions.EntryOrder.OrderType
		baseAmount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount
		side = sm.Strategy.GetModel().Conditions.EntryOrder.Side
		break
	case WaitForEntry:
		if isTrailingEntry {
			return // do nothing because we dont know entry price, coz didnt hit activation price yet
		}

		orderType = sm.Strategy.GetModel().Conditions.EntryOrder.OrderType
		side = sm.Strategy.GetModel().Conditions.EntryOrder.Side
		baseAmount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount
		break
	case Stoploss:
		reduceOnly = true
		baseAmount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount - sm.Strategy.GetModel().State.ExecutedAmount
		side = "buy"

		if sm.Strategy.GetModel().Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		if sm.Strategy.GetModel().Conditions.TimeoutLoss == 0 {
			orderType = sm.Strategy.GetModel().Conditions.StopLossType
			isStopOrdersSupport := isFutures // || orderType == "limit"

			stopLoss := sm.Strategy.GetModel().Conditions.StopLoss
			if side == "sell" {
				orderPrice = sm.Strategy.GetModel().State.EntryPrice * (1 - stopLoss/100/leverage)
			} else {
				orderPrice = sm.Strategy.GetModel().State.EntryPrice * (1 + stopLoss/100/leverage)
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
			if price > 0 && sm.Strategy.GetModel().State.StopLossAt == 0 {
				sm.Strategy.GetModel().State.StopLossAt = time.Now().Unix()
				go func(lastTimestamp int64) {
					time.Sleep(time.Duration(sm.Strategy.GetModel().Conditions.TimeoutLoss) * time.Second)
					currentState, _ := sm.State.State(context.TODO())
					if currentState == Stoploss && sm.Strategy.GetModel().State.StopLossAt == lastTimestamp {
						sm.placeOrder(price, step)
					}
				}(sm.Strategy.GetModel().State.StopLossAt)
				return
			} else if price > 0 && sm.Strategy.GetModel().State.StopLossAt > 0 {
				orderType = "market"
				break
			} else {
				return // cant do anything here
			}
		}
		break
	case TakeProfit:
		prefix := "take-profit-"
		reduceOnly = true
		if sm.SelectedExitTarget >= len(sm.Strategy.GetModel().Conditions.ExitLevels) {
			return
		}
		target := sm.Strategy.GetModel().Conditions.ExitLevels[sm.SelectedExitTarget]
		isTrailingTarget := target.ActivatePrice > 0
		isSpotMarketOrder := target.OrderType == "market" && isSpot
		if price == 0 && isTrailingTarget {
			// trailing exit, we cant place exit order now
			return
		}
		if price > 0 && !isSpotMarketOrder {
			return // order was placed before, exit
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
					orderPrice = sm.Strategy.GetModel().State.EntryPrice * (1 + target.Price/100/leverage)
				} else {
					orderPrice = sm.Strategy.GetModel().State.EntryPrice * (1 - target.Price/100/leverage)
				}
				break
			}
		}
		isNewTrailingMaximum := price == -1
		if isNewTrailingMaximum && isTrailingTarget {
			prefix = "stop-"
			ifShouldCancelPreviousOrder = true
			if target.OrderType == "market" {
				if isFutures {
					orderType = prefix + target.OrderType
				} else if price == 0 {
					return // we cant place stop-market orders on spot so we'll wait for exact price
				}
			} else {
				recursiveCall = true // if its not instant order check maybe we can try to place other positions
			}
			orderPrice = sm.Strategy.GetModel().State.TrailingEntryPrice * (1 - target.EntryDeviation/100/leverage)
		}
		if sm.SelectedExitTarget < len(sm.Strategy.GetModel().Conditions.ExitLevels)-1 {
			baseAmount = target.Amount
			if target.Type == 1 {
				if baseAmount == 0 {
					baseAmount = 100
				}
				baseAmount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount * (baseAmount / 100)
			}
		} else {
			baseAmount = sm.getLastTargetAmount()
		}
		// sm.Strategy.GetModel().State.ExecutedAmount += amount
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
			baseAmount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount
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
				Symbol:     sm.Strategy.GetModel().Conditions.Pair,
				MarketType: sm.Strategy.GetModel().Conditions.MarketType,
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
		if step == TrailingEntry && orderType != "market" && ifShouldCancelPreviousOrder && len(sm.Strategy.GetModel().State.ExecutedOrders) > 0 {
			count := len(sm.Strategy.GetModel().State.ExecutedOrders)
			existingOrderId := sm.Strategy.GetModel().State.ExecutedOrders[count-1]
			response := sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.CancelOrderRequestParams{
					OrderId:    existingOrderId,
					MarketType: sm.Strategy.GetModel().Conditions.MarketType,
					Pair:       sm.Strategy.GetModel().Conditions.Pair,
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
				if len(sm.Strategy.GetModel().State.ExecutedOrders) > 0 && step != TrailingEntry {
					count := len(sm.Strategy.GetModel().State.ExecutedOrders)
					existingOrderId := sm.Strategy.GetModel().State.ExecutedOrders[count-1]
					sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
						KeyId: sm.KeyId,
						KeyParams: trading.CancelOrderRequestParams{
							OrderId:    existingOrderId,
							MarketType: sm.Strategy.GetModel().Conditions.MarketType,
							Pair:       sm.Strategy.GetModel().Conditions.Pair,
						},
					})
				}
				sm.Strategy.GetModel().State.ExecutedOrders = append(sm.Strategy.GetModel().State.ExecutedOrders, response.Data.Id)
			}
			if response.Data.Id != "0" {
				go sm.waitForOrder(response.Data.Id, step)
			} else {
				println("order 0")
			}
			sm.Strategy.GetModel().State.Orders = append(sm.Strategy.GetModel().State.Orders, response.Data.Id)
			sm.StateMgmt.UpdateOrders(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			break
		} else {
			println(response.Status)
			//if response.Status == "OK" {
			//	break
			//}
			if len(response.Data.Msg) > 0 {
				sm.Strategy.GetModel().Enabled = false
				sm.Strategy.GetModel().State.State = Error
				sm.Strategy.GetModel().State.Msg = response.Data.Msg
				go sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)

				break
			}
		}
	}
	canPlaceAnotherOrderForNextTarget := sm.SelectedExitTarget+1 < len(sm.Strategy.GetModel().Conditions.ExitLevels)
	if recursiveCall && canPlaceAnotherOrderForNextTarget {
		sm.SelectedExitTarget += 1
		sm.placeOrder(price, step)
	}
}