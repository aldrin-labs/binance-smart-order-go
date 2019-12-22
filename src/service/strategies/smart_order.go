package strategies

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	WaitForEntry       = "WaitForEntry"
	TrailingEntry      = "TrailingEntry"
	InEntry            = "InEntry"
	TakeProfit         = "TakeProfit"
	Stoploss           = "Stoploss"
	WaitOrderOnTimeout = "WaitOrderOnTimeout"
	WaitOrder          = "WaitOrder"
	End                = "End"
	Canceled           = "Canceled"
	EnterNextTarget    = "EnterNextTarget"
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
	UpdateConditions(strategyId primitive.ObjectID, state *models.MongoStrategyCondition)
	UpdateState(strategyId primitive.ObjectID, state *models.MongoStrategyState)
	GetPosition(strategyId primitive.ObjectID, symbol string)
	GetOrder(orderId string) *models.MongoOrder
	SubscribeToOrder(orderId string, onOrderStatusUpdate func(orderId string, orderStatus string)) error
}

type SmartOrder struct {
	Strategy              *Strategy
	State                 *stateless.StateMachine
	ExchangeName          string
	KeyId                 *primitive.ObjectID
	DataFeed              IDataFeed
	ExchangeApi           trading.ITrading
	StateMgmt             IStateMgmt
	OrdersMap             sync.Map
	StatusByOrderId       sync.Map
	QuantityPrecision     int32
	Lock                  bool
	LastTrailingTimestamp int64
	SelectedExitTarget    int
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func (sm *SmartOrder) toFixed(num float64) float64 {
	output := math.Pow(10, float64(sm.QuantityPrecision))
	return float64(round(num*output)) / output
}

func NewSmartOrder(strategy *Strategy, DataFeed IDataFeed, TradingAPI trading.ITrading, keyId *primitive.ObjectID, stateMgmt IStateMgmt) *SmartOrder {
	sm := &SmartOrder{Strategy: strategy, DataFeed: DataFeed, ExchangeApi: TradingAPI, KeyId: keyId, StateMgmt: stateMgmt, Lock: false}
	initState := WaitForEntry
	sm.QuantityPrecision = 3
	// if state is not empty but if its in the end and open ended, then we skip state value, since want to start over
	if strategy.Model.State.State != "" && !(strategy.Model.State.State == End && strategy.Model.Conditions.ContinueIfEnded == true) {
		initState = strategy.Model.State.State
	}
	State := stateless.NewStateMachine(initState)

	// define triggers and input types:
	State.SetTriggerParameters(TriggerTrade, reflect.TypeOf(OHLCV{}))
	State.SetTriggerParameters(CheckExistingOrders, reflect.TypeOf(""), reflect.TypeOf(""))

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
		sm.checkExistingOrders).OnEntry(sm.enterWaitingEntry)
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
		sm.checkLoss).OnEntry(sm.enterStopLoss)
	State.Configure(End).OnEntry(sm.enterEnd)

	State.Activate()

	sm.State = State
	sm.ExchangeName = "binance"
	// fmt.Printf(sm.State.ToGraph())
	// fmt.Printf("DONE\n")
	isFirstRunSoStateisEmpty := strategy.Model.State.State == ""
	if isFirstRunSoStateisEmpty {
		entryIsNotTrailing := sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice == 0
		if entryIsNotTrailing { // then we must know exact price
			sm.placeOrder(sm.Strategy.Model.Conditions.EntryOrder.Price, WaitForEntry)
		}
	}
	return sm
}

