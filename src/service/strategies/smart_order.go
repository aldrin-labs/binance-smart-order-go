package strategies

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"reflect"
	"time"
)

const (
	WaitForEntry  = "WaitForEntry"
	TrailingEntry = "TrailingEntry"
	InEntry       = "InEntry"
	TakeProfit    = "TakeProfit"
	Stoploss      = "Stoploss"
	End           = "End"
	EnterNextTarget = "EnterNextTarget"
)

const (
	TriggerTrade = "Trade"
	CheckProfitTrade = "CheckProfitTrade"
	CheckTrailingProfitTrade = "CheckTrailingProfitTrade"
	CheckTrailingLossTrade = "CheckTrailingLossTrade"
	CheckLossTrade = "CheckLossTrade"
)

type OHLCV struct {
	Open, High, Low, Close, Volume float64
}

type IDataFeed interface {
	GetPriceForPairAtExchange(pair string, exchange string) OHLCV
}

type ITrading interface {
	CreateOrder(exchange string, pair string, price float64, amount float64, side string) string
}

type SmartOrder struct {
	Model        *models.MongoStrategy
	State        *stateless.StateMachine
	ExchangeName string
	DataFeed     IDataFeed
	ExchangeApi  ITrading
}

func NewSmartOrder(smartOrder *models.MongoStrategy, DataFeed IDataFeed, TradingAPI ITrading) *SmartOrder {
	sm := &SmartOrder{Model: smartOrder, DataFeed: DataFeed, ExchangeApi: TradingAPI}
	State := stateless.NewStateMachine(WaitForEntry)
	State.SetTriggerParameters(TriggerTrade, reflect.TypeOf(OHLCV{}))
	State.Configure(WaitForEntry).PermitDynamic(TriggerTrade, sm.exitWaitEntry, sm.checkWaitEntry)

	State.Configure(TrailingEntry).Permit(TriggerTrade, InEntry, sm.checkTrailingEntry).OnEntry(sm.enterTrailingEntry)

	State.Configure(InEntry).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).OnEntry(sm.enterEntry)

	State.Configure(TakeProfit).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).OnEntry(sm.enterTakeProfit)

	State.Configure(Stoploss).PermitDynamic(CheckProfitTrade, sm.exit,
		sm.checkProfit).PermitDynamic(CheckTrailingProfitTrade, sm.exit,
		sm.checkTrailingProfit).PermitDynamic(CheckLossTrade, sm.exit,
		sm.checkLoss).OnEntry(sm.enterStopLoss)
	State.Activate()
	sm.ExchangeName = "binance"
	sm.State = State
	// fmt.Printf(sm.State.ToGraph())
	// fmt.Printf("DONE\n")
	return sm
}

func (sm *SmartOrder) enterTrailingEntry(ctx context.Context, args ...interface{}) error {
	currentOHLCV := args[0].(OHLCV)
	sm.Model.State.TrailingEntryPrice = currentOHLCV.Close
	return nil
}
func (sm *SmartOrder) checkTrailingEntry(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	edgePrice := sm.Model.State.TrailingEntryPrice
	if edgePrice == 0 {
		println("edgePrice=0")
		sm.Model.State.TrailingEntryPrice = currentOHLCV.Open
		return false
	}
	println(currentOHLCV.Close, edgePrice, currentOHLCV.Close/edgePrice-1)
	switch sm.Model.Condition.Side {
	case "buy":
		if (currentOHLCV.Close/edgePrice-1)*100 >= sm.Model.Condition.EntryDeviation {
			return true
		}
		break
	case "sell":
		if (edgePrice/currentOHLCV.Close-1)*100 >= sm.Model.Condition.EntryDeviation {
			return true
		}
		break
	}
	if currentOHLCV.Close < edgePrice {
		sm.Model.State.TrailingEntryPrice = currentOHLCV.Close
	}
	return false
}

