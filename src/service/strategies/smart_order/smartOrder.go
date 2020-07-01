package smart_order

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math"
	"reflect"
	"sync"
	"time"
)

const (
	WaitForEntry       = "WaitForEntry"
	TrailingEntry      = "TrailingEntry"
	PartiallyEntry     = "PartiallyEntry"
	InEntry            = "InEntry"
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
	TriggerOrderExecuted     = "TriggerOrderExecuted"
	CheckExistingOrders      = "CheckExistingOrders"
	CheckHedgeLoss           = "CheckHedgeLoss"
	CheckProfitTrade         = "CheckProfitTrade"
	CheckTrailingProfitTrade = "CheckTrailingProfitTrade"
	CheckTrailingLossTrade   = "CheckTrailingLossTrade"
	CheckLossTrade           = "CheckLossTrade"
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
	OrdersMap               map[string]bool
	StatusByOrderId         sync.Map
	QuantityAmountPrecision int64
	QuantityPricePrecision  int64
	Lock                    bool
	StopLock				bool
	LastTrailingTimestamp   int64
	SelectedExitTarget      int
	OrdersMux sync.Mutex
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
	State := stateless.NewStateMachine(initState)

	// define triggers and input types:
	State.SetTriggerParameters(TriggerTrade, reflect.TypeOf(interfaces.OHLCV{}))
	State.SetTriggerParameters(CheckExistingOrders, reflect.TypeOf(models.MongoOrder{}))

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
		sm.checkWaitEntry).PermitDynamic(CheckExistingOrders, sm.exitWaitEntry,
		sm.checkExistingOrders).OnEntry(sm.onStart)
	State.Configure(TrailingEntry).Permit(TriggerTrade, InEntry,
		sm.checkTrailingEntry).Permit(CheckExistingOrders, InEntry,
		sm.checkExistingOrders).OnEntry(sm.enterTrailingEntry)

	State.Configure(InEntry).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders).PermitDynamic(CheckHedgeLoss, sm.exit,
		sm.checkLossHedge).OnEntry(sm.enterEntry)

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

	State.Configure(End).OnEntry(sm.enterEnd)

	State.Activate()

	sm.State = State
	sm.ExchangeName = "binance"
	// fmt.Printf(sm.State.ToGraph())
	// fmt.Printf("DONE\n")
	_ = sm.onStart(nil)
	return sm
}

func (sm *SmartOrder) checkIfShouldCancelIfAnyActive(){
	model := sm.Strategy.GetModel()

	if model.Conditions.CancelIfAnyActive && sm.StateMgmt.AnyActiveStrats(model) {
		model.Enabled = false
		sm.Strategy.GetModel().State.State = Canceled
		sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)
	}
}

func (sm *SmartOrder) onStart(ctx context.Context, args ...interface{}) error {
	sm.checkIfShouldCancelIfAnyActive()
	sm.hedge()
	sm.checkIfPlaceOrderInstantlyOnStart()
	go sm.checkTimeouts()
	return nil
}

