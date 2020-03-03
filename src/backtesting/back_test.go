package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/qmuntal/stateless"
	backtest "gitlab.com/crypto_project/core/strategy_service/src/backtesting/mocks"
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

	df := backtest.NewBTDataFeed("binance", "BTC", "USDT", 60, 1, 1578455936, 1578476936)

	//fmt.Printf("OHLCVs %v \n", df)
	smartOrderModel := getBacktestStrategy()

	tradingApi := tests.NewMockedTradingAPI()

	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(tradingApi)
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		println("transition:", transition.Source.(string), transition.Destination.(string), transition.Trigger.(string), transition.IsReentry())
	})
	go smartOrder.Start()

	time.Sleep(10 * time.Second)

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
		Pair: "BTC_USDT",
		EntryOrder: models.MongoEntryPoint{
			Side:      "buy",
			Price:     7000,
			Amount:    0.001,
			OrderType: "limit",
		},
		ExitLevels: []models.MongoEntryPoint{
			{
				Type:      1,
				OrderType: "limit",
				Price:     10,
				Amount:    100,
			},
		},
	}

	return smartOrder
}
