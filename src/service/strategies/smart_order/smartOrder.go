package smart_order

import (
	"context"
	"log"
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	WaitForEntry       = "WaitForEntry"
	TrailingEntry      = "TrailingEntry"
	PartiallyEntry     = "PartiallyEntry"
	InEntry            = "InEntry"
	InMultiEntry       = "InMultiEntry"
	TakeProfit         = "TakeProfit"
	Stoploss           = "Stoploss"
	WaitOrderOnTimeout = "WaitOrderOnTimeout"
	WaitOrder          = "WaitOrder"
	End                = "End"
	Canceled           = "Canceled"
	EnterNextTarget    = "EnterNextTarget"
	Timeout            = "Timeout"
	Error              = "Error"
	HedgeLoss          = "HedgeLoss"
	WaitLossHedge      = "WaitLossHedge"
)

const (
	TriggerTrade             = "Trade"
	TriggerSpread            = "Spread"
	TriggerOrderExecuted     = "TriggerOrderExecuted"
	CheckExistingOrders      = "CheckExistingOrders"
	CheckHedgeLoss           = "CheckHedgeLoss"
	CheckProfitTrade         = "CheckProfitTrade"
	CheckTrailingProfitTrade = "CheckTrailingProfitTrade"
	CheckTrailingLossTrade   = "CheckTrailingLossTrade"
	CheckSpreadProfitTrade   = "CheckSpreadProfitTrade"
	CheckLossTrade           = "CheckLossTrade"
	Restart                  = "Restart"
	ReEntry                  = "ReEntry"
	TriggerTimeout           = "TriggerTimeout"
)

type SmartOrder struct {
	Strategy                interfaces.IStrategy
	State                   *stateless.StateMachine
	ExchangeName            string
	KeyId                   *primitive.ObjectID
	DataFeed                interfaces.IDataFeed
	ExchangeApi             trading.ITrading
	StateMgmt               interfaces.IStateMgmt
	IsWaitingForOrder       sync.Map // TODO: this must be filled on start of SM if not first start (e.g. restore the state by checking order statuses)
	IsEntryOrderPlaced      bool     // we need it for case when response from createOrder was returned after entryTimeout was executed
	OrdersMap               map[string]bool
	StatusByOrderId         sync.Map
	QuantityAmountPrecision int64
	QuantityPricePrecision  int64
	Lock                    bool
	StopLock                bool
	LastTrailingTimestamp   int64
	SelectedExitTarget      int
	SelectedEntryTarget     int
	OrdersMux               sync.Mutex
	StopMux                 sync.Mutex
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func (sm *SmartOrder) toFixed(num float64, precision int64) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

func NewSmartOrder(strategy interfaces.IStrategy, DataFeed interfaces.IDataFeed, TradingAPI trading.ITrading, keyId *primitive.ObjectID, stateMgmt interfaces.IStateMgmt) *SmartOrder {

	sm := &SmartOrder{Strategy: strategy, DataFeed: DataFeed, ExchangeApi: TradingAPI, KeyId: keyId, StateMgmt: stateMgmt, Lock: false, SelectedExitTarget: 0, OrdersMap: map[string]bool{}}
	initState := WaitForEntry
	pricePrecision, amountPrecision := stateMgmt.GetMarketPrecision(strategy.GetModel().Conditions.Pair, strategy.GetModel().Conditions.MarketType)
	sm.QuantityPricePrecision = pricePrecision
	sm.QuantityAmountPrecision = amountPrecision
	// if state is not empty but if its in the end and open ended, then we skip state value, since want to start over
	if strategy.GetModel().State != nil && strategy.GetModel().State.State != "" && !(strategy.GetModel().State.State == End && strategy.GetModel().Conditions.ContinueIfEnded == true) {
		initState = strategy.GetModel().State.State
	}
	State := stateless.NewStateMachineWithMode(initState, 1)

	// define triggers and input types:
	State.SetTriggerParameters(TriggerTrade, reflect.TypeOf(interfaces.OHLCV{}))
	State.SetTriggerParameters(CheckExistingOrders, reflect.TypeOf(models.MongoOrder{}))
	State.SetTriggerParameters(TriggerSpread, reflect.TypeOf(interfaces.SpreadData{}))

	/*
		Smart Order life cycle:
			1) first need to go into entry, so we wait for entry and put orders before if possible
			2) we may go into waiting for trailing entry if activate price was specified ( and placing stop-limit/market orders to catch entry, and cancel existing )
			3) ok we are in entry and now wait for profit or loss ( also we can try to place all orders )
			4) we may go to exit on timeout if profit/loss
			5) so we'll wait for any target or trailing target
			6) or stop-loss
	*/
	State.Configure(WaitForEntry).PermitDynamic(TriggerTrade, sm.exitWaitEntry,
		sm.checkWaitEntry).PermitDynamic(TriggerSpread, sm.exitWaitEntry,
		sm.checkSpreadEntry).PermitDynamic(CheckExistingOrders, sm.exitWaitEntry,
		sm.checkExistingOrders).Permit(TriggerTimeout, Timeout).OnEntry(sm.onStart)

	State.Configure(TrailingEntry).Permit(TriggerTrade, InEntry,
		sm.checkTrailingEntry).Permit(CheckExistingOrders, InEntry,
		sm.checkExistingOrders).OnEntry(sm.enterTrailingEntry)

	State.Configure(InEntry).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders).PermitDynamic(CheckHedgeLoss, sm.exit,
		sm.checkLossHedge).OnEntry(sm.enterEntry)

	State.Configure(InMultiEntry).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders).PermitReentry(CheckExistingOrders,
		sm.checkExistingOrders).OnEntry(sm.entryMultiEntry)

	State.Configure(WaitLossHedge).PermitDynamic(CheckHedgeLoss, sm.exit,
		sm.checkLossHedge).OnEntry(sm.enterWaitLossHedge)

	State.Configure(TakeProfit).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders).PermitDynamic(CheckHedgeLoss, sm.exit,
		sm.checkLossHedge).OnEntry(sm.enterTakeProfit)

	State.Configure(Stoploss).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders).PermitDynamic(CheckHedgeLoss, sm.exit,
		sm.checkLossHedge).OnEntry(sm.enterStopLoss)

	State.Configure(HedgeLoss).PermitDynamic(CheckTrailingLossTrade, sm.exit,
		sm.checkTrailingHedgeLoss).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders)

	State.Configure(Timeout).Permit(Restart, WaitForEntry)

	State.Configure(End).PermitReentry(CheckExistingOrders, sm.checkExistingOrders).OnEntry(sm.enterEnd)

	_ = State.Activate()

	sm.State = State
	sm.ExchangeName = "binance"
	// fmt.Printf(sm.State.ToGraph())
	// fmt.Printf("DONE\n")
	_ = sm.onStart(nil)
	return sm
}

