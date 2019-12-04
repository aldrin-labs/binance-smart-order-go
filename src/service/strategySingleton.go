package service

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"sync"
)

// StrategyService strategy service type
type StrategyService struct {
	strategies map[string]*strategies.Strategy
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

func (ss *StrategyService) Init(wg *sync.WaitGroup) {
	ctx := context.Background()
	var coll = mongodb.GetCollection("core_strategies")
	cur, err := coll.Find(ctx, bson.D{})
	if err != nil {
		wg.Done()
		log.Fatal(err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		strategy, err := strategies.GetStrategy(cur)
		println(strategy)
		if err != nil {
			log.Fatal(err)
		}
		println("objid " + strategy.Model.Id.String())
		GetStrategyService().strategies[strategy.Model.Id.String()] = strategy
		go strategy.Start()
	}
	ss.WatchStrategies()
	if err := cur.Err(); err != nil {
		wg.Done()
		log.Fatal(err)
	}
}

func GetStrategy(strategy *models.MongoStrategy) *strategies.Strategy {
	return &strategies.Strategy{Model:strategy}
}

func (ss *StrategyService) AddStrategy(strategy * models.MongoStrategy) {
	sig := GetStrategy(strategy)
	println("objid ", sig.Model.Id.String())
	ss.strategies[sig.Model.Id.String()] = sig
	go sig.Start()

}

const CollName = "core_strategies"
func (ss *StrategyService) WatchStrategies() error {
	ctx := context.Background()
	var coll = mongodb.GetCollection(CollName)
	cs, err := coll.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
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
			println(err)
		}
		if ss.strategies[event.FullDocument.Id.String()] != nil {
			ss.strategies[event.FullDocument.Id.String()].HotReload(event.FullDocument)
			if event.FullDocument.Enabled == false {
				delete(ss.strategies, event.FullDocument.Id.String())
			}
		} else {
			if event.FullDocument.Enabled == true {
				ss.AddStrategy(&event.FullDocument)
			}
		}
	}
	return nil
}