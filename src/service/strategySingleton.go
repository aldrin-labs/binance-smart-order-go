package service

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/makeronly_order"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"fmt"
	// "gitlab.com/crypto_project/core/strategy_service/src/sources/redis"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/binance"
	statsd_client "gitlab.com/crypto_project/core/strategy_service/src/statsd"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"
)

// A StrategyService singleton, the root for smart trades runtimes.
type StrategyService struct {
	pairs	   map[string]bool
	strategies map[string]*strategies.Strategy
	trading    trading.ITrading
	dataFeed   interfaces.IDataFeed
	stateMgmt  interfaces.IStateMgmt
	statsd     statsd_client.StatsdClient
	log        *zap.Logger
}

var singleton *StrategyService
var once sync.Once

// GetStrategyService returns a pointer to instantiated service singleton.
func GetStrategyService() *StrategyService {
	once.Do(func() {
		pairs := map[string]bool { // TODO(khassanov): get pairs
			"BTCUSDT": true,
			"BTCUSDC": true,
			"ETHUSDT": true,
		}
		// df := redis.InitRedis()
		df := binance.InitBinance()
		tr := trading.InitTrading()
		statsd := statsd_client.StatsdClient{}
		statsd.Init()
		sm := mongodb.StateMgmt{Statsd: &statsd}
		logger, _ := zap.NewProduction() // TODO(khassanov): handle the error
		logger = logger.With(zap.String("logger", "ss"))
		singleton = &StrategyService{
			pairs: pairs,
			strategies: map[string]*strategies.Strategy{},
			dataFeed:   df,
			trading:    tr,
			stateMgmt:  &sm,
			statsd:     statsd,
			log: 		logger,
		}
		logger.Info("strategy service instantiated")
		statsd.Inc("strategy_service.instantiated")
	})
	return singleton
}

// Init loads enabled strategies from persistent storage and instantiates subscriptions to positions, orders and strategies updates.
func (ss *StrategyService) Init(wg *sync.WaitGroup, isLocalBuild bool) {
	t1 := time.Now()
	ss.log.Info("strategy service init",
		zap.Bool("isLocalBuild", isLocalBuild),
	)
	ctx := context.Background()
	var coll = mongodb.GetCollection("core_strategies")
	// testStrat, _ := primitive.ObjectIDFromHex("5deecc36ba8a424bfd363aaf")
	// , {"_id", testStrat}
	additionalCondition := bson.E{}
	accountId := os.Getenv("ACCOUNT_ID")

	if isLocalBuild {
		additionalCondition.Key = "accountId"
		additionalCondition.Value, _ = primitive.ObjectIDFromHex(accountId)
	}

	//cur, err := coll.Find(ctx, bson.D{{"enabled",true}})
	cur, err := coll.Find(ctx, bson.D{{"enabled", true}, additionalCondition})
	if err != nil {
		ss.log.Error("can't read strategies",
			zap.String("err", err.Error()),
		)
		wg.Done() // TODO(khassanov): should we really fail here?
		//log.Fatal(err)
	}
	if cur == nil {
		ss.log.Error("finding strategies, cur == nil")
		wg.Done()
	}
	defer cur.Close(ctx)

	// Add strategies exists to runtime
	var strategiesAdded int64 = 0
	for cur.Next(ctx) {

		// create a value into which the single document can be decoded
		strategy, err := strategies.GetStrategy(cur, ss.dataFeed, ss.trading, ss.stateMgmt, ss, &ss.statsd)
		if err != nil {
			ss.log.Error("failing to process enabled strategy",
				zap.String("err", err.Error()),
				zap.String("cur", cur.Current.String()),
			)
			continue
			ss.log.Fatal("",
				zap.String("err", err.Error()),
			) // TODO(khassanov): unreachable?
		}
		if strategy.Model.AccountId != nil && strategy.Model.AccountId.Hex() == "5e4ce62b1318ef1b1e85b6f4" {
			continue
		}
		if _, ok := ss.pairs[strategy.Model.Conditions.Pair]; !ok { continue } // skip a foreign pair
		if ok, err := strategy.Settle(); !ok || err != nil {
			continue // TODO(khassanov): distinguish a state locked in dlm and network errors
		}
		// try to lock
		ss.log.Info("adding existing strategy",
			zap.String("ObjectID", strategy.Model.ID.String()),
		)
		GetStrategyService().strategies[strategy.Model.ID.String()] = strategy
		go strategy.Start()
		strategiesAdded++
	}
	ss.statsd.Gauge("strategy_service.strategies_added_on_init", strategiesAdded)
	ss.statsd.Gauge("strategy_service.active_strategies", int64(len(ss.strategies)))

	go ss.InitPositionsWatch()                      // subscribe to position updates
	go ss.stateMgmt.InitOrdersWatch()               // subscribe to order updates
	_ = ss.WatchStrategies(isLocalBuild, accountId) // subscribe to new smart trades to add them into runtime

	if err := cur.Err(); err != nil { // TODO(khassanov): can we retry here?
		wg.Done()
		ss.log.Fatal("log.Fatal at the end of init func")
	}
	ss.statsd.Inc("strategy_service.instantiated")
	ss.statsd.TimingDuration("strategy_service.init", time.Since(t1))
}