func (sm *SmartOrder) checkIfShouldCancelIfAnyActive() {
	model := sm.Strategy.GetModel()

	if model.Conditions.CancelIfAnyActive && sm.StateMgmt.AnyActiveStrats(model) {
		model.Enabled = false
		sm.Strategy.GetModel().State.State = Canceled
		sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)
	}
}

func (sm *SmartOrder) onStart(ctx context.Context, args ...interface{}) error {
	log.Print("onStart executed")
	sm.checkIfShouldCancelIfAnyActive()
	sm.hedge()
	sm.checkIfPlaceOrderInstantlyOnStart()
	go sm.checkTimeouts()
	return nil
}

func (sm *SmartOrder) checkIfPlaceOrderInstantlyOnStart() {
	model := sm.Strategy.GetModel()
	isMultiEntry := len(model.Conditions.EntryLevels) > 0
	isFirstRunSoStateIsEmpty := model.State.State == "" ||
		(model.State.State == WaitForEntry && model.Conditions.ContinueIfEnded && model.Conditions.WaitingEntryTimeout > 0)
	if isFirstRunSoStateIsEmpty && model.Enabled &&
		!model.Conditions.EntrySpreadHunter && !isMultiEntry {
		entryIsNotTrailing := model.Conditions.EntryOrder.ActivatePrice == 0
		if entryIsNotTrailing { // then we must know exact price
			sm.IsWaitingForOrder.Store(WaitForEntry, true)
			sm.PlaceOrder(model.Conditions.EntryOrder.Price, 0.0, WaitForEntry)
		}
	}
	if isMultiEntry {
		sm.placeMultiEntryOrders()
	}
}

