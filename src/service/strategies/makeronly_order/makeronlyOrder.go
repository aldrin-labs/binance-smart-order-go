package makeronly_order

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"reflect"
	"sync"
	"time"
)

const (
	PlaceOrder      = "PlaceOrder"
	PartiallyFilled = "PartiallyFilled"
	Filled          = "Filled"
	Canceled        = "Canceled"
	Error           = "Error"
)

const (
	TriggerSpread        = "Spread"
	TriggerOrderExecuted = "TriggerOrderExecuted"
	CheckExistingOrders  = "CheckExistingOrders"
)

type MakerOnlyOrder struct {
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
	StopLock                bool
	LastTrailingTimestamp   int64
	SelectedExitTarget      int
	TemplateOrderId         string
	OrdersMux               sync.Mutex
	MakerOnlyOrder          *models.MongoOrder

	OrderParams trading.Order
}

func (sm *MakerOnlyOrder) IsOrderExistsInMap(orderId string) bool {
	return false
}

func (sm *MakerOnlyOrder) SetSelectedExitTarget(selectedExitTarget int) {}

func (sm *MakerOnlyOrder) Stop() {
	attempts := 0
	ctx := context.TODO()
	state, _ := sm.State.State(ctx)

	go sm.CancelEntryOrder()
	log.Println("stop func state ", state, " enabled ", sm.Strategy.GetModel().Enabled)
	// user canceled order
	if state != Filled && !sm.Strategy.GetModel().Enabled {
		sm.Strategy.GetModel().State.State = Canceled
		go sm.StateMgmt.UpdateState(sm.Strategy.GetModel().ID, sm.Strategy.GetModel().State)

		if sm.MakerOnlyOrder != nil {
			sm.MakerOnlyOrder.Status = "canceled"
			go sm.StateMgmt.SaveOrder(*sm.MakerOnlyOrder, sm.KeyId, sm.Strategy.GetModel().Conditions.MarketType)
		} else {
			go func() {
				for {
					if sm.MakerOnlyOrder != nil || attempts >= 50 {
						sm.MakerOnlyOrder.Status = "canceled"
						go sm.StateMgmt.SaveOrder(*sm.MakerOnlyOrder, sm.KeyId, sm.Strategy.GetModel().Conditions.MarketType)
						break
					}
					attempts += 1
					time.Sleep(1 * time.Second)
				}
			}()
		}
	} else {
		go sm.StateMgmt.DisableStrategy(sm.Strategy.GetModel().ID)
		if sm.MakerOnlyOrder != nil {
			go sm.StateMgmt.SaveOrder(*sm.MakerOnlyOrder, sm.KeyId, sm.Strategy.GetModel().Conditions.MarketType)
		} else {
			go func() {
				for {
					if sm.MakerOnlyOrder != nil || attempts >= 50 {
						go sm.StateMgmt.SaveOrder(*sm.MakerOnlyOrder, sm.KeyId, sm.Strategy.GetModel().Conditions.MarketType)
						break
					}
					attempts += 1
					time.Sleep(1 * time.Second)
				}
			}()
		}
	}
}

func (sm *MakerOnlyOrder) CancelEntryOrder() {
	model := sm.Strategy.GetModel()
	if model.State.EntryOrderId != "" {
		response := sm.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
			KeyId: sm.KeyId,
			KeyParams: trading.CancelOrderRequestParams{
				OrderId:    model.State.EntryOrderId,
				MarketType: model.Conditions.MarketType,
				Pair:       model.Conditions.Pair,
			},
		})
		model.State.EntryOrderId = ""
		if response.Data.OrderId == "" {
			// order was executed should be processed in other thread
			return
		}
		// we canceled prev order now time to place new one
	}
}
func (sm *MakerOnlyOrder) TryCancelAllOrders(orderIds []string)             {}
func (sm *MakerOnlyOrder) TryCancelAllOrdersConsistently(orderIds []string) {}
func NewMakerOnlyOrder(strategy interfaces.IStrategy, DataFeed interfaces.IDataFeed, TradingAPI trading.ITrading, keyId *primitive.ObjectID, stateMgmt interfaces.IStateMgmt) *MakerOnlyOrder {
	PO := &MakerOnlyOrder{Strategy: strategy, DataFeed: DataFeed, ExchangeApi: TradingAPI, KeyId: keyId, StateMgmt: stateMgmt, Lock: false, SelectedExitTarget: 0, OrdersMap: map[string]bool{}}
	initState := PlaceOrder
	model := strategy.GetModel()
	go func() {
		var mongoOrder *models.MongoOrder
		for {
			mongoOrder = stateMgmt.GetOrder(strategy.GetModel().Conditions.MakerOrderId.Hex())
			if mongoOrder != nil {
				break
			}
		}
		PO.MakerOnlyOrder = mongoOrder
	}()
	// if state is not empty but if its in the end and open ended, then we skip state value, since want to start over
	if model.State != nil && model.State.State != "" && !(model.State.State == Filled && model.Conditions.ContinueIfEnded == true) {
		initState = model.State.State
	}

	State := stateless.NewStateMachineWithMode(initState, 1)
	// define triggers and input types:
	State.SetTriggerParameters(TriggerSpread, reflect.TypeOf(interfaces.SpreadData{}))

	/*
		Post Only Order life cycle:
			1) place order at best bid/ask
			2) wait N time
			3) if possible place at better/worse price or stay
	*/
	State.Configure(PlaceOrder).Permit(CheckExistingOrders, Filled)
	State.Configure(Filled).OnEntry(PO.enterFilled)

	State.Activate()

	PO.State = State
	PO.ExchangeName = "binance"
	// fmt.Printf(PO.State.ToGraph())
	// fmt.Printf("DONE\n")
	if model.State.ColdStart {
		go strategy.GetStateMgmt().CreateStrategy(model)
	}
	return PO
}

func (sm *MakerOnlyOrder) Start() {
	ctx := context.TODO()
	state, _ := sm.State.State(ctx)
	localState := sm.Strategy.GetModel().State.State

	for state != Filled && state != Canceled && (sm.MakerOnlyOrder == nil || sm.MakerOnlyOrder.Status == "open") &&
		localState != Filled && localState != Canceled {
		if sm.Strategy.GetModel().Enabled == false {
			break
		}
		if !sm.Lock {
			sm.processEventLoop()
		}
		time.Sleep(3 * time.Second)
		state, _ = sm.State.State(ctx)
		localState = sm.Strategy.GetModel().State.State
		log.Println("localState ", localState)
		log.Println("sm.Strategy.GetModel().Enabled", sm.Strategy.GetModel().Enabled)
		log.Println("lastUpdate", sm.Strategy.GetModel().LastUpdate)
	}
	sm.Stop()
	println("STOPPED postonly")
}

func (sm *MakerOnlyOrder) processEventLoop() {
	log.Println("loop")
	currentSpread := sm.DataFeed.GetSpreadForPairAtExchange(sm.Strategy.GetModel().Conditions.Pair, sm.ExchangeName, sm.Strategy.GetModel().Conditions.MarketType)
	if currentSpread != nil {
		if sm.Strategy.GetModel().State.EntryOrderId == "" {
			sm.PlaceOrder(0, 0.0, PlaceOrder)
		} else if sm.Strategy.GetModel().State.EntryPrice != currentSpread.BestBid && sm.Strategy.GetModel().State.EntryPrice != currentSpread.BestAsk {
			sm.PlaceOrder(0, 0.0, PlaceOrder)
		}
	}
}
