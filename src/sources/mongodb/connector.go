package mongodb

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	statsd_client "gitlab.com/crypto_project/core/strategy_service/src/statsd"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"
)

var mongoClient *mongo.Client
var log *zap.Logger

func init() {
	_ = godotenv.Load()
	if os.Getenv("LOCAL") == "true" {
		log, _ = zap.NewDevelopment()
	} else {
		log, _ = zap.NewProduction() // TODO: handle the error
	}
	log = log.With(zap.String("logger", "srcMongo"))
}

func GetCollection(colName string) *mongo.Collection {
	client := GetMongoClientInstance()
	return client.Database(os.Getenv("MONGODBNAME")).Collection(colName)
}

func GetMongoClientInstance() *mongo.Client {
	if mongoClient == nil {
		url := os.Getenv("MONGODB")
		isLocalBuild := os.Getenv("LOCAL") == "true"
		timeout := 10 * time.Second
		ctx, _ := context.WithTimeout(context.Background(), timeout)
		client, _ := mongo.Connect(ctx, options.Client().SetDirect(isLocalBuild).
			//client, _ := mongo.Connect(ctx, options.Client().SetDirect(false).
			SetReadPreference(readpref.Primary()).
			SetWriteConcern(writeconcern.New(writeconcern.WMajority())).
			SetRetryWrites(true).
			SetReplicaSet("rs0").
			SetConnectTimeout(timeout).ApplyURI(url))
		mongoClient = client
	}
	return mongoClient
}

func Connect(url string, connectTimeout time.Duration) (*mongo.Client, error) {
	ctx, _ := context.WithTimeout(context.Background(), connectTimeout)
	timeout := 10 * time.Second
	isLocalBuild := os.Getenv("LOCAL") == "true"
	mongoClient, err := mongo.Connect(ctx, options.Client().SetDirect(isLocalBuild).
		// mongoClient, err := mongo.Connect(ctx, options.Client().SetDirect(false).
		SetReadPreference(readpref.Primary()).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority())).
		SetRetryWrites(true).
		SetReplicaSet("rs0").
		SetConnectTimeout(timeout).ApplyURI(url))
	return mongoClient, err
}

type StateMgmt struct {
	OrderCallbacks *sync.Map
	Statsd         *statsd_client.StatsdClient
}

// InitOrdersWatch subscribes to orders updates and invokes StateMgnt callback on `filled` and `canceled` orders update event received.
func (sm *StateMgmt) InitOrdersWatch() {
	log.Info("watching for new orders in the storage")
	sm.OrderCallbacks = &sync.Map{}
	CollName := "core_orders"
	ctx := context.Background()
	var coll = GetCollection(CollName)
	pipeline := mongo.Pipeline{bson.D{
		{"$match", bson.M{"$or": []interface{}{
			bson.M{"fullDocument.status": "filled"},
			bson.M{"fullDocument.status": "canceled"},
		}},
		},
	}}
	cs, err := coll.Watch(ctx, pipeline, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		panic(err.Error())
	}
	//require.NoError(cs, err)
	defer cs.Close(ctx)
	for cs.Next(ctx) {
		var eventDecoded models.MongoOrderUpdateEvent
		err := cs.Decode(&eventDecoded)
		//	data := next.String()
		// log.Print(data)
		//		err := json.Unmarshal([]byte(data), &event)
		if err != nil {
			log.Error("event decode",
				zap.Error(err),
				zap.String("orderRaw", fmt.Sprintf("%+v", cs.Current)),
			)
		}
		go func(event models.MongoOrderUpdateEvent) {
			if event.FullDocument.Status == "filled" || event.FullDocument.Status == "canceled" {
				orderId := event.FullDocument.OrderId
				if event.FullDocument.PostOnlyInitialOrderId != "" {
					orderId = event.FullDocument.PostOnlyInitialOrderId
				}
				log.Info("order",
					zap.String("orderId", orderId),
					zap.String("status", event.FullDocument.Status),
					zap.Time("event.FullDocument.UpdatedAt", event.FullDocument.UpdatedAt),
				)
				getCallBackRaw, ok := sm.OrderCallbacks.Load(orderId)
				if ok {
					log.Debug("callback found",
						zap.String("orderId", orderId),
						zap.String("fullDocument", fmt.Sprintf("%+v", event.FullDocument)),
					)
					callback := getCallBackRaw.(func(order *models.MongoOrder))
					callback(&event.FullDocument)
				}
			}
		}(eventDecoded)
	}
	log.Warn("watching for new orders stopped")
}