func (sm *SmartOrder) placeMultiEntryOrders() {
	// we execute this func again for 1 option
	go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.Orders)

	model := sm.Strategy.GetModel()
	sm.SelectedEntryTarget = 0
	currentPrice := model.Conditions.EntryLevels[0].Price

	sumAmount := 0.0
	sumTotal := 0.0

	// here we should place all entry orders
	for _, target := range model.Conditions.EntryLevels {
		currentAmount := 0.0

		if target.Type == 0 {
			currentAmount = target.Amount
			currentPrice = target.Price
			sm.PlaceOrder(currentPrice, currentAmount, WaitForEntry)

		} else {
			currentAmount = model.Conditions.EntryOrder.Amount / 100 * target.Amount
			if model.Conditions.EntryOrder.Side == "buy" {
				currentPrice = currentPrice * (100 - target.Price/model.Conditions.Leverage) / 100
			} else {
				currentPrice = currentPrice * (100 + target.Price/model.Conditions.Leverage) / 100
			}
			sm.PlaceOrder(currentPrice, currentAmount, WaitForEntry)
		}

		sumAmount += currentAmount
		sumTotal += currentAmount * currentPrice
	}

	// here we should place one SL for all entries
	//weightedAverage := sumTotal / sumAmount
	sm.PlaceOrder(currentPrice, 0.0, Stoploss)
	if model.Conditions.ForcedLoss > 0 {
		sm.PlaceOrder(currentPrice, 0.0, "ForcedLoss")
	}
}

func (sm *SmartOrder) getLastTargetAmount() float64 {
	sumAmount := 0.0
	length := len(sm.Strategy.GetModel().Conditions.ExitLevels)
	for i, target := range sm.Strategy.GetModel().Conditions.ExitLevels {
		if i < length-1 {
			baseAmount := target.Amount
			if target.Type == 1 {
				if baseAmount == 0 {
					baseAmount = 100
				}
				baseAmount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount * (baseAmount / 100)
			}
			baseAmount = sm.toFixed(baseAmount, sm.QuantityAmountPrecision)
			sumAmount += baseAmount
		}
	}
	endTargetAmount := sm.Strategy.GetModel().Conditions.EntryOrder.Amount - sumAmount
	return endTargetAmount
}

func (sm *SmartOrder) exitWaitEntry(ctx context.Context, args ...interface{}) (stateless.State, error) {
	if sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice != 0 {
		log.Print("move to", TrailingEntry)
		return TrailingEntry, nil
	}
	if len(sm.Strategy.GetModel().Conditions.EntryLevels) > 0 {
		log.Print("move to ", InMultiEntry)
		return InMultiEntry, nil
	}
	log.Print("move to", InEntry)
	return InEntry, nil
}

func (sm *SmartOrder) checkWaitEntry(ctx context.Context, args ...interface{}) bool {
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(WaitForEntry)
	if ok && isWaitingForOrder.(bool) {
		return false
	}
	if sm.Strategy.GetModel().Conditions.EntrySpreadHunter {
		return false
	}
	currentOHLCV := args[0].(interfaces.OHLCV)
	model := sm.Strategy.GetModel()
	conditionPrice := model.Conditions.EntryOrder.Price
	isInstantMarketOrder := model.Conditions.EntryOrder.ActivatePrice == 0 && model.Conditions.EntryOrder.OrderType == "market"
	if model.Conditions.EntryOrder.ActivatePrice == -1 || isInstantMarketOrder {
		return true
	}
	isTrailing := model.Conditions.EntryOrder.ActivatePrice != 0
	if isTrailing {
		conditionPrice = model.Conditions.EntryOrder.ActivatePrice
	}

	switch model.Conditions.EntryOrder.Side {
	case "buy":
		if currentOHLCV.Close <= conditionPrice {
			return true
		}
		break
	case "sell":
		if currentOHLCV.Close >= conditionPrice {
			return true
		}
		break
	}

	return false
}