func (sm *SmartOrder) exitWaitEntry(ctx context.Context, args ...interface{}) (stateless.State, error) {
	println("act price", sm.Model.Condition.ActivationPrice)
	if sm.Model.Condition.ActivationPrice > 0 {
		println("move to", TrailingEntry)
		return TrailingEntry, nil
	}
	println("move to", InEntry)
	return InEntry, nil
}
func (sm *SmartOrder) checkWaitEntry(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	conditionPrice := sm.Model.Condition.Price
	if sm.Model.Condition.ActivationPrice > 0 {
		conditionPrice = sm.Model.Condition.ActivationPrice
	}
	switch sm.Model.Condition.Side {
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
	currentOHLCV := args[0].(OHLCV)
	sm.Model.State.EntryPrice = currentOHLCV.Close
	sm.ExchangeApi.CreateOrder(
		sm.ExchangeName,
		sm.Model.Condition.Pair,
		currentOHLCV.Close,
		sm.Model.Condition.Amount,
		sm.Model.Condition.Side,
	)
	return nil
}

func (sm *SmartOrder) exit(ctx context.Context, args ...interface{}) (stateless.State, error) {
	state, _ := sm.State.State(context.TODO())
	nextState := End
	if sm.Model.State.ExecutedAmount == sm.Model.Condition.Amount { // all trades executed, nothing more to trade
		return End, nil
	}
	switch state {
	case InEntry:
		switch sm.Model.State.State {
		case TakeProfit:
			nextState = TakeProfit
			break
		case Stoploss:
			nextState = Stoploss
			break
		}
		break
	case TakeProfit:
		switch sm.Model.State.State {
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
		switch sm.Model.State.State {
		case End:
			nextState = End
			break
		}
		break

	}
	return nextState, nil
}

func (sm *SmartOrder) checkProfit(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	if sm.Model.Condition.ExitLeveles != nil {
		amount := 0.0
		switch sm.Model.Condition.Side {
		case "buy":
			for i, level := range sm.Model.Condition.ExitLeveles {
				if sm.Model.State.ReachedTargetCount < i+1 {
					if level.Type == 1 && currentOHLCV.Close >= sm.Model.State.EntryPrice * (1 + level.Price / 100) ||
						level.Type == 0 && currentOHLCV.Close >= level.Price {
						sm.Model.State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += sm.Model.Condition.Amount * (level.Amount / 100)
						}
					}
				}
			}
			break
		case "sell":
			for i, level := range sm.Model.Condition.ExitLeveles {
				if sm.Model.State.ReachedTargetCount < i+1 {
					if level.Type == 1 && currentOHLCV.Close <= sm.Model.State.EntryPrice * level.Price ||
											level.Type == 0 && currentOHLCV.Close <= level.Price {
						sm.Model.State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += sm.Model.Condition.Amount * (level.Amount / 100)
						}
					}
				}
			}
			break
		}
		if sm.Model.State.ExecutedAmount == sm.Model.Condition.Amount {
			sm.Model.State.Amount = 0
			sm.Model.State.State = TakeProfit // took all profits, exit now
			return true
		}
		if amount > 0 {
			sm.Model.State.Amount = amount
			sm.Model.State.State = TakeProfit
			currentState, _ := sm.State.State(context.Background())
			if currentState == TakeProfit {
				sm.Model.State.State = EnterNextTarget
			}
			return true
		}
	}
	if sm.Model.Condition.TakeProfit > 0 {
		switch sm.Model.Condition.Side {
		case "buy":
			if (currentOHLCV.Close/sm.Model.State.EntryPrice-1)*100 >= sm.Model.Condition.TakeProfit {
				sm.Model.State.State = TakeProfit
				return true
			}
			break
		case "sell":
			if (sm.Model.State.EntryPrice/currentOHLCV.Close-1)*100 >= sm.Model.Condition.TakeProfit {
				sm.Model.State.State = TakeProfit
				return true
			}
			break
		}
	}
	return false
}
func (sm *SmartOrder) checkTrailingProfit(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	switch sm.Model.Condition.Side {
	case "buy":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if level.ActivatePrice > 0 {
				if currentOHLCV.Close >= level.Price {
					return true
				}
			}
		}
		break
	case "sell":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if level.ActivatePrice > 0 {
				if currentOHLCV.Close <= level.Price {
					return true
				}
			}
		}
		break
	}

	return false
}

