package strategies

import (
	"context"
	"fmt"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"reflect"
	"time"
)

const (
	WaitForEntry  = 0
	TrailingEntry = 1
	InEntry       = 2
	TakeProfit    = 3
	Stoploss      = 4
	End           = 5
)

const (
	TriggerTrade = 0
)

type OHLCV struct {
	Open, High, Low, Close, Volume float64
}

type IDataFeed interface {
	GetPriceForPairAtExchange(pair string, exchange string) OHLCV
}

type ITrading interface {
	createOrder(exchange string, pair string, price float64, amount float64, side string) string
}

type SmartOrder struct {
	Model *models.MongoStrategy
	State *stateless.StateMachine
	ExchangeName string
	DataFeed IDataFeed
	ExchangeApi ITrading
}

func NewSmartOrder(smartOrder *models.MongoStrategy, DataFeed IDataFeed) *SmartOrder {
	sm := &SmartOrder{Model: smartOrder, DataFeed: DataFeed}
	State := stateless.NewStateMachine(WaitForEntry)
	State.SetTriggerParameters(TriggerTrade, reflect.TypeOf(0.0))
	State.Configure(WaitForEntry).Permit(TriggerTrade, sm.exitWaitEntry, sm.checkWaitEntry, sm.checkTrailingEntry)

	State.Configure(TrailingEntry).Permit(TriggerTrade, InEntry, sm.checkWaitEntry)

	State.Configure(InEntry).PermitDynamic(TriggerTrade, sm.exit,
		sm.checkProfit,
		sm.checkTrailingProfit,
		sm.checkTrailingLoss,
		sm.checkLoss).OnEntry(sm.enterEntry)

	State.Configure(TakeProfit).PermitDynamic(TriggerTrade, sm.exit,
		sm.checkProfit,
		sm.checkTrailingProfit,
		sm.checkTrailingLoss,
		sm.checkLoss).OnEntry(sm.enterTakeProfit)

	State.Configure(Stoploss).PermitDynamic(TriggerTrade, sm.exit,
		sm.checkProfit,
		sm.checkTrailingProfit,
		sm.checkTrailingLoss,
		sm.checkLoss).OnEntry(sm.stopLoss)

	sm.ExchangeName = "binance"
	sm.State = State
	fmt.Printf("%+v\n", sm.State.ToGraph())
	fmt.Printf("DONE\n")
	return sm
}

func (sm *SmartOrder) checkTrailingEntry(ctx context.Context, args ...interface{})  bool{
	currentPrice := args[0].(float64)
	edgePrice := sm.Model.State.TrailingEntryPrice
	switch sm.Model.Condition.Side {
	case "buy":
		if (currentPrice/edgePrice - 1) >= sm.Model.Condition.EntryDeviation {
			return true
		}
		break
	case "sell":
		if (edgePrice/currentPrice - 1) >= sm.Model.Condition.EntryDeviation {
			return true
		}
		break
	}

	return false
}

func (sm *SmartOrder) exitWaitEntry(ctx context.Context, args ...interface{}) (stateless.State, error){
	if sm.Model.Condition.ActivationPrice > 0 {
		return TrailingEntry, nil
	}
	return InEntry, nil
}
func (sm *SmartOrder) checkWaitEntry(ctx context.Context, args ...interface{})  bool{
	price := args[0].(float64)
	conditionPrice := sm.Model.Condition.Price
	if sm.Model.Condition.ActivationPrice > 0 {
		conditionPrice = sm.Model.Condition.ActivationPrice
	}
	switch sm.Model.Condition.Side {
	case "buy":
		if price <= conditionPrice {
			return true
		}
		break
	case "sell":
		if price >= conditionPrice {
			return true
		}
		break
	}

	return false
}

func (sm *SmartOrder) enterEntry(ctx context.Context, args ...interface{}) error{
	price := args[0].(float64)
	sm.ExchangeApi.createOrder(
		sm.ExchangeName,
		sm.Model.Condition.Pair,
		price,
		sm.Model.Condition.Amount,
		sm.Model.Condition.Side,
	)
	return nil
}

func (sm *SmartOrder) exit(ctx context.Context, args ...interface{}) (stateless.State, error){
	if sm.Model.Condition.ActivationPrice > 0 {
		return TrailingEntry, nil
	}
	return InEntry, nil
}

func (sm *SmartOrder) checkProfit(ctx context.Context, args ...interface{}) bool {
	currentPrice := args[0].(float64)
	switch sm.Model.Condition.Side {
	case "buy":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if currentPrice  >= level.Price {
				return true
			}
		}
		break
	case "sell":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if currentPrice  <= level.Price {
				return true
			}
		}
		break
	}

	return false
}
func (sm *SmartOrder) checkTrailingProfit(ctx context.Context, args ...interface{}) bool {
	currentPrice := args[0].(float64)
	switch sm.Model.Condition.Side {
	case "buy":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if currentPrice  >= level.Price {
				return true
			}
		}
		break
	case "sell":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if currentPrice  <= level.Price {
				return true
			}
		}
		break
	}

	return false
}

func (sm *SmartOrder) checkLoss(ctx context.Context, args ...interface{}) bool {
	currentPrice := args[0].(float64)
	switch sm.Model.Condition.Side {
	case "buy":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if currentPrice  >= level.Price {
				return true
			}
		}
		break
	case "sell":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if currentPrice  <= level.Price {
				return true
			}
		}
		break
	}

	return false
}
func (sm *SmartOrder) checkTrailingLoss(ctx context.Context, args ...interface{}) bool {
	currentPrice := args[0].(float64)
	switch sm.Model.Condition.Side {
	case "buy":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if currentPrice  >= level.Price {
				return true
			}
		}
		break
	case "sell":
		for _, level := range sm.Model.Condition.ExitLeveles {
			if currentPrice  <= level.Price {
				return true
			}
		}
		break
	}

	return false
}

func (sm *SmartOrder) enterTakeProfit(ctx context.Context, args ...interface{}) error{
	price := args[0].(float64)
	sm.ExchangeApi.createOrder(
		sm.ExchangeName,
		sm.Model.Condition.Pair,
		price,
		sm.Model.Condition.Amount,
		sm.Model.Condition.Side,
	)
	return nil
}

func (sm *SmartOrder) stopLoss(context.Context, ...interface{}) error{
	return nil
}

func (sm* SmartOrder) Start(){
	for {
		sm.processEventLoop()
		time.Sleep(100 * time.Millisecond)
	}
}

func (sm* SmartOrder) processEventLoop(){
	currentPrice := sm.DataFeed.GetPriceForPairAtExchange(sm.Model.Condition.Pair, sm.ExchangeName)
	sm.State.Fire(TriggerTrade, currentPrice)
}

func StartEventLoop() {
}