func (sm *SmartOrder) entryMultiEntry(ctx context.Context, args ...interface{}) error {
	sm.StopMux.Lock()
	log.Print("entryMultiEntry")
	model := sm.Strategy.GetModel()

	isWaitingStopLoss, stopLossOk := sm.IsWaitingForOrder.Load(Stoploss)
	if (!stopLossOk || !isWaitingStopLoss.(bool)) && len(model.State.StopLossOrderIds) == 0 {
		sm.IsWaitingForOrder.Store(Stoploss, true)
		// if stop loss was not placed in the start then we should wait some time before all entry will be executed
		time.AfterFunc(3 * time.Second, func() {sm.PlaceOrder(model.State.EntryPrice, 0.0, Stoploss)})
	}

	isWaitingForcedLoss, forcedLossOk := sm.IsWaitingForOrder.Load("ForcedLoss")
	if model.Conditions.ForcedLoss > 0 && (!forcedLossOk || !isWaitingForcedLoss.(bool)) && len(model.State.ForcedLossOrderIds) == 0 {
		sm.IsWaitingForOrder.Store("ForcedLoss", true)
		time.AfterFunc(3 * time.Second, func() {sm.PlaceOrder(model.State.EntryPrice, 0.0, "ForcedLoss")})
	}

	if model.Conditions.EntryLevels[sm.SelectedEntryTarget].PlaceWithoutLoss {
		sm.PlaceOrder(0, 0.0, "WithoutLoss")
	}

	// cancel old TAP
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	if ok && isWaitingForOrder.(bool) {
		state, _ := sm.State.State(ctx)
		if state == End {
			return nil
		}
	}

	sm.IsWaitingForOrder.Store(TakeProfit, true)
	ids := model.State.TakeProfitOrderIds[:]
	sm.TryCancelAllOrdersConsistently(ids)
	sm.PlaceOrder(0, 0.0, TakeProfit)

	sm.SelectedEntryTarget += 1
	sm.StopMux.Unlock()
	return nil
}

func (sm *SmartOrder) enterEntry(ctx context.Context, args ...interface{}) error {

	if len(sm.Strategy.GetModel().Conditions.EntryLevels) > 0 {
		return nil
	}
	//log.Print("enter entry")
	isSpot := sm.Strategy.GetModel().Conditions.MarketType == 0
	// if we returned to InEntry from Stoploss timeout
	if sm.Strategy.GetModel().State.StopLossAt == -1 {
		sm.Strategy.GetModel().State.StopLossAt = 0
		sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)

		if isSpot {
			sm.TryCancelAllOrdersConsistently(sm.Strategy.GetModel().State.Orders)
			sm.PlaceOrder(0, 0.0, TakeProfit)
		}
		return nil
	}
	if currentOHLCV, ok := args[0].(interfaces.OHLCV); ok {
		sm.Strategy.GetModel().State.EntryPrice = currentOHLCV.Close
	}
	sm.Strategy.GetModel().State.State = InEntry
	sm.Strategy.GetModel().State.TrailingEntryPrice = 0
	sm.Strategy.GetModel().State.ExecutedOrders = []string{}
	go sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)

	//if !sm.Strategy.GetModel().Conditions.EntrySpreadHunter {
	if isSpot {
		sm.PlaceOrder(sm.Strategy.GetModel().State.EntryPrice, 0.0, InEntry)
		if !sm.Strategy.GetModel().Conditions.TakeProfitExternal {
			log.Print("place take-profit from enterEntry")
			sm.PlaceOrder(0, 0.0, TakeProfit)
		}
	} else {
		go sm.PlaceOrder(sm.Strategy.GetModel().State.EntryPrice, 0.0, InEntry)
		if !sm.Strategy.GetModel().Conditions.TakeProfitExternal {
			sm.PlaceOrder(0, 0.0, TakeProfit)
		}
	}
	//}

	if !sm.Strategy.GetModel().Conditions.StopLossExternal && !isSpot {
		go sm.PlaceOrder(0, 0.0, Stoploss)
	}

	forcedLossOnSpot := !isSpot || (sm.Strategy.GetModel().Conditions.MandatoryForcedLoss && sm.Strategy.GetModel().Conditions.TakeProfitExternal)

	if sm.Strategy.GetModel().Conditions.ForcedLoss > 0 && forcedLossOnSpot &&
		(!sm.Strategy.GetModel().Conditions.StopLossExternal || sm.Strategy.GetModel().Conditions.MandatoryForcedLoss) {
		go sm.PlaceOrder(0, 0.0, "ForcedLoss")
	}
	// wait for creating hedgeStrategy
	if sm.Strategy.GetModel().Conditions.Hedging && sm.Strategy.GetModel().Conditions.HedgeStrategyId == nil ||
		sm.Strategy.GetModel().Conditions.HedgeStrategyId != nil {
		for {
			if sm.Strategy.GetModel().Conditions.HedgeStrategyId == nil {
				time.Sleep(1 * time.Second)
			} else {
				go sm.waitForHedge()
				return nil
			}
		}
	}

	return nil
}

