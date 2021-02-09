package smart_order

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.uber.org/zap"
)

func (sm *SmartOrder) exit(ctx context.Context, args ...interface{}) (stateless.State, error) {
	state, _ := sm.State.State(context.TODO())
	nextState := End
	model := sm.Strategy.GetModel()
	amount := model.Conditions.EntryOrder.Amount
	if model.Conditions.MarketType == 0 {
		amount = amount * 0.99 // TODO(khassanov): why?
	}

	// TODO
	// here we should handle case when:
	// timeout started, state went to Stoploss but TakeProfit target executed so we go to End (coz this case not handled)
	// {"level":"info","ts":1612872344.9915164,"caller":"smart_order/exit.go:18","msg":"exiting with","logger":"sm-60227a7a7aa73b63a13a700e","smart order state":"Stoploss","model state":"TakeProfit","executed amount":0.002,"conditions entry amount":0.004,"is executed amount >= amount in exit":false}

	sm.Strategy.GetLogger().Info("exiting with",
		zap.String("smart order state", state.(string)),
		zap.String("model state", model.State.State),
		zap.Float64("executed amount", model.State.ExecutedAmount),
		zap.Float64("conditions entry amount", amount),
		zap.Bool("is executed amount >= amount in exit", model.State.ExecutedAmount >= amount),
	)
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
			sm.Strategy.GetLogger().Info("cancel all orders in exit")
			go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.Orders)

			newState := models.MongoStrategyState{
				State:          "",
				ExecutedAmount: 0,
				Amount:         0,
				Iteration:      sm.Strategy.GetModel().State.Iteration + 1,
			}
			model.State = &newState
			sm.IsEntryOrderPlaced = false
			sm.StateMgmt.UpdateExecutedAmount(model.ID, model.State)
			sm.StateMgmt.UpdateState(model.ID, &newState)
			sm.StateMgmt.SaveStrategyConditions(sm.Strategy.GetModel())
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
		case InMultiEntry:
			nextState = InMultiEntry
			break
		}
		break
	case InMultiEntry:
		switch model.State.State {
		case InMultiEntry:
			nextState = InMultiEntry
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
	sm.Strategy.GetLogger().Info("next state in end",
		zap.String("next state", nextState),
	)
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
