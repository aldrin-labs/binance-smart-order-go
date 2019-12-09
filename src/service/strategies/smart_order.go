package strategies

import (
	"context"
	"fmt"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *OHLCV
}

type ITrading interface {
	CreateOrder(exchange string, pair string, price float64, amount float64, side string) string
}

type IStateMgmt interface {
	UpdateState(strategyId primitive.ObjectID, state *models.MongoStrategyState)
}

type SmartOrder struct {
	Strategy 	 *Strategy
	State        *stateless.StateMachine
	ExchangeName string
	KeyId		 *primitive.ObjectID
	DataFeed     IDataFeed
	ExchangeApi  trading.ITrading
	StateMgmt 	 IStateMgmt
}

func NewSmartOrder(strategy *Strategy, DataFeed IDataFeed, TradingAPI trading.ITrading, keyId *primitive.ObjectID, stateMgmt IStateMgmt) *SmartOrder {
	sm := &SmartOrder{Strategy: strategy, DataFeed: DataFeed, ExchangeApi: TradingAPI, KeyId: keyId, StateMgmt: stateMgmt}
	initState := WaitForEntry
	if strategy.Model.State.State != "" {
		initState = strategy.Model.State.State
	}
	State := stateless.NewStateMachine(initState)
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

	State.Configure(End).OnEntry(sm.enterEnd)
	State.Activate()
	sm.ExchangeName = "binance"
	sm.State = State
	// fmt.Printf(sm.State.ToGraph())
	// fmt.Printf("DONE\n")
	return sm
}

func (sm *SmartOrder) enterTrailingEntry(ctx context.Context, args ...interface{}) error {
	currentOHLCV := args[0].(OHLCV)
	sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
	sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
	return nil
}
func (sm *SmartOrder) checkTrailingEntry(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	edgePrice := sm.Strategy.Model.State.TrailingEntryPrice
	if edgePrice == 0 {
		println("edgePrice=0")
		sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Open
		return false
	}
	println(currentOHLCV.Close, edgePrice, currentOHLCV.Close/edgePrice-1)
	switch sm.Strategy.Model.Conditions.EntryOrder.Side {
	case "buy":
		if (currentOHLCV.Close/edgePrice-1)*100 >= sm.Strategy.Model.Conditions.EntryDeviation {
			return true
		}
		break
	case "sell":
		if (edgePrice/currentOHLCV.Close-1)*100 >= sm.Strategy.Model.Conditions.EntryDeviation {
			return true
		}
		break
	}
	if currentOHLCV.Close < edgePrice {
		sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
	}
	return false
}

func (sm *SmartOrder) exitWaitEntry(ctx context.Context, args ...interface{}) (stateless.State, error) {
	println("act price", sm.Strategy.Model.Conditions.ActivationPrice)
	if sm.Strategy.Model.Conditions.ActivationPrice > 0 {
		println("move to", TrailingEntry)
		return TrailingEntry, nil
	}
	println("move to", InEntry)
	return InEntry, nil
}
func (sm *SmartOrder) checkWaitEntry(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	conditionPrice := sm.Strategy.Model.Conditions.EntryOrder.Price
	if sm.Strategy.Model.Conditions.ActivationPrice > 0 {
		conditionPrice = sm.Strategy.Model.Conditions.ActivationPrice
	}
	switch sm.Strategy.Model.Conditions.EntryOrder.Side {
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
	sm.Strategy.Model.State.EntryPrice = currentOHLCV.Close
	sm.Strategy.Model.State.State = InEntry
	sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
	baseAmount := sm.Strategy.Model.Conditions.EntryOrder.Amount / currentOHLCV.Close
	for {
		response := sm.ExchangeApi.CreateOrder(
			trading.CreateOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.Order{
					Symbol: sm.Strategy.Model.Conditions.Pair,
					Type:   sm.Strategy.Model.Conditions.EntryOrder.OrderType,
					Side:   sm.Strategy.Model.Conditions.EntryOrder.Side,
					Amount: baseAmount,
					Price:  currentOHLCV.Close,
					Params: trading.OrderParams{
						StopPrice: 0,
						Type:      sm.Strategy.Model.Conditions.EntryOrder.OrderType,
					},
				},
			},
		)
		if response.Status == "OK" {
			sm.Strategy.Model.State.Orders = append(sm.Strategy.Model.State.Orders, response.Data.Id)
			sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)

			if response.Data.Status == "closed" {
				break
			} else {
				fmt.Printf("ORDER NOT CLOSED %+v\n", response)
				return sm.WaitForOrder(response.Data.Id)
			}

		}
	}
	return nil
}

func (sm *SmartOrder) WaitForOrder(orderId string) error {
	// implement here calling to mongodb.SubscribeToOrderStatus(orderId, status)

	// that will wait for order to be executed by using mongo change streams
	// and every 5 sec do direct call to mongodb to make sure you have latest state
	//
	return nil
}