func (sm *SmartOrder) checkProfit(ctx context.Context, args ...interface{}) bool {
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	model := sm.Strategy.GetModel()
	if ok &&
		isWaitingForOrder.(bool) &&
		model.Conditions.TimeoutIfProfitable == 0 &&
		model.Conditions.TimeoutLoss == 0 &&
		model.Conditions.WithoutLossAfterProfit == 0 {
		return false
	}

	if model.Conditions.TakeProfitExternal || model.Conditions.TakeProfitSpreadHunter {
		return false
	}
	if len(sm.Strategy.GetModel().Conditions.EntryLevels) > 0 {
		return false
	}

	currentOHLCV := args[0].(interfaces.OHLCV)
	if model.Conditions.TimeoutIfProfitable > 0 {
		isProfitable := (model.Conditions.EntryOrder.Side == "buy" && model.State.EntryPrice < currentOHLCV.Close) ||
			(model.Conditions.EntryOrder.Side == "sell" && model.State.EntryPrice > currentOHLCV.Close)
		if isProfitable && model.State.ProfitableAt == 0 {
			model.State.ProfitableAt = time.Now().Unix()
			go func(profitableAt int64) {
				time.Sleep(time.Duration(model.Conditions.TimeoutIfProfitable) * time.Second)
				stillTimeout := profitableAt == model.State.ProfitableAt
				if stillTimeout {
					sm.PlaceOrder(-1, 0.0, TakeProfit)
				}
			}(model.State.ProfitableAt)
		} else if !isProfitable {
			model.State.ProfitableAt = 0
		}
	}

	if model.Conditions.ExitLevels != nil {
		amount := 0.0
		switch model.Conditions.EntryOrder.Side {
		case "buy":
			for i, level := range model.Conditions.ExitLevels {
				if model.State.ReachedTargetCount < i+1 && level.ActivatePrice == 0 {
					if level.Type == 1 && currentOHLCV.Close >= (model.State.EntryPrice*(100+level.Price/model.Conditions.Leverage)/100) ||
						level.Type == 0 && currentOHLCV.Close >= level.Price {
						model.State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += model.Conditions.EntryOrder.Amount * (level.Amount / 100)
						}
					}
				}

				//if currentOHLCV.Close >= model.State.EntryPrice * (1 + model.Conditions.WithoutLossAfterProfit/model.Conditions.Leverage/100){
				//	sm.PlaceOrder(-1, "WithoutLoss")
				//}
			}
			break
		case "sell":
			for i, level := range model.Conditions.ExitLevels {
				if model.State.ReachedTargetCount < i+1 && level.ActivatePrice == 0 {
					if level.Type == 1 && currentOHLCV.Close <= (model.State.EntryPrice*((100-level.Price/model.Conditions.Leverage)/100)) ||
						level.Type == 0 && currentOHLCV.Close <= level.Price {
						model.State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += model.Conditions.EntryOrder.Amount * (level.Amount / 100)
						}
					}
					//
					//if currentOHLCV.Close <= model.State.EntryPrice * (1 - model.Conditions.WithoutLossAfterProfit/model.Conditions.Leverage/100){
					//	sm.PlaceOrder(-1, "WithoutLoss")
					//}
				}
			}
			break
		}
		if model.State.ExecutedAmount == model.Conditions.EntryOrder.Amount {
			model.State.Amount = 0
			model.State.State = TakeProfit // took all profits, exit now
			sm.StateMgmt.UpdateState(model.ID, model.State)
			return true
		}
		if amount > 0 {
			model.State.Amount = amount
			model.State.State = TakeProfit
			currentState, _ := sm.State.State(context.Background())
			if currentState == TakeProfit {
				model.State.State = EnterNextTarget
				sm.StateMgmt.UpdateState(model.ID, model.State)
			}
			return true
		}
	}
	return false
}

