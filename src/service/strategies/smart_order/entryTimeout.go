package smart_order

import (
	"context"
	"time"
)

func (sm *SmartOrder) checkTimeouts() {
	if sm.Strategy.GetModel().Conditions.WaitingEntryTimeout > 0 {
		go func() {
			time.Sleep(time.Duration(sm.Strategy.GetModel().Conditions.WaitingEntryTimeout) * time.Second)
			currentState, _ := sm.State.State(context.TODO())
			if (currentState == WaitForEntry || currentState == TrailingEntry) && sm.Lock == false {
				sm.Lock = true
				sm.Strategy.GetModel().Enabled = false
				sm.Strategy.GetModel().State.State = Timeout
				sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)
				sm.Lock = false
			}
		}()
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