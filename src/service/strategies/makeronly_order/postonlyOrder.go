package makeronly_order

import (
	"context"
	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"reflect"
	"sync"
	"time"
)

const (
	PlaceOrder      = "PlaceOrder"
	PartiallyFilled = "PartiallyFilled"
	Filled          = "Filled"
	Canceled        = "Canceled"
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

	OrderParams trading.Order
}

func (po *MakerOnlyOrder) createTemplateOrder() {

}

func (sm *MakerOnlyOrder) IsOrderExistsInMap(orderId string) bool {
	return false
}

func (sm *MakerOnlyOrder) SetSelectedExitTarget(selectedExitTarget int){

}


func (sm *MakerOnlyOrder) Stop(){}

func (sm *MakerOnlyOrder) TryCancelAllOrders(orderIds []string){}
func (sm *MakerOnlyOrder) TryCancelAllOrdersConsistently(orderIds []string){}
func NewPostOnlyOrder(strategy interfaces.IStrategy, DataFeed interfaces.IDataFeed, TradingAPI trading.ITrading, keyId *primitive.ObjectID, stateMgmt interfaces.IStateMgmt) *MakerOnlyOrder {

	PO := &MakerOnlyOrder{Strategy: strategy, DataFeed: DataFeed, ExchangeApi: TradingAPI, KeyId: keyId, StateMgmt: stateMgmt, Lock: false, SelectedExitTarget: 0, OrdersMap: map[string]bool{}}
	initState := PlaceOrder
	pricePrecision, amountPrecision := stateMgmt.GetMarketPrecision(strategy.GetModel().Conditions.Pair, strategy.GetModel().Conditions.MarketType)
	PO.QuantityPricePrecision = pricePrecision
	PO.QuantityAmountPrecision = amountPrecision
	// if state is not empty but if its in the end and open ended, then we skip state value, since want to start over
	if strategy.GetModel().State != nil && strategy.GetModel().State.State != "" && !(strategy.GetModel().State.State == Filled && strategy.GetModel().Conditions.ContinueIfEnded == true) {
		initState = strategy.GetModel().State.State
	}
	State := stateless.NewStateMachine(initState)

	// define triggers and input types:
	State.SetTriggerParameters(CheckExistingOrders, reflect.TypeOf(models.MongoOrder{}))
	State.SetTriggerParameters(TriggerSpread, reflect.TypeOf(interfaces.SpreadData{}))

	/*
		Post Only Order life cycle:
			1) place order at best bid/ask
			2) wait N time
			3) if possible place at better/worse price or stay
	*/
	State.Configure(PlaceOrder).Permit(TriggerSpread, Filled,
		PO.checkSpread).Permit(CheckExistingOrders, Filled,
		PO.checkSpread)

	State.Configure(PartiallyFilled).Permit(TriggerSpread, Filled,
		PO.checkSpread).Permit(CheckExistingOrders, Filled,
		PO.checkSpread)

	State.Configure(Filled)

	State.Activate()

	PO.State = State
	PO.ExchangeName = "binance"
	// fmt.Printf(PO.State.ToGraph())
	// fmt.Printf("DONE\n")
	return PO
}

func (sm *MakerOnlyOrder) Start() {
	ctx := context.TODO()

	state, _ := sm.State.State(ctx)
	for state != Filled && state != Canceled {
		if sm.Strategy.GetModel().Enabled == false {
			break
		}
		if !sm.Lock {
			if sm.Strategy.GetModel().Conditions.EntrySpreadHunter {
				//sm.processSpreadEventLoop()
			} else {
				//sm.processEventLoop()
			}
		}
		time.Sleep(15 * time.Millisecond)
		state, _ = sm.State.State(ctx)
	}
	//sm.Stop()
	println("STOPPED")
}