func (sm *SmartOrder) checkLoss(ctx context.Context, args ...interface{}) bool {

	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(Stoploss)
	model := sm.Strategy.GetModel()
	existTimeout := model.Conditions.TimeoutWhenLoss != 0 || model.Conditions.TimeoutLoss != 0 || model.Conditions.ForcedLoss != 0
	isSpot := model.Conditions.MarketType == 0
	forcedSLWithAlert := model.Conditions.StopLossExternal && model.Conditions.MandatoryForcedLoss

	if ok && isWaitingForOrder.(bool) && !existTimeout && !model.Conditions.MandatoryForcedLoss {
		return false
	}
	if model.Conditions.StopLossExternal && !model.Conditions.MandatoryForcedLoss {
		return false
	}
	if model.State.ExecutedAmount >= model.Conditions.EntryOrder.Amount {
		return false
	}
	if len(sm.Strategy.GetModel().Conditions.EntryLevels) > 0 {
		return false
	}

	currentOHLCV := args[0].(interfaces.OHLCV)
	isTrailingHedgeOrder := model.Conditions.HedgeStrategyId != nil || model.Conditions.Hedging == true
	if isTrailingHedgeOrder {
		return false
	}
	stopLoss := model.Conditions.StopLoss / model.Conditions.Leverage
	forcedLoss := model.Conditions.ForcedLoss / model.Conditions.Leverage
	currentState := model.State.State
	stateFromStateMachine, _ := sm.State.State(ctx)

	// if we did not have time to go out to check existing orders (market)
	if currentState == End || stateFromStateMachine == End {
		return true
	}

	// after return from Stoploss state to InEntry we should change state in machine
	if stateFromStateMachine == Stoploss && model.State.StopLossAt == -1 && model.Conditions.TimeoutLoss > 0 {
		return true
	}

	// try exit on timeout
	if model.Conditions.TimeoutWhenLoss > 0 && !forcedSLWithAlert {
		isLoss := (model.Conditions.EntryOrder.Side == "buy" && model.State.EntryPrice > currentOHLCV.Close) || (model.Conditions.EntryOrder.Side == "sell" && model.State.EntryPrice < currentOHLCV.Close)
		if isLoss && model.State.LossableAt == 0 {
			model.State.LossableAt = time.Now().Unix()
			go func(lossAt int64) {
				time.Sleep(time.Duration(model.Conditions.TimeoutWhenLoss) * time.Second)
				stillTimeout := lossAt == model.State.LossableAt
				if stillTimeout {
					sm.PlaceOrder(-1, 0.0, Stoploss)
				}
			}(model.State.LossableAt)
		} else if !isLoss {
			model.State.LossableAt = 0
		}
		return false
	}

	switch model.Conditions.EntryOrder.Side {
	case "buy":
		if isSpot && forcedLoss > 0 && (1-currentOHLCV.Close/model.State.EntryPrice)*100 >= forcedLoss {
			sm.PlaceOrder(currentOHLCV.Close, 0.0, Stoploss)
			return false
		}

		if forcedSLWithAlert {
			return false
		}

		if (1-currentOHLCV.Close/model.State.EntryPrice)*100 >= stopLoss {
			if model.State.ExecutedAmount < model.Conditions.EntryOrder.Amount {
				model.State.Amount = model.Conditions.EntryOrder.Amount - model.State.ExecutedAmount
			}

			// if order go to StopLoss from InEntry state or returned to Stoploss while timeout
			if currentState == InEntry {
				model.State.State = Stoploss
				sm.StateMgmt.UpdateState(model.ID, model.State)

				if model.State.StopLossAt == 0 {
					return true
				}
			}
		} else {
			if currentState == Stoploss {
				model.State.State = InEntry
				sm.StateMgmt.UpdateState(model.ID, model.State)
			}
		}
		break
	case "sell":
		if isSpot && forcedLoss > 0 && (currentOHLCV.Close/model.State.EntryPrice-1)*100 >= forcedLoss {
			sm.PlaceOrder(currentOHLCV.Close, 0.0, Stoploss)
			return false
		}

		if forcedSLWithAlert {
			return false
		}

		if (currentOHLCV.Close/model.State.EntryPrice-1)*100 >= stopLoss {
			if model.State.ExecutedAmount < model.Conditions.EntryOrder.Amount {
				model.State.Amount = model.Conditions.EntryOrder.Amount - model.State.ExecutedAmount
			}

			if currentState == InEntry {
				model.State.State = Stoploss
				sm.StateMgmt.UpdateState(model.ID, model.State)

				if model.State.StopLossAt == 0 {
					return true
				}
			}
		} else {
			if currentState == Stoploss {
				model.State.State = InEntry
				sm.StateMgmt.UpdateState(model.ID, model.State)
			}
		}
		break
	}

	return false
}
func (sm *SmartOrder) enterEnd(ctx context.Context, args ...interface{}) error {
	sm.Strategy.GetModel().State.State = End
	sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)
	return nil
}