func (sm *SmartOrder) placeOrder(price float64, step string) {
	baseAmount := 0.0
	orderType := "market"
	stopPrice := 0.0
	side := ""
	recursiveCall := false

	prefix := "stop-"
	isFutures := sm.Strategy.Model.Conditions.MarketType == 1
	isSpot := sm.Strategy.Model.Conditions.MarketType == 0
	isTrailingEntry := sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice > 0
	saveOrder := false
	switch step {
	case TrailingEntry:
		orderType = sm.Strategy.Model.Conditions.EntryOrder.OrderType // TODO find out to remove duplicate lines with 154 & 164
		isStopOrdersSupport := isFutures || orderType == "limit"
		if isStopOrdersSupport { // we can place stop order, lets place it
			orderType = prefix + sm.Strategy.Model.Conditions.EntryOrder.OrderType
		} else {
			return
		}
		baseAmount = sm.toFixed((sm.Strategy.Model.Conditions.EntryOrder.Amount * sm.Strategy.Model.Conditions.Leverage) / price)
		side = sm.Strategy.Model.Conditions.EntryOrder.Side
		break
	case InEntry:
		isStopOrdersSupport := isFutures || orderType == "limit"
		if !isTrailingEntry || isStopOrdersSupport {
			return // if it wasnt trailing we knew the price and placed order already (limit or market)
			// but if it was trailing with stop-orders support we also already placed order
		} // so here we only place after trailing market order for spot market:
		orderType = sm.Strategy.Model.Conditions.EntryOrder.OrderType
		baseAmount = sm.toFixed((sm.Strategy.Model.Conditions.EntryOrder.Amount * sm.Strategy.Model.Conditions.Leverage) / price)
		side = sm.Strategy.Model.Conditions.EntryOrder.Side
		break
	case WaitForEntry:
		if isTrailingEntry {
			return // do nothing because we dont know entry price, coz didnt hit activation price yet
		}

		orderType = sm.Strategy.Model.Conditions.EntryOrder.OrderType
		side = sm.Strategy.Model.Conditions.EntryOrder.Side
		baseAmount = sm.toFixed((sm.Strategy.Model.Conditions.EntryOrder.Amount * sm.Strategy.Model.Conditions.Leverage) / price)
		break
	case Stoploss:
		amountPrice := price
		if amountPrice == 0 {
			stoploss := 1 - (sm.Strategy.Model.Conditions.StopLoss / sm.Strategy.Model.Conditions.Leverage)
			amountPrice = sm.Strategy.Model.State.EntryPrice * stoploss
		}
		baseAmount = sm.toFixed((sm.Strategy.Model.State.Amount * sm.Strategy.Model.Conditions.Leverage) / amountPrice)
		side = "buy"
		if sm.Strategy.Model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		if sm.Strategy.Model.Conditions.TimeoutLoss == 0 {
			if isSpot {
				if price > 0 {
					break // keep market order
				} else {
					return // it is attempt to place an order but we are on spot market without stop-market orders here
				}
			} else {
				orderType = "stop-market" // ok we are in futures and can place order before it happened
			}
		} else {
			if price > 0 && sm.Strategy.Model.State.StopLossAt == 0 {
				sm.Strategy.Model.State.StopLossAt = time.Now().Unix()
				go func() {
					time.Sleep(time.Duration(sm.Strategy.Model.Conditions.TimeoutLoss) * time.Second)
					currentState, _ := sm.State.State(context.TODO())
					if currentState == Stoploss {
						sm.placeOrder(price, step)
					}
				}()
			}
			return
		}
		break
	case TakeProfit:
		target := sm.Strategy.Model.Conditions.ExitLevels[sm.SelectedExitTarget]
		isTrailingTarget := target.ActivatePrice > 0
		if price == 0 && isTrailingTarget || sm.Strategy.Model.State.Amount == 0 {
			// trailing exit, we cant place exit order now
			return
		}
		isSpotMarketOrder := target.OrderType == "market" && isSpot
		if price == 0 && !isTrailingTarget {
			orderType = target.OrderType
			if target.OrderType == "market" {
				if isFutures {
					orderType = prefix + target.OrderType
					recursiveCall = true
				} else {
					return // we cant place market order on spot at exists before it happened, because there is no stop markets
				}
			} else {
				recursiveCall = true
			}
			switch target.Type {
			case 0:
				price = target.Price
				break
			case 1:
				price = sm.Strategy.Model.State.EntryPrice * (1 + target.Price/100/sm.Strategy.Model.Conditions.Leverage)
				break
			}
		}
		if price > 0 && !isSpotMarketOrder {
			return // order was placed before, exit
		}
		isNewTrailingMaximum := price == -1
		if isNewTrailingMaximum && isTrailingTarget {
			saveOrder = true
			if target.OrderType == "market" {
				if isFutures {
					orderType = prefix + target.OrderType
				} else {
					return // we cant place stop-market orders on spot so we'll wait for exact price
				}
			} else {
				recursiveCall = true
			}
			price = price * (1 - target.EntryDeviation/100/sm.Strategy.Model.Conditions.Leverage)
		}
		side = "buy"
		if sm.Strategy.Model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		baseAmount = sm.toFixed((sm.Strategy.Model.State.Amount * sm.Strategy.Model.Conditions.Leverage) / price)

		sm.Strategy.Model.State.ExitPrice = price
		sm.Strategy.Model.State.ExecutedAmount += sm.Strategy.Model.State.Amount
		sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
		break
	}

	advancedOrderType := orderType
	if strings.Contains(orderType, "stop") {
		orderType = "stop"
	}
	for {
		response := sm.ExchangeApi.CreateOrder(
			trading.CreateOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.Order{
					Symbol:     sm.Strategy.Model.Conditions.Pair,
					MarketType: sm.Strategy.Model.Conditions.MarketType,
					Type:       orderType,
					Side:       side,
					Amount:     baseAmount,
					Price:      price,
					Params: trading.OrderParams{
						StopPrice: stopPrice,
						Type:      advancedOrderType,
					},
				},
			},
		)
		if response.Status == "OK" {

			switch step  {
			case InEntry, WaitForEntry, TrailingEntry: {
				if orderType == "market" {
					time.Sleep(4000 * time.Millisecond)
					sm.Strategy.Model.State.EntryPrice = sm.StateMgmt.GetOrder(response.Data.Id).Average
				} else {
					sm.Strategy.Model.State.EntryPrice = response.Data.Average
				}
			}
			}
			if orderType != "market" {
				if saveOrder {
					// cancel existing order if there is such
					if len(sm.Strategy.Model.State.ExecutedOrders) > 0 {
						existingOrderId := sm.Strategy.Model.State.ExecutedOrders[0]
						sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
							KeyId: sm.KeyId,
							OrderId: existingOrderId,
						})
					}
					sm.Strategy.Model.State.ExecutedOrders = append(sm.Strategy.Model.State.ExecutedOrders, response.Data.Id)
				}
				go sm.waitForOrder(response.Data.Id, step)
			}
			sm.Strategy.Model.State.Orders = append(sm.Strategy.Model.State.Orders, response.Data.Id)
			sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			break
		} else {
			println(response.Status)
		}
	}
	if recursiveCall && sm.SelectedExitTarget+1 < len(sm.Strategy.Model.Conditions.ExitLevels) {
		sm.placeOrder(price, step)
	}
}

