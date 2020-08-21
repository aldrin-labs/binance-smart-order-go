package smart_order

import (
	"context"
	"fmt"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies/smart_order"
	"gitlab.com/crypto_project/core/strategy_service/tests"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

func TestSmartOrderEntryBySpread(t *testing.T) {
	smartOrderModel := GetTestSmartOrderStrategy("entrySpread")
	// price falls
	fakeOHLCVDataStream := []interfaces.OHLCV{{
		Open:   7800,
		High:   7101,
		Low:    7750,
		Close:  7900,
		Volume: 30,
	}, {
		Open:   7005,
		High:   7100,
		Low:    7800,
		Close:  7900,
		Volume: 30,
	}, { // Hit entry
		Open:   7950,
		High:   7305,
		Low:    7950,
		Close:  7090,
		Volume: 30,
	}, { // Hit entry
		Open:   7950,
		High:   7305,
		Low:    7950,
		Close:  6990,
		Volume: 30,
	}}

	fakeDataStream := []interfaces.SpreadData{{
		BestAsk: 7006,
		BestBid: 6000,
		Close:  7005,
	}, {
		BestAsk: 7006,
		BestBid: 6000,
		Close:  7005,
	}, {
		BestAsk: 7006,
		BestBid: 6000,
		Close:  7005,
	}}
	df := tests.NewMockedSpreadDataFeed(fakeDataStream, fakeOHLCVDataStream)

	tradingApi := *tests.NewMockedTradingAPI()
	strategy := strategies.Strategy{
		Model: &smartOrderModel,
	}
	keyId := primitive.NewObjectID()
	sm := tests.NewMockedStateMgmt(&tradingApi, df)
	smartOrder := smart_order.NewSmartOrder(&strategy, df, tradingApi, &keyId, &sm, "")
	go smartOrder.Start()
	time.Sleep(1800 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(smart_order.InEntry)
	if !isInState {
		state, _ := smartOrder.State.State(context.Background())
		stateStr := fmt.Sprintf("%v", state)
		t.Error("SmartOrder state is not InEntry (State: " + stateStr + ")")
	}
}