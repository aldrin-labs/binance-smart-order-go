package smart_order

import (
	"context"
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading/orders"
	"go.uber.org/zap"
	"strings"
	"time"
)

// PlaceOrder is a procedure calculates create order request and dispatches it to trading interface.
func (sm *SmartOrder) PlaceOrder(price, amount float64, step string) {
	sm.Strategy.GetLogger().Debug("place order",
		zap.Float64("price", price),
		zap.Float64("amount", amount),
		zap.String("step", step),
	)
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
	isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || model.Conditions.Hedging == true
	leverage := model.Conditions.Leverage
	if isSpot || leverage == 0 {
		leverage = 1
	}

	// Calculate request
	switch step {
	case TrailingEntry:
		sm.Strategy.GetLogger().Info("trailing entry order placing")
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
			orderPrice = model.State.TrailingEntryPrice * (1 - model.Conditions.EntryOrder.EntryDeviation/100/leverage)
		} else {
			orderPrice = model.State.TrailingEntryPrice * (1 + model.Conditions.EntryOrder.EntryDeviation/100/leverage)
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

		if len(model.Conditions.EntryLevels) > 0 {
			baseAmount = amount
		}
		//log.Print("orderPrice in waitForEntry", orderPrice)
		break
	case HedgeLoss:
		reduceOnly = true
		baseAmount = model.Conditions.EntryOrder.Amount - model.State.ExecutedAmount
		side = oppositeSide
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
		side = oppositeSide

		if model.Conditions.StopLossPrice > 0 {
			orderPrice = model.Conditions.StopLossPrice
			if isFutures {
				orderType = prefix + model.Conditions.StopLossType
			} else {
				orderType = model.Conditions.StopLossType
			}
			break
		}

		if len(model.Conditions.EntryLevels) > 0 {
			if amount > 0 {
				baseAmount = amount
			}
			stopLoss := model.Conditions.StopLoss
			if side == "sell" {
				orderPrice = price * (1 - stopLoss/100/leverage)
			} else {
				orderPrice = price * (1 + stopLoss/100/leverage)
			}
			orderType = prefix + model.Conditions.StopLossType
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
					if currentState == Stoploss && model.State.StopLossAt == lastTimestamp {
						sm.PlaceOrder(price, 0.0, step)
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
		side = oppositeSide
		baseAmount = model.Conditions.EntryOrder.Amount
		orderType = "market"

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

		if len(model.Conditions.EntryLevels) > 0 {
			if side == "sell" {
				orderPrice = price * (1 - model.Conditions.ForcedLoss/100/leverage)
			} else {
				orderPrice = price * (1 + model.Conditions.ForcedLoss/100/leverage)
			}
		}
		break
	case "WithoutLoss":
		// entry price + commission
		reduceOnly = true
		side = oppositeSide
		baseAmount = model.Conditions.EntryOrder.Amount
		orderType = prefix + "limit"
		fee := 0.12
		sm.Strategy.GetLogger().Info("WithoutLoss",
			zap.Float64("amount", amount),
			zap.Float64("entry price", model.State.EntryPrice),
		)
		if amount > 0 {
			baseAmount = amount
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
		} else if len(model.Conditions.EntryLevels) > 0 {
			fee = fee * float64(sm.SelectedEntryTarget+2)
		} else {
			fee = fee * 2
		}

		if fee*leverage > model.Conditions.WithoutLossAfterProfit &&
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

		if len(model.Conditions.EntryLevels) > 0 {
			orderType = "take-profit-" + "limit"
			break
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
			sm.Strategy.GetLogger().Debug("don't place order",
				zap.Int("SelectedExitTarget", sm.SelectedExitTarget),
				zap.Int("len(model.Conditions.ExitLevels)", len(model.Conditions.ExitLevels)),
			)
			return
		}
		target := model.Conditions.ExitLevels[sm.SelectedExitTarget]
		isTrailingTarget := target.ActivatePrice != 0
		isSpotMarketOrder := target.OrderType == "market" && isSpot
		baseAmount = model.Conditions.EntryOrder.Amount
		side = oppositeSide
		//log.Print("take profit price, orderPrice", price, orderPrice)

		//if model.Conditions.TakeProfitSpreadHunter && price > 0 {
		//	orderType = "maker-only"
		//	if model.Conditions.TakeProfitWaitingTime > 0 {
		//		frequency = float64(model.Conditions.TakeProfitWaitingTime)
		//	}
		//	break
		//}

		if price == 0 && isTrailingTarget {
			// trailing exit, we cant place exit order now
			sm.Strategy.GetLogger().Debug("don't place order",
				zap.Float64("price", price),
				zap.Bool("isTrailingTarget", isTrailingTarget),
			)
			return
		}
		if price > 0 && !isSpotMarketOrder {
			sm.Strategy.GetLogger().Debug("don't place order",
				zap.Float64("price", price),
				zap.Bool("!isSpotMarketOrder", !isSpotMarketOrder),
			)
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
					sm.Strategy.GetLogger().Debug("don't place order",
						zap.Bool("isFutures", isFutures),
						zap.String("target.OrderType", target.OrderType),
						zap.Bool("!isTrailingTarget", !isTrailingTarget),
						zap.Float64("price", price),
					)
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
				sm.Strategy.GetLogger().Debug("don't place order",
					zap.Float64("price", price),
					zap.Bool("isFutures", isFutures),
					zap.Bool("isTrailingTarget", isTrailingTarget),
					zap.Bool("isNewTrailingMaximum", isNewTrailingMaximum),
				)
				return // we cant place stop-market orders on spot so we'll wait for exact price
			}
			if side == "sell" {
				orderPrice = model.State.TrailingEntryPrice * (1 - target.EntryDeviation/100/leverage)
			} else {
				orderPrice = model.State.TrailingEntryPrice * (1 + target.EntryDeviation/100/leverage)
			}
			if model.Conditions.TakeProfitExternal { // TV alert?
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

		if len(model.Conditions.EntryLevels) > 0 {
			baseAmount = sm.getAveragingEntryAmount(model, sm.SelectedEntryTarget)
		}
		//log.Print("take profit price, orderPrice in the end", price, orderPrice)
		// model.State.ExecutedAmount += amount
		break
	case Canceled:
		currentState, _ := sm.stateMachine.State(context.TODO())
		thereIsNoEntryToExit := (currentState == WaitForEntry && model.State.Amount == 0) || currentState == TrailingEntry ||
			currentState == End || model.State.ExecutedAmount >= model.Conditions.EntryOrder.Amount

		// if canceled order was already placed
		isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(Canceled)
		if thereIsNoEntryToExit || (ok && isWaitingForOrder.(bool)) {
			return
		}
		side = oppositeSide
		reduceOnly = true
		baseAmount = model.Conditions.EntryOrder.Amount
		orderType = "market"
		if isSpot {
			sm.TryCancelAllOrdersConsistently(sm.Strategy.GetModel().State.Orders)
		}
		if model.State.Amount > 0 && currentState == WaitForEntry {
			baseAmount = model.State.Amount
		}
		break
	}

	// Respect fees paid
	// TODO: reset commission if PlaceEntryAfterTAP set and TakeProfit executes
	if side == "sell" && isSpot {
		if step == TakeProfit && len(sm.Strategy.GetModel().Conditions.ExitLevels) > 1 { // split targets
			baseAmount = baseAmount - sm.Strategy.GetModel().State.Commission*
				sm.Strategy.GetModel().Conditions.ExitLevels[sm.SelectedExitTarget].Amount/100.0
		} else {
			baseAmount = baseAmount - sm.Strategy.GetModel().State.Commission
		}
	}

	// Respect exchange rules on values precision
	sm.Strategy.GetLogger().Info("before rounding",
		zap.Float64("baseAmount", baseAmount),
		zap.Float64("orderPrice", orderPrice),
		zap.String("step", step),
	)
	baseAmount = sm.toFixedAmount(baseAmount, Floor)
	orderPrice = sm.toFixedPrice(orderPrice, Nearest)
	sm.Strategy.GetLogger().Info("after rounding",
		zap.Float64("baseAmount", baseAmount),
		zap.Float64("orderPrice", orderPrice),
	)

	advancedOrderType := orderType
	if strings.Contains(orderType, "stop") || strings.Contains(orderType, "take-profit") {
		orderType = "stop"
		stopPrice = orderPrice
	}

	// Call trading API with retries
	sm.callTradingAPI(baseAmount, orderPrice,
		orderType, isSpot, step, model,
		ifShouldCancelPreviousOrder, isTrailingHedgeOrder, side,
		reduceOnly, stopPrice, frequency, advancedOrderType,
		attemptsToPlaceOrder, recursiveCall, price, leverage)
}

func (sm *SmartOrder) callTradingAPI(
	baseAmount float64,
	orderPrice float64,
	orderType string,
	isSpot bool,
	step string,
	model *models.MongoStrategy,
	ifShouldCancelPreviousOrder bool,
	isTrailingHedgeOrder bool,
	side string,
	reduceOnly bool,
	stopPrice float64,
	frequency float64,
	advancedOrderType string,
	attemptsToPlaceOrder int,
	recursiveCall bool,
	price float64,
	leverage float64,
) {
	for {
		if baseAmount == 0 || orderType == "limit" && orderPrice == 0 {
			return
		}
		request := orders.CreateOrderRequest{
			KeyId: sm.KeyId,
			KeyParams: orders.Order{
				Symbol:     model.Conditions.Pair,
				MarketType: model.Conditions.MarketType,
				Type:       orderType,
				Side:       side,
				Amount:     baseAmount,
				Price:      orderPrice,
				ReduceOnly: &reduceOnly,
				StopPrice:  stopPrice,
				Frequency:  frequency,
			},
		}
		if request.KeyParams.Type == "stop" {
			request.KeyParams.Params = orders.OrderParams{
				Type: advancedOrderType,
			}
		}
		if isSpot {
			// if SM wants to exit in a short period after entry executed, ES may have no balance updated and set
			// exit order amount to zero. That's why we should not use MaxIfNotEnough option here.
			request.KeyParams.Params.MaxIfNotEnough = 0
			request.KeyParams.Params.Retry = true
			request.KeyParams.Params.RetryTimeout = 1000
			request.KeyParams.Params.RetryCount = 5
		}
		isSpotTAP := isSpot && step == TakeProfit && model.Conditions.ExitLevels[sm.SelectedExitTarget].ActivatePrice != 0
		if (step == TrailingEntry || isSpotTAP) && orderType != "market" && ifShouldCancelPreviousOrder && len(model.State.ExecutedOrders) > 0 {
			count := len(model.State.ExecutedOrders)
			existingOrderId := model.State.ExecutedOrders[count-1]
			response := sm.ExchangeApi.CancelOrder(orders.CancelOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: orders.CancelOrderRequestParams{
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
		} else {
			request.KeyParams.PositionSide = "BOTH"
		}
		if step == WaitForEntry {
			sm.IsEntryOrderPlaced = true
			sm.IsWaitingForOrder.Store(step, true)
		}

		sm.Strategy.GetLogger().Info("create order",
			zap.String("step", step),
			zap.Float64("amount", baseAmount),
			zap.Float64("price", orderPrice),
			zap.Float64("stopPrice", stopPrice),
			zap.String("request", fmt.Sprint(request)),
		)
		var response orders.OrderResponse
		if request.KeyParams.Type == "maker-only" {
			response = sm.Strategy.GetSingleton().CreateOrder(request)
		} else {
			response = sm.ExchangeApi.CreateOrder(request)
		}

		// Update state with order attempt results
		sm.Strategy.GetLogger().Info("got response",
			zap.String("status", response.Status),
			zap.String("orderId", response.Data.OrderId),
		)
		if response.Status == "OK" && response.Data.OrderId != "0" && response.Data.OrderId != "" {
			sm.IsWaitingForOrder.Store(step, true)
			if ifShouldCancelPreviousOrder {
				// cancel existing order if there is such ( and its not TrailingEntry )
				if len(model.State.ExecutedOrders) > 0 && step != TrailingEntry {
					count := len(model.State.ExecutedOrders)
					existingOrderId := model.State.ExecutedOrders[count-1]
					sm.ExchangeApi.CancelOrder(orders.CancelOrderRequest{
						KeyId: sm.KeyId,
						KeyParams: orders.CancelOrderRequestParams{
							OrderId:    existingOrderId,
							MarketType: model.Conditions.MarketType,
							Pair:       model.Conditions.Pair,
						},
					})
				}
				model.State.ExecutedOrders = append(model.State.ExecutedOrders, response.Data.OrderId)
			}
			if response.Data.OrderId != "0" {
				sm.OrdersMux.Lock()
				sm.OrdersMap[response.Data.OrderId] = true
				sm.OrdersMux.Unlock()
				go sm.waitForOrder(response.Data.OrderId, step)

				// save placed orders id to state SL/TAP
				if step == Stoploss {
					model.State.StopLossOrderIds = append(model.State.StopLossOrderIds, response.Data.OrderId)
				} else if step == "ForcedLoss" {
					model.State.ForcedLossOrderIds = append(model.State.ForcedLossOrderIds, response.Data.OrderId)
				} else if step == TakeProfit {
					model.State.TakeProfitOrderIds = append(model.State.TakeProfitOrderIds, response.Data.OrderId)
				} else if step == WaitForEntry {
					model.State.WaitForEntryIds = append(model.State.WaitForEntryIds, response.Data.OrderId)
				}
			} else {
				sm.Strategy.GetLogger().Info("order 0")
			}
			if step != Canceled {
				sm.OrdersMux.Lock()
				sm.Strategy.GetLogger().Info("adding order to state.Orders",
					zap.String("order id", response.Data.OrderId),
				)
				model.State.Orders = append(model.State.Orders, response.Data.OrderId)
				sm.OrdersMux.Unlock()
				go sm.StateMgmt.UpdateOrders(model.ID, model.State)
			}
			break
		} else {
			// if error
			if len(response.Data.Msg) > 0 {
				//TODO
				// need correct message from exchange_service when down
				//if len(response.Data.Msg) > 0 && strings.Contains(response.Data.Msg, "network error") {
				//	time.Sleep(time.Second * 5)
				//	continue
				//}
				if checkResponseNeedsWait(response.Data.Msg, "Key is processing", attemptsToPlaceOrder, 1, time.Minute*1) {
					attemptsToPlaceOrder++
					continue
				}
				if checkResponseNeedsWait(response.Data.Msg, "position side does not match", attemptsToPlaceOrder, 3, time.Second*5) {
					attemptsToPlaceOrder++
					continue
				}
				if checkResponseNeedsWait(response.Data.Msg, "invalid json", attemptsToPlaceOrder, 3, time.Second*2) {
					attemptsToPlaceOrder++
					continue
				}
				if checkResponseNeedsWait(response.Data.Msg, "ReduceOnly Order Failed", attemptsToPlaceOrder, 3, time.Second*5) {
					attemptsToPlaceOrder++
					continue
				}
				if checkResponseNeedsWait(response.Data.Msg, "Cannot read property 'text' of undefined", attemptsToPlaceOrder, 3, time.Second*5) {
					attemptsToPlaceOrder++
					continue
				}
				if strings.Contains(response.Data.Msg, "immediately trigger") {
					if step == TrailingEntry {
						orderType = "market"
						stopPrice = 0.0
						ifShouldCancelPreviousOrder = false
						continue
					} else if len(model.Conditions.EntryLevels) > 0 &&
						(step == Stoploss || step == "ForcedLoss") && attemptsToPlaceOrder < 3 {

						lossPercentage := model.Conditions.StopLoss
						if step == "ForcedLoss" {
							lossPercentage = model.Conditions.ForcedLoss
						}

						price = sm.getLastTargetPrice(model)

						if side == "sell" {
							orderPrice = price * (1 - lossPercentage/100/leverage)
						} else {
							orderPrice = price * (1 + lossPercentage/100/leverage)
						}

						attemptsToPlaceOrder += 1
						time.Sleep(5 * time.Second)
						continue
					} else {
						sm.PlaceOrder(0, 0.0, Canceled)
						break
					}
				}
				if strings.Contains(response.Data.Msg, "ReduceOnly Order is rejected") {
					model.Enabled = false
					go sm.StateMgmt.UpdateState(model.ID, model.State)
					break
				}

				model.Enabled = false
				model.State.State = Error
				model.State.Msg = response.Data.Msg
				sm.Statsd.Inc("smart_order.error_state")
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
		sm.PlaceOrder(price, 0.0, step)
	}
}

func checkResponseNeedsWait(resp string, req string, attempts int, maxAttempts int, sleepDuration time.Duration) bool {
	if strings.Contains(resp, req) && attempts < maxAttempts {
		time.Sleep(sleepDuration)
		return true
	}
	return false
}
