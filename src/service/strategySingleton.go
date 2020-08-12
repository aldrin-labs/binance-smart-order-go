package service

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
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
	"time"
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
		println("log.Fatal on finding enabled strategies")
		wg.Done()
		log.Fatal(err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		// create a value into which the single document can be decoded
		strategy, err := strategies.GetStrategy(cur, ss.dataFeed, ss.trading, ss.stateMgmt)
		if err != nil {
			println("log.Fatal on processing enabled strategy")
			log.Fatal(err)
		}
		println("objid " + strategy.Model.ID.String())
		GetStrategyService().strategies[strategy.Model.ID.String()] = strategy
		go strategy.Start()
	}
	go ss.InitPositionsWatch()
	ss.WatchStrategies(isLocalBuild, accountId)
	//ss.WatchStrategies()
	if err := cur.Err(); err != nil {
		println("log.Fatal at the end of init func")
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
		println("log.Fatal on watching strategies")
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
			println("event decode error on processing strategy", err.Error())
		}

		if isLocalBuild && (event.FullDocument.AccountId == nil || event.FullDocument.AccountId.Hex() != accountId) {
			return nil
		}
		if ss.strategies[event.FullDocument.ID.String()] != nil {
			ss.strategies[event.FullDocument.ID.String()].HotReload(event.FullDocument)
			ss.EditConditions(ss.strategies[event.FullDocument.ID.String()])
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
		println("panic error on watching positions")
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
			println("event decode in processing position", err.Error())
		}

		go func(event models.MongoPositionUpdateEvent) {
			var collStrategies = mongodb.GetCollection(CollStrategiesName)
			cur, err := collStrategies.Find(ctx, bson.D{
				{"conditions.marketType", 1},
				{"enabled", true},
				{"accountId", event.FullDocument.KeyId},
				{"conditions.pair", event.FullDocument.Symbol}},
			)

			if err != nil {
				println("log.Fatal on finding enabled strategies by position")
				log.Fatal(err)
			}

			defer cur.Close(ctx)

			for cur.Next(ctx) {
				var strategyEventDecoded models.MongoStrategy
				err := cur.Decode(&strategyEventDecoded)

				if err != nil {
					println("event decode on processing strategy found by position close", err.Error())
				}

				// if SM created before last position update
				// then we caught position event before actual update
				if positionEventDecoded.FullDocument.PositionAmt == 0 {
					strategy := ss.strategies[strategyEventDecoded.ID.String()]
					if strategy != nil && strategy.GetModel().Conditions.PositionWasClosed {
						println("disabled by position close")
						strategy.GetModel().Enabled = false
						collStrategies.FindOneAndUpdate(ctx, bson.D{{"_id", strategyEventDecoded.ID}}, bson.M{"$set": bson.M{"enabled": false}})
					}
				}
			}
		}(positionEventDecoded)
	}
}

