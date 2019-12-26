package mongodb

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"os"
	"time"
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
		client, _ := mongo.Connect(ctx, options.Client().SetDirect(true).
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
	mongoClient, err := mongo.Connect(ctx, options.Client().SetDirect(true).
		SetReadPreference(readpref.Primary()).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority())).
		SetRetryWrites(true).
		SetReplicaSet("rs0").
		SetConnectTimeout(timeout).ApplyURI(url))
	return mongoClient, err
}

type StateMgmt struct {

}

func (sm *StateMgmt) DisableStrategy(strategyId primitive.ObjectID) {
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
	res, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		println("error in arg", err)
	}
	println(res)
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
			isOrderStillOpen = executedOrder == nil || (executedOrder.Status != "filled" && executedOrder.Status != "closed" && executedOrder.Status != "cancelled" && executedOrder.Status != "expired")
		}
	}()
	time.Sleep(3 * time.Second)
	CollName := "core_orders"
	ctx := context.Background()
	var coll = GetCollection(CollName)
	pipeline := mongo.Pipeline{bson.D{
		{"$match",
			bson.D{
				{"fullDocument.orderId", orderId},
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
			println("event decode", err)
		}
		onOrderStatusUpdate(&event.FullDocument)
	}
	return nil
}
func (sm *StateMgmt) GetPosition(strategyId primitive.ObjectID, symbol string) {

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
		println(err)
	}
	return order
}

func (sm *StateMgmt) UpdateConditions(strategyId primitive.ObjectID, state *models.MongoStrategyCondition) {
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
	res, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		println("error in arg", err)
	}
	println(res)
}

func (sm *StateMgmt) UpdateState(strategyId primitive.ObjectID, state *models.MongoStrategyState) {
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
					"state", state,
				},
			},
		},
	}
	res, err := col.UpdateOne(context.TODO(), request, update)
	if err != nil {
		println("error in arg", err)
	}
	println(res)
}