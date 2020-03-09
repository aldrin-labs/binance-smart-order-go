package backtesting

/*
	This file can be run as test and allows to quick local backtesting of SMs
*/

import (
	"fmt"
	"testing"

	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBacktest(t *testing.T) {
	smartOrderModel := getBacktestStrategy()
	backtestResult := backtestStrategy(smartOrderModel)
	fmt.Printf("Spent: %.02f Gained: %.02f Profit: %.02f \n", backtestResult.Spent, backtestResult.Gained, backtestResult.Gained-backtestResult.Spent)
}

func getBacktestStrategy() models.MongoStrategy {

	smartOrder := models.MongoStrategy{
		ID:          &primitive.ObjectID{},
		Conditions:  &models.MongoStrategyCondition{},
		State:       &models.MongoStrategyState{Amount: 1000},
		TriggerWhen: models.TriggerOptions{},
		Expiration:  models.ExpirationSchema{},
		OwnerId:     primitive.ObjectID{},
		Social:      models.MongoSocial{},
		Enabled:     true,
	}

	smartOrder.Conditions = &models.MongoStrategyCondition{
		Pair:         "BTC_USDT",
		MarketType:   1,
		Leverage:     100,
		StopLossType: "market",
		StopLoss:     3,
		EntryOrder: &models.MongoEntryPoint{
			Side:           "buy",
			ActivatePrice:  8750,
			Amount:         0.05,
			EntryDeviation: 2,
			OrderType:      "market",
		},
		ExitLevels: []*models.MongoEntryPoint{{
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