func (sm *StateMgmt) EnableStrategy(strategyId *primitive.ObjectID) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	update = bson.D{
		{
			"$set", bson.D{
				{
					"enabled", true,
				},
			},
		},
	}
	_, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
	}
	sm.Statsd.TimingDuration("state_mgmt.enable_strategy", time.Since(t1))
}

func (sm *StateMgmt) DisableStrategy(strategyId *primitive.ObjectID) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	log.Info("disabling strategy",
		zap.String("id", strategyId.String()),
	)
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	update = bson.D{
		{
			"$set", bson.D{
				{
					"enabled", false,
				},
			},
		},
	}
	_, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
	}
	sm.CheckDisabledStrategy(strategyId, 2, 30)
	sm.Statsd.TimingDuration("state_mgmt.disable_strategy", time.Since(t1))
}

func (sm *StateMgmt) CheckDisabledStrategy(strategyId *primitive.ObjectID, times int, timeout int64) {
	strategy := sm.GetStrategy(strategyId)
	log.Info("CheckDisabledStrategy times enabled",
		zap.Int("times", times),
		zap.Bool("enabled", strategy.Enabled),
	)
	if times <= 0 || strategy == nil || strategy.Enabled == false {
		return
	} else {
		time.Sleep(time.Duration(timeout) * time.Second)
		if strategy.Enabled {
			sm.DisableStrategy(strategyId)
		}
		sm.CheckDisabledStrategy(strategyId, times-1, timeout)
	}
}

func (sm *StateMgmt) SubscribeToOrder(orderId string, onOrderStatusUpdate func(order *models.MongoOrder)) error {
	sm.OrderCallbacks.Store(orderId, onOrderStatusUpdate)
	executedOrder := sm.GetOrder(orderId)
	log.Info("subscribing to order",
		zap.Bool("executedOrder is nil", executedOrder == nil),
	)
	if executedOrder != nil { // looks like if this is true, we store a callback above forever, no?
		onOrderStatusUpdate(executedOrder)
	}
	return nil
}

// GetAssets returns IDs related to given market pair (e. g. BTCUSD) and type (e. g. Spot) given.
func GetAssets(pair string, marketType int64) (*primitive.ObjectID, *primitive.ObjectID, *primitive.ObjectID) {
	CollName := "core_markets"
	ctx := context.Background()
	var request bson.D
	request = bson.D{
		{"name", pair},
		{"marketType", marketType},
	}
	var coll = GetCollection(CollName)
	var market *models.MongoMarket
	err := coll.FindOne(ctx, request).Decode(&market)
	if err != nil {
		log.Error("strategy decode error", zap.Error(err))
		return nil, nil, nil
	}
	return market.BaseId, market.QuoteId, market.ID
}

func GetKeyIdAndExchangeId(keyId *primitive.ObjectID) (*primitive.ObjectID, *primitive.ObjectID) {
	CollName := "core_keys"
	ctx := context.Background()
	request := bson.D{
		{"_id", keyId},
	}
	var coll = GetCollection(CollName)

	var foundKey *models.MongoKey
	err := coll.FindOne(ctx, request).Decode(&foundKey)
	if err != nil {
		log.Error("strategy decode error", zap.Error(err))
		return nil, nil
	}
	return keyId, foundKey.ExchangeId

}

// SaveOrder upserts the order in a persistent storage.
func (sm *StateMgmt) SaveOrder(order models.MongoOrder, keyId *primitive.ObjectID, marketType int64) {
	t1 := time.Now()
	log.Info("saving order", zap.String("order", fmt.Sprintf("%v", order)))
	baseId, quoteId, marketId := GetAssets(order.Symbol, marketType)
	_, exchangeId := GetKeyIdAndExchangeId(keyId)
	opts := options.Update().SetUpsert(true)
	filter := bson.D{{"_id", order.ID}}
	update := bson.D{{"$set", bson.D{
		{"_id", order.ID},
		{"id", order.OrderId},
		{"keyId", keyId},
		{"baseId", baseId},
		{"quoteId", quoteId},
		{"exchangeId", exchangeId},
		{"marketId", marketId},
		{"filled", order.Filled},
		{"average", order.Average},
		{"status", order.Status},
		{"symbol", order.Symbol},
		{"side", order.Side},
		{"type", order.Type},
		{"reduceOnly", order.ReduceOnly},
		{"positionSide", order.PositionSide},
		{"timestamp", float64(time.Now().UnixNano() / 1000000)},
	}}}
	CollName := "core_orders"
	ctx := context.Background()
	var coll = GetCollection(CollName)
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Error("", zap.Error(err))
	}
	sm.Statsd.TimingDuration("state_mgmt.save_order", time.Since(t1))
}

