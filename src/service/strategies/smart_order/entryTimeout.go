package smart_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"time"
)

func (sm *SmartOrder) checkTimeouts() {
	if sm.Strategy.GetModel().Conditions.WaitingEntryTimeout > 0 {
		go func(iteration int) {
			time.Sleep(time.Duration(sm.Strategy.GetModel().Conditions.WaitingEntryTimeout) * time.Second)
			currentState, _ := sm.State.State(context.TODO())
			if (currentState == WaitForEntry || currentState == TrailingEntry) && sm.Lock == false && iteration == sm.Strategy.GetModel().State.Iteration {
				sm.Lock = true
				switch len(sm.Strategy.GetModel().State.Orders) {
				case 0:
					// rare case when we have already placed entry order, but sm.Strategy.GetModel().State.Orders is empty (so we didn't receive response for this order)
					if sm.IsEntryOrderPlaced {
						count := 0
						for {
							// 5000 tries - 5 sec
							if count > 10 * 100 * 5 {
								// error in entry order
								break
							}
							// received update
							if len(sm.Strategy.GetModel().State.Orders) > 0 {
								res := sm.tryCancelEntryOrder()
								if res.Status == "OK" {
									println("order canceled continue timeout code")
									break
								} else {
									println("order already filled")
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
					println("orderId in check timeout")
					res := sm.tryCancelEntryOrder()
					if sm.Strategy.GetModel().State.Amount > 0 {
						// if entry order was partially filled
						println("amount in check sm.Strategy.GetModel().State.Amount pair", sm.Strategy.GetModel().State.Amount, sm.Strategy.GetModel().Conditions.Pair)
						sm.Lock = false
						return
					}
					// if ok then we canceled order and we can go to next iteration
					if res.Status == "OK" {
						println("order canceled continue timeout code")
						break
					} else {
						// otherwise order was already filled
						println("order already filled")
						sm.Lock = false
						return
					}
				default:
					sm.Lock = false
					return
				}

				err := sm.State.Fire(TriggerTimeout)
				if err != nil {
					println("fire checkTimeout err", err.Error())
				}
				sm.Strategy.GetModel().State.State = Timeout
				sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)
				//println("updated state to Timeout, pair, enabled", sm.Strategy.GetModel().Conditions.Pair, sm.Strategy.GetModel().Enabled)
				sm.Lock = false
			}
		}(sm.Strategy.GetModel().State.Iteration)
	}

	if sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice != 0 &&
		sm.Strategy.GetModel().Conditions.ActivationMoveTimeout > 0 {
		go func() {
			currentState, _ := sm.State.State(context.TODO())
			for currentState == WaitForEntry && sm.Strategy.GetModel().Enabled {
				time.Sleep(time.Duration(sm.Strategy.GetModel().Conditions.ActivationMoveTimeout) * time.Second)
				currentState, _ = sm.State.State(context.TODO())
				if currentState == WaitForEntry && sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice != 0 {
					activatePrice := sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice
					side := sm.Strategy.GetModel().Conditions.EntryOrder.Side
						if side == "sell" {
						activatePrice = activatePrice * (1 - sm.Strategy.GetModel().Conditions.ActivationMoveStep/100/sm.Strategy.GetModel().Conditions.Leverage)
					} else {
						activatePrice = activatePrice * (1 + sm.Strategy.GetModel().Conditions.ActivationMoveStep/100/sm.Strategy.GetModel().Conditions.Leverage)
					}
					sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice = activatePrice
				}
			}
		}()
	}

}

func (sm *SmartOrder) tryCancelEntryOrder() trading.OrderResponse {
	orderId := sm.Strategy.GetModel().State.Orders[0]
	println("orderId in check timeout")
	var res trading.OrderResponse
	if orderId != "0" {
		res = sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
			KeyId: sm.KeyId,
			KeyParams: trading.CancelOrderRequestParams{
				OrderId:    orderId,
				MarketType: sm.Strategy.GetModel().Conditions.MarketType,
				Pair:       sm.Strategy.GetModel().Conditions.Pair,
			},
		})
	} else {
		res = trading.OrderResponse{
			Status: "ERR",
		}
	}
	return res
}