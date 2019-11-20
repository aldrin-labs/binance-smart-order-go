package mongodb

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"os"
)

type ChangeStream struct {
	///TODO: save which collection was used
	stream *mongo.ChangeStream
}

func BuildChangeStream(client *mongo.Client, colName string, pipeline []bson.M) (*mongo.ChangeStream, error) {
	var coll *mongo.Collection = client.Database(os.Getenv("MONGODBNAME")).Collection(colName)
	//TODO: check replica mechanic
	ctx := context.Background()

	cur, err := coll.Watch(ctx, pipeline)
	if err != nil {
		// Handle err
		return nil, err
	}

	return cur, nil
}

/*func ParseChangeStream(ctx context.Context,
	changeStream *ChangeStream,
	parser *parser.Parser) error {
	log.Println("waiting for parsers")
StreamLoop:
	for {
		select {
		case <-ctx.Done():
			err := changeStream.stream.Close(ctx)
			log.Println(err)
			if err != nil {
				return err
			}
			return nil
		default:
			for changeStream.stream.Next(ctx) {
				elem := &bson.D{}
				log.Println("new ele", elem)
				if err := changeStream.stream.Decode(elem); err != nil {
					log.Fatal(err)
				}
				evt, err := ParseData(elem)
				if err != nil {
					log.Println(err)
					continue
				}
				_, isRemoved, err := parser.ParseEvent(evt)
				if err != nil {
					log.Println(err)
					continue
				}
				if isRemoved {

				} else {
				}
			}

			if err := changeStream.stream.Err(); err != nil {
				log.Println(err)
				break StreamLoop
			}
		}
	}

	//closing stream and exiting from the func
	err := changeStream.stream.Close(ctx)
	if err != nil {
		return err
	}
	return nil
}
*/
/*func ParseData(doc *bson.D) (*events.Event, error) {
	change := doc.Map()
	log.Println(change)
	m := (change["fullDocument"]).(bson.D).Map()
	data := make(map[string]interface{})
	var notificationType int32
	for k, v := range m {
		switch k {
		case "notificationType":
			notificationType = (m["notificationType"]).(int32)
			break
		case "accountIds":

			break
		default:
			data[k] = v
		}
	}
	log.Println(m)
	return &events.Event{EventType: events.GetEventType(notificationType),
		Data: events.NewData(data)}, nil
}*/