func (sm *SmartOrder) enterTakeProfit(ctx context.Context, args ...interface{}) error {
	if currentOHLCV, ok := args[0].(interfaces.OHLCV); ok {
		if sm.Strategy.GetModel().State.Amount > 0 {

			if sm.Lock {
				return nil
			}
			sm.Lock = true
			sm.PlaceOrder(currentOHLCV.Close, 0.0, TakeProfit)

			//if sm.Strategy.GetModel().State.ExecutedAmount - sm.Strategy.GetModel().Conditions.EntryOrder.Amount == 0 { // re-check all takeprofit conditions to exit trade ( no additional trades needed no )
			//	ohlcv := args[0].(OHLCV)
			//	err := sm.State.FireCtx(context.TODO(), TriggerTrade, ohlcv)
			//
			//	sm.Lock = false
			//	return err
			//}
		}
		sm.Lock = false
	}
	return nil
}

func (sm *SmartOrder) enterStopLoss(ctx context.Context, args ...interface{}) error {
	if currentOHLCV, ok := args[0].(interfaces.OHLCV); ok {
		if sm.Strategy.GetModel().State.Amount > 0 {
			side := "buy"
			if sm.Strategy.GetModel().Conditions.EntryOrder.Side == side {
				side = "sell"
			}
			if sm.Lock {
				return nil
			}
			sm.Lock = true
			if sm.Strategy.GetModel().Conditions.MarketType == 0 {
				if len(sm.Strategy.GetModel().State.Orders) < 2 {
					// if we go to stop loss once place TAP and didn't receive TAP id yet
					time.Sleep(3 * time.Second)
				}
				sm.TryCancelAllOrdersConsistently(sm.Strategy.GetModel().State.Orders)
			}
			// sm.cancelOpenOrders(sm.Strategy.GetModel().Conditions.Pair)
			defer sm.PlaceOrder(currentOHLCV.Close, 0.0, Stoploss)
		}
		// if timeout specified then do this sell on timeout
		sm.Lock = false
	}
	_ = sm.State.Fire(CheckLossTrade, args[0])
	return nil
}

func (sm *SmartOrder) SetSelectedExitTarget(selectedExitTarget int) {
	sm.SelectedExitTarget = selectedExitTarget
}

func (sm *SmartOrder) IsOrderExistsInMap(orderId string) bool {
	sm.OrdersMux.Lock()
	_, ok := sm.OrdersMap[orderId]
	sm.OrdersMux.Unlock()
	if ok {
		return true
	}

	return false
}

func (sm *SmartOrder) TryCancelAllOrdersConsistently(orderIds []string) {
	for _, orderId := range orderIds {
		if orderId != "0" {
			sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.CancelOrderRequestParams{
					OrderId:    orderId,
					MarketType: sm.Strategy.GetModel().Conditions.MarketType,
					Pair:       sm.Strategy.GetModel().Conditions.Pair,
				},
			})
		}
	}
}

func (sm *SmartOrder) TryCancelAllOrders(orderIds []string) {
	for _, orderId := range orderIds {
		if orderId != "0" {
			go sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.CancelOrderRequestParams{
					OrderId:    orderId,
					MarketType: sm.Strategy.GetModel().Conditions.MarketType,
					Pair:       sm.Strategy.GetModel().Conditions.Pair,
				},
			})
		}
	}
}

func (sm *SmartOrder) Start() {
	ctx := context.TODO()

	state, _ := sm.State.State(context.Background())
	localState := sm.Strategy.GetModel().State.State
	for state != End && localState != End && state != Canceled && state != Timeout {
		if sm.Strategy.GetModel().Enabled == false {
			state, _ = sm.State.State(ctx)
			break
		}
		if !sm.Lock {
			if sm.Strategy.GetModel().Conditions.EntrySpreadHunter && state != InEntry {
				sm.processSpreadEventLoop()
			} else {
				sm.processEventLoop()
			}
		}
		time.Sleep(60 * time.Millisecond)
		state, _ = sm.State.State(ctx)
		localState = sm.Strategy.GetModel().State.State
	}
	sm.Stop()
	log.Print("STOPPED smartorder ", state.(string))
}

