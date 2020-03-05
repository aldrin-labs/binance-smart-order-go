package backtest

import (
	"context"
	"fmt"
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

func TestBacktest(t *testing.T) {

	err := godotenv.Load("../../.env")
	if err != nil {
		println("Error loading .env file")
	}

	// get historical data
	df := backtest.NewBTDataFeed("binance", "BTC", "USDT", 60, 1, 1583317500, 1583317500+86400)
	//fmt.Printf("OHLCVs %v \n", df)
	printClosePrices(df.GetTickerData())

	// load SM to test
	smartOrderModel := getBacktestStrategy()

	tradingApi := tests.NewMockedTradingAPIWithMarketAccess(df)

	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi)
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		fmt.Printf("Transitioned from %s to %s on %s (Is reentry: %v) \n", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})

	// run SM
	go smartOrder.Start()
	time.Sleep(7 * time.Second)

	// extract info about filled/canceled orders, their cost and volumes from tradingApi

	moneySpent := 0.0
	moneyGained := 0.0
	tradingApi.OrdersMap.Range(func(key, value interface{}) bool {
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

	//fmt.Printf("Orders map %v", tradingApi.OrdersMap)

	// TODO: calculate Profit and Loss
	fmt.Printf("Spent: %.02f Gained: %.02f Profit: %.02f", moneySpent, moneyGained, moneyGained-moneySpent)

	//amountSold, _ := tradingApi.AmountSum.Load("BTC_USDTsell")
	//fmt.Printf("Total sold: %f \n", amountSold.(float64))

	// profit := getProfit(smartOrderModel.Conditions.EntryOrder.Side)
	// fmt.Printf("Total profit: %f \n", profit)
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
