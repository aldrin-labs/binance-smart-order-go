package redis

import (
	"github.com/go-redis/redis"
	"os"
	"time"
)

type Connector struct {
	redisOpts      redis.Options
	connectTimeout time.Duration
}

func NewConnector(addr string, password string, db int) (*Connector) {
	return &Connector{redisOpts: redis.Options{Addr: addr, Password: password, DB: db}}
}

func (conn *Connector) Connect() *redis.Client {
	return redis.NewClient(&conn.redisOpts)
}


var redisClient *redis.Client

func GetRedisClientInstance() *redis.Client {
	if redisClient == nil {
		client := redis.NewClient(&redis.Options{
			Addr:     os.Getenv("REDIS_HOST"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0,                // use default DB
		})
		redisClient = client
	}
	return redisClient
}