func (sm *SmartOrder) enterWaitingEntry(ctx context.Context, args ...interface{}) error {
	entryIsNotTrailing := sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice == 0
	if entryIsNotTrailing { // then we must know exact price
		sm.placeOrder(sm.Strategy.Model.Conditions.EntryOrder.Price, WaitForEntry)
	}
	return nil
}

func (sm *SmartOrder) enterTrailingEntry(ctx context.Context, args ...interface{}) error {
	currentOHLCV := args[0].(OHLCV)
	sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
	sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
	return nil
}

func (sm *SmartOrder) waitForOrder(orderId string, orderStatus string) {
	sm.StatusByOrderId.Store(orderId, orderStatus)
	_ = sm.StateMgmt.SubscribeToOrder(orderId, sm.orderCallback)
}
func (sm *SmartOrder) orderCallback(orderId string, orderStatus string) {
	_ = sm.State.Fire(CheckExistingOrders, orderId, orderStatus)
}
func (sm *SmartOrder) checkExistingOrders(ctx context.Context, args ...interface{}) bool {
	orderId := args[0].(string)
	orderStatus := args[1].(string)
	step, ok := sm.StatusByOrderId.Load(orderId)
	if !ok {
		return false
	}
	switch orderStatus {
	case "closed":
		switch step {
		case WaitForEntry:
			// if stop-market save price
			sm.Strategy.Model.State.State = InEntry
			return true
		case TakeProfit:
			sm.Strategy.Model.State.State = TakeProfit
			return true
		case Stoploss:
			sm.Strategy.Model.State.State = Stoploss
			return true
		}
		break
	case "canceled":
		switch step {
		case WaitForEntry:
			sm.Strategy.Model.State.State = Canceled
			return true
		case InEntry:
			sm.Strategy.Model.State.State = Canceled
			return true
		}
		break
	}
	return false
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
	deviation := sm.Strategy.Model.Conditions.EntryOrder.EntryDeviation / sm.Strategy.Model.Conditions.Leverage
	switch sm.Strategy.Model.Conditions.EntryOrder.Side {
	case "buy":
		if currentOHLCV.Close < edgePrice {
			sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
			edgePrice = sm.Strategy.Model.State.TrailingEntryPrice
			sm.placeOrder(-1, TrailingEntry)
		}
		if (currentOHLCV.Close/edgePrice-1)*100 >= deviation {
			return true
		}
		break
	case "sell":
		if currentOHLCV.Close > edgePrice {
			sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
			edgePrice = sm.Strategy.Model.State.TrailingEntryPrice
			sm.placeOrder(-1, TrailingEntry)
		}
		if (1-currentOHLCV.Close/edgePrice)*100 >= deviation {
			return true
		}
		break
	}
	return false
}