func (sm *StateMgmt) SubscribeToHedge(strategyId *primitive.ObjectID, onStrategyUpdate func(strategy *models.MongoStrategy)) error {
	go func() {
		var strategy *models.MongoStrategy
		isOrderStillOpen := true
		for isOrderStillOpen {
			strategy = sm.GetStrategy(strategyId)
			if strategy != nil {
				onStrategyUpdate(strategy)
			}
			time.Sleep(10 * time.Second)
			isOrderStillOpen = strategy == nil || strategy.State == nil || strategy.State.ExitPrice == 0
		}
	}()
	time.Sleep(3 * time.Second)
	CollName := "core_strategies"
	ctx := context.Background()
	var coll = GetCollection(CollName)
	pipeline := mongo.Pipeline{bson.D{
		{"$match",
			bson.D{
				{"fullDocument._id", strategyId},
			},
		},
	}}
	cs, err := coll.Watch(ctx, pipeline, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
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
			log.Error("event decode", zap.Error(err))
		}
		onStrategyUpdate(&event.FullDocument)
	}
	return nil
}

func (sm *StateMgmt) GetPosition(strategyId *primitive.ObjectID, symbol string) {

}

func (sm *StateMgmt) AnyActiveStrats(strategy *models.MongoStrategy) bool {
	t1 := time.Now()
	CollName := "core_strategies"
	ctx := context.Background()
	var request bson.D
	request = bson.D{
		{"_id", bson.D{{"$ne", strategy.ID}}},
		{"enabled", true},
		{"accountId", strategy.AccountId},
		{"conditions.pair", strategy.Conditions.Pair},
		{"conditions.marketType", strategy.Conditions.MarketType},
	}
	var coll = GetCollection(CollName)
	var foundStrategy *models.MongoStrategy
	err := coll.FindOne(ctx, request).Decode(&foundStrategy)
	if err != nil {
		log.Error("strategy decode error", zap.Error(err))
		return false
	}
	sm.Statsd.TimingDuration("state_mgmt.any_active_strats", time.Since(t1))
	if foundStrategy.ID.Hex() != strategy.ID.Hex() {
		return true
	}
	return false
}

func (sm *StateMgmt) GetOrder(orderId string) *models.MongoOrder {
	t1 := time.Now()
	CollName := "core_orders"
	ctx := context.Background()
	var request bson.D
	request = bson.D{
		{"id", orderId},
	}
	var coll = GetCollection(CollName)

	var order *models.MongoOrder
	err := coll.FindOne(ctx, request).Decode(&order)
	if err != nil {
		log.Error("", zap.Error(err))
	}
	sm.Statsd.TimingDuration("state_mgmt.get_order", time.Since(t1))
	return order
}

func (sm *StateMgmt) GetOrderById(orderId *primitive.ObjectID) *models.MongoOrder {
	t1 := time.Now()
	CollName := "core_orders"
	ctx := context.Background()
	var request bson.D
	request = bson.D{
		{"_id", orderId},
	}
	var coll = GetCollection(CollName)

	var order *models.MongoOrder
	err := coll.FindOne(ctx, request).Decode(&order)
	if err != nil {
		log.Error("", zap.Error(err))
	}
	sm.Statsd.TimingDuration("state_mgmt.get_order_by_id", time.Since(t1))
	return order
}

func (sm *StateMgmt) SaveStrategy(strategy *models.MongoStrategy) *models.MongoStrategy {
	t1 := time.Now()
	log.Info("saving strategy")
	opts := options.Update().SetUpsert(true)
	filter := bson.D{{"_id", strategy.ID}}
	update := strategy
	CollName := "core_strategies"
	ctx := context.Background()
	var coll = GetCollection(CollName)
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Error("", zap.Error(err))
	}
	sm.Statsd.TimingDuration("state_mgmt.save_strategy", time.Since(t1))
	return strategy
}

func (sm *StateMgmt) CreateStrategy(strategy *models.MongoStrategy) *models.MongoStrategy {
	t1 := time.Now()
	log.Info("creating strategy")
	CollName := "core_strategies"
	ctx := context.Background()
	var coll = GetCollection(CollName)
	_, err := coll.InsertOne(ctx, strategy)
	if err != nil {
		log.Error("", zap.Error(err))
	}
	sm.Statsd.TimingDuration("state_mgmt.create_strategy", time.Since(t1))
	return strategy
}