func (sm *SmartOrder) checkIfPlaceOrderInstantlyOnStart() {
	isFirstRunSoStateisEmpty := sm.Strategy.GetModel().State.State == ""
	if isFirstRunSoStateisEmpty && sm.Strategy.GetModel().Enabled {
		entryIsNotTrailing := sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice == 0
		if entryIsNotTrailing { // then we must know exact price
			sm.IsWaitingForOrder.Store(WaitForEntry, true)
			sm.PlaceOrder(sm.Strategy.GetModel().Conditions.EntryOrder.Price, WaitForEntry)
		}
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
		println("move to", TrailingEntry)
		return TrailingEntry, nil
	}
	println("move to", InEntry)
	return InEntry, nil
}
func (sm *SmartOrder) checkWaitEntry(ctx context.Context, args ...interface{}) bool {
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(WaitForEntry)
	if ok && isWaitingForOrder.(bool) {
		return false
	}
	currentOHLCV := args[0].(interfaces.OHLCV)
	model := sm.Strategy.GetModel()
	conditionPrice := model.Conditions.EntryOrder.Price
	isInstantMarketOrder := model.Conditions.EntryOrder.ActivatePrice == 0 && model.Conditions.EntryOrder.OrderType == "market"
	if  model.Conditions.EntryOrder.ActivatePrice == -1 || isInstantMarketOrder {
		return true
	}
	isTrailing := model.Conditions.EntryOrder.ActivatePrice != 0
	if isTrailing {
		conditionPrice = model.Conditions.EntryOrder.ActivatePrice
	}
	println("conditionPrice, OHLCV", conditionPrice, currentOHLCV.Close)
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

func (sm *SmartOrder) enterEntry(ctx context.Context, args ...interface{}) error {
	isSpot := sm.Strategy.GetModel().Conditions.MarketType == 0
	// if we returned to InEntry from Stoploss timeout
	if sm.Strategy.GetModel().State.StopLossAt == -1 {
		sm.Strategy.GetModel().State.StopLossAt = 0
		sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)

		if isSpot {
			sm.TryCancelAllOrdersConsistently(sm.Strategy.GetModel().State.Orders)
			sm.PlaceOrder(0, TakeProfit)
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

	if isSpot {
		sm.PlaceOrder(sm.Strategy.GetModel().State.EntryPrice, InEntry)
		if !sm.Strategy.GetModel().Conditions.TakeProfitExternal {
			sm.PlaceOrder(0, TakeProfit)
		}
	} else {
		go sm.PlaceOrder(sm.Strategy.GetModel().State.EntryPrice, InEntry)
		if !sm.Strategy.GetModel().Conditions.TakeProfitExternal {
			sm.PlaceOrder(0, TakeProfit)
		}
	}

	if !sm.Strategy.GetModel().Conditions.StopLossExternal {
		go sm.PlaceOrder(0, Stoploss)
	}

	if sm.Strategy.GetModel().Conditions.ForcedLoss > 0 && !isSpot && !sm.Strategy.GetModel().Conditions.StopLossExternal {
		go sm.PlaceOrder(0, "ForcedLoss")
	}
	// wait for creating hedgeStrategy
	if sm.Strategy.GetModel().Conditions.Hedging && sm.Strategy.GetModel().Conditions.HedgeStrategyId == nil || sm.Strategy.GetModel().Conditions.HedgeStrategyId != nil {
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

	if model.Conditions.TakeProfitExternal {
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
					sm.PlaceOrder(-1, TakeProfit)
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
	existTimeout := model.Conditions.TimeoutWhenLoss != 0 || model.Conditions.TimeoutLoss != 0
	isSpot := model.Conditions.MarketType == 0

	if ok && isWaitingForOrder.(bool) && !existTimeout && !isSpot  {
		return false
	}
	if model.Conditions.StopLossExternal {
		return false
	}
	if model.State.ExecutedAmount >= model.Conditions.EntryOrder.Amount {
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
	if model.Conditions.TimeoutWhenLoss > 0 {
		isLoss := (model.Conditions.EntryOrder.Side == "buy" && model.Conditions.EntryOrder.Price > currentOHLCV.Close) || (model.Conditions.EntryOrder.Side == "sell" && model.Conditions.EntryOrder.Price < currentOHLCV.Close)
		if isLoss && model.State.LossableAt == 0 {
			model.State.LossableAt = time.Now().Unix()
			go func(lossAt int64) {
				time.Sleep(time.Duration(model.Conditions.TimeoutWhenLoss) * time.Second)
				stillTimeout := lossAt == model.State.LossableAt
				if stillTimeout {
					sm.PlaceOrder(-1, Stoploss)
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
			sm.PlaceOrder(currentOHLCV.Close, Stoploss)
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
		if isSpot && forcedLoss > 0 && (currentOHLCV.Close/model.State.EntryPrice-1)*100 >= forcedLoss  {
			sm.PlaceOrder(currentOHLCV.Close, Stoploss)
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
			sm.PlaceOrder(currentOHLCV.Close, TakeProfit)

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
				sm.TryCancelAllOrdersConsistently(sm.Strategy.GetModel().State.Orders)
			}
			// sm.cancelOpenOrders(sm.Strategy.GetModel().Conditions.Pair)
			defer sm.PlaceOrder(currentOHLCV.Close, Stoploss)
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

	state, _ := sm.State.State(ctx)
	for state != End && state != Canceled {
		if sm.Strategy.GetModel().Enabled == false {
			break
		}
		if !sm.Lock {
			if sm.Strategy.GetModel().Conditions.EntrySpreadHunter {
				sm.processSpreadEventLoop()
			} else {
				sm.processEventLoop()
			}
		}
		time.Sleep(15 * time.Millisecond)
		state, _ = sm.State.State(ctx)
	}
	sm.Stop()
	println("STOPPED")
}

func (sm *SmartOrder) Stop() {
	if sm.StopLock {
		sm.StateMgmt.DisableStrategy(sm.Strategy.GetModel().ID)
		go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.Orders)
		return
	}
	sm.StopLock = true
	state, _ := sm.State.State(context.Background())
	if sm.Strategy.GetModel().Conditions.MarketType == 0 && state != End {
		sm.TryCancelAllOrdersConsistently(sm.Strategy.GetModel().State.Orders)
	} else {
		go sm.TryCancelAllOrders(sm.Strategy.GetModel().State.Orders)
	}
	if state != End {
		sm.PlaceOrder(0, Canceled)
	}
	sm.StateMgmt.DisableStrategy(sm.Strategy.GetModel().ID)
	sm.StopLock = false
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
		// println(sm.Strategy.GetModel().Conditions.Pair, sm.Strategy.GetModel().State.TrailingEntryPrice, currentOHLCV.Close, err.Error())
	}
}

func (sm *SmartOrder) processSpreadEventLoop() {
	currentSpreadP := sm.DataFeed.GetSpreadForPairAtExchange(sm.Strategy.GetModel().Conditions.Pair, sm.ExchangeName, sm.Strategy.GetModel().Conditions.MarketType)
	if currentSpreadP != nil {
		currentSpreadData := *currentSpreadP
		currentSpread := currentSpreadData.BestBid
		state, err := sm.State.State(context.TODO())
		err = sm.State.FireCtx(context.TODO(), TriggerTrade, currentSpread)
		if err == nil {
			return
		}
		if state == InEntry || state == TakeProfit || state == Stoploss || state == HedgeLoss {
			err = sm.State.FireCtx(context.TODO(), CheckLossTrade, currentSpread)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckProfitTrade, currentSpread)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckTrailingLossTrade, currentSpread)
			if err == nil {
				return
			}
			err = sm.State.FireCtx(context.TODO(), CheckTrailingProfitTrade, currentSpread)
			if err == nil {
				return
			}
		}
		// println(sm.Strategy.GetModel().Conditions.Pair, sm.Strategy.GetModel().State.TrailingEntryPrice, currentOHLCV.Close, err.Error())
	}
}
