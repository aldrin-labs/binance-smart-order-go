package testing

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

func TestStartingForEntry(t *testing.T) {
	smartOrderModel := models.MongoStrategy{
		Id:          primitive.ObjectID{},
		MonType:     models.MongoStrategyType{},
		Condition:   models.MongoStrategyCondition{
			TargetPrice:               7000,
			ActivationPrice:           6900,
			EntryDeviation:            10,
			Amount:                    10,
			Pair:                      "BTC_USDT",
			Side:                      "buy",
		},
		State:       models.MongoStrategyState{},
		TriggerWhen: models.TriggerOptions{},
		Expiration:  models.ExpirationSchema{},
		OwnerId:     primitive.ObjectID{},
		Social:      models.MongoSocial{},
	}
	fakeDataStream := []strategies.OHLCV{{
		Open:   0,
		High:   0,
		Low:    0,
		Close:  0,
		Volume: 0,
	}, {
		Open:   0,
		High:   0,
		Low:    0,
		Close:  0,
		Volume: 0,
	}, {
		Open:   0,
		High:   0,
		Low:    0,
		Close:  0,
		Volume: 0,
	}}
	df := NewMockedDataFeed(fakeDataStream)
	smartOrder := strategies.NewSmartOrder(&smartOrderModel, df)
	go smartOrder.Start()
	time.Sleep(300 * time.Millisecond)
	isInState, _ := smartOrder.State.IsInState(strategies.InEntry)
	if !isInState {
		t.Error("SmartOrder is not InEntry")
	}
}