func (sm *StateMgmt) GetStrategy(strategyId *primitive.ObjectID) *models.MongoStrategy {
	t1 := time.Now()
	CollName := "core_strategies"
	ctx := context.Background()
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var coll = GetCollection(CollName)

	var strategy *models.MongoStrategy
	err := coll.FindOne(ctx, request).Decode(&strategy)
	if err != nil {
		log.Error("", zap.Error(err))
	}
	sm.Statsd.TimingDuration("state_mgmt.get_strategy", time.Since(t1))
	return strategy
}

func (sm *StateMgmt) GetMarketPrecision(pair string, marketType int64) (int64, int64) {
	t1 := time.Now()
	CollName := "core_markets"
	ctx := context.Background()
	var request bson.D
	request = bson.D{
		{"name", pair},
		{"marketType", marketType},
	}

	var coll = GetCollection(CollName)
	var market *models.MongoMarket
	err := coll.FindOne(ctx, request).Decode(&market)
	if err != nil {
		log.Error("read market precision", zap.Error(err))
	}
	log.Info("got market precision",
		zap.String("pair", pair),
		zap.Int64("marketType", marketType),
		zap.Int64("PricePrecision", market.Properties.Binance.PricePrecision),
		zap.Int64("QuantityPrecision", market.Properties.Binance.QuantityPrecision),
	)
	sm.Statsd.TimingDuration("state_mgmt.get_market_precision", time.Since(t1))
	return market.Properties.Binance.PricePrecision, market.Properties.Binance.QuantityPrecision
}

func (sm *StateMgmt) UpdateConditions(strategyId *primitive.ObjectID, state *models.MongoStrategyCondition) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	update = bson.D{
		{
			"$set", bson.D{
				{
					"conditions", state,
				},
			},
		},
	}
	_, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
	}
	sm.Statsd.TimingDuration("state_mgmt.update_conditions", time.Since(t1))
	// log.Print(res)
}
func (sm *StateMgmt) UpdateState(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	updates := bson.D{
		{
			"state.state", state.State,
		},
	}
	if len(state.Msg) > 0 {
		updates = append(updates, bson.E{Key: "state.msg", Value: state.Msg})
	}
	update = bson.D{
		{
			"$set", updates,
		},
	}
	updated, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	log.Info("updated state",
		zap.Int64("count", updated.ModifiedCount),
		zap.String("state", state.State),
	)
	sm.Statsd.TimingDuration("state_mgmt.update_state", time.Since(t1))
}

func (sm *StateMgmt) UpdateStrategyState(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	updates := bson.D{
		{
			"state", state,
		},
	}
	if len(state.Msg) > 0 {
		updates = append(updates, bson.E{Key: "state.msg", Value: state.Msg})
	}
	update = bson.D{
		{
			"$set", updates,
		},
	}
	updated, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	log.Info("updated state of strategy",
		zap.Int64("count", updated.ModifiedCount),
		zap.String("state", state.State),
	)
	sm.Statsd.TimingDuration("state_mgmt.update_strategy_state", time.Since(t1))
}

func (sm *StateMgmt) UpdateExecutedAmount(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	updates := bson.D{
		{
			"state.executedAmount", state.ExecutedAmount,
		},
		{
			"state.exitPrice", state.ExitPrice,
		},
	}
	if state.ExecutedAmount == 0 {
		updates = append(updates, bson.E{Key: "state.executedOrders", Value: []string{}})
		updates = append(updates, bson.E{Key: "state.entryPrice", Value: 0})
		updates = append(updates, bson.E{Key: "state.amount", Value: 0})
		updates = append(updates, bson.E{Key: "state.reachedTargetCount", Value: 0})
	}
	var update bson.D
	update = bson.D{
		{
			"$set", updates,
		},
	}
	updated, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	log.Info("updated executed amount state",
		zap.Int64("count", updated.ModifiedCount),
		zap.String("state", state.State),
	)
	sm.Statsd.TimingDuration("state_mgmt.update_executed_amount", time.Since(t1))
}
func (sm *StateMgmt) UpdateOrders(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	if len(state.ExecutedOrders) == 0 || state.ExecutedOrders == nil {
		return
	}
	var update bson.D
	update = bson.D{
		{
			"$addToSet", bson.D{
				{
					"state.executedOrders", bson.D{
						{
							"$each", state.ExecutedOrders,
						},
					},
				},
				{
					"state.orders", bson.D{
						{
							"$each", state.Orders,
						},
					},
				},
			},
		},
	}
	updated, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	log.Info("updated order state",
		zap.Int64("count", updated.ModifiedCount),
		zap.String("state", state.State),
	)
	sm.Statsd.TimingDuration("state_mgmt.update_orders", time.Since(t1))
}
func (sm *StateMgmt) UpdateEntryPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	update = bson.D{
		{
			"$set", bson.D{
				{
					"state.entryPrice", state.EntryPrice,
				},
				{
					"state.state", state.State,
				},
				{
					"state.positionAmount", state.PositionAmount,
				},
			},
		},
	}
	updated, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	log.Info("updated entryPrice state",
		zap.Int64("count", updated.ModifiedCount),
		zap.String("state", state.State),
	)
	sm.Statsd.TimingDuration("state_mgmt.update_entry_price", time.Since(t1))
}

