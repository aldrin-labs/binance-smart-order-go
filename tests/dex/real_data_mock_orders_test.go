package dex

import (
	"context"
	"fmt"
	"github.com/go-redsync/redsync/v4"
	"gitlab.com/crypto_project/core/strategy_service/src/sources"

	"github.com/qmuntal/stateless"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/tests"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"strconv"
	"testing"
	"time"
)

func TestRealSerumDataEntry(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("simpleEntry")
	// price dips in the middle (This has no meaning now, reuse and then remove fake data stream)
	df := sources.InitDataFeed()
	time.Sleep(15 * time.Second)
	tradingApi := tests.NewMockedTradingAPI()
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmtWithOpts(tradingApi, df, "BTC_USDT", "serum", 0)
	logger, statsd := tests.GetLoggerStatsd()
	strategy := strategies.Strategy{
		Model:     &smartOrderModel,
		StateMgmt: &sm,
		Log: logger,
		Statsd: statsd,
		SettlementMutex: &redsync.Mutex{},
	}
	smartOrder := smart_order.New(&strategy, df, tradingApi, strategy.Statsd, &keyId, &sm)
	smartOrder.State.OnTransitioned(func(context context.Context, transition stateless.Transition) {
		log.Print("transition: source ", transition.Source.(string), ", destination ", transition.Destination.(string), ", trigger ", transition.Trigger.(string), ", isReentry ", transition.IsReentry())
	})
	go smartOrder.Start()
	for {
		time.Sleep(8000 * time.Millisecond)
		buyCallCount, _ := tradingApi.CallCount.Load("buy")
		btcUsdtCallCount, _ := tradingApi.CallCount.Load("BTC_USDT")
		fmt.Println("Success! There were " + strconv.Itoa(buyCallCount.(int)) + " trading api calls with buy params and " + strconv.Itoa(btcUsdtCallCount.(int)) + " with BTC_USDT params")
	}
	t.Error("")
}