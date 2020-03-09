package backtesting

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/qmuntal/stateless"
	backtestingMocks "gitlab.com/crypto_project/core/strategy_service/src/backtesting/mocks"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"gitlab.com/crypto_project/core/strategy_service/tests"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BacktestResult struct {
	Spent  float64
	Gained float64
}

func BacktestStrategy(smartOrderModel models.MongoStrategy, historicalDataParams backtestingMocks.HistoricalParams) BacktestResult {
	loadENV("../../.env")

	// get historical data
	df := backtestingMocks.NewBTDataFeed(historicalDataParams)
	//printClosePrices(df.GetTickerData())

	// load SM to test
	tradingApi := tests.NewMockedTradingAPIWithMarketAccess(df)
	sm := tests.NewMockedStateMgmt(tradingApi)

	smartOrder := getSmartOrder(smartOrderModel, df, tradingApi, sm)

	// run SM
	go smartOrder.Start()
	waitUntilSmartOrderEnds(smartOrder)

	//fmt.Printf("Orders map %v", tradingApi.OrdersMap)

	// calculate Profit and Loss
	backtestResult := getBacktestResult(tradingApi)

	//amountSold, _ := tradingApi.AmountSum.Load("BTC_USDTsell")
	//fmt.Printf("Total sold: %f \n", amountSold.(float64))

	return backtestResult
}

func waitUntilSmartOrderEnds(smartOrder *smart_order.SmartOrder) {
	for true {
		isInState, _ := smartOrder.State.IsInState(smart_order.End)
		if isInState {
			log.Println("SM ended")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func getBacktestResult(tradingAPI *tests.MockTrading) BacktestResult {
	// extract info about filled/canceled orders, their cost and volumes from tradingApi
	moneySpent := 0.0
	moneyGained := 0.0
	tradingAPI.OrdersMap.Range(func(key, value interface{}) bool {
		//fmt.Printf("%v", order)

		order := value.(models.MongoOrder)
		if order.Status == "filled" && order.Side == "buy" {
			moneySpent += order.StopPrice * order.Filled
		}

		if order.Status == "filled" && order.Side == "sell" {
			moneyGained += order.StopPrice * order.Filled
		}

		return true
	})

	return BacktestResult{
		Spent:  moneySpent,
		Gained: moneyGained,
	}
}

func getSmartOrder(smartOrderModel models.MongoStrategy, dataFeed *backtestingMocks.BTDataFeed, TradingAPI *tests.MockTrading, StateManagement tests.MockStateMgmt) *smart_order.SmartOrder {
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	smartOrder := smart_order.NewSmartOrder(&strategy, dataFeed, TradingAPI, &keyId, &StateManagement)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		fmt.Printf("Transitioned from %s to %s on %s (Is reentry: %v) \n", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	return smartOrder
}

func loadENV(path string) {
	err := godotenv.Load(path)
	if err != nil {
		println("Error loading .env file")
	}
}