func (sm *SmartOrder) checkLoss(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	switch sm.Model.Condition.Side {
	case "buy":
		if (sm.Model.State.EntryPrice/currentOHLCV.Close - 1)*100 >= sm.Model.Condition.StopLoss {
			if sm.Model.State.ExecutedAmount < sm.Model.Condition.Amount {
				sm.Model.State.Amount = sm.Model.Condition.Amount - sm.Model.State.ExecutedAmount
			}
			sm.Model.State.State = Stoploss
			return true
		}
		break
	case "sell":
		if (currentOHLCV.Close/sm.Model.State.EntryPrice - 1)*100 >= sm.Model.Condition.StopLoss {
			if sm.Model.State.ExecutedAmount < sm.Model.Condition.Amount {
				sm.Model.State.Amount = sm.Model.Condition.Amount - sm.Model.State.ExecutedAmount
			}
			sm.Model.State.State = Stoploss
			return true
		}
		break
	}

	return false
}
func (sm *SmartOrder) checkTrailingLoss(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	switch sm.Model.Condition.Side {
	case "buy":
		if (sm.Model.State.EntryPrice/currentOHLCV.Close - 1)*100 >= sm.Model.Condition.StopLoss {
			return true
		}
		break
	case "sell":
		if (currentOHLCV.Close/sm.Model.State.EntryPrice - 1)*100 >= sm.Model.Condition.StopLoss {
			return true
		}
		break
	}

	return false
}

func (sm *SmartOrder) enterTakeProfit(ctx context.Context, args ...interface{}) error {
	if sm.Model.State.Amount > 0 {
		price := args[0].(OHLCV)
		side := "buy"
		if sm.Model.Condition.Side == side {
			side = "sell"
		}
		sm.ExchangeApi.CreateOrder(
			sm.ExchangeName,
			sm.Model.Condition.Pair,
			price.Close,
			sm.Model.State.Amount,
			side,
		)

		sm.Model.State.ExecutedAmount += sm.Model.State.Amount
		if sm.Model.State.ExecutedAmount - sm.Model.Condition.Amount == 0 { // re-check all takeprofit conditions to exit trade ( no additional trades needed no )
			ohlcv := args[0].(OHLCV)
			err := sm.State.FireCtx(context.TODO(), TriggerTrade, ohlcv)

			return err
		}
	}
	return nil
}

func (sm *SmartOrder) enterStopLoss(ctx context.Context, args ...interface{}) error {
	price := args[0].(OHLCV)
	side := "buy"
	if sm.Model.Condition.Side == side {
		side = "sell"
	}
	sm.cancelOpenOrders(sm.Model.Condition.Pair)
	sm.ExchangeApi.CreateOrder(
		sm.ExchangeName,
		sm.Model.Condition.Pair,
		price.Close,
		sm.Model.State.Amount,
		side,
	)
	_ = sm.State.Fire(CheckLossTrade, args[0])
	// if timeout specified then do this sell on timeout
	return nil
}

func (sm *SmartOrder) cancelOpenOrders(pair string) {
}

func (sm *SmartOrder) Start() {
	state, _ := sm.State.State(context.Background())
	for state != End {
		sm.processEventLoop()
		time.Sleep(100 * time.Millisecond)
		state, _ = sm.State.State(context.Background())
	}
}

func (sm *SmartOrder) Stop() {
}

func (sm *SmartOrder) processEventLoop() {
	currentOHLCV := sm.DataFeed.GetPriceForPairAtExchange(sm.Model.Condition.Pair, sm.ExchangeName)
	println("new trade", currentOHLCV.Close)
	err := sm.State.FireCtx(context.TODO(), TriggerTrade, currentOHLCV)
	err = sm.State.FireCtx(context.TODO(), CheckLossTrade, currentOHLCV)
	err = sm.State.FireCtx(context.TODO(), CheckProfitTrade, currentOHLCV)
	err = sm.State.FireCtx(context.TODO(), CheckTrailingLossTrade, currentOHLCV)
	err = sm.State.FireCtx(context.TODO(), CheckTrailingProfitTrade, currentOHLCV)
	if err != nil {
		println(err.Error())
	}
}

func RunSmartOrder(strategy *Strategy, df IDataFeed, td ITrading) IStrategyRuntime {
	runtime := NewSmartOrder(strategy.Model, strategy.Datafeed, strategy.Trading)
	runtime.Start()

	return runtime
}
