package testing

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"time"
)

type MockDataFeed struct {
	tickerData  []strategies.OHLCV
	currentTick int
}

func NewMockedDataFeed(mockedStream []strategies.OHLCV) strategies.IDataFeed {
	dataFeed := MockDataFeed{
		tickerData:  mockedStream,
		currentTick: -1,
	}

	return &dataFeed
}

func (df *MockDataFeed) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *strategies.OHLCV {
	df.currentTick += 1
	if df.currentTick >= len(df.tickerData) {
		time.Sleep(60 * time.Second)
	}

	return &df.tickerData[df.currentTick]
}

func (df *MockDataFeed) SubscribeToPairUpdate() {

}
