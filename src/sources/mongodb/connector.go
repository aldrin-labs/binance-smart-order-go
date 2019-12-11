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
		client, err := Connect(os.Getenv("MONGODB"), time.Duration(time.Duration.Seconds(3)))
		if err != nil {
			println("cant get mongodb client", err)
			return nil
		}
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
func (sm *StateMgmt) GetPosition(strategyId primitive.ObjectID, symbol string) {

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