// GetStrategy creates strategy instance with given arguments.
func GetStrategy(strategy *models.MongoStrategy, df interfaces.IDataFeed, tr trading.ITrading, st interfaces.IStateMgmt, statsd *statsd_client.StatsdClient, ss *StrategyService) *strategies.Strategy {
	// TODO(khassanov): why we use this instead of the same from the `strategy` package?
	// TODO(khassanov): remove code copy got from the same in the strategy package
	logger, _ := zap.NewProduction() // TODO(khassanov): handle the error
	loggerName := fmt.Sprintf("sm-%v", strategy.ID.Hex())
	logger = logger.With(zap.String("logger", loggerName))
	return &strategies.Strategy{
		Model: strategy,
		Datafeed: df,
		Trading: tr,
		StateMgmt: st,
		Singleton: ss,
		Statsd: statsd,
		Log: logger,
	}
}

// AddStrategy instantiates given strategy to store in the service instance and start it.
func (ss *StrategyService) AddStrategy(strategy *models.MongoStrategy) {
	if ss.strategies[strategy.ID.String()] == nil {
		sig := GetStrategy(strategy, ss.dataFeed, ss.trading, ss.stateMgmt, &ss.statsd, ss)
		if ok, err := sig.Settle(); !ok || err != nil {
			return // TODO(khassanov): distinguish a state locked in dlm and network errors
		}
		ss.log.Info("start",
			zap.String("objid", sig.Model.ID.String()),
		)
		ss.strategies[sig.Model.ID.String()] = sig
		go sig.Start()
		ss.statsd.Inc("strategy_service.add_strategy")
		ss.statsd.Gauge("strategy_service.active_strategies", int64(len(ss.strategies)))
	}
}