func (ss *StrategyService) EditConditions(strategy *strategies.Strategy) {
	// here we should determine what was changed
	model := strategy.GetModel()
	isSpot := model.Conditions.MarketType == 0
	sm := strategy.StrategyRuntime
	isInEntry := model.State != nil && model.State.State != smart_order.TrailingEntry && model.State.State != smart_order.WaitForEntry

	if model.State == nil || sm == nil { return }
	if !isInEntry { return }

	entryOrder := model.Conditions.EntryOrder

	// entry order change
	if entryOrder.Amount != model.State.EntryPointAmount || entryOrder.Side != model.State.EntryPointSide || entryOrder.OrderType != model.State.EntryPointType || (entryOrder.Price != model.State.EntryPointPrice && entryOrder.EntryDeviation == 0) || entryOrder.EntryDeviation != model.State.EntryPointDeviation {
		if isSpot {
			sm.TryCancelAllOrdersConsistently(model.State.Orders)
			time.Sleep(5 * time.Second)
		} else {
			go sm.TryCancelAllOrders(model.State.Orders)
		}

		entryIsNotTrailing := model.Conditions.EntryOrder.ActivatePrice == 0

		if entryIsNotTrailing {
			sm.PlaceOrder(model.Conditions.EntryOrder.Price, smart_order.WaitForEntry)
		} else if model.State.TrailingEntryPrice > 0 {
			sm.PlaceOrder(-1, smart_order.TrailingEntry)
		}
	}

	// SL change
	if model.Conditions.StopLoss != model.State.StopLoss || model.Conditions.StopLossPrice != model.State.StopLossPrice {
		// we should also think about case when SL was placed by timeout, but didn't executed coz of limit order for example
		// with this we'll cancel it, and new order wont placed
		// for this we'll need currentOHLCV in price field
		if isSpot {
			sm.TryCancelAllOrdersConsistently(model.State.StopLossOrderIds)
			time.Sleep(5 * time.Second)
		} else {
			go sm.TryCancelAllOrders(model.State.StopLossOrderIds)
		}

		sm.PlaceOrder(0, smart_order.Stoploss)
	}

	if model.Conditions.ForcedLoss != model.State.ForcedLoss || model.Conditions.ForcedLossPrice != model.State.ForcedLossPrice {
		if isSpot {
		} else {
			sm.TryCancelAllOrders(model.State.ForcedLossOrderIds)
			sm.PlaceOrder(0, "ForcedLoss")
		}
	}

	if model.Conditions.TrailingExitPrice != model.State.TrailingExitPrice || model.Conditions.TakeProfitPrice != model.State.TakeProfitPrice {
		sm.PlaceOrder(-1, smart_order.TakeProfit)
	}

	if model.Conditions.TakeProfitHedgePrice != model.State.TakeProfitHedgePrice {
		sideCoefficient := 1.0
		feePercentage := 0.04 * 4
		if model.Conditions.EntryOrder.Side == "sell" {
			sideCoefficient = -1.0
		}

		currentProfitPercentage := ((model.Conditions.TakeProfitHedgePrice / model.State.EntryPrice) * 100 - 100) * model.Conditions.Leverage * sideCoefficient

		if currentProfitPercentage > feePercentage {
			strategy.GetModel().State.TrailingHedgeExitPrice = model.Conditions.TakeProfitHedgePrice
			sm.PlaceOrder(-1, smart_order.HedgeLoss)
		}
	}

	// TAP change
	// split targets
	if len(model.Conditions.ExitLevels) > 0 {
		if len(model.Conditions.ExitLevels) > 1 || (model.Conditions.ExitLevels[0].Amount > 0) {
			wasChanged := false

			if len(model.State.TakeProfit) != len(model.Conditions.ExitLevels) {
				wasChanged = true
			} else {
				for i, target := range model.Conditions.ExitLevels {
					if (target.Price != model.State.TakeProfit[i].Price || target.Amount != model.State.TakeProfit[i].Amount) && !wasChanged {
						wasChanged = true
					}
				}
			}

			// add some logic if some of targets was executed
			if wasChanged {
				ids := model.State.TakeProfitOrderIds[:]
				lastExecutedTarget := 0
				for i, id := range ids {
					orderStillOpen := sm.IsOrderExistsInMap(id)
					if orderStillOpen {
						sm.SetSelectedExitTarget(i)
						lastExecutedTarget = i
						break
					}
				}

				idsToCancel := ids[lastExecutedTarget:]
				if isSpot {
					sm.TryCancelAllOrdersConsistently(idsToCancel)
					time.Sleep(5 * time.Second)
				} else {
					sm.TryCancelAllOrdersConsistently(idsToCancel)
				}

				// here we delete canceled orders
				if lastExecutedTarget-1 >= 0 {
					strategy.GetModel().State.TakeProfitOrderIds = ids[:lastExecutedTarget]
				} else {
					strategy.GetModel().State.TakeProfitOrderIds = make([]string, 0)
				}

				sm.PlaceOrder(0, smart_order.TakeProfit)
			}
		} else if model.Conditions.ExitLevels[0].ActivatePrice > 0 && (model.Conditions.ExitLevels[0].EntryDeviation != model.State.TakeProfit[0].EntryDeviation) {
			// trailing TAP
			ids := model.State.TakeProfitOrderIds[:]
			if isSpot {
				sm.TryCancelAllOrdersConsistently(ids)
				time.Sleep(5 * time.Second)
			} else {
				go sm.TryCancelAllOrders(ids)
			}

			sm.PlaceOrder(-1, smart_order.TakeProfit)
		} else if model.Conditions.ExitLevels[0].Price != model.State.TakeProfit[0].Price { // simple TAP
			ids := model.State.TakeProfitOrderIds[:]
			if isSpot {
				sm.TryCancelAllOrdersConsistently(ids)
				time.Sleep(5 * time.Second)
			} else {
				go sm.TryCancelAllOrders(ids)
			}

			sm.PlaceOrder(0, smart_order.TakeProfit)
		}
	}

	strategy.StateMgmt.SaveStrategyConditions(strategy.Model)
}