func (sm *SmartOrder) exitWaitEntry(ctx context.Context, args ...interface{}) (stateless.State, error) {
	println("act price", sm.Strategy.Model.Conditions.ActivationPrice)
	if sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice > 0 {
		println("move to", TrailingEntry)
		return TrailingEntry, nil
	}
	println("move to", InEntry)
	return InEntry, nil
}
func (sm *SmartOrder) checkWaitEntry(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	conditionPrice := sm.Strategy.Model.Conditions.EntryOrder.Price
	if sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice == 0 && sm.Strategy.Model.Conditions.EntryOrder.OrderType == "market" {
		return true
	}
	isTrailing := sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice > 0
	if isTrailing {
		conditionPrice = sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice
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

	sm.placeOrder(currentOHLCV.Close, InEntry)
	sm.placeOrder(0, TakeProfit)
	sm.placeOrder(0, Stoploss)
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
		if sm.Strategy.Model.Conditions.ContinueIfEnded {
			oppositeSide := sm.Strategy.Model.Conditions.EntryOrder.Side
			if oppositeSide == "buy" {
				oppositeSide = "sell"
			} else {
				oppositeSide = "buy"
			}
			sm.Strategy.Model.Conditions.EntryOrder.Side = oppositeSide
			sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice = sm.Strategy.Model.State.ExitPrice
			sm.StateMgmt.UpdateConditions(sm.Strategy.Model.ID, &sm.Strategy.Model.Conditions)

			newState := models.MongoStrategyState{
				State: WaitForEntry,
			}
			sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &newState)
			return WaitForEntry, nil
		}
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
	if nextState == End && sm.Strategy.Model.Conditions.ContinueIfEnded {
		newState := models.MongoStrategyState{
			State:              WaitForEntry,
			TrailingEntryPrice: 0,
			EntryPrice:         0,
			Amount:             0,
			Orders:             nil,
			ExecutedAmount:     0,
			ReachedTargetCount: 0,
		}
		sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &newState)
		return WaitForEntry, nil
	}
	return nextState, nil
}