// CreateOrder instantiates smart trade strategy with requested parameters and adds it to the service runtime.
func (ss *StrategyService) CreateOrder(request trading.CreateOrderRequest) trading.OrderResponse {
	t1 := time.Now()
	ss.statsd.Inc("strategy_service.create_request")
	id := primitive.NewObjectID()
	var reduceOnly bool
	if request.KeyParams.ReduceOnly == nil {
		reduceOnly = false
	} else {
		reduceOnly = *request.KeyParams.ReduceOnly
	}

	hedgeMode := request.KeyParams.PositionSide != "BOTH"

	order := models.MongoOrder{
		ID:           id,
		Status:       "open",
		OrderId:      id.Hex(),
		Filled:       0,
		Average:      0,
		Amount:       request.KeyParams.Amount,
		Side:         request.KeyParams.Side,
		Type:         "maker-only",
		Symbol:       request.KeyParams.Symbol,
		PositionSide: request.KeyParams.PositionSide,
		ReduceOnly:   reduceOnly,
		Timestamp:    float64(time.Now().UnixNano() / 1000000),
	}
	go ss.stateMgmt.SaveOrder(order, request.KeyId, request.KeyParams.MarketType)
	strategy := models.MongoStrategy{
		ID:        &id,
		Type:      2,
		Enabled:   true,
		AccountId: request.KeyId,
		Conditions: &models.MongoStrategyCondition{
			AccountId:              request.KeyId,
			Hedging:                false,
			HedgeMode:              hedgeMode,
			HedgeKeyId:             nil,
			HedgeStrategyId:        nil,
			MakerOrderId:           &id,
			TemplateToken:          "",
			MandatoryForcedLoss:    false,
			PositionWasClosed:      false,
			SkipInitialSetup:       false,
			CancelIfAnyActive:      false,
			TrailingExitExternal:   false,
			TrailingExitPrice:      0,
			StopLossPrice:          0,
			ForcedLossPrice:        0,
			TakeProfitPrice:        0,
			TakeProfitHedgePrice:   0,
			StopLossExternal:       false,
			TakeProfitExternal:     false,
			WithoutLossAfterProfit: 0,
			EntrySpreadHunter:      false,
			EntryWaitingTime:       0,
			TakeProfitSpreadHunter: false,
			TakeProfitWaitingTime:  0,
			KeyAssetId:             nil,
			Pair:                   request.KeyParams.Symbol,
			MarketType:             request.KeyParams.MarketType,
			EntryOrder: &models.MongoEntryPoint{
				ActivatePrice:           0,
				EntryDeviation:          0,
				Price:                   0,
				Side:                    request.KeyParams.Side,
				ReduceOnly:              reduceOnly,
				Amount:                  request.KeyParams.Amount,
				HedgeEntry:              0,
				HedgeActivation:         0,
				HedgeOppositeActivation: 0,
				PlaceWithoutLoss:        false,
				Type:                    0,
				OrderType:               "limit",
			},
			WaitingEntryTimeout:        0,
			ActivationMoveStep:         0,
			ActivationMoveTimeout:      0,
			TimeoutIfProfitable:        0,
			TimeoutWhenProfit:          0,
			ContinueIfEnded:            false,
			TimeoutBeforeOpenPosition:  0,
			ChangeTrendIfLoss:          false,
			ChangeTrendIfProfit:        false,
			MoveStopCloser:             false,
			MoveForcedStopAtEntry:      false,
			TimeoutWhenLoss:            0,
			TimeoutLoss:                0,
			StopLoss:                   0,
			StopLossType:               "",
			ForcedLoss:                 0,
			HedgeLossDeviation:         0,
			CreatedByTemplate:          false,
			TemplateStrategyId:         nil,
			Leverage:                   0,
			EntryLevels:                nil,
			ExitLevels:                 nil,
			CloseStrategyAfterFirstTAP: false,
			PlaceEntryAfterTAP:         false,
		},
		State: &models.MongoStrategyState{
			ColdStart:              true,
			State:                  "",
			Msg:                    "",
			EntryOrderId:           "",
			Iteration:              0,
			EntryPointPrice:        0,
			EntryPointType:         "",
			EntryPointSide:         "",
			EntryPointAmount:       0,
			EntryPointDeviation:    0,
			StopLoss:               0,
			StopLossPrice:          0,
			StopLossOrderIds:       nil,
			ForcedLoss:             0,
			ForcedLossPrice:        0,
			ForcedLossOrderIds:     nil,
			TakeProfit:             nil,
			TakeProfitPrice:        0,
			TakeProfitHedgePrice:   0,
			TakeProfitOrderIds:     nil,
			TrailingEntryPrice:     0,
			HedgeExitPrice:         0,
			TrailingHedgeExitPrice: 0,
			TrailingExitPrice:      0,
			TrailingExitPrices:     nil,
			EntryPrice:             0,
			ExitPrice:              0,
			Amount:                 0,
			Orders:                 nil,
			ExecutedOrders:         nil,
			ExecutedAmount:         0,
			ReachedTargetCount:     0,
			TrailingCheckAt:        0,
			StopLossAt:             0,
			LossableAt:             0,
			ProfitableAt:           0,
			ProfitAt:               0,
		},
		TriggerWhen:     models.TriggerOptions{},
		Expiration:      models.ExpirationSchema{},
		LastUpdate:      0,
		SignalIds:       nil,
		OrderIds:        nil,
		WaitForOrderIds: nil,
		OwnerId:         primitive.ObjectID{},
		Social:          models.MongoSocial{},
		CreatedAt:       time.Time{},
	}
	go ss.AddStrategy(&strategy)
	hex := id.Hex()
	response := trading.OrderResponse{
		Status: "OK",
		Data: trading.OrderResponseData{
			OrderId: hex,
			Status:  "open",
			Amount:  request.KeyParams.Amount,
			Type:    "maker-only",
			Price:   0,
			Average: 0,
			Filled:  0,
			Code:    0,
			Msg:     "",
		},
	}
	ss.statsd.TimingDuration("strategy_service.create_order", time.Since(t1))
	return response
}

