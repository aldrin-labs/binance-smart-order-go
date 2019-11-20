package strategies

import (
	"context"
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/service"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
)

func Start(signal primitive.ObjectID, stategy *service.Strategy) {
	var changeStream, err = mongodb.BuildChangeStream(mongodb.GetMongoClientInstance(), "core_signals", []bson.M{})

	if err != nil {
		//if err is not nil, it means something bad happened, let's finish our func
		return
	}
	ctx := context.Background()
	//Handling change stream in a cycle
	for {
		select {
		case <-ctx.Done(): //if parent context was cancelled
			err := changeStream.Close(ctx) //we are closing the stream
			if err != nil {
				println("Change stream closed")
			}
			return //exiting from the func
		default:
			//making a struct for unmarshalling
			changeDoc := struct {
				FullDocument models.MongoSignal `bson:"fullDocument"`
			}{}

			for changeStream.Next(ctx) {
				elem := changeDoc
				if err := changeStream.Decode(changeDoc); err != nil {
					log.Fatal(err)
				}
				fmt.Printf("%+v\n", elem)
				// do something with elem....
				stategy.CreateOrder()
			}
		}

	}
}
