package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/trading/orders"
	"go.uber.org/zap"
	"time"
)

func (sm *SmartOrder) checkTimeouts() {
	if sm.Strategy.GetModel().Conditions.WaitingEntryTimeout > 0 {
		go func(iteration int) {
			time.Sleep(time.Duration(sm.Strategy.GetModel().Conditions.WaitingEntryTimeout) * time.Second)
			currentState, _ := sm.GetState(context.TODO())
			sm.Strategy.GetLogger().Info("", // TODO(khassanov): clarify it
				zap.String("currentState ", currentState.(string)),
				zap.Bool("sm.Lock", sm.Lock),
				zap.Bool("iteration == sm.Strategy.GetModel().State.Iteration ", iteration == sm.Strategy.GetModel().State.Iteration),
			)
			if (currentState == WaitForEntry || currentState == TrailingEntry) && sm.Lock == false && iteration == sm.Strategy.GetModel().State.Iteration {
				sm.Lock = true
				switch len(sm.Strategy.GetModel().State.Orders) {
				case 0:
					// rare case when we have already placed entry order, but sm.Strategy.GetModel().State.Orders is empty (so we didn't receive response for this order)
					if sm.IsEntryOrderPlaced {
						count := 0
						for {
							// 5000 tries - 5 sec
							if count > 10*100*5 {
								// error in entry order
								break
							}
							// received update
							if len(sm.Strategy.GetModel().State.Orders) > 0 {
								res := sm.tryCancelEntryOrder()
								if res.Status == "OK" {
									sm.Strategy.GetLogger().Info("order canceled continue timeout code")
									break
								} else {
									sm.Strategy.GetLogger().Info("order already filled")
									sm.Lock = false
									return
								}
							}
							count += 1
							time.Sleep(time.Millisecond * 10)
						}
					} else {
						break
					}
				case 1:
					sm.Strategy.GetLogger().Info("orderId in check timeout")
					res := sm.tryCancelEntryOrder()
					// if ok then we canceled order and we can go to next iteration
					if res.Status == "OK" {
						sm.Strategy.GetLogger().Info("order canceled continue timeout code")
						break
					} else {
						// otherwise order was already filled
						sm.Strategy.GetLogger().Info("order already filled")
						sm.Lock = false
						return
					}
				default:
					if len(sm.Strategy.GetModel().Conditions.EntryLevels) > 0 {
						sm.TryCancelAllOrders(sm.Strategy.GetModel().State.Orders)
					} else {
						sm.Lock = false
						return
					}
				}

				err := sm.FireTrigger(TriggerTimeout)
				if err != nil {
					sm.Strategy.GetLogger().Warn("fire checkTimeout err",
						zap.String("err", err.Error()),
					)
				}
				sm.Strategy.GetModel().State.State = Timeout
				sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)
				//log.Print("updated state to Timeout, pair, enabled", sm.Strategy.GetModel().Conditions.Pair, sm.Strategy.GetModel().Enabled)
				sm.Lock = false
			}
		}(sm.Strategy.GetModel().State.Iteration)
	}

	if sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice != 0 &&
		sm.Strategy.GetModel().Conditions.ActivationMoveTimeout > 0 {
		go func() {
			currentState, _ := sm.GetState(context.TODO())
			for currentState == WaitForEntry && sm.Strategy.GetModel().Enabled {
				time.Sleep(time.Duration(sm.Strategy.GetModel().Conditions.ActivationMoveTimeout) * time.Second)
				currentState, _ = sm.GetState(context.TODO())
				if currentState == WaitForEntry && sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice != 0 {
					activatePrice := sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice
					side := sm.Strategy.GetModel().Conditions.EntryOrder.Side
					if side == "sell" {
						activatePrice = activatePrice * (1 - sm.Strategy.GetModel().Conditions.ActivationMoveStep/100/sm.Strategy.GetModel().Conditions.Leverage)
					} else {
						activatePrice = activatePrice * (1 + sm.Strategy.GetModel().Conditions.ActivationMoveStep/100/sm.Strategy.GetModel().Conditions.Leverage)
					}
					sm.Strategy.GetLogger().Info("changed activate price",
						zap.Float64("from", sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice),
						zap.Float64("to", activatePrice),
					)
					sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice = activatePrice
				}
			}
		}()
	}

}

func (sm *SmartOrder) tryCancelEntryOrder() orders.OrderResponse {
	orderId := sm.Strategy.GetModel().State.Orders[0]
	sm.Strategy.GetLogger().Info("orderId in check timeout")
	var res orders.OrderResponse
	if orderId != "0" {
		res = sm.ExchangeApi.CancelOrder(orders.CancelOrderRequest{
			KeyId: sm.KeyId,
			KeyParams: orders.CancelOrderRequestParams{
				OrderId:    orderId,
				MarketType: sm.Strategy.GetModel().Conditions.MarketType,
				Pair:       sm.Strategy.GetModel().Conditions.Pair,
			},
		})
	} else {
		res = orders.OrderResponse{
			Status: "ERR",
		}
	}
	return res
}