// CancelOrder tries to cancel an order notifying state manager to update a persistent storage.
func (ss *StrategyService) CancelOrder(request trading.CancelOrderRequest) trading.OrderResponse {
	t1 := time.Now()
	ss.statsd.Inc("strategy_service.cancel_request")
	id, _ := primitive.ObjectIDFromHex(request.KeyParams.OrderId)
	strategy := ss.strategies[id.String()]
	order := ss.stateMgmt.GetOrderById(&id)

	ss.log.Info("cancelling order",
		zap.String("id", id.String()),
		zap.String("order", fmt.Sprintf("%v", order)),
		zap.String("request.KeyParams.OrderId", request.KeyParams.OrderId),
	)

	if order == nil {
		return trading.OrderResponse{
			Status: "ERR",
			Data: trading.OrderResponseData{
				OrderId: "",
				Msg:     "",
				Status:  "",
				Type:    "",
				Price:   0,
				Average: 0,
				Amount:  0,
				Filled:  0,
				Code:    0,
			},
		}
	}

	ss.log.Info("",
		zap.String("strategy", fmt.Sprintf("%+v", strategy)),
	)

	if strategy != nil {
		pointStrategy := *ss.strategies[id.String()]
		pointStrategy.GetModel().LastUpdate = 10
		pointStrategy.GetModel().Enabled = false
		pointStrategy.GetModel().State.State = makeronly_order.Canceled
		pointStrategy.GetStateMgmt().DisableStrategy(&id)
		pointStrategy.GetStateMgmt().UpdateState(&id, strategy.GetModel().State)
		pointStrategy.HotReload(*pointStrategy.GetModel())

	}

	updatedOrder := models.MongoOrder{
		ID:         id,
		Status:     "canceled",
		OrderId:    order.OrderId,
		Filled:     order.Filled,
		Average:    order.Average,
		Side:       order.Side,
		Type:       order.Type,
		Symbol:     order.Symbol,
		ReduceOnly: order.ReduceOnly,
		Timestamp:  order.Timestamp,
		Amount:     order.Amount,
	}

	go ss.stateMgmt.SaveOrder(updatedOrder, request.KeyId, request.KeyParams.MarketType)

	ss.statsd.TimingDuration("strategy_service.cancel_order", time.Since(t1))
	return trading.OrderResponse{
		Status: "OK",
		Data: trading.OrderResponseData{
			OrderId: request.KeyParams.OrderId,
			Msg:     "",
			Status:  "canceled",
			Type:    order.Type,
			Price:   0,
			Average: 0,
			Amount:  0,
			Filled:  0,
			Code:    0,
		},
	}
}

const CollName = "core_strategies"

