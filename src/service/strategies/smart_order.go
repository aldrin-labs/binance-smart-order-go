package strategies

import (
	"context"
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
	GetPriceForPairAtExchange(pair string, exchange string) *OHLCV
}

type ITrading interface {
	CreateOrder(exchange string, pair string, price float64, amount float64, side string) string
}

type SmartOrder struct {
	Model        *models.MongoStrategy
	State        *stateless.StateMachine
	ExchangeName string
	KeyId		 *primitive.ObjectID
	DataFeed     IDataFeed
	ExchangeApi  trading.ITrading
}

func NewSmartOrder(smartOrder *models.MongoStrategy, DataFeed IDataFeed, TradingAPI trading.ITrading, keyId *primitive.ObjectID) *SmartOrder {
	sm := &SmartOrder{Model: smartOrder, DataFeed: DataFeed, ExchangeApi: TradingAPI, KeyId: keyId}
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
	switch sm.Model.Conditions.EntryOrder.Side {
	case "buy":
		if (currentOHLCV.Close/edgePrice-1)*100 >= sm.Model.Conditions.EntryDeviation {
			return true
		}
		break
	case "sell":
		if (edgePrice/currentOHLCV.Close-1)*100 >= sm.Model.Conditions.EntryDeviation {
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
	println("act price", sm.Model.Conditions.ActivationPrice)
	if sm.Model.Conditions.ActivationPrice > 0 {
		println("move to", TrailingEntry)
		return TrailingEntry, nil
	}
	println("move to", InEntry)
	return InEntry, nil
}
func (sm *SmartOrder) checkWaitEntry(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	conditionPrice := sm.Model.Conditions.Price
	if sm.Model.Conditions.ActivationPrice > 0 {
		conditionPrice = sm.Model.Conditions.ActivationPrice
	}
	switch sm.Model.Conditions.EntryOrder.Side {
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
		trading.CreateOrderRequest{
			KeyId: sm.KeyId,
			KeyParams: trading.Order{
				Symbol: sm.Model.Conditions.Pair,
				Type:   sm.Model.Conditions.EntryOrder.Type,
				Side:   sm.Model.Conditions.EntryOrder.Side,
				Amount: sm.Model.Conditions.EntryOrder.Amount,
				Price:  currentOHLCV.Close,
				Params: trading.OrderParams{
					StopPrice: 0,
					Type:      sm.Model.Conditions.EntryOrder.Type,
				},
			},
		},
	)
	return nil
}

func (sm *SmartOrder) exit(ctx context.Context, args ...interface{}) (stateless.State, error) {
	state, _ := sm.State.State(context.TODO())
	nextState := End
	if sm.Model.State.ExecutedAmount == sm.Model.Conditions.EntryOrder.Amount { // all trades executed, nothing more to trade
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
	if sm.Model.Conditions.ExitLevels != nil {
		amount := 0.0
		switch sm.Model.Conditions.EntryOrder.Side {
		case "buy":
			for i, level := range sm.Model.Conditions.ExitLevels {
				if sm.Model.State.ReachedTargetCount < i+1 {
					if level.Type == 1 && currentOHLCV.Close >= sm.Model.State.EntryPrice * (1 + level.Price / 100) ||
						level.Type == 0 && currentOHLCV.Close >= level.Price {
						sm.Model.State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += sm.Model.Conditions.EntryOrder.Amount * (level.Amount / 100)
						}
					}
				}
			}
			break
		case "sell":
			for i, level := range sm.Model.Conditions.ExitLevels {
				if sm.Model.State.ReachedTargetCount < i+1 {
					if level.Type == 1 && currentOHLCV.Close <= sm.Model.State.EntryPrice * level.Price ||
											level.Type == 0 && currentOHLCV.Close <= level.Price {
						sm.Model.State.ReachedTargetCount += 1
						if level.Type == 0 {
							amount += level.Amount
						} else if level.Type == 1 {
							amount += sm.Model.Conditions.EntryOrder.Amount * (level.Amount / 100)
						}
					}
				}
			}
			break
		}
		if sm.Model.State.ExecutedAmount == sm.Model.Conditions.EntryOrder.Amount {
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
	if sm.Model.Conditions.TakeProfit > 0 {
		switch sm.Model.Conditions.EntryOrder.Side {
		case "buy":
			if (currentOHLCV.Close/sm.Model.State.EntryPrice-1)*100 >= sm.Model.Conditions.TakeProfit {
				sm.Model.State.State = TakeProfit
				return true
			}
			break
		case "sell":
			if (sm.Model.State.EntryPrice/currentOHLCV.Close-1)*100 >= sm.Model.Conditions.TakeProfit {
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
	switch sm.Model.Conditions.EntryOrder.Side {
	case "buy":
		for _, level := range sm.Model.Conditions.ExitLevels {
			if level.ActivatePrice > 0 {
				if currentOHLCV.Close >= level.Price {
					return true
				}
			}
		}
		break
	case "sell":
		for _, level := range sm.Model.Conditions.ExitLevels {
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
	switch sm.Model.Conditions.EntryOrder.Side {
	case "buy":
		if (sm.Model.State.EntryPrice/currentOHLCV.Close - 1)*100 >= sm.Model.Conditions.StopLoss {
			if sm.Model.State.ExecutedAmount < sm.Model.Conditions.EntryOrder.Amount {
				sm.Model.State.Amount = sm.Model.Conditions.EntryOrder.Amount - sm.Model.State.ExecutedAmount
			}
			sm.Model.State.State = Stoploss
			return true
		}
		break
	case "sell":
		if (currentOHLCV.Close/sm.Model.State.EntryPrice - 1)*100 >= sm.Model.Conditions.StopLoss {
			if sm.Model.State.ExecutedAmount < sm.Model.Conditions.EntryOrder.Amount {
				sm.Model.State.Amount = sm.Model.Conditions.EntryOrder.Amount - sm.Model.State.ExecutedAmount
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
	switch sm.Model.Conditions.EntryOrder.Side {
	case "buy":
		if (sm.Model.State.EntryPrice/currentOHLCV.Close - 1)*100 >= sm.Model.Conditions.StopLoss {
			return true
		}
		break
	case "sell":
		if (currentOHLCV.Close/sm.Model.State.EntryPrice - 1)*100 >= sm.Model.Conditions.StopLoss {
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
		if sm.Model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}

		sm.ExchangeApi.CreateOrder(
			trading.CreateOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.Order{
					Symbol: sm.Model.Conditions.Pair,
					Type:   sm.Model.Conditions.EntryOrder.Type,
					Side:   side,
					Amount: sm.Model.State.Amount,
					Price:  price.Close,
					Params: trading.OrderParams{
						StopPrice: 0,
						Type:      sm.Model.Conditions.EntryOrder.Type,
					},
				},
			},
		)

		sm.Model.State.ExecutedAmount += sm.Model.State.Amount
		if sm.Model.State.ExecutedAmount - sm.Model.Conditions.EntryOrder.Amount == 0 { // re-check all takeprofit conditions to exit trade ( no additional trades needed no )
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
	if sm.Model.Conditions.EntryOrder.Side == side {
		side = "sell"
	}
	sm.cancelOpenOrders(sm.Model.Conditions.Pair)

	sm.ExchangeApi.CreateOrder(
		trading.CreateOrderRequest{
			KeyId: sm.KeyId,
			KeyParams: trading.Order{
				Symbol: sm.Model.Conditions.Pair,
				Type:   sm.Model.Conditions.EntryOrder.Type,
				Side:   side,
				Amount: sm.Model.State.Amount,
				Price:  price.Close,
				Params: trading.OrderParams{
					StopPrice: 0,
					Type:      sm.Model.Conditions.EntryOrder.Type,
				},
			},
		},
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
	currentOHLCV := sm.DataFeed.GetPriceForPairAtExchange(sm.Model.Conditions.Pair, sm.ExchangeName)
	if currentOHLCV != nil {
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
	runtime := NewSmartOrder(strategy.Model, df, td, keyId)
	runtime.Start()

	return runtime
}