func (sm *SmartOrder) checkProfit(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)

	if sm.Strategy.Model.Conditions.TimeoutIfProfitable > 0 {
		isProfitable := (sm.Strategy.Model.Conditions.EntryOrder.Side == "buy" && sm.Strategy.Model.Conditions.EntryOrder.Price < currentOHLCV.Close) || (sm.Strategy.Model.Conditions.EntryOrder.Side == "sell" && sm.Strategy.Model.Conditions.EntryOrder.Price > currentOHLCV.Close)
		if isProfitable && sm.Strategy.Model.State.ProfitableAt == 0 {
			sm.Strategy.Model.State.ProfitableAt = time.Now().Unix()
			go func(profitableAt int64) {
				time.Sleep(time.Duration(sm.Strategy.Model.Conditions.TimeoutIfProfitable) * time.Second)
				stillTimeout := profitableAt == sm.Strategy.Model.State.ProfitableAt
				if stillTimeout {
					sm.placeOrder(-1, TakeProfit)
				}
			}(sm.Strategy.Model.State.ProfitableAt)
		} else if !isProfitable {
			sm.Strategy.Model.State.ProfitableAt = 0
		}
	}

	if sm.Strategy.Model.Conditions.ExitLevels != nil {
		amount := 0.0
		switch sm.Strategy.Model.Conditions.EntryOrder.Side {
		case "buy":
			for i, level := range sm.Strategy.Model.Conditions.ExitLevels {
				if sm.Strategy.Model.State.ReachedTargetCount < i+1 {
					if level.Type == 1 && currentOHLCV.Close >= (sm.Strategy.Model.State.EntryPrice*(100+level.Price/sm.Strategy.Model.Conditions.Leverage)/100) ||
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
					if level.Type == 1 && currentOHLCV.Close <= (sm.Strategy.Model.State.EntryPrice*((100-level.Price/sm.Strategy.Model.Conditions.Leverage)/100)) ||
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
	return false
}
func (sm *SmartOrder) checkTrailingProfit(ctx context.Context, args ...interface{}) bool {
	currentOHLCV := args[0].(OHLCV)
	switch sm.Strategy.Model.Conditions.EntryOrder.Side {
	case "buy":
		for i, target := range sm.Strategy.Model.Conditions.ExitLevels {
			isTrailingTarget := target.ActivatePrice > 0
			if isTrailingTarget {
				isActivated := len(sm.Strategy.Model.State.TrailingExitPrices) > i
				deviation := target.EntryDeviation / sm.Strategy.Model.Conditions.Leverage

				activatePrice := target.ActivatePrice
				if target.Type == 1 {
					activatePrice = sm.Strategy.Model.State.EntryPrice * ( 1 + deviation)
				}
				didCrossActivatePrice := currentOHLCV.Close >= activatePrice

				if !isActivated && didCrossActivatePrice {
					sm.Strategy.Model.State.TrailingExitPrices = append(sm.Strategy.Model.State.TrailingExitPrices, currentOHLCV.Close)
					return true
				}
				edgePrice := sm.Strategy.Model.State.TrailingExitPrices[i]

				if currentOHLCV.Close > edgePrice {
					sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
					edgePrice = sm.Strategy.Model.State.TrailingEntryPrice

					go func(trailingPrice float64) {
						time.Sleep(2 * time.Second)
						trailingDidnChange := trailingPrice == sm.Strategy.Model.State.TrailingEntryPrice
						trailingChangedALot := 2 < (sm.Strategy.Model.State.TrailingEntryPrice/trailingPrice-1)*100*sm.Strategy.Model.Conditions.Leverage
						if trailingDidnChange || trailingChangedALot {
							sm.placeOrder(-1, TrailingEntry)
						}
					}(sm.Strategy.Model.State.TrailingEntryPrice)
				}

				deviationFromEdge := (edgePrice/currentOHLCV.Close-1)*100
				if deviationFromEdge >= deviation {
					return true
				}
			}
		}
		break
	case "sell":
		for i, target := range sm.Strategy.Model.Conditions.ExitLevels {
			isTrailingTarget := target.ActivatePrice > 0
			if isTrailingTarget {
				isActivated := len(sm.Strategy.Model.State.TrailingExitPrices) > i
				deviation := target.EntryDeviation / sm.Strategy.Model.Conditions.Leverage
				activateDeviation := target.ActivatePrice / sm.Strategy.Model.Conditions.Leverage

				activatePrice := target.ActivatePrice
				if target.Type == 1 {
					activatePrice = sm.Strategy.Model.State.EntryPrice * ( 1 - activateDeviation)
				}
				didCrossActivatePrice := currentOHLCV.Close <= activatePrice

				if !isActivated && didCrossActivatePrice {
					sm.Strategy.Model.State.TrailingExitPrices = append(sm.Strategy.Model.State.TrailingExitPrices, currentOHLCV.Close)
					return true
				}
				edgePrice := sm.Strategy.Model.State.TrailingExitPrices[i]

				if currentOHLCV.Close < edgePrice {
					sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
					edgePrice = sm.Strategy.Model.State.TrailingEntryPrice

					go func(trailingPrice float64) {
						time.Sleep(2 * time.Second)
						trailingDidnChange := trailingPrice == sm.Strategy.Model.State.TrailingEntryPrice
						trailingChangedALot := 2 < (trailingPrice/sm.Strategy.Model.State.TrailingEntryPrice-1)*100*sm.Strategy.Model.Conditions.Leverage
						if trailingDidnChange || trailingChangedALot {
							sm.placeOrder(-1, TrailingEntry)
						}
					}(sm.Strategy.Model.State.TrailingEntryPrice)
				}
				
				deviationFromEdge := (currentOHLCV.Close/edgePrice-1)*100
				if deviationFromEdge >= deviation {
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
	stopLoss := sm.Strategy.Model.Conditions.StopLoss / sm.Strategy.Model.Conditions.Leverage

	// try exit on timeout
	if sm.Strategy.Model.Conditions.TimeoutWhenLoss > 0 {
		isLoss := (sm.Strategy.Model.Conditions.EntryOrder.Side == "buy" && sm.Strategy.Model.Conditions.EntryOrder.Price > currentOHLCV.Close) || (sm.Strategy.Model.Conditions.EntryOrder.Side == "sell" && sm.Strategy.Model.Conditions.EntryOrder.Price < currentOHLCV.Close)
		if isLoss && sm.Strategy.Model.State.LossableAt == 0 {
			sm.Strategy.Model.State.LossableAt = time.Now().Unix()
			go func(lossAt int64) {
				time.Sleep(time.Duration(sm.Strategy.Model.Conditions.TimeoutWhenLoss) * time.Second)
				stillTimeout := lossAt == sm.Strategy.Model.State.LossableAt
				if stillTimeout {
					sm.placeOrder(-1, Stoploss)
				}
			}(sm.Strategy.Model.State.LossableAt)
		} else if !isLoss {
			sm.Strategy.Model.State.LossableAt = 0
		}
	}
	switch sm.Strategy.Model.Conditions.EntryOrder.Side {
	case "buy":
		if (1-currentOHLCV.Close/sm.Strategy.Model.State.EntryPrice)*100 >= stopLoss {
			if sm.Strategy.Model.State.ExecutedAmount < sm.Strategy.Model.Conditions.EntryOrder.Amount {
				sm.Strategy.Model.State.Amount = sm.Strategy.Model.Conditions.EntryOrder.Amount - sm.Strategy.Model.State.ExecutedAmount
			}
			sm.Strategy.Model.State.State = Stoploss
			sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			return true
		}
		break
	case "sell":
		if (currentOHLCV.Close/sm.Strategy.Model.State.EntryPrice-1)*100 >= stopLoss {
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
func (sm *SmartOrder) enterEnd(ctx context.Context, args ...interface{}) error {
	sm.Strategy.Model.State.State = "End"
	sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
	return nil
}

func (sm *SmartOrder) enterTakeProfit(ctx context.Context, args ...interface{}) error {
	if sm.Strategy.Model.State.Amount > 0 {
		price := args[0].(OHLCV)

		if sm.Lock {
			return nil
		}
		sm.Lock = true
		sm.placeOrder(price.Close, TakeProfit)

		//if sm.Strategy.Model.State.ExecutedAmount - sm.Strategy.Model.Conditions.EntryOrder.Amount == 0 { // re-check all takeprofit conditions to exit trade ( no additional trades needed no )
		//	ohlcv := args[0].(OHLCV)
		//	err := sm.State.FireCtx(context.TODO(), TriggerTrade, ohlcv)
		//
		//	sm.Lock = false
		//	return err
		//}
	}
	sm.Lock = false
	return nil
}

func (sm *SmartOrder) enterStopLoss(ctx context.Context, args ...interface{}) error {
	if sm.Strategy.Model.State.Amount > 0 {
		price := args[0].(OHLCV)
		side := "buy"
		if sm.Strategy.Model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		if sm.Lock {
			return nil
		}
		sm.Lock = true

		sm.cancelOpenOrders(sm.Strategy.Model.Conditions.Pair)

		baseAmount := sm.toFixed((sm.Strategy.Model.State.Amount * sm.Strategy.Model.Conditions.Leverage) / price.Close)
		sm.ExchangeApi.CreateOrder(
			trading.CreateOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.Order{
					Symbol:     sm.Strategy.Model.Conditions.Pair,
					MarketType: sm.Strategy.Model.Conditions.MarketType,
					Type:       sm.Strategy.Model.Conditions.EntryOrder.OrderType,
					Side:       side,
					Amount:     baseAmount,
					Price:      price.Close,
					Params: trading.OrderParams{
						StopPrice: 0,
						Type:      sm.Strategy.Model.Conditions.EntryOrder.OrderType,
					},
				},
			},
		)
		_ = sm.State.Fire(CheckLossTrade, args[0])
		sm.Strategy.Model.State.ExitPrice = price.Close
		sm.Strategy.Model.State.ExecutedAmount += sm.Strategy.Model.State.Amount
		sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
	}
	// if timeout specified then do this sell on timeout
	sm.Lock = false

	return nil
}

func (sm *SmartOrder) cancelOpenOrders(pair string) {
}

func (sm *SmartOrder) Start() {
	state, _ := sm.State.State(context.Background())
	for state != End {
		if sm.Strategy.Model.Enabled == false {
			return
		}
		if !sm.Lock {
			sm.processEventLoop()
		}
		time.Sleep(1000 * time.Millisecond)
		state, _ = sm.State.State(context.Background())
	}
	println("STOPPED")
}

func (sm *SmartOrder) Stop() {
	// TODO: implement here canceling all open orders, etc
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
		KeyAssets := mongodb.GetCollection("core_key_assets") // TODO: move to statemgmt, avoid any direct dependecies here
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