// WatchStrategies subscribes to strategies to add new strategies to runtime or update local data together with
// persistent storage updates.
// TODO(khassanov) can we remove `isLocalBuild` parameter in favor of environment variable?
func (ss *StrategyService) WatchStrategies(isLocalBuild bool, accountId string) error {
	//func (ss *StrategyService) WatchStrategies() error {
	ctx := context.Background()
	var coll = mongodb.GetCollection(CollName)

	cs, err := coll.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	//cs, err := coll.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		ss.log.Error("can't watch for strategies")
		return err
	}
	//require.NoError(cs, err)
	defer cs.Close(ctx)

	for cs.Next(ctx) {
		var event models.MongoStrategyUpdateEvent
		err := cs.Decode(&event)

		//	data := next.String()
		// log.Print(data)
		//		err := json.Unmarshal([]byte(data), &event)
		if err != nil {
			ss.log.Info("event decode error on processing strategy",
				zap.String("err", err.Error()),
			)
			continue
		}

		if event.FullDocument.ID == nil {
			ss.log.Error("new smart order id is nil",
				zap.String("event.FullDocument", fmt.Sprintf("%+v", event.FullDocument)),
			)
			continue
		}

		// disable SM for Anton in dev
		if event.FullDocument.AccountId != nil && event.FullDocument.AccountId.Hex() == "5e4ce62b1318ef1b1e85b6f4" {
			continue
		}

		if _, ok := ss.pairs[event.FullDocument.Conditions.Pair]; !ok { continue } // skip a foreign pair

		if event.FullDocument.Type == 2 && event.FullDocument.State.ColdStart { // 2 means maker only
			sig := GetStrategy(&event.FullDocument, ss.dataFeed, ss.trading, ss.stateMgmt, &ss.statsd, ss)
			ss.strategies[event.FullDocument.ID.String()] = sig
			ss.log.Info("continue in maker-only cold start")
			continue
		}

		if isLocalBuild && (event.FullDocument.AccountId == nil || event.FullDocument.AccountId.Hex() != accountId) {
			ss.log.Warn("continue watchStrategies in accountId incomparable",
				zap.String("event account id", event.FullDocument.AccountId.Hex()),
				zap.String("account id in .env", accountId),
			)
			continue
		}

		if ss.strategies[event.FullDocument.ID.String()] != nil {
			ss.strategies[event.FullDocument.ID.String()].HotReload(event.FullDocument)
			ss.EditConditions(ss.strategies[event.FullDocument.ID.String()])
			if event.FullDocument.Enabled == false {
				delete(ss.strategies, event.FullDocument.ID.String())
			}
		} else { // brand new smart trade
			if event.FullDocument.Enabled == true {
				ss.AddStrategy(&event.FullDocument)
				ss.statsd.Inc("strategy_service.add_strategy_from_db")
			}
		}
	}
	ss.log.Info("Finish WatchStrategies")
	return nil
}