func (sm *StateMgmt) SwitchToHedgeMode(keyId *primitive.ObjectID, trading trading.ITrading) {
	t1 := time.Now()
	col := GetCollection("core_keys")
	var request bson.D
	request = bson.D{
		{"_id", keyId},
	}
	var keyFound models.MongoKey
	err := col.FindOne(context.TODO(), request).Decode(&keyFound)
	if err != nil {
		log.Error("no such key found: error in arg", zap.Error(err))
		return
	}
	if keyFound.HedgeMode {
	}
	sm.Statsd.TimingDuration("state_mgmt.switch_to_hedge_mode", time.Since(t1))
}

func (sm *StateMgmt) UpdateHedgeExitPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	update = bson.D{
		{
			"$set", bson.D{
				{
					"state.hedgeExitPrice", state.HedgeExitPrice,
				},
				{
					"state.state", state.State,
				},
			},
		},
	}
	updated, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	log.Info("updated hedgeExitPrice state",
		zap.Int64("count", updated.ModifiedCount),
		zap.String("state", state.State),
	)
	sm.Statsd.TimingDuration("state_mgmt.update_hedge_exit_price", time.Since(t1))
}

func (sm *StateMgmt) SavePNL(templateStrategyId *primitive.ObjectID, profitAmount float64) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", templateStrategyId},
	}

	var update bson.D
	update = bson.D{
		{
			"$inc", bson.D{
				{
					"conditions.templatePnl", profitAmount,
				},
			},
		},
	}
	_, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	log.Info("Updated template strategy with",
		zap.String("id", templateStrategyId.String()),
		zap.Float64("PnL changed", profitAmount),
	)
	sm.Statsd.TimingDuration("state_mgmt.save_pnl", time.Since(t1))
}

func (sm *StateMgmt) EnableHedgeLossStrategy(strategyId *primitive.ObjectID) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	update = bson.D{
		{
			"$set", bson.D{
				{
					"conditions.takeProfitExternal", false,
				},
			},
		},
	}
	_, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	sm.Statsd.TimingDuration("state_mgmt.enable_hadge_loss_strategy", time.Since(t1))
	// log.Print(res)
}

// SaveStrategyConditions updates dynamic state with given static (persistent) conditions.
func (sm *StateMgmt) SaveStrategyConditions(strategy *models.MongoStrategy) {
	strategy.State.EntryPointPrice = strategy.Conditions.EntryOrder.Price
	strategy.State.EntryPointType = strategy.Conditions.EntryOrder.OrderType
	strategy.State.EntryPointSide = strategy.Conditions.EntryOrder.Side
	strategy.State.EntryPointAmount = strategy.Conditions.EntryOrder.Amount
	strategy.State.EntryPointDeviation = strategy.Conditions.EntryOrder.EntryDeviation

	strategy.State.StopLoss = strategy.Conditions.StopLoss
	strategy.State.ForcedLoss = strategy.Conditions.ForcedLoss
	strategy.State.TakeProfit = strategy.Conditions.ExitLevels
	strategy.State.StopLossPrice = strategy.Conditions.StopLossPrice
	strategy.State.ForcedLossPrice = strategy.Conditions.ForcedLossPrice
	strategy.State.TrailingExitPrice = strategy.Conditions.TrailingExitPrice
	strategy.State.TakeProfitPrice = strategy.Conditions.TakeProfitPrice
	strategy.State.TakeProfitHedgePrice = strategy.Conditions.TakeProfitHedgePrice
}

// UpdateStateAndConditions updates state and conditions in a persistent storage.
func (sm *StateMgmt) UpdateStateAndConditions(strategyId *primitive.ObjectID, model *models.MongoStrategy) {
	t1 := time.Now()
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	var update bson.D
	update = bson.D{
		{
			"$set", bson.D{
				{
					"conditions", model.Conditions,
				},
				{
					"state", model.State,
				},
			},
		},
	}
	_, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		log.Error("error in arg", zap.Error(err))
		return
	}
	sm.Statsd.TimingDuration("state_mgmt.update_state_and_conditions", time.Since(t1))
	// log.Print(res)
}