func (sm *SmartOrder) Stop() {
	log.Print("stop func orders len", len(sm.Strategy.GetModel().State.Orders))
	if sm.StopLock {
		if sm.Strategy.GetModel().Conditions.ContinueIfEnded == false {
			sm.StateMgmt.DisableStrategy(sm.Strategy.GetModel().ID)
		}
		log.Print("cancel orders in stop at start")
		go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.Orders)
		return
	}
	sm.StopLock = true
	state, _ := sm.State.State(context.Background())
	if sm.Strategy.GetModel().Conditions.MarketType == 0 && state != End {
		log.Print("cancel orders in stop")
		sm.TryCancelAllOrdersConsistently(sm.Strategy.GetModel().State.Orders)
	} else {
		log.Print("cancel orders a bit lower than start of stop")
		go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.Orders)
	}
	StateS := sm.Strategy.GetModel().State.State
	if state != End && StateS != Timeout && sm.Strategy.GetModel().Conditions.EntrySpreadHunter == false {
		sm.PlaceOrder(0, 0.0, Canceled)
	}
	if sm.Strategy.GetModel().Conditions.ContinueIfEnded == false {
		sm.StateMgmt.DisableStrategy(sm.Strategy.GetModel().ID)
	}
	sm.StopLock = false
	if (StateS == Timeout || state == Timeout) && sm.Strategy.GetModel().Conditions.ContinueIfEnded == true && !sm.Strategy.GetModel().Conditions.PositionWasClosed {
		sm.IsWaitingForOrder = sync.Map{}
		sm.IsEntryOrderPlaced = false
		sm.StateMgmt.EnableStrategy(sm.Strategy.GetModel().ID)
		sm.Strategy.GetModel().Enabled = true
		stateModel := sm.Strategy.GetModel().State
		stateModel.State = WaitForEntry
		stateModel.EntryPrice = 0
		stateModel.ExecutedAmount = 0
		stateModel.Amount = 0
		stateModel.Orders = []string{}
		stateModel.Iteration += 1
		sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, stateModel)
		sm.StateMgmt.UpdateExecutedAmount(sm.Strategy.GetModel().ID, stateModel)
		sm.StateMgmt.SaveStrategyConditions(sm.Strategy.GetModel())
		_ = sm.State.Fire(Restart)
		//_ = sm.onStart(nil)
		sm.Start()
	}
}

func (sm *SmartOrder) processEventLoop() {
	currentOHLCVp := sm.DataFeed.GetPriceForPairAtExchange(sm.Strategy.GetModel().Conditions.Pair, sm.ExchangeName, sm.Strategy.GetModel().Conditions.MarketType)
	if currentOHLCVp != nil {
		currentOHLCV := *currentOHLCVp
		state, err := sm.State.State(context.TODO())
		err = sm.State.FireCtx(context.TODO(), TriggerTrade, currentOHLCV)
		if err == nil {
			return
		}
		if state == InEntry || state == TakeProfit || state == Stoploss || state == HedgeLoss {
			err = sm.State.FireCtx(context.TODO(), CheckLossTrade, currentOHLCV)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckProfitTrade, currentOHLCV)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckTrailingLossTrade, currentOHLCV)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckTrailingProfitTrade, currentOHLCV)
			if err == nil {
				return
			}
		}
		// log.Print(sm.Strategy.GetModel().Conditions.Pair, sm.Strategy.GetModel().State.TrailingEntryPrice, currentOHLCV.Close, err.Error())
	}
}

func (sm *SmartOrder) processSpreadEventLoop() {
	currentSpreadP := sm.DataFeed.GetSpreadForPairAtExchange(sm.Strategy.GetModel().Conditions.Pair, sm.ExchangeName, sm.Strategy.GetModel().Conditions.MarketType)
	if currentSpreadP != nil {
		currentSpread := *currentSpreadP
		ohlcv := interfaces.OHLCV{
			Close: currentSpread.BestBid,
		}
		state, err := sm.State.State(context.TODO())
		err = sm.State.FireCtx(context.TODO(), TriggerSpread, currentSpread)
		if err == nil {
			return
		}
		if state == InEntry || state == TakeProfit || state == Stoploss || state == HedgeLoss {
			err = sm.State.FireCtx(context.TODO(), CheckSpreadProfitTrade, currentSpread)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckLossTrade, ohlcv)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckProfitTrade, ohlcv)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckTrailingLossTrade, ohlcv)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckTrailingProfitTrade, ohlcv)
			if err == nil {
				return
			}
		}
		// log.Print(sm.Strategy.GetModel().Conditions.Pair, sm.Strategy.GetModel().State.TrailingEntryPrice, currentOHLCV.Close, err.Error())
	}
}
