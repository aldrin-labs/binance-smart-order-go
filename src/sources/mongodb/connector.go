package mongodb

import (
	"context"
	"os"
	"time"

	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

var mongoClient *mongo.Client

func GetCollection(colName string) *mongo.Collection {
	client := GetMongoClientInstance()
	return client.Database(os.Getenv("MONGODBNAME")).Collection(colName)
}

func GetMongoClientInstance() *mongo.Client {
	if mongoClient == nil {
		url := os.Getenv("MONGODB")
		timeout := 10 * time.Second
		ctx, _ := context.WithTimeout(context.Background(), timeout)
		client, _ := mongo.Connect(ctx, options.Client().SetDirect(false).
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
	mongoClient, err := mongo.Connect(ctx, options.Client().SetDirect(false).
		SetReadPreference(readpref.Primary()).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority())).
		SetRetryWrites(true).
		SetReplicaSet("rs0").
		SetConnectTimeout(timeout).ApplyURI(url))
	return mongoClient, err
}

type StateMgmt struct {
}

func (sm *StateMgmt) DisableStrategy(strategyId *primitive.ObjectID) {
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
					"enabled", false,
				},
			},
		},
	}
	_, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		println("error in arg", err.Error())
	}
}

// TODO: refactor so it will be one global subscribtion to orders collection instead of one per order
func (sm *StateMgmt) SubscribeToOrder(orderId string, onOrderStatusUpdate func(order *models.MongoOrder)) error {
	go func() {
		executedOrder := sm.GetOrder(orderId)
		isOrderStillOpen := true
		for isOrderStillOpen {
			executedOrder = sm.GetOrder(orderId)
			if executedOrder != nil {
				onOrderStatusUpdate(executedOrder)
			}
			time.Sleep(2 * time.Second)
			isOrderStillOpen = executedOrder == nil || (executedOrder.Status != "expired" && executedOrder.Status != "filled" && executedOrder.Status != "closed" && executedOrder.Status != "canceled")
		}
	}()
	time.Sleep(3 * time.Second)
	CollName := "core_orders"
	ctx := context.Background()
	var coll = GetCollection(CollName)
	pipeline := mongo.Pipeline{bson.D{
		{"$match",
			bson.D{
				{"fullDocument.id", orderId},
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
		var event models.MongoOrderUpdateEvent
		err := cs.Decode(&event)
		//	data := next.String()
		// println(data)
		//		err := json.Unmarshal([]byte(data), &event)
		if err != nil {
			println("event decode", err.Error())
		}
		onOrderStatusUpdate(&event.FullDocument)
	}
	return nil
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
		// println(data)
		//		err := json.Unmarshal([]byte(data), &event)
		if err != nil {
			println("event decode", err.Error())
		}
		onStrategyUpdate(&event.FullDocument)
	}
	return nil
}

func (sm *StateMgmt) GetPosition(strategyId *primitive.ObjectID, symbol string) {

}

func (sm *StateMgmt) AnyActiveStrats(strategy *models.MongoStrategy) bool {
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
		println("strategy decode error: ", err.Error())
		return false
	}

	if foundStrategy.ID.Hex() != strategy.ID.Hex() {
		return true
	}

	return false
}

func (sm *StateMgmt) GetOrder(orderId string) *models.MongoOrder {
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
		println(err.Error())
	}
	return order
}

func (sm *StateMgmt) GetStrategy(strategyId *primitive.ObjectID) *models.MongoStrategy {
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
		println(err.Error())
	}
	return strategy
}

func (sm *StateMgmt) GetMarketPrecision(pair string, marketType int64) (int64, int64) {
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
		println(err.Error())
	}

	return market.Properties.Binance.PricePrecision, market.Properties.Binance.QuantityPrecision
}

func (sm *StateMgmt) UpdateConditions(strategyId *primitive.ObjectID, state *models.MongoStrategyCondition) {
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
		println("error in arg", err.Error())
	}
	// println(res)
}
func (sm *StateMgmt) UpdateState(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
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
		println("error in arg", err.Error())
		return
	}
	println("updated state", updated.ModifiedCount, state.State)
}
func (sm *StateMgmt) UpdateExecutedAmount(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
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
		println("error in arg", err.Error())
		return
	}
	println("updated state", updated.ModifiedCount, state.State)
}
func (sm *StateMgmt) UpdateOrders(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
	col := GetCollection("core_strategies")
	var request bson.D
	request = bson.D{
		{"_id", strategyId},
	}
	if len(state.ExecutedOrders) == 0 {
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
			},
		},
	}
	updated, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		println("error in arg", err.Error())
		return
	}
	println("updated state", updated.ModifiedCount, state.State)
}
func (sm *StateMgmt) UpdateEntryPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
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
			},
		},
	}
	updated, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		println("error in arg", err.Error())
		return
	}
	println("updated state", updated.ModifiedCount, state.State)
}
func (sm *StateMgmt) UpdateHedgeExitPrice(strategyId *primitive.ObjectID, state *models.MongoStrategyState) {
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
		println("error in arg", err.Error())
		return
	}
	println("updated state", updated.ModifiedCount, state.State)
}
