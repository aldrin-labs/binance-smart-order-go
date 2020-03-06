package backtest

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/qmuntal/stateless"
	backtest "gitlab.com/crypto_project/core/strategy_service/src/backtesting/mocks"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
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

func TestBacktest(t *testing.T) {

	loadENV("../../.env")

	// get historical data
	df := backtest.NewBTDataFeed("binance", "BTC", "USDT", 60, 1, 1583317500, 1583317500+86400)
	printClosePrices(df.GetTickerData())

	// load SM to test
	smartOrderModel := getBacktestStrategy()
	tradingApi := tests.NewMockedTradingAPIWithMarketAccess(df)
	sm := tests.NewMockedStateMgmt(tradingApi)

	smartOrder := getSmartOrder(smartOrderModel, df, tradingApi, sm)

	// run SM
	go smartOrder.Start()
	waitUntilSmartOrderEnds(smartOrder)

	//fmt.Printf("Orders map %v", tradingApi.OrdersMap)

	// calculate Profit and Loss
	backtestResult := getBacktestResult(tradingApi)
	fmt.Printf("Spent: %.02f Gained: %.02f Profit: %.02f \n", backtestResult.Spent, backtestResult.Gained, backtestResult.Gained-backtestResult.Spent)

	//amountSold, _ := tradingApi.AmountSum.Load("BTC_USDTsell")
	//fmt.Printf("Total sold: %f \n", amountSold.(float64))
}

func getBacktestStrategy() models.MongoStrategy {

	smartOrder := models.MongoStrategy{
		ID:          primitive.ObjectID{},
		Conditions:  models.MongoStrategyCondition{},
		State:       models.MongoStrategyState{Amount: 1000},
		TriggerWhen: models.TriggerOptions{},
		Expiration:  models.ExpirationSchema{},
		OwnerId:     primitive.ObjectID{},
		Social:      models.MongoSocial{},
		Enabled:     true,
	}

	smartOrder.Conditions = models.MongoStrategyCondition{
		Pair:         "BTC_USDT",
		MarketType:   1,
		Leverage:     100,
		StopLossType: "market",
		StopLoss:     3,
		EntryOrder: models.MongoEntryPoint{
			Side:           "buy",
			ActivatePrice:  8750,
			Amount:         0.05,
			EntryDeviation: 2,
			OrderType:      "market",
		},
		ExitLevels: []models.MongoEntryPoint{{
			OrderType:      "market",
			Type:           1,
			ActivatePrice:  10,
			EntryDeviation: 2,
			Amount:         100,
		}},
	}

	return smartOrder
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

func getProfit(entrySide string, entryAmount float64, entryPrice float64, exitPrice float64) float64 {
	profit := 0.0
	return profit
}

func printClosePrices(OHLCVs []interfaces.OHLCV) {
	for k, v := range OHLCVs {
		newLine := ""
		if (k+1)%10 == 0 {
			newLine = "\n"
		} else {
			newLine = ""
		}
		fmt.Printf("%.2f %s", v.Close, newLine)
	}
	println()
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

func getSmartOrder(smartOrderModel models.MongoStrategy, dataFeed *backtest.BTDataFeed, TradingAPI *tests.MockTrading, StateManagement tests.MockStateMgmt) *smart_order.SmartOrder {
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