func (sm *SmartOrder) exit(ctx context.Context, args ...interface{}) (stateless.State, error) {
	state, _ := sm.State.State(context.TODO())
	nextState := End
	if sm.Strategy.Model.State.ExecutedAmount == sm.Strategy.Model.Conditions.EntryOrder.Amount { // all trades executed, nothing more to trade
		return End, nil
	}
	switch state {
	case InEntry:
		switch sm.Strategy.Model.State.State {
		case TakeProfit:
			nextState = TakeProfit
			break
		case Stoploss:
			nextState = Stoploss
			break
		}
		break
	case TakeProfit:
		switch sm.Strategy.Model.State.State {
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
		switch sm.Strategy.Model.State.State {
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
	if sm.Strategy.Model.Conditions.ExitLevels != nil {
		amount := 0.0
		switch sm.Strategy.Model.Conditions.EntryOrder.Side {
		case "buy":
			for i, level := range sm.Strategy.Model.Conditions.ExitLevels {
				if sm.Strategy.Model.State.ReachedTargetCount < i+1 {
					if level.Type == 1 && currentOHLCV.Close >= sm.Strategy.Model.State.EntryPrice * (1 + level.Price / 100) ||
						level.Type == 0 && currentOHLCV.Close >= level.Price {
						sm.Strategy.Model.State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += sm.Strategy.Model.Conditions.EntryOrder.Amount * (level.Amount / 100)
						}
					}
				}
			}
			break
		case "sell":
			for i, level := range sm.Strategy.Model.Conditions.ExitLevels {
				if sm.Strategy.Model.State.ReachedTargetCount < i+1 {
					if level.Type == 1 && currentOHLCV.Close <= sm.Strategy.Model.State.EntryPrice * level.Price ||
											level.Type == 0 && currentOHLCV.Close <= level.Price {
						sm.Strategy.Model.State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += sm.Strategy.Model.Conditions.EntryOrder.Amount * (level.Amount / 100)
						}
					}
				}
			}
			break
		}
		if sm.Strategy.Model.State.ExecutedAmount == sm.Strategy.Model.Conditions.EntryOrder.Amount {
			sm.Strategy.Model.State.Amount = 0
			sm.Strategy.Model.State.State = TakeProfit // took all profits, exit now
			sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			return true
		}
		if amount > 0 {
			sm.Strategy.Model.State.Amount = amount
			sm.Strategy.Model.State.State = TakeProfit
			currentState, _ := sm.State.State(context.Background())
			if currentState == TakeProfit {
				sm.Strategy.Model.State.State = EnterNextTarget
				sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			}
			return true
		}
	}
	if sm.Strategy.Model.Conditions.TakeProfit > 0 {
		switch sm.Strategy.Model.Conditions.EntryOrder.Side {
		case "buy":
			if (currentOHLCV.Close/sm.Strategy.Model.State.EntryPrice-1)*100 >= sm.Strategy.Model.Conditions.TakeProfit {
				sm.Strategy.Model.State.State = TakeProfit
				sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
				return true
			}
			break
		case "sell":
			if (sm.Strategy.Model.State.EntryPrice/currentOHLCV.Close-1)*100 >= sm.Strategy.Model.Conditions.TakeProfit {
				sm.Strategy.Model.State.State = TakeProfit
				sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
				return true
			}
			break
		}
	}
	return false
}
func (sm *SmartOrder) checkTrailingProfit(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	switch sm.Strategy.Model.Conditions.EntryOrder.Side {
	case "buy":
		for _, level := range sm.Strategy.Model.Conditions.ExitLevels {
			if level.ActivatePrice > 0 {
				if currentOHLCV.Close >= level.Price {
					return true
				}
			}
		}
		break
	case "sell":
		for _, level := range sm.Strategy.Model.Conditions.ExitLevels {
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
	switch sm.Strategy.Model.Conditions.EntryOrder.Side {
	case "buy":
		if (sm.Strategy.Model.State.EntryPrice/currentOHLCV.Close - 1)*100 >= sm.Strategy.Model.Conditions.StopLoss {
			if sm.Strategy.Model.State.ExecutedAmount < sm.Strategy.Model.Conditions.EntryOrder.Amount {
				sm.Strategy.Model.State.Amount = sm.Strategy.Model.Conditions.EntryOrder.Amount - sm.Strategy.Model.State.ExecutedAmount
			}
			sm.Strategy.Model.State.State = Stoploss
			sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			return true
		}
		break
	case "sell":
		if (currentOHLCV.Close/sm.Strategy.Model.State.EntryPrice - 1)*100 >= sm.Strategy.Model.Conditions.StopLoss {
			if sm.Strategy.Model.State.ExecutedAmount < sm.Strategy.Model.Conditions.EntryOrder.Amount {
				sm.Strategy.Model.State.Amount = sm.Strategy.Model.Conditions.EntryOrder.Amount - sm.Strategy.Model.State.ExecutedAmount
			}
			sm.Strategy.Model.State.State = Stoploss
			sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			return true
		}
		break
	}

	return false
}
func (sm *SmartOrder) checkTrailingLoss(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	switch sm.Strategy.Model.Conditions.EntryOrder.Side {
	case "buy":
		if (sm.Strategy.Model.State.EntryPrice/currentOHLCV.Close - 1)*100 >= sm.Strategy.Model.Conditions.StopLoss {
			return true
		}
		break
	case "sell":
		if (currentOHLCV.Close/sm.Strategy.Model.State.EntryPrice - 1)*100 >= sm.Strategy.Model.Conditions.StopLoss {
			return true
		}
		break
	}

	return false
}
func (sm *SmartOrder) enterEnd(ctx context.Context, args ...interface{}) error {
	sm.Strategy.Model.State.State = "End"
	sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
	return nil
}


func (sm *SmartOrder) enterTakeProfit(ctx context.Context, args ...interface{}) error {
	if sm.Strategy.Model.State.Amount > 0 {
		price := args[0].(OHLCV)
		side := "buy"
		if sm.Strategy.Model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		baseAmount := sm.Strategy.Model.State.Amount / price.Close
		sm.ExchangeApi.CreateOrder(
			trading.CreateOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.Order{
					Symbol: sm.Strategy.Model.Conditions.Pair,
					Type:   sm.Strategy.Model.Conditions.EntryOrder.OrderType,
					Side:   side,
					Amount: baseAmount,
					TimeInForce: "GTC",
					Price:  price.Close,
					Params: trading.OrderParams{
						StopPrice: 0,
						Type:      sm.Strategy.Model.Conditions.EntryOrder.OrderType,
					},
				},
			},
		)

		sm.Strategy.Model.State.ExecutedAmount += sm.Strategy.Model.State.Amount
		sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
		if sm.Strategy.Model.State.ExecutedAmount - sm.Strategy.Model.Conditions.EntryOrder.Amount == 0 { // re-check all takeprofit conditions to exit trade ( no additional trades needed no )
			ohlcv := args[0].(OHLCV)
			err := sm.State.FireCtx(context.TODO(), TriggerTrade, ohlcv)

			return err
		}
	}
	return nil
}

func (sm *SmartOrder) enterStopLoss(ctx context.Context, args ...interface{}) error {
	if sm.Strategy.Model.State.Amount > 0 {
		price := args[0].(OHLCV)
		side := "buy"
		if sm.Strategy.Model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		sm.cancelOpenOrders(sm.Strategy.Model.Conditions.Pair)

		baseAmount := sm.Strategy.Model.State.Amount / price.Close
		sm.ExchangeApi.CreateOrder(
			trading.CreateOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.Order{
					Symbol: sm.Strategy.Model.Conditions.Pair,
					Type:   sm.Strategy.Model.Conditions.EntryOrder.OrderType,
					Side:   side,
					Amount: baseAmount,
					Price:  price.Close,
					TimeInForce: "GTC",
					Params: trading.OrderParams{
						StopPrice: 0,
						Type:      sm.Strategy.Model.Conditions.EntryOrder.OrderType,
					},
				},
			},
		)
		_ = sm.State.Fire(CheckLossTrade, args[0])
	}
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
	currentOHLCVp := sm.DataFeed.GetPriceForPairAtExchange(sm.Strategy.Model.Conditions.Pair, sm.ExchangeName, sm.Strategy.Model.Conditions.MarketType)
	if currentOHLCVp != nil {
		currentOHLCV := *currentOHLCVp
		println("new trade", currentOHLCV.Close)
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
		println(sm.Strategy.Model.Conditions.Pair, sm.Strategy.Model.State.TrailingEntryPrice, sm.Strategy.Model.Conditions.EntryDeviation, currentOHLCV.Close, err.Error())
	}
}

type KeyAsset struct {
	KeyId primitive.ObjectID `json:"keyId" bson:"keyId"`
}

func RunSmartOrder(strategy *Strategy, df IDataFeed, td trading.ITrading, keyId *primitive.ObjectID) IStrategyRuntime {
	if keyId == nil {
		KeyAssets := mongodb.GetCollection("core_key_assets")
		keyAssetId := strategy.Model.Conditions.KeyAssetId.String()
		var request bson.D
		request = bson.D{
			{"_id", strategy.Model.Conditions.KeyAssetId},
		}
		println(keyAssetId)
		ctx := context.Background()
		var keyAsset KeyAsset
		err := KeyAssets.FindOne(ctx, request).Decode(&keyAsset)
		if err != nil {
			println("keyAssetsCursor", err)
		}
		keyId = &keyAsset.KeyId
	}
	runtime := NewSmartOrder(strategy, df, td, keyId, strategy.StateMgmt)
	runtime.Start()

	return runtime
}