// InitPositionsWatch subscribes to smart trade updates for each position update received to disable smart trade if position closed externally.
func (ss *StrategyService) InitPositionsWatch() {
	CollPositionsName := "core_positions"
	CollStrategiesName := "core_strategies"
	ctx := context.Background()
	var collPositions = mongodb.GetCollection(CollPositionsName)
	pipeline := mongo.Pipeline{}

	cs, err := collPositions.Watch(ctx, pipeline, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		ss.log.Info("panic error on watching positions")
		panic(err.Error())
	}
	//require.NoError(cs, err)
	defer cs.Close(ctx)
	for cs.Next(ctx) {
		var positionEventDecoded models.MongoPositionUpdateEvent
		err := cs.Decode(&positionEventDecoded)
		//	data := next.String()
		// log.Print(data)
		//		err := json.Unmarshal([]byte(data), &event)
		if err != nil {
			ss.log.Info("event decode in processing position",
				zap.String("err", err.Error()),
			)
		}

		go func(event models.MongoPositionUpdateEvent) {
			var collStrategies = mongodb.GetCollection(CollStrategiesName)
			cur, err := collStrategies.Find(ctx, bson.D{
				{"conditions.marketType", 1},
				{"enabled", true},
				{"accountId", event.FullDocument.KeyId},
				{"conditions.pair", event.FullDocument.Symbol}},
			)

			if err != nil {
				ss.log.Fatal("on finding enabled strategies by position",
					zap.String("err", err.Error()),
				)
			}

			defer cur.Close(ctx)

			for cur.Next(ctx) {
				var strategyEventDecoded models.MongoStrategy
				err := cur.Decode(&strategyEventDecoded)

				if err != nil {
					ss.log.Error("event decode on processing strategy found by position close",
						zap.String("err", err.Error()),
					)
				}

				// if SM created before last position update
				// then we caught position event before actual update
				if positionEventDecoded.FullDocument.PositionAmt == 0 {
					strategy := ss.strategies[strategyEventDecoded.ID.String()]
					if strategy != nil && strategy.GetModel().Conditions.PositionWasClosed {
						ss.log.Info("disabled by position close")
						strategy.GetModel().Enabled = false
						collStrategies.FindOneAndUpdate(ctx, bson.D{{"_id", strategyEventDecoded.ID}}, bson.M{"$set": bson.M{"enabled": false}})
					}
				}
			}
		}(positionEventDecoded)
	}
	ss.log.Info("InitPositionsWatchEnd")
}

