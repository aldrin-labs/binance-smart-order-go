package mongodb

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(url).SetDirect(true))
	return mongoClient, err
}
