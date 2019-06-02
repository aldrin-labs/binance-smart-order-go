package service

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"sync"
)

// StrategyService strategy service type
type StrategyService struct {
    strategies map[string]*Strategy
}
var singleton *StrategyService
var once sync.Once

// GetStrategyService to get singleton
func GetStrategyService() *StrategyService {
    once.Do(func() {
        singleton = &StrategyService{}
    })
    return singleton
}

func InitSingleton(wg *sync.WaitGroup) {
	ctx := context.Background()
	var coll = mongodb.GetCollection("core_strategies")
	cur, err := coll.Find(ctx, bson.D{})
	if err != nil {
		wg.Done()
		log.Fatal(err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		strategy, err := GetStrategy(cur)
		println(strategy)
		if err != nil {
			log.Fatal(err)
		}
		println("objid " + strategy.model.Id.String())
		GetStrategyService().strategies[strategy.model.Id.String()] = strategy
		go strategy.Start()
	}
	if err := cur.Err(); err != nil {
		wg.Done()
		log.Fatal(err)
	}
}