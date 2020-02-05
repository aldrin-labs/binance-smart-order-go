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
	UpdateEntryPrice(strategyId primitive.ObjectID, state *models.MongoStrategyState)
	UpdateState(strategyId primitive.ObjectID, state *models.MongoStrategyState)
	UpdateOrders(strategyId primitive.ObjectID, state *models.MongoStrategyState)
	UpdateExecutedAmount(strategyId primitive.ObjectID, state *models.MongoStrategyState)
	GetPosition(strategyId primitive.ObjectID, symbol string)
	GetOrder(orderId string) *models.MongoOrder
	SubscribeToOrder(orderId string, onOrderStatusUpdate func(order *models.MongoOrder)) error
	DisableStrategy(strategyId primitive.ObjectID)
	GetMarketPrecision(pair string, marketType int64) (int64, int64)
}

type SmartOrder struct {
	Strategy                *Strategy
	State                   *stateless.StateMachine
	ExchangeName            string
	KeyId                   *primitive.ObjectID
	DataFeed                IDataFeed
	ExchangeApi             trading.ITrading
	StateMgmt               IStateMgmt
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

func NewSmartOrder(strategy *Strategy, DataFeed IDataFeed, TradingAPI trading.ITrading, keyId *primitive.ObjectID, stateMgmt IStateMgmt) *SmartOrder {
	sm := &SmartOrder{Strategy: strategy, DataFeed: DataFeed, ExchangeApi: TradingAPI, KeyId: keyId, StateMgmt: stateMgmt, Lock: false, SelectedExitTarget: 0}
	initState := WaitForEntry
	pricePrecision, amountPrecision := stateMgmt.GetMarketPrecision(strategy.Model.Conditions.Pair, strategy.Model.Conditions.MarketType)
	sm.QuantityPricePrecision = pricePrecision
	sm.QuantityAmountPrecision = amountPrecision
	// if state is not empty but if its in the end and open ended, then we skip state value, since want to start over
	if strategy.Model.State.State != "" && !(strategy.Model.State.State == End && strategy.Model.Conditions.ContinueIfEnded == true) {
		initState = strategy.Model.State.State
	}
	State := stateless.NewStateMachine(initState)

	// define triggers and input types:
	State.SetTriggerParameters(TriggerTrade, reflect.TypeOf(OHLCV{}))
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
	isFirstRunSoStateisEmpty := strategy.Model.State.State == ""
	if sm.Strategy.Model.Conditions.WaitingEntryTimeout > 0 {
		go func() {
			time.Sleep(time.Duration(sm.Strategy.Model.Conditions.WaitingEntryTimeout) * time.Second)
			currentState, _ := sm.State.State(context.TODO())
			if currentState == WaitForEntry || currentState == TrailingEntry {
				sm.Strategy.Model.Enabled = false
			}
		}()
	}

	if sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice > 0 &&
		sm.Strategy.Model.Conditions.ActivationMoveTimeout > 0 {
		go func() {
			currentState, _ := sm.State.State(context.TODO())
			for currentState == WaitForEntry {
				time.Sleep(time.Duration(sm.Strategy.Model.Conditions.ActivationMoveTimeout) * time.Second)
				currentState, _ = sm.State.State(context.TODO())
				if currentState == WaitForEntry {
					activatePrice := sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice
					side := sm.Strategy.Model.Conditions.EntryOrder.Side
					if side == "sell" {
						activatePrice = sm.Strategy.Model.State.TrailingEntryPrice * (1 - sm.Strategy.Model.Conditions.ActivationMoveStep/100/sm.Strategy.Model.Conditions.Leverage)
					} else {
						activatePrice = sm.Strategy.Model.State.TrailingEntryPrice * (1 + sm.Strategy.Model.Conditions.ActivationMoveStep/100/sm.Strategy.Model.Conditions.Leverage)
					}
					sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice = activatePrice
				}
			}
		}()
	}

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
	orderPrice := price

	recursiveCall := false
	reduceOnly := false

	oppositeSide := "buy"
	if sm.Strategy.Model.Conditions.EntryOrder.Side == oppositeSide {
		oppositeSide = "sell"
	}
	prefix := "stop-"
	isFutures := sm.Strategy.Model.Conditions.MarketType == 1
	isSpot := sm.Strategy.Model.Conditions.MarketType == 0
	isTrailingEntry := sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice > 0
	ifShouldCancelPreviousOrder := false
	switch step {
	case TrailingEntry:
		orderType = sm.Strategy.Model.Conditions.EntryOrder.OrderType // TODO find out to remove duplicate lines with 154 & 164
		isStopOrdersSupport := isFutures || orderType == "limit"
		if isStopOrdersSupport { // we can place stop order, lets place it
			orderType = prefix + sm.Strategy.Model.Conditions.EntryOrder.OrderType
		} else {
			return
		}
		baseAmount = sm.Strategy.Model.Conditions.EntryOrder.Amount

		isNewTrailingMaximum := price == -1
		isTrailingTarget := sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice > 0
		if isNewTrailingMaximum && isTrailingTarget {
			ifShouldCancelPreviousOrder = true
			if sm.Strategy.Model.Conditions.EntryOrder.OrderType == "market" {
				if isFutures {
					orderType = prefix + sm.Strategy.Model.Conditions.EntryOrder.OrderType
				} else {
					return // we cant place stop-market orders on spot so we'll wait for exact price
				}
			}
		} else {
			return
		}
		side = sm.Strategy.Model.Conditions.EntryOrder.Side
		if side == "sell" {
			orderPrice = sm.Strategy.Model.State.TrailingEntryPrice * (1 - sm.Strategy.Model.Conditions.EntryOrder.EntryDeviation/100/sm.Strategy.Model.Conditions.Leverage)
		} else {
			orderPrice = sm.Strategy.Model.State.TrailingEntryPrice * (1 + sm.Strategy.Model.Conditions.EntryOrder.EntryDeviation/100/sm.Strategy.Model.Conditions.Leverage)
		}
		break
	case InEntry:
		isStopOrdersSupport := isFutures || orderType == "limit"
		if !isTrailingEntry || isStopOrdersSupport {
			return // if it wasnt trailing we knew the price and placed order already (limit or market)
			// but if it was trailing with stop-orders support we also already placed order
		} // so here we only place after trailing market order for spot market:
		orderType = sm.Strategy.Model.Conditions.EntryOrder.OrderType
		baseAmount = sm.Strategy.Model.Conditions.EntryOrder.Amount
		side = sm.Strategy.Model.Conditions.EntryOrder.Side
		break
	case WaitForEntry:
		if isTrailingEntry {
			return // do nothing because we dont know entry price, coz didnt hit activation price yet
		}

		orderType = sm.Strategy.Model.Conditions.EntryOrder.OrderType
		side = sm.Strategy.Model.Conditions.EntryOrder.Side
		baseAmount = sm.Strategy.Model.Conditions.EntryOrder.Amount
		break
	case Stoploss:
		reduceOnly = true
		baseAmount = sm.Strategy.Model.Conditions.EntryOrder.Amount - sm.Strategy.Model.State.ExecutedAmount
		side = "buy"

		if sm.Strategy.Model.Conditions.EntryOrder.Side == side {
			side = "sell"
		}
		if sm.Strategy.Model.Conditions.TimeoutLoss == 0 {
			orderType = sm.Strategy.Model.Conditions.StopLossType
			isStopOrdersSupport := isFutures || orderType == "limit"
			if isSpot {
				if price > 0 {
					break // keep market order
				} else if !isStopOrdersSupport {
					return // it is attempt to place an order but we are on spot market without stop-market orders here
				}
			}
			orderType = prefix + orderType // ok we are in futures and can place order before it happened

			stopLoss := sm.Strategy.Model.Conditions.StopLoss
			if side == "sell" {
				orderPrice = sm.Strategy.Model.State.EntryPrice * (1 - stopLoss/100/sm.Strategy.Model.Conditions.Leverage)
			} else {
				orderPrice = sm.Strategy.Model.State.EntryPrice * (1 + stopLoss/100/sm.Strategy.Model.Conditions.Leverage)
			}
		} else {
			if price > 0 && sm.Strategy.Model.State.StopLossAt == 0 {
				sm.Strategy.Model.State.StopLossAt = time.Now().Unix()
				go func(lastTimestamp int64) {
					time.Sleep(time.Duration(sm.Strategy.Model.Conditions.TimeoutLoss) * time.Second)
					currentState, _ := sm.State.State(context.TODO())
					if currentState == Stoploss && sm.Strategy.Model.State.StopLossAt == lastTimestamp {
						sm.placeOrder(price, step)
					}
				}(sm.Strategy.Model.State.StopLossAt)
				return
			} else if price > 0 && sm.Strategy.Model.State.StopLossAt > 0 {
				orderType = "market"
				break
			} else {
				return // cant do anything here
			}
		}
		break
	case TakeProfit:
		prefix := "take-profit-"
		reduceOnly = true
		if sm.SelectedExitTarget >= len(sm.Strategy.Model.Conditions.ExitLevels) {
			return
		}
		target := sm.Strategy.Model.Conditions.ExitLevels[sm.SelectedExitTarget]
		isTrailingTarget := target.ActivatePrice > 0
		isSpotMarketOrder := target.OrderType == "market" && isSpot
		if price == 0 && isTrailingTarget {
			// trailing exit, we cant place exit order now
			return
		}
		if price > 0 && !isSpotMarketOrder {
			return // order was placed before, exit
		}

		side = oppositeSide
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
				orderPrice = target.Price
				break
			case 1:
				if side == "sell" {
					orderPrice = sm.Strategy.Model.State.EntryPrice * (1 + target.Price/100/sm.Strategy.Model.Conditions.Leverage)
				} else {
					orderPrice = sm.Strategy.Model.State.EntryPrice * (1 - target.Price/100/sm.Strategy.Model.Conditions.Leverage)
				}
				break
			}
		}
		isNewTrailingMaximum := price == -1
		if isNewTrailingMaximum && isTrailingTarget {
			prefix = "stop-"
			ifShouldCancelPreviousOrder = true
			if target.OrderType == "market" {
				if isFutures {
					orderType = prefix + target.OrderType
				} else if price == 0 {
					return // we cant place stop-market orders on spot so we'll wait for exact price
				}
			} else {
				recursiveCall = true // if its not instant order check maybe we can try to place other positions
			}
			orderPrice = sm.Strategy.Model.State.TrailingEntryPrice * (1 - target.EntryDeviation/100/sm.Strategy.Model.Conditions.Leverage)
		}
		if sm.SelectedExitTarget < len(sm.Strategy.Model.Conditions.ExitLevels) - 1 {
			baseAmount = target.Amount
			if target.Type == 1 {
				if baseAmount == 0 {
					baseAmount = 100
				}
				baseAmount = sm.Strategy.Model.Conditions.EntryOrder.Amount * (baseAmount / 100)
			}
		} else {
			baseAmount = sm.getLastTargetAmount()
		}
		// sm.Strategy.Model.State.ExecutedAmount += amount
		break
	case Canceled:
		{
			currentState, _ := sm.State.State(context.TODO())
			thereIsNoEntryToExit := currentState == WaitForEntry || currentState == TrailingEntry || currentState == End
			if thereIsNoEntryToExit {
				return
			}
			side = oppositeSide
			reduceOnly = true
			baseAmount = sm.Strategy.Model.Conditions.EntryOrder.Amount
			break
		}
	}

	baseAmount = sm.toFixed(baseAmount, sm.QuantityAmountPrecision)
	orderPrice = sm.toFixed(orderPrice, sm.QuantityPricePrecision)

	advancedOrderType := orderType
	if strings.Contains(orderType, "stop") || strings.Contains(orderType, "take-profit") {
		orderType = "stop"
		stopPrice = orderPrice
	}
	for {
		if baseAmount == 0 {
			return
		}
		request := trading.CreateOrderRequest{
			KeyId: sm.KeyId,
			KeyParams: trading.Order{
				Symbol:     sm.Strategy.Model.Conditions.Pair,
				MarketType: sm.Strategy.Model.Conditions.MarketType,
				Type:       orderType,
				Side:       side,
				Amount:     baseAmount,
				Price:      orderPrice,
				ReduceOnly: reduceOnly,
				StopPrice:  stopPrice,
			},
		}
		if request.KeyParams.Type == "stop" {
			request.KeyParams.Params = trading.OrderParams{
				Type: advancedOrderType,
			}
		}
		if step == TrailingEntry && orderType != "market" && ifShouldCancelPreviousOrder && len(sm.Strategy.Model.State.ExecutedOrders) > 0 {
			count := len(sm.Strategy.Model.State.ExecutedOrders)
			existingOrderId := sm.Strategy.Model.State.ExecutedOrders[count-1]
			response := sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.CancelOrderRequestParams{
					OrderId:    existingOrderId,
					MarketType: sm.Strategy.Model.Conditions.MarketType,
					Pair:       sm.Strategy.Model.Conditions.Pair,
				},
			})
			if response.Status == "ERR" { // looks like order was already executed or canceled in other thread
				return
			}
		}
		response := sm.ExchangeApi.CreateOrder(request)
		if response.Status == "OK" && response.Data.Id != "0" && response.Data.Id != "" {
			sm.IsWaitingForOrder.Store(step, true)
			if ifShouldCancelPreviousOrder {
				// cancel existing order if there is such ( and its not TrailingEntry )
				if len(sm.Strategy.Model.State.ExecutedOrders) > 0 && step != TrailingEntry {
					count := len(sm.Strategy.Model.State.ExecutedOrders)
					existingOrderId := sm.Strategy.Model.State.ExecutedOrders[count-1]
					sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
						KeyId: sm.KeyId,
						KeyParams: trading.CancelOrderRequestParams{
							OrderId:    existingOrderId,
							MarketType: sm.Strategy.Model.Conditions.MarketType,
							Pair:       sm.Strategy.Model.Conditions.Pair,
						},
					})
				}
				sm.Strategy.Model.State.ExecutedOrders = append(sm.Strategy.Model.State.ExecutedOrders, response.Data.Id)
			}
			if response.Data.Id != "0" {
				go sm.waitForOrder(response.Data.Id, step)
			} else {
				println("order 0")
			}
			sm.Strategy.Model.State.Orders = append(sm.Strategy.Model.State.Orders, response.Data.Id)
			sm.StateMgmt.UpdateOrders(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			break
		} else {
			println(response.Status)
			if response.Status == "OK" {
				break
			}
		}
	}
	canPlaceAnotherOrderForNextTarget := sm.SelectedExitTarget+1 < len(sm.Strategy.Model.Conditions.ExitLevels)
	if recursiveCall && canPlaceAnotherOrderForNextTarget {
		sm.SelectedExitTarget += 1
		sm.placeOrder(price, step)
	}
}

