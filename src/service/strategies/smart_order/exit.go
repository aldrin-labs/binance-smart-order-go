package smart_order

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
)

func (sm *SmartOrder) exit(ctx context.Context, args ...interface{}) (stateless.State, error) {
	state, _ := sm.State.State(context.TODO())
	nextState := End
	model := sm.Strategy.GetModel()
	amount := model.Conditions.EntryOrder.Amount
	if model.Conditions.MarketType == 0 {
		amount = amount * 0.99
	}
	//println("model.State.ExecutedAmount >= amount  in exit", model.State.ExecutedAmount >= amount )
	if model.State.State != WaitLossHedge && model.State.ExecutedAmount >= amount { // all trades executed, nothing more to trade
		if model.Conditions.ContinueIfEnded {
			isParentHedge := model.Conditions.Hedging == true
			isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || isParentHedge

			if isTrailingHedgeOrder && !isParentHedge {
				return End, nil
			}
			//oppositeSide := model.Conditions.EntryOrder.Side
			//if oppositeSide == "buy" {
			//	oppositeSide = "sell"
			//} else {
			//	oppositeSide = "buy"
			//}
			//model.Conditions.EntryOrder.Side = oppositeSide
			if model.Conditions.EntryOrder.ActivatePrice > 0 {
				model.Conditions.EntryOrder.ActivatePrice = model.State.ExitPrice
			}
			go sm.StateMgmt.UpdateConditions(model.ID, model.Conditions)
			println("cancel all orders in exit")
			go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.Orders)

			newState := models.MongoStrategyState{
				State: "",
				ExecutedAmount: 0,
				Iteration: sm.Strategy.GetModel().State.Iteration + 1,
			}
			model.State = &newState
			sm.IsEntryOrderPlaced = false
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
			sm.StateMgmt.UpdateState(model.ID, &newState)
			sm.StateMgmt.SaveStrategyConditions(sm.Strategy.GetModel())
			println("go into WaitForEntry")
			return WaitForEntry, nil
		}
		return End, nil
	}
	switch state {
	case InEntry:
		switch model.State.State {
		case TakeProfit:
			nextState = TakeProfit
			break
		case Stoploss:
			nextState = Stoploss
			break
		case InEntry:
			nextState = InEntry
			break
		case HedgeLoss:
			nextState = HedgeLoss
			break
		case WaitLossHedge:
			nextState = WaitLossHedge
			break
		}
		break
	case TakeProfit:
		switch model.State.State {
		case "EnterNextTarget":
			nextState = TakeProfit
			break
		case TakeProfit:
			nextState = End
			break
		case Stoploss:
			nextState = Stoploss
			break
		case HedgeLoss:
			nextState = HedgeLoss
			break
		case WaitLossHedge:
			nextState = WaitLossHedge
			break
		}
		break
	case Stoploss:
		switch model.State.State {
		case InEntry:
			nextState = InEntry
			break
		case End:
			nextState = End
			break
		case HedgeLoss:
			nextState = HedgeLoss
			break
		case WaitLossHedge:
			nextState = WaitLossHedge
			break
		}
		break
	}
	if nextState == End && model.Conditions.ContinueIfEnded {
		newState := models.MongoStrategyState{
			State:              WaitForEntry,
			TrailingEntryPrice: 0,
			EntryPrice:         0,
			Amount:             0,
			Orders:             nil,
			ExecutedAmount:     0,
			ReachedTargetCount: 0,
		}
		sm.StateMgmt.UpdateState(model.ID, &newState)
		sm.StateMgmt.SaveStrategyConditions(sm.Strategy.GetModel())
		return WaitForEntry, nil
	}
	return nextState, nil
}
