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
	Error			= "Error"
)

const (
	TriggerTrade             = "Trade"
	TriggerOrderExecuted     = "TriggerOrderExecuted"
	CheckExistingOrders      = "CheckExistingOrders"
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
	OrdersMap               sync.Map
	StatusByOrderId         sync.Map
	QuantityAmountPrecision int64
	QuantityPricePrecision  int64
	Lock                    bool
	LastTrailingTimestamp   int64
	SelectedExitTarget      int
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func (sm *SmartOrder) toFixed(num float64, precision int64) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

func NewSmartOrder(strategy interfaces.IStrategy, DataFeed interfaces.IDataFeed, TradingAPI trading.ITrading, keyId *primitive.ObjectID, stateMgmt interfaces.IStateMgmt) *SmartOrder {

	sm := &SmartOrder{Strategy: strategy, DataFeed: DataFeed, ExchangeApi: TradingAPI, KeyId: keyId, StateMgmt: stateMgmt, Lock: false, SelectedExitTarget: 0}

	initState := WaitForEntry
	pricePrecision, amountPrecision := stateMgmt.GetMarketPrecision(strategy.GetModel().Conditions.Pair, strategy.GetModel().Conditions.MarketType)
	sm.QuantityPricePrecision = pricePrecision
	sm.QuantityAmountPrecision = amountPrecision
	// if state is not empty but if its in the end and open ended, then we skip state value, since want to start over
	if strategy.GetModel().State.State != "" && !(strategy.GetModel().State.State == End && strategy.GetModel().Conditions.ContinueIfEnded == true) {
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
		sm.checkExistingOrders)
	State.Configure(TrailingEntry).Permit(TriggerTrade, InEntry,
		sm.checkTrailingEntry).Permit(CheckExistingOrders, InEntry,
		sm.checkExistingOrders).OnEntry(sm.enterTrailingEntry)
	State.Configure(InEntry).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders).OnEntry(sm.enterEntry)
	State.Configure(TakeProfit).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders).OnEntry(sm.enterTakeProfit)
	State.Configure(Stoploss).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).PermitDynamic(CheckExistingOrders, sm.exit,
		sm.checkExistingOrders).OnEntry(sm.enterStopLoss)
	State.Configure(End).OnEntry(sm.enterEnd)

	State.Activate()

	sm.State = State
	sm.ExchangeName = "binance"
	// fmt.Printf(sm.State.ToGraph())
	// fmt.Printf("DONE\n")
	sm.checkTimeouts()
	sm.checkIfPlaceOrderInstantlyOnStart()

	return sm
}

