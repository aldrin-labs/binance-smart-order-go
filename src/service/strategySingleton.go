package service

import (
	"context"
	"fmt"
	"github.com/go-redsync/redsync/v4"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/makeronly_order"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/redis"

	// "gitlab.com/crypto_project/core/strategy_service/src/sources/redis"
	cpu_info "github.com/shirou/gopsutil/cpu"
	cpu_load "github.com/shirou/gopsutil/load"
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
	"syscall"
	"time"
)

// A StrategyService singleton, the root for smart trades runtimes.
type StrategyService struct {
	pairs      map[int8]map[string]struct{} // spot and futures pairs
	strategies map[string]*strategies.Strategy
	trading    trading.ITrading
	dataFeed   interfaces.IDataFeed
	stateMgmt  interfaces.IStateMgmt
	statsd     statsd_client.StatsdClient
	log        *zap.Logger
	full       bool // indicates whether an instance full or can take more strategies
	ramFull    bool // indicates close to RAM limit
	cpuFull    bool // indicates out of CPU usage limit
}

var singleton *StrategyService
var once sync.Once

// GetStrategyService returns a pointer to instantiated service singleton.
func GetStrategyService() *StrategyService {
	once.Do(func() {
		logger, _ := zap.NewProduction() // TODO(khassanov): handle the error
		logger = logger.With(zap.String("logger", "ss"))
		// df := redis.InitRedis()
		df := binance.InitBinance()
		tr := trading.InitTrading()
		statsd := statsd_client.StatsdClient{}
		statsd.Init()
		sm := mongodb.StateMgmt{Statsd: &statsd}
		singleton = &StrategyService{
			pairs:      map[int8]map[string]struct{}{0: map[string]struct{}{}, 1: map[string]struct{}{}},
			strategies: map[string]*strategies.Strategy{},
			dataFeed:   df,
			trading:    tr,
			stateMgmt:  &sm,
			statsd:     statsd,
			log:        logger,
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

	// Select pairs to process
	mode := os.Getenv("MODE")
	coll := mongodb.GetCollection("core_markets")
	ss.log.Info("reading pairs for mode given", zap.String("mode", mode))
	switch mode {
	case "":
		ss.log.Warn("Mode not set, starting with 'All' mode. "+
			"Please set 'MODE' environment variable to 'Bitcoin', 'Altcoins' or 'All'",
			zap.String("mode", mode),
		)
		fallthrough
	case "All":
		filter := primitive.Regex{Pattern: ".*"} // match all
		ss.setPairs(ctx, coll, filter)
	case "Bitcoin":
		filter := primitive.Regex{Pattern: "^.*BTC.*$"} // contains BTC substring
		ss.setPairs(ctx, coll, filter)
	case "Altcoins":
		filter := primitive.Regex{Pattern: "^((?!BTC).)*$"} // does not contain BTC substring
		ss.setPairs(ctx, coll, filter)
	default:
		ss.log.Fatal("Can't start in provided mode. "+
			"Please set 'MODE' environment variable to 'Bitcoin', 'Altcoins' or 'All'",
			zap.String("mode", mode),
		)
	}
	// testStrat, _ := primitive.ObjectIDFromHex("5deecc36ba8a424bfd363aaf")
	// , {"_id", testStrat}
	additionalCondition := bson.E{}
	accountId := os.Getenv("ACCOUNT_ID")
	if isLocalBuild {
		additionalCondition.Key = "accountId"
		additionalCondition.Value, _ = primitive.ObjectIDFromHex(accountId)
	}
	// Add strategies exists to runtime
	ss.log.Info("reading storage for strategies to add on init")
	coll = mongodb.GetCollection("core_strategies")
	cur, err := coll.Find(ctx, bson.D{{"enabled", true}, additionalCondition})
	if err != nil {
		ss.log.Error("can't read strategies",
			zap.Error(err),
		)
		wg.Done() // TODO(khassanov): should we really fail here?
		//log.Fatal(err)
	}
	if cur == nil {
		ss.log.Error("finding strategies, cur == nil")
		wg.Done()
	}
	defer cur.Close(ctx)
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
		if _, ok := ss.pairs[int8(strategy.Model.Conditions.MarketType)][strategy.Model.Conditions.Pair]; !ok {
			continue // skip a foreign pair
		}
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
	ss.log.Info("strategies settled on init", zap.Int64("count", strategiesAdded))

	go ss.InitPositionsWatch()                     // subscribe to position updates
	go ss.stateMgmt.InitOrdersWatch()              // subscribe to order updates
	go ss.WatchStrategies(isLocalBuild, accountId) // subscribe to new smart trades to add them into runtime
	go ss.runReporting()
	go ss.runIsFullTracking()

	if err := cur.Err(); err != nil { // TODO(khassanov): can we retry here?
		wg.Done()
		ss.log.Fatal("log.Fatal at the end of init func")
	}
	ss.statsd.Inc("strategy_service.instantiated")
	dt := time.Since(t1)
	ss.statsd.TimingDuration("strategy_service.init", dt)
	ss.log.Info("init complete, ready to settle strategies", zap.Duration("elapsed while init", dt))
}

// GetStrategy creates strategy instance with given arguments.
func GetStrategy(strategy *models.MongoStrategy, df interfaces.IDataFeed, tr trading.ITrading, st interfaces.IStateMgmt, statsd *statsd_client.StatsdClient, ss *StrategyService) *strategies.Strategy {
	// TODO(khassanov): why we use this instead of the same from the `strategy` package?
	// TODO(khassanov): remove code copy got from the same in the strategy package
	logger, _ := zap.NewProduction() // TODO(khassanov): handle the error
	loggerName := fmt.Sprintf("sm-%v", strategy.ID.Hex())
	logger = logger.With(zap.String("logger", loggerName))
	rs := redis.GetRedsync()
	mutexName := fmt.Sprintf("strategy:%v:%v", strategy.Conditions.Pair, strategy.ID.Hex())
	mutex := rs.NewMutex(mutexName,
		redsync.WithTries(2),
		redsync.WithRetryDelay(200*time.Millisecond),
		redsync.WithExpiry(10*time.Second), // TODO(khassanov): use parameter to conform with extend call period
	) // upsert
	return &strategies.Strategy{
		Model:           strategy,
		SettlementMutex: mutex,
		Datafeed:        df,
		Trading:         tr,
		StateMgmt:       st,
		Singleton:       ss,
		Statsd:          statsd,
		Log:             logger,
	}
}

// AddStrategy instantiates given strategy to store in the service instance and start it.
func (ss *StrategyService) AddStrategy(strategy *models.MongoStrategy) {
	if ss.strategies[strategy.ID.String()] == nil {
		sig := GetStrategy(strategy, ss.dataFeed, ss.trading, ss.stateMgmt, &ss.statsd, ss)
		if ok, err := sig.Settle(); !ok || err != nil {
			return // TODO(khassanov): distinguish a state locked in dlm and network errors
		}
		ss.log.Info("adding strategy",
			zap.String("ObjectID", sig.Model.ID.Hex()),
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

// WatchStrategies subscribes to strategies to add new strategies to runtime or update local data together with
// persistent storage updates.
// TODO(khassanov) can we remove `isLocalBuild` parameter in favor of environment variable?
func (ss *StrategyService) WatchStrategies(isLocalBuild bool, accountId string) error {
	ss.log.Info("watching for new strategies in the storage")
	ctx := context.Background()
	var coll = mongodb.GetCollection("core_strategies")
	cs, err := coll.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	//cs, err := coll.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		ss.log.Error("can't watch for strategies", zap.Error(err))
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

		if _, ok := ss.pairs[int8(event.FullDocument.Conditions.MarketType)][event.FullDocument.Conditions.Pair]; !ok {
			continue // skip a foreign pair
		}

		if event.FullDocument.Type == 2 && event.FullDocument.State.ColdStart { // 2 means maker only
			sig := GetStrategy(&event.FullDocument, ss.dataFeed, ss.trading, ss.stateMgmt, &ss.statsd, ss)
			ss.strategies[event.FullDocument.ID.String()] = sig
			ss.log.Info("continue in maker-only cold start")
			continue
		}

		if isLocalBuild && (event.FullDocument.AccountId == nil || event.FullDocument.AccountId.Hex() != accountId) {
			ss.log.Warn("continue watchStrategies in accountId incomparable",
				zap.String("ObjectID", event.FullDocument.ID.Hex()),
				zap.String("event AccountID", event.FullDocument.AccountId.Hex()),
				zap.String("AccountID in .env", accountId),
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
			if ss.full {
				ss.log.Debug("ignoring new strategy while the instance is full",
					zap.Bool("full", ss.full),
					zap.String("strategy", event.FullDocument.ID.Hex()),
					zap.Bool("strategy enabled", event.FullDocument.Enabled),
				)
				continue
			}
			if event.FullDocument.Enabled == true {
				ss.AddStrategy(&event.FullDocument)
				ss.statsd.Inc("strategy_service.add_strategy_from_db")
			}
		}
	}
	ss.log.Warn("watching for new strategies stopped")
	return nil
}

// InitPositionsWatch subscribes to smart trade updates for each position update received to disable smart trade if position closed externally.
func (ss *StrategyService) InitPositionsWatch() {
	ss.log.Info("watching for new positions in the storage")
	ctx := context.Background()
	var collPositions = mongodb.GetCollection("core_positions")
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
			var collStrategies = mongodb.GetCollection("core_strategies")
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
	ss.log.Warn("watching for new positions stopped")
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

// runReporting each minute sends how much strategies settled service has for the moment.
func (ss *StrategyService) runReporting() {
	ss.log.Info("starting statistics reporting")
	ticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-ticker.C:
			numStrategiesByPair := make(map[string]int64)
			for _, strategy := range ss.strategies {
				if _, ok := numStrategiesByPair[strategy.Model.Conditions.Pair]; ok {
					numStrategiesByPair[strategy.Model.Conditions.Pair]++
				} else {
					numStrategiesByPair[strategy.Model.Conditions.Pair] = 1
				}
			}
			for pair, numStrategies := range numStrategiesByPair {
				metricName := fmt.Sprintf("strategy_service.pairs.%v", pair)
				ss.statsd.Gauge(metricName, numStrategies)
			}
		}
	}
}

// setPairs reads pairs from mongodb collection with the filter given and writes them to instance pairs field.
func (ss *StrategyService) setPairs(ctx context.Context, collection *mongo.Collection, filter primitive.Regex) {
	filterDocument := bson.M{
		"name":       filter,
		"marketType": bson.M{"$ne": nil},
	}
	cur, err := collection.Find(ctx, filterDocument) // read all BTC markets
	if err != nil {
		ss.log.Fatal("can't read markets to get pairs", zap.Error(err))
		// TODO(khassanov): should we really fail here? We had wg.Done() here before
	}
	if cur == nil {
		ss.log.Fatal("core_markets mongodb read cur == nil")
		// TODO(khassanov): should we really fail here? We had wg.Done() here before
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var market models.MongoMarket
		err = cur.Decode(&market)
		if err != nil {
			ss.log.Error("can't decode market", zap.Error(err))
			continue
		}
		ss.pairs[int8(market.MarketType)][market.Name] = struct{}{}
	}
	ss.log.Info("pairs to process set",
		zap.Int("spot pairs", len(ss.pairs[0])),
		zap.Int("futures pairs", len(ss.pairs[1])),
		zap.String("pairs", fmt.Sprintf("%v", ss.pairs)),
	)
}

// trackIsFull monitors resources continuously and sets or resets 'full' flag when instance is close to memory limit
// or CPU usage limit.
func (ss *StrategyService) runIsFullTracking() {
	ss.log.Info("starting resources tracking")
	var err error
	var loadAvg *cpu_load.AvgStat
	var loadAvgScaled float64     // load average scaled by number of cores
	var cpuCoresCount int         // number of CPU cores
	var sysinfo syscall.Sysinfo_t // contains memory usage
	var isFullPrev bool
	for {
		isFullPrev = ss.full
		// check CPU usage
		loadAvg, err = cpu_load.Avg()
		if err != nil {
			ss.log.Error("load avg read", zap.Error(err))
		}
		cpuCoresCount, err = cpu_info.Counts(true)
		if err != nil {
			ss.log.Error("cpu count", zap.Error(err))
		}
		loadAvgScaled = loadAvg.Load5 / float64(cpuCoresCount)
		if loadAvgScaled > 1.2 { // TODO(khassanov): remove magic number
			ss.cpuFull = true
		} else {
			ss.cpuFull = false
		}
		// check for free RAM
		err = syscall.Sysinfo(&sysinfo)
		if err != nil {
			ss.log.Error("free RAM read", zap.Error(err))
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if sysinfo.Freeram < 10*1024*1024 { // ten megabytes
			ss.ramFull = true
		} else if sysinfo.Freeram > 12*1024*1024 { // 12 megabytes
			ss.ramFull = false
		}
		// set we are full or not
		if ss.cpuFull || ss.ramFull {
			ss.full = true
		} else if !ss.cpuFull && !ss.ramFull {
			ss.full = false
		}
		if isFullPrev != ss.full {
			ss.log.Info("switching settlement state", zap.Bool("skip incoming strategies", ss.full))
		}
		ss.log.Debug("resources check",
			zap.Uint64("free RAM, bytes", sysinfo.Freeram),
			zap.Float64("load avg 5 scaled", loadAvgScaled),
			zap.Float64("load avg 5", loadAvg.Load5),
			zap.Int("cpu count", cpuCoresCount),
		)
		time.Sleep(1 * time.Second)
	}
}