func (sm *SmartOrder) getLastTargetAmount() float64 {
	sumAmount := 0.0
	length := len(sm.Strategy.Model.Conditions.ExitLevels)
	for i, target := range sm.Strategy.Model.Conditions.ExitLevels {
		if i < length-1 {
			baseAmount := target.Amount
			if target.Type == 1 {
				if baseAmount == 0 {
					baseAmount = 100
				}
				baseAmount = sm.Strategy.Model.Conditions.EntryOrder.Amount * (baseAmount / 100)
			}
			baseAmount = sm.toFixed(baseAmount, sm.QuantityAmountPrecision)
			sumAmount += baseAmount
		}
	}
	endTargetAmount := sm.Strategy.Model.Conditions.EntryOrder.Amount - sumAmount
	return endTargetAmount
}

func (sm *SmartOrder) enterTrailingEntry(ctx context.Context, args ...interface{}) error {
	if currentOHLCV, ok := args[0].(OHLCV); ok {
		sm.Strategy.Model.State.State = TrailingEntry
		sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
		sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
	} else {
		panic("no ohlcv in trailing !!")
	}
	return nil
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
			if sm.Strategy.Model.State.EntryPrice > 0 {
				return false
			}
			sm.Strategy.Model.State.EntryPrice = order.Average
			sm.Strategy.Model.State.State = InEntry
			sm.StateMgmt.UpdateEntryPrice(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			return true
		case WaitForEntry:
			if sm.Strategy.Model.State.EntryPrice > 0 {
				return false
			}
			sm.Strategy.Model.State.EntryPrice = order.Average
			sm.Strategy.Model.State.State = InEntry
			sm.StateMgmt.UpdateEntryPrice(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			return true
		case TakeProfit:
			if order.Filled > 0 {
				sm.Strategy.Model.State.ExecutedAmount += order.Filled
			}
			sm.Strategy.Model.State.ExitPrice = order.Average
			if sm.Strategy.Model.State.ExecutedAmount >= sm.Strategy.Model.Conditions.EntryOrder.Amount {
				sm.Strategy.Model.State.State = End
			} else {
				go sm.placeOrder(0, Stoploss)
			}
			sm.StateMgmt.UpdateExecutedAmount(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			return true
		case Stoploss:
			if order.Filled > 0 {
				sm.Strategy.Model.State.ExecutedAmount += order.Filled
			}
			sm.Strategy.Model.State.ExitPrice = order.Average
			sm.StateMgmt.UpdateExecutedAmount(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
			if sm.Strategy.Model.State.ExecutedAmount >= sm.Strategy.Model.Conditions.EntryOrder.Amount {
				sm.Strategy.Model.State.State = End
			}
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
	//isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TrailingEntry)
	//if ok && isWaitingForOrder.(bool) {
	//	return false
	//}
	currentOHLCV := args[0].(OHLCV)
	edgePrice := sm.Strategy.Model.State.TrailingEntryPrice
	if len(sm.Strategy.Model.State.ExecutedOrders) == 0 {
		isFutures := sm.Strategy.Model.Conditions.MarketType == 1
		orderType := sm.Strategy.Model.Conditions.EntryOrder.OrderType
		isStopOrdersSupport := isFutures || orderType == "limit"

		if isStopOrdersSupport { // we can place stop order, lets place it
			if sm.Lock == false {
				sm.Lock = true
				if sm.Strategy.Model.State.TrailingEntryPrice == 0 {
					sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
				}
				go func() {
					sm.placeOrder(-1, TrailingEntry)
					time.Sleep(3000 * time.Millisecond)
					sm.Lock = false
				}() // it will give some time for order execution, to avoid double send of orders
			}
			return false
		}
	}

	if edgePrice == 0 {
		println("edgePrice=0")
		sm.Strategy.Model.State.TrailingEntryPrice = currentOHLCV.Close
		return false
	}
	// println(currentOHLCV.Close, edgePrice, currentOHLCV.Close/edgePrice-1)
	deviation := sm.Strategy.Model.Conditions.EntryOrder.EntryDeviation / sm.Strategy.Model.Conditions.Leverage
	side := sm.Strategy.Model.Conditions.EntryOrder.Side
	isSpotMarketEntry := sm.Strategy.Model.Conditions.MarketType == 0 && sm.Strategy.Model.Conditions.EntryOrder.OrderType == "market"
	switch side {
	case "buy":
		if !isSpotMarketEntry && currentOHLCV.Close < edgePrice {
			edgePrice = sm.Strategy.Model.State.TrailingEntryPrice
			go sm.placeTrailingOrder(currentOHLCV.Close, time.Now().UnixNano(), 0, side, true, TrailingEntry)
		}
		if isSpotMarketEntry && (currentOHLCV.Close/edgePrice-1)*100 >= deviation {
			return true
		}
		break
	case "sell":
		if !isSpotMarketEntry && currentOHLCV.Close > edgePrice {
			go sm.placeTrailingOrder(currentOHLCV.Close, time.Now().UnixNano(), 0, side, true, TrailingEntry)
		}
		if isSpotMarketEntry && (1-currentOHLCV.Close/edgePrice)*100 >= deviation {
			return true
		}
		break
	}
	return false
}

func (sm *SmartOrder) exitWaitEntry(ctx context.Context, args ...interface{}) (stateless.State, error) {
	if sm.Strategy.Model.Conditions.EntryOrder.ActivatePrice > 0 {
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
	if currentOHLCV, ok := args[0].(OHLCV); ok {
		sm.Strategy.Model.State.EntryPrice = currentOHLCV.Close
	}
	sm.Strategy.Model.State.State = InEntry
	sm.Strategy.Model.State.TrailingEntryPrice = 0
	sm.Strategy.Model.State.ExecutedOrders = []string{}
	go sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)

	go sm.placeOrder(sm.Strategy.Model.State.EntryPrice, InEntry)
	go sm.placeOrder(0, TakeProfit)
	go sm.placeOrder(0, Stoploss)
	return nil
}

func (sm *SmartOrder) exit(ctx context.Context, args ...interface{}) (stateless.State, error) {
	state, _ := sm.State.State(context.TODO())
	nextState := End
	if sm.Strategy.Model.State.ExecutedAmount >= sm.Strategy.Model.Conditions.EntryOrder.Amount { // all trades executed, nothing more to trade
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
		case InEntry:
			nextState = InEntry
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
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	if ok && isWaitingForOrder.(bool) {
		return false
	}
	currentOHLCV := args[0].(OHLCV)

	if sm.Strategy.Model.Conditions.TimeoutIfProfitable > 0 {
		isProfitable := (sm.Strategy.Model.Conditions.EntryOrder.Side == "buy" && sm.Strategy.Model.State.EntryPrice < currentOHLCV.Close) ||
			(sm.Strategy.Model.Conditions.EntryOrder.Side == "sell" && sm.Strategy.Model.State.EntryPrice > currentOHLCV.Close)
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
				if sm.Strategy.Model.State.ReachedTargetCount < i+1 && level.ActivatePrice == 0 {
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
				if sm.Strategy.Model.State.ReachedTargetCount < i+1 && level.ActivatePrice == 0 {
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
	//isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(TakeProfit)
	//if ok && isWaitingForOrder.(bool) {
	//	return false
	//}
	currentOHLCV := args[0].(OHLCV)

	side := sm.Strategy.Model.Conditions.EntryOrder.Side
	switch side {
	case "buy":
		for i, target := range sm.Strategy.Model.Conditions.ExitLevels {
			isTrailingTarget := target.ActivatePrice > 0
			if isTrailingTarget {
				isActivated := i < len(sm.Strategy.Model.State.TrailingExitPrices)
				deviation := target.EntryDeviation / 100 / sm.Strategy.Model.Conditions.Leverage
				activateDeviation := target.ActivatePrice / 100 / sm.Strategy.Model.Conditions.Leverage

				activatePrice := target.ActivatePrice
				if target.Type == 1 {
					activatePrice = sm.Strategy.Model.State.EntryPrice * (1 + activateDeviation)
				}
				didCrossActivatePrice := currentOHLCV.Close >= activatePrice

				if !isActivated && didCrossActivatePrice {
					sm.Strategy.Model.State.TrailingExitPrices = append(sm.Strategy.Model.State.TrailingExitPrices, currentOHLCV.Close)
					return false
				} else if !isActivated && !didCrossActivatePrice {
					return false
				}
				edgePrice := sm.Strategy.Model.State.TrailingExitPrices[i]

				if currentOHLCV.Close > edgePrice {
					sm.Strategy.Model.State.TrailingExitPrices[i] = currentOHLCV.Close
					edgePrice = sm.Strategy.Model.State.TrailingExitPrices[i]

					go sm.placeTrailingOrder(edgePrice, time.Now().UnixNano(), i, side, false, TakeProfit)
				}

				deviationFromEdge := (edgePrice/currentOHLCV.Close - 1) * 100
				if deviationFromEdge > deviation {
					isSpot := sm.Strategy.Model.Conditions.MarketType == 0
					isSpotMarketOrder := target.OrderType == "market" && isSpot
					if isSpotMarketOrder {
						sm.Strategy.Model.State.State = TakeProfit
						sm.placeOrder(edgePrice, TakeProfit)

						return true
					}
				}
			}
		}
		break
	case "sell":
		for i, target := range sm.Strategy.Model.Conditions.ExitLevels {
			isTrailingTarget := target.ActivatePrice > 0
			if isTrailingTarget {
				isActivated := i < len(sm.Strategy.Model.State.TrailingExitPrices)
				deviation := target.EntryDeviation / 100 / sm.Strategy.Model.Conditions.Leverage
				activateDeviation := target.ActivatePrice / 100 / sm.Strategy.Model.Conditions.Leverage

				activatePrice := target.ActivatePrice
				if target.Type == 1 {
					activatePrice = sm.Strategy.Model.State.EntryPrice * (1 - activateDeviation)
				}
				didCrossActivatePrice := currentOHLCV.Close <= activatePrice

				if !isActivated && didCrossActivatePrice {
					sm.Strategy.Model.State.TrailingExitPrices = append(sm.Strategy.Model.State.TrailingExitPrices, currentOHLCV.Close)
					return false
				} else if !isActivated && !didCrossActivatePrice {
					return false
				}
				// isActivated
				edgePrice := sm.Strategy.Model.State.TrailingExitPrices[i]

				if currentOHLCV.Close < edgePrice {
					sm.Strategy.Model.State.TrailingExitPrices[i] = currentOHLCV.Close
					edgePrice = sm.Strategy.Model.State.TrailingExitPrices[i]

					go sm.placeTrailingOrder(edgePrice, time.Now().UnixNano(), i, side, false, TakeProfit)
				}

				deviationFromEdge := (currentOHLCV.Close/edgePrice - 1) * 100
				if deviationFromEdge > deviation {
					isSpot := sm.Strategy.Model.Conditions.MarketType == 0
					isSpotMarketOrder := target.OrderType == "market" && isSpot
					if isSpotMarketOrder {
						sm.Strategy.Model.State.State = TakeProfit
						sm.placeOrder(edgePrice, TakeProfit)

						return true
					}
				}
			}
		}
		break
	}

	return false
}

func (sm *SmartOrder) placeTrailingOrder(newTrailingPrice float64, trailingCheckAt int64, i int, entrySide string, isEntry bool, step string) {
	sm.Strategy.Model.State.TrailingCheckAt = trailingCheckAt
	time.Sleep(2 * time.Second)
	edgePrice := sm.Strategy.Model.State.TrailingEntryPrice
	if isEntry == false {
		edgePrice = sm.Strategy.Model.State.TrailingExitPrices[i]
	}
	trailingDidnChange := trailingCheckAt == sm.Strategy.Model.State.TrailingCheckAt
	newTrailingIncreaseProfits := (newTrailingPrice < edgePrice && !isEntry) || (newTrailingPrice > edgePrice && isEntry)
	priceRelation := edgePrice/newTrailingPrice
	if isEntry {
		priceRelation = newTrailingPrice/edgePrice
	}
	significantPercent := 0.016 * sm.Strategy.Model.Conditions.Leverage
	trailingChangedALot := newTrailingIncreaseProfits && (significantPercent < (priceRelation-1)*100*sm.Strategy.Model.Conditions.Leverage)
	if entrySide == "buy" {
		if !isEntry {
			priceRelation = newTrailingPrice/edgePrice
		} else {
			priceRelation = edgePrice/newTrailingPrice
		}
		newTrailingIncreaseProfits = (newTrailingPrice > edgePrice && !isEntry) || (newTrailingPrice < edgePrice && isEntry)
		trailingChangedALot = newTrailingIncreaseProfits && (significantPercent < (priceRelation-1)*100*sm.Strategy.Model.Conditions.Leverage)
	}
	if trailingDidnChange || trailingChangedALot {
		if sm.Lock == false {
			sm.Lock = true
			sm.Strategy.Model.State.TrailingEntryPrice = newTrailingPrice
			sm.placeOrder(-1, step)
			time.Sleep(3000 * time.Millisecond) // it will give some time for order execution, to avoid double send of orders
			sm.Lock = false
		}
	}
}

func (sm *SmartOrder) checkLoss(ctx context.Context, args ...interface{}) bool {
	isWaitingForOrder, ok := sm.IsWaitingForOrder.Load(Stoploss)
	if ok && isWaitingForOrder.(bool) {
		return false
	}
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
	sm.Strategy.Model.State.State = End
	sm.StateMgmt.UpdateState(sm.Strategy.Model.ID, &sm.Strategy.Model.State)
	return nil
}

func (sm *SmartOrder) enterTakeProfit(ctx context.Context, args ...interface{}) error {
	if currentOHLCV, ok := args[0].(OHLCV); ok {
		if sm.Strategy.Model.State.Amount > 0 {

			if sm.Lock {
				return nil
			}
			sm.Lock = true
			sm.placeOrder(currentOHLCV.Close, TakeProfit)

			//if sm.Strategy.Model.State.ExecutedAmount - sm.Strategy.Model.Conditions.EntryOrder.Amount == 0 { // re-check all takeprofit conditions to exit trade ( no additional trades needed no )
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
	if currentOHLCV, ok := args[0].(OHLCV); ok {
		if sm.Strategy.Model.State.Amount > 0 {
			side := "buy"
			if sm.Strategy.Model.Conditions.EntryOrder.Side == side {
				side = "sell"
			}
			if sm.Lock {
				return nil
			}
			sm.Lock = true

			// sm.cancelOpenOrders(sm.Strategy.Model.Conditions.Pair)
			sm.placeOrder(currentOHLCV.Close, Stoploss)
		}
		// if timeout specified then do this sell on timeout
		sm.Lock = false
	}
	_ = sm.State.Fire(CheckLossTrade, args[0])
	return nil
}

func (sm *SmartOrder) tryCancelAllOrders() {
	orderIds := sm.Strategy.Model.State.Orders
	for _, orderId := range orderIds {
		if orderId != "0" {
			sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
				KeyId: sm.KeyId,
				KeyParams: trading.CancelOrderRequestParams{
					OrderId:    orderId,
					MarketType: sm.Strategy.Model.Conditions.MarketType,
					Pair:       sm.Strategy.Model.Conditions.Pair,
				},
			})
		}
	}
}
func (sm *SmartOrder) Start() {
	state, _ := sm.State.State(context.Background())
	for state != End && state != Canceled {
		if sm.Strategy.Model.Enabled == false {
			state = Canceled
			break
		}
		if !sm.Lock {
			sm.processEventLoop()
		}
		time.Sleep(200 * time.Millisecond)
		state, _ = sm.State.State(context.Background())
	}
	if state == Canceled {
		sm.placeOrder(0, Canceled)
		sm.tryCancelAllOrders()
	}
	sm.StateMgmt.DisableStrategy(sm.Strategy.Model.ID)
	println("STOPPED")
}

func (sm *SmartOrder) Stop() {
	// TODO: implement here canceling all open orders, etc
}

func (sm *SmartOrder) processEventLoop() {
	currentOHLCVp := sm.DataFeed.GetPriceForPairAtExchange(sm.Strategy.Model.Conditions.Pair, sm.ExchangeName, sm.Strategy.Model.Conditions.MarketType)
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
		// println(sm.Strategy.Model.Conditions.Pair, sm.Strategy.Model.State.TrailingEntryPrice, currentOHLCV.Close, err.Error())
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
			println("keyAssetsCursor", err.Error())
		}
		keyId = &keyAsset.KeyId
	}
	runtime := NewSmartOrder(strategy, df, td, keyId, strategy.StateMgmt)
	runtime.Start()

	return runtime
}
