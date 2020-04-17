package service

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/redis"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"sync"
)

// StrategyService strategy service type
type StrategyService struct {
	strategies map[string]*strategies.Strategy
	trading    trading.ITrading
	dataFeed   interfaces.IDataFeed
	stateMgmt  interfaces.IStateMgmt
}

var singleton *StrategyService
var once sync.Once

// GetStrategyService to get singleton
func GetStrategyService() *StrategyService {
	once.Do(func() {
		df := redis.InitRedis()
		tr := trading.InitTrading()
		sm := mongodb.StateMgmt{}
		go sm.InitOrdersWatch()
		singleton = &StrategyService{
			strategies: map[string]*strategies.Strategy{},
			dataFeed: df,
			trading: tr,
			stateMgmt: &sm,
		}
	})
	return singleton
}
func (ss *StrategyService) Init(wg *sync.WaitGroup, isLocalBuild bool) {
//func (ss *StrategyService) Init(wg *sync.WaitGroup) {
	ctx := context.Background()
	var coll = mongodb.GetCollection("core_strategies")
	// testStrat, _ := primitive.ObjectIDFromHex("5deecc36ba8a424bfd363aaf")
	// , {"_id", testStrat}
	additionalCondition := bson.E{}
	accountId := os.Getenv("ACCOUNT_ID")

	if isLocalBuild {
		additionalCondition.Key = "accountId"
		additionalCondition.Value, _ =  primitive.ObjectIDFromHex(accountId)
	}

	//cur, err := coll.Find(ctx, bson.D{{"enabled",true}})
	cur, err := coll.Find(ctx, bson.D{{"enabled", true}, additionalCondition})
	if err != nil {
		wg.Done()
		log.Fatal(err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		// create a value into which the single document can be decoded
		strategy, err := strategies.GetStrategy(cur, ss.dataFeed, ss.trading, ss.stateMgmt)
		if err != nil {
			log.Fatal(err)
		}
		println("objid " + strategy.Model.ID.String())
		GetStrategyService().strategies[strategy.Model.ID.String()] = strategy
		go strategy.Start()
	}
	//go ss.InitPositionsWatch()
	ss.WatchStrategies(isLocalBuild, accountId)
	//ss.WatchStrategies()
	if err := cur.Err(); err != nil {
		wg.Done()
		log.Fatal(err)
	}
}

func GetStrategy(strategy *models.MongoStrategy, df interfaces.IDataFeed, tr trading.ITrading, st interfaces.IStateMgmt) *strategies.Strategy {
	return &strategies.Strategy{Model:strategy, Datafeed: df, Trading: tr, StateMgmt: st}
}

func (ss *StrategyService) AddStrategy(strategy * models.MongoStrategy) {
	if ss.strategies[strategy.ID.String()] == nil {
		sig := GetStrategy(strategy, ss.dataFeed, ss.trading, ss.stateMgmt)
		println("start objid ", sig.Model.ID.String())
		ss.strategies[sig.Model.ID.String()] = sig
		go sig.Start()
	}
}

const CollName = "core_strategies"
func (ss *StrategyService) WatchStrategies(isLocalBuild bool, accountId string) error {
//func (ss *StrategyService) WatchStrategies() error {
	ctx := context.Background()
	var coll = mongodb.GetCollection(CollName)

	cs, err := coll.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	//cs, err := coll.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
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

		if isLocalBuild && event.FullDocument.AccountId.Hex() != accountId {
			return nil
		}

		if ss.strategies[event.FullDocument.ID.String()] != nil {
			ss.strategies[event.FullDocument.ID.String()].HotReload(event.FullDocument)
			if event.FullDocument.Enabled == false {
				delete(ss.strategies, event.FullDocument.ID.String())
			}
		} else {
			if event.FullDocument.Enabled == true {
				ss.AddStrategy(&event.FullDocument)
			}
		}
	}
	return nil
}

func (ss *StrategyService) InitPositionsWatch() {
	CollPositionsName := "core_positions"
	CollStrategiesName := "core_strategies"
	ctx := context.Background()
	var collPositions = mongodb.GetCollection(CollPositionsName)
	pipeline := mongo.Pipeline{}

	cs, err := collPositions.Watch(ctx, pipeline, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		panic(err.Error())
	}
	//require.NoError(cs, err)
	defer cs.Close(ctx)
	for cs.Next(ctx) {
		var positionEventDecoded models.MongoPositionUpdateEvent
		err := cs.Decode(&positionEventDecoded)
		//	data := next.String()
		// println(data)
		//		err := json.Unmarshal([]byte(data), &event)
		if err != nil {
			println("event decode", err.Error())
		}

		println("positionEventDecoded", positionEventDecoded.FullDocument.Symbol, positionEventDecoded.FullDocument.KeyId.Hex(), positionEventDecoded.FullDocument.PositionAmt)


		go func(event models.MongoPositionUpdateEvent) {
			var collStrategies = mongodb.GetCollection(CollStrategiesName)

			println("event.FullDocument.Symbol", event.FullDocument.Symbol)
			cur, err := collStrategies.Find(ctx, bson.D{
				{"conditions.marketType", 1},
				{"enabled", true},
				{"accountId", event.FullDocument.KeyId},
				{"conditions.pair", event.FullDocument.Symbol},})

			if err != nil {
				log.Fatal(err)
			}

			defer cur.Close(ctx)

			for cur.Next(ctx) {
				var strategyEventDecoded models.MongoStrategy
				err := cur.Decode(&strategyEventDecoded)

				if err != nil {
					println("event decode", err.Error())
				}

				println("strategyEventDecoded", strategyEventDecoded.ID.Hex())
				// if SM created before last position update
				// then we caught position event before actual update
				if positionEventDecoded.FullDocument.PositionAmt == 0 {
					if ss.strategies[strategyEventDecoded.ID.String()].GetModel().State.PositionWasPlaced {
						println("disabled by position close")
						collStrategies.FindOneAndUpdate(ctx, bson.D{{"_id", strategyEventDecoded.ID}}, bson.M{"$set": bson.M{"enabled": false}})
					} else {
						ss.strategies[strategyEventDecoded.ID.String()].GetModel().State.PositionWasPlaced = true
					}
				} else {
					ss.strategies[strategyEventDecoded.ID.String()].GetModel().State.PositionWasPlaced = true
				}
			}
		}(positionEventDecoded)
	}
}