func (sm *SmartOrder) checkIfPlaceOrderInstantlyOnStart() {
	isFirstRunSoStateisEmpty := sm.Strategy.GetModel().State.State == ""
	if isFirstRunSoStateisEmpty {
		entryIsNotTrailing := sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice == 0
		if entryIsNotTrailing { // then we must know exact price
			sm.IsWaitingForOrder.Store(WaitForEntry, true)
			go sm.placeOrder(sm.Strategy.GetModel().Conditions.EntryOrder.Price, WaitForEntry)
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

func (sm *SmartOrder) waitForOrder(orderId string, orderStatus string) {
	sm.StatusByOrderId.Store(orderId, orderStatus)
	_ = sm.StateMgmt.SubscribeToOrder(orderId, sm.orderCallback)
}
func (sm *SmartOrder) orderCallback(order *models.MongoOrder) {
	err := sm.State.Fire(CheckExistingOrders, *order)
	if err != nil {
		// println(err.Error())
	}
}

func (sm *SmartOrder) checkExistingOrders(ctx context.Context, args ...interface{}) bool {
	if args == nil {
		return false
	}
	order := args[0].(models.MongoOrder)
	orderId := order.OrderId
	orderStatus := order.Status
	step, ok := sm.StatusByOrderId.Load(orderId)
	if !ok {
		return false
	}
	switch orderStatus {
	case "closed", "filled": // TODO i
		switch step {
		case TrailingEntry:
			if sm.Strategy.GetModel().State.EntryPrice > 0 {
				return false
			}
			sm.Strategy.GetModel().State.EntryPrice = order.Average
			sm.Strategy.GetModel().State.State = InEntry
			sm.StateMgmt.UpdateEntryPrice(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			return true
		case WaitForEntry:
			if sm.Strategy.GetModel().State.EntryPrice > 0 {
				return false
			}
			sm.Strategy.GetModel().State.EntryPrice = order.Average
			sm.Strategy.GetModel().State.State = InEntry
			sm.StateMgmt.UpdateEntryPrice(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			return true
		case TakeProfit:
			if order.Filled > 0 {
				sm.Strategy.GetModel().State.ExecutedAmount += order.Filled
			}
			sm.Strategy.GetModel().State.ExitPrice = order.Average
			if sm.Strategy.GetModel().State.ExecutedAmount >= sm.Strategy.GetModel().Conditions.EntryOrder.Amount {
				sm.Strategy.GetModel().State.State = End
			} else {
				go sm.placeOrder(0, Stoploss)
			}
			sm.StateMgmt.UpdateExecutedAmount(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			return true
		case Stoploss:
			if order.Filled > 0 {
				sm.Strategy.GetModel().State.ExecutedAmount += order.Filled
			}
			sm.Strategy.GetModel().State.ExitPrice = order.Average
			sm.StateMgmt.UpdateExecutedAmount(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			if sm.Strategy.GetModel().State.ExecutedAmount >= sm.Strategy.GetModel().Conditions.EntryOrder.Amount {
				sm.Strategy.GetModel().State.State = End
			}
			return true
		}
		break
	case "canceled":
		switch step {
		case WaitForEntry:
			sm.Strategy.GetModel().State.State = Canceled
			return true
		case InEntry:
			sm.Strategy.GetModel().State.State = Canceled
			return true
		}
		break
	}
	return false
}


func (sm *SmartOrder) exitWaitEntry(ctx context.Context, args ...interface{}) (stateless.State, error) {
	if sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice > 0 {
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
	conditionPrice := sm.Strategy.GetModel().Conditions.EntryOrder.Price
	if sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice == 0 && sm.Strategy.GetModel().Conditions.EntryOrder.OrderType == "market" {
		return true
	}
	isTrailing := sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice > 0
	if isTrailing {
		conditionPrice = sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice
	}
	switch sm.Strategy.GetModel().Conditions.EntryOrder.Side {
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
	if currentOHLCV, ok := args[0].(interfaces.OHLCV); ok {
		sm.Strategy.GetModel().State.EntryPrice = currentOHLCV.Close
	}
	sm.Strategy.GetModel().State.State = InEntry
	sm.Strategy.GetModel().State.TrailingEntryPrice = 0
	sm.Strategy.GetModel().State.ExecutedOrders = []string{}
	go sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)

	go sm.placeOrder(sm.Strategy.GetModel().State.EntryPrice, InEntry)
	go sm.placeOrder(0, TakeProfit)
	go sm.placeOrder(0, Stoploss)
	return nil
}

func (sm *SmartOrder) exit(ctx context.Context, args ...interface{}) (stateless.State, error) {
	state, _ := sm.State.State(context.TODO())
	nextState := End
	if sm.Strategy.GetModel().State.ExecutedAmount >= sm.Strategy.GetModel().Conditions.EntryOrder.Amount { // all trades executed, nothing more to trade
		if sm.Strategy.GetModel().Conditions.ContinueIfEnded {
			oppositeSide := sm.Strategy.GetModel().Conditions.EntryOrder.Side
			if oppositeSide == "buy" {
				oppositeSide = "sell"
			} else {
				oppositeSide = "buy"
			}
			sm.Strategy.GetModel().Conditions.EntryOrder.Side = oppositeSide
			sm.Strategy.GetModel().Conditions.EntryOrder.ActivatePrice = sm.Strategy.GetModel().State.ExitPrice
			sm.StateMgmt.UpdateConditions(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().Conditions)

			newState := models.MongoStrategyState{
				State: WaitForEntry,
			}
			sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &newState)
			return WaitForEntry, nil
		}
		return End, nil
	}
	switch state {
	case InEntry:
		switch sm.Strategy.GetModel().State.State {
		case TakeProfit:
			nextState = TakeProfit
			break
		case Stoploss:
			nextState = Stoploss
			break
		case InEntry:
			nextState = InEntry
			break
		}
		break
	case TakeProfit:
		switch sm.Strategy.GetModel().State.State {
		case "EnterNextTarget":
			nextState = TakeProfit
			break
		case TakeProfit:
			nextState = End
			break
		case Stoploss:
			nextState = Stoploss
			break
		}
		break
	case Stoploss:
		switch sm.Strategy.GetModel().State.State {
		case End:
			nextState = End
			break
		}
		break
	}
	if nextState == End && sm.Strategy.GetModel().Conditions.ContinueIfEnded {
		newState := models.MongoStrategyState{
			State:              WaitForEntry,
			TrailingEntryPrice: 0,
			EntryPrice:         0,
			Amount:             0,
			Orders:             nil,
			ExecutedAmount:     0,
			ReachedTargetCount: 0,
		}
		sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &newState)
		return WaitForEntry, nil
	}
	return nextState, nil
}

func (sm *SmartOrder) checkProfit(ctx context.Context, args ...interface{}) bool {
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	if ok && isWaitingForOrder.(bool) {
		return false
	}
	currentOHLCV := args[0].(interfaces.OHLCV)

	if sm.Strategy.GetModel().Conditions.TimeoutIfProfitable > 0 {
		isProfitable := (sm.Strategy.GetModel().Conditions.EntryOrder.Side == "buy" && sm.Strategy.GetModel().State.EntryPrice < currentOHLCV.Close) ||
			(sm.Strategy.GetModel().Conditions.EntryOrder.Side == "sell" && sm.Strategy.GetModel().State.EntryPrice > currentOHLCV.Close)
		if isProfitable && sm.Strategy.GetModel().State.ProfitableAt == 0 {
			sm.Strategy.GetModel().State.ProfitableAt = time.Now().Unix()
			go func(profitableAt int64) {
				time.Sleep(time.Duration(sm.Strategy.GetModel().Conditions.TimeoutIfProfitable) * time.Second)
				stillTimeout := profitableAt == sm.Strategy.GetModel().State.ProfitableAt
				if stillTimeout {
					sm.placeOrder(-1, TakeProfit)
				}
			}(sm.Strategy.GetModel().State.ProfitableAt)
		} else if !isProfitable {
			sm.Strategy.GetModel().State.ProfitableAt = 0
		}
	}

	if sm.Strategy.GetModel().Conditions.ExitLevels != nil {
		amount := 0.0
		switch sm.Strategy.GetModel().Conditions.EntryOrder.Side {
		case "buy":
			for i, level := range sm.Strategy.GetModel().Conditions.ExitLevels {
				if sm.Strategy.GetModel().State.ReachedTargetCount < i+1 && level.ActivatePrice == 0 {
					if level.Type == 1 && currentOHLCV.Close >= (sm.Strategy.GetModel().State.EntryPrice*(100+level.Price/sm.Strategy.GetModel().Conditions.Leverage)/100) ||
						level.Type == 0 && currentOHLCV.Close >= level.Price {
						sm.Strategy.GetModel().State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += sm.Strategy.GetModel().Conditions.EntryOrder.Amount * (level.Amount / 100)
						}
					}
				}
			}
			break
		case "sell":
			for i, level := range sm.Strategy.GetModel().Conditions.ExitLevels {
				if sm.Strategy.GetModel().State.ReachedTargetCount < i+1 && level.ActivatePrice == 0 {
					if level.Type == 1 && currentOHLCV.Close <= (sm.Strategy.GetModel().State.EntryPrice*((100-level.Price/sm.Strategy.GetModel().Conditions.Leverage)/100)) ||
						level.Type == 0 && currentOHLCV.Close <= level.Price {
						sm.Strategy.GetModel().State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += sm.Strategy.GetModel().Conditions.EntryOrder.Amount * (level.Amount / 100)
						}
					}
				}
			}
			break
		}
		if sm.Strategy.GetModel().State.ExecutedAmount == sm.Strategy.GetModel().Conditions.EntryOrder.Amount {
			sm.Strategy.GetModel().State.Amount = 0
			sm.Strategy.GetModel().State.State = TakeProfit // took all profits, exit now
			sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			return true
		}
		if amount > 0 {
			sm.Strategy.GetModel().State.Amount = amount
			sm.Strategy.GetModel().State.State = TakeProfit
			currentState, _ := sm.State.State(context.Background())
			if currentState == TakeProfit {
				sm.Strategy.GetModel().State.State = EnterNextTarget
				sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			}
			return true
		}
	}
	return false
}

func (sm *SmartOrder) checkLoss(ctx context.Context, args ...interface{}) bool {
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(Stoploss)
	if ok && isWaitingForOrder.(bool) {
		return false
	}
	currentOHLCV := args[0].(interfaces.OHLCV)
	stopLoss := sm.Strategy.GetModel().Conditions.StopLoss / sm.Strategy.GetModel().Conditions.Leverage

	// try exit on timeout
	if sm.Strategy.GetModel().Conditions.TimeoutWhenLoss > 0 {
		isLoss := (sm.Strategy.GetModel().Conditions.EntryOrder.Side == "buy" && sm.Strategy.GetModel().Conditions.EntryOrder.Price > currentOHLCV.Close) || (sm.Strategy.GetModel().Conditions.EntryOrder.Side == "sell" && sm.Strategy.GetModel().Conditions.EntryOrder.Price < currentOHLCV.Close)
		if isLoss && sm.Strategy.GetModel().State.LossableAt == 0 {
			sm.Strategy.GetModel().State.LossableAt = time.Now().Unix()
			go func(lossAt int64) {
				time.Sleep(time.Duration(sm.Strategy.GetModel().Conditions.TimeoutWhenLoss) * time.Second)
				stillTimeout := lossAt == sm.Strategy.GetModel().State.LossableAt
				if stillTimeout {
					sm.placeOrder(-1, Stoploss)
				}
			}(sm.Strategy.GetModel().State.LossableAt)
		} else if !isLoss {
			sm.Strategy.GetModel().State.LossableAt = 0
		}
	}
	switch sm.Strategy.GetModel().Conditions.EntryOrder.Side {
	case "buy":
		if (1-currentOHLCV.Close/sm.Strategy.GetModel().State.EntryPrice)*100 >= stopLoss {
			if sm.Strategy.GetModel().State.ExecutedAmount < sm.Strategy.GetModel().Conditions.EntryOrder.Amount {
				sm.Strategy.GetModel().State.Amount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount - sm.Strategy.GetModel().State.ExecutedAmount
			}
			sm.Strategy.GetModel().State.State = Stoploss
			sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			return true
		}
		break
	case "sell":
		if (currentOHLCV.Close/sm.Strategy.GetModel().State.EntryPrice-1)*100 >= stopLoss {
			if sm.Strategy.GetModel().State.ExecutedAmount < sm.Strategy.GetModel().Conditions.EntryOrder.Amount {
				sm.Strategy.GetModel().State.Amount = sm.Strategy.GetModel().Conditions.EntryOrder.Amount - sm.Strategy.GetModel().State.ExecutedAmount
			}
			sm.Strategy.GetModel().State.State = Stoploss
			sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
			return true
		}
		break
	}

	return false
}
func (sm *SmartOrder) enterEnd(ctx context.Context, args ...interface{}) error {
	sm.Strategy.GetModel().State.State = End
	sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, &sm.Strategy.GetModel().State)
	return nil
}

func (sm *SmartOrder) enterTakeProfit(ctx context.Context, args ...interface{}) error {
	if currentOHLCV, ok := args[0].(interfaces.OHLCV); ok {
		if sm.Strategy.GetModel().State.Amount > 0 {

			if sm.Lock {
				return nil
			}
			sm.Lock = true
			sm.placeOrder(currentOHLCV.Close, TakeProfit)

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

			// sm.cancelOpenOrders(sm.Strategy.GetModel().Conditions.Pair)
			sm.placeOrder(currentOHLCV.Close, Stoploss)
		}
		// if timeout specified then do this sell on timeout
		sm.Lock = false
	}
	_ = sm.State.Fire(CheckLossTrade, args[0])
	return nil
}

func (sm *SmartOrder) tryCancelAllOrders() {
	orderIds := sm.Strategy.GetModel().State.Orders
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
func (sm *SmartOrder) Start() {
	state, _ := sm.State.State(context.Background())
	for state != End && state != Canceled {
		if sm.Strategy.GetModel().Enabled == false {
			break
		}
		if !sm.Lock {
			sm.processEventLoop()
		}
		time.Sleep(200 * time.Millisecond)
		state, _ = sm.State.State(context.Background())
	}
	sm.Stop()
	println("STOPPED")
}

func (sm *SmartOrder) Stop() {
	state, _ := sm.State.State(context.Background())
	if state != End {
		sm.placeOrder(0, Canceled)
		sm.tryCancelAllOrders()
	}
	sm.StateMgmt.DisableStrategy(sm.Strategy.GetModel().ID)
}

func (sm *SmartOrder) processEventLoop() {
	currentOHLCVp := sm.DataFeed.GetPriceForPairAtExchange(sm.Strategy.GetModel().Conditions.Pair, sm.ExchangeName, sm.Strategy.GetModel().Conditions.MarketType)
	if currentOHLCVp != nil {
		currentOHLCV := *currentOHLCVp
		// println("new trade", currentOHLCV.Close)
		state, err := sm.State.State(context.TODO())
		err = sm.State.FireCtx(context.TODO(), TriggerTrade, currentOHLCV)
		if err == nil {
			return
		}
		if state == InEntry || state == TakeProfit || state == Stoploss {
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