func (ss *StrategyService) EditConditions(strategy *strategies.Strategy) {
	// here we should determine what was changed
	model := strategy.GetModel()
	isSpot := model.Conditions.MarketType == 0
	sm := strategy.StrategyRuntime
	isInEntry := model.State != nil && model.State.State != smart_order.TrailingEntry && model.State.State != smart_order.WaitForEntry

	if model.State == nil || sm == nil {
		return
	}
	if !isInEntry {
		return
	}

	entryOrder := model.Conditions.EntryOrder

	// entry order change
	if entryOrder.Amount != model.State.EntryPointAmount || entryOrder.Side != model.State.EntryPointSide || entryOrder.OrderType != model.State.EntryPointType || (entryOrder.Price != model.State.EntryPointPrice && entryOrder.EntryDeviation == 0) || entryOrder.EntryDeviation != model.State.EntryPointDeviation {
		if isSpot {
			sm.TryCancelAllOrdersConsistently(model.State.Orders)
			time.Sleep(5 * time.Second)
		} else {
			go sm.TryCancelAllOrders(model.State.Orders)
		}

		entryIsNotTrailing := model.Conditions.EntryOrder.ActivatePrice == 0

		if entryIsNotTrailing {
			sm.PlaceOrder(model.Conditions.EntryOrder.Price, 0.0, smart_order.WaitForEntry)
		} else if model.State.TrailingEntryPrice > 0 {
			sm.PlaceOrder(-1, 0.0, smart_order.TrailingEntry)
		}
	}

	// SL change
	if model.Conditions.StopLoss != model.State.StopLoss || model.Conditions.StopLossPrice != model.State.StopLossPrice {
		// we should also think about case when SL was placed by timeout, but didn't executed coz of limit order for example
		// with this we'll cancel it, and new order wont placed
		// for this we'll need currentOHLCV in price field
		if isSpot {
			sm.TryCancelAllOrdersConsistently(model.State.StopLossOrderIds)
			time.Sleep(5 * time.Second)
		} else {
			go sm.TryCancelAllOrders(model.State.StopLossOrderIds)
		}

		sm.PlaceOrder(0, 0.0, smart_order.Stoploss)
	}

	if model.Conditions.ForcedLoss != model.State.ForcedLoss || model.Conditions.ForcedLossPrice != model.State.ForcedLossPrice {
		if isSpot {
		} else {
			sm.TryCancelAllOrders(model.State.ForcedLossOrderIds)
			sm.PlaceOrder(0, 0.0, "ForcedLoss")
		}
	}

	if model.Conditions.TrailingExitPrice != model.State.TrailingExitPrice || model.Conditions.TakeProfitPrice != model.State.TakeProfitPrice {
		sm.PlaceOrder(-1, 0.0, smart_order.TakeProfit)
	}

	if model.Conditions.TakeProfitHedgePrice != model.State.TakeProfitHedgePrice {
		sideCoefficient := 1.0
		feePercentage := 0.04 * 4
		if model.Conditions.EntryOrder.Side == "sell" {
			sideCoefficient = -1.0
		}

		currentProfitPercentage := ((model.Conditions.TakeProfitHedgePrice/model.State.EntryPrice)*100 - 100) * model.Conditions.Leverage * sideCoefficient

		if currentProfitPercentage > feePercentage {
			strategy.GetModel().State.TrailingHedgeExitPrice = model.Conditions.TakeProfitHedgePrice
			sm.PlaceOrder(-1, 0.0, smart_order.HedgeLoss)
		}
	}

	ss.log.Info("",
		zap.Int("len(model.State.TrailingExitPrices)", len(model.State.TrailingExitPrices)),
	)

	// TAP change
	// split targets
	if len(model.Conditions.ExitLevels) > 0 {
		if len(model.Conditions.ExitLevels) > 1 || (model.Conditions.ExitLevels[0].Amount > 0) {
			wasChanged := false

			if len(model.State.TakeProfit) != len(model.Conditions.ExitLevels) {
				wasChanged = true
			} else {
				for i, target := range model.Conditions.ExitLevels {
					if (target.Price != model.State.TakeProfit[i].Price || target.Amount != model.State.TakeProfit[i].Amount) && !wasChanged {
						wasChanged = true
					}
				}
			}

			// add some logic if some of targets was executed
			if wasChanged {
				ids := model.State.TakeProfitOrderIds[:]
				lastExecutedTarget := 0
				for i, id := range ids {
					orderStillOpen := sm.IsOrderExistsInMap(id)
					if orderStillOpen {
						sm.SetSelectedExitTarget(i)
						lastExecutedTarget = i
						break
					}
				}

				idsToCancel := ids[lastExecutedTarget:]
				if isSpot {
					sm.TryCancelAllOrdersConsistently(idsToCancel)
					time.Sleep(5 * time.Second)
				} else {
					sm.TryCancelAllOrdersConsistently(idsToCancel)
				}

				// here we delete canceled orders
				if lastExecutedTarget-1 >= 0 {
					strategy.GetModel().State.TakeProfitOrderIds = ids[:lastExecutedTarget]
				} else {
					strategy.GetModel().State.TakeProfitOrderIds = make([]string, 0)
				}

				sm.PlaceOrder(0, 0.0, smart_order.TakeProfit)
			}
		} else if model.Conditions.ExitLevels[0].ActivatePrice > 0 &&
			(model.Conditions.ExitLevels[0].EntryDeviation != model.State.TakeProfit[0].EntryDeviation) &&
			len(model.State.TrailingExitPrices) > 0 {
			// trailing TAP
			ids := model.State.TakeProfitOrderIds[:]
			if isSpot {
				sm.TryCancelAllOrdersConsistently(ids)
				time.Sleep(5 * time.Second)
			} else {
				go sm.TryCancelAllOrders(ids)
			}

			sm.PlaceOrder(-1, 0.0, smart_order.TakeProfit)
		} else if model.Conditions.ExitLevels[0].Price != model.State.TakeProfit[0].Price { // simple TAP
			ids := model.State.TakeProfitOrderIds[:]
			if isSpot {
				sm.TryCancelAllOrdersConsistently(ids)
				time.Sleep(5 * time.Second)
			} else {
				go sm.TryCancelAllOrders(ids)
			}

			sm.PlaceOrder(0, 0.0, smart_order.TakeProfit)
		}
	}

	ss.statsd.Inc("strategy_service.edited_conditions`")
	strategy.StateMgmt.SaveStrategyConditions(strategy.Model)
}
