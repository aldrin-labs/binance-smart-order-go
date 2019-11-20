package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"strings"
)

type ChangeStream struct {
}

func SubscribeToRedis(client *redis.Client) <-chan *redis.Message {
	redSub := client.PSubscribe("*:*:60")
	_, err := redSub.Receive()
	if err != nil {
		panic(err)
	}

	return nil
}

func ParseChangeStream(ctx context.Context, redisQueue <-chan *redis.Message) error {
	log.Println("waiting for parsers")

	for {
		select {
		case msg := <-redisQueue:
			var payload bson.M
			jsonResp := strings.Replace(msg.Payload, "'", "\"", -1)
			err := json.Unmarshal([]byte(jsonResp), &payload)
			//fmt.Printf("%s_%s\n", payload.Fsym, payload.Tsym)
			if err != nil {
				fmt.Printf("Error: %s", err.Error())
			}
			// to := dto.NewDTO()
			// to.AddData(fmt.Sprintf("%s_%s", payload.Fsym, payload.Tsym), payload)
			// sub.notifier.CheckFilters(to)
		case <-ctx.Done():
			return nil
		default:

		}
	}
}

func RunDataPull() error {
	return nil
}
