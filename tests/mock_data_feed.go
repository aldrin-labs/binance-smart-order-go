package tests

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
	len := len(df.tickerData)
	if df.currentTick >= len {
		time.Sleep(5 * time.Second)
		df.currentTick = len - 1 // ok we wont stop everything, just keep returning last price
	}

	return &df.tickerData[df.currentTick]
}

func (df *MockDataFeed) SubscribeToPairUpdate() {

}

func (df *MockDataFeed) GetPrice(pair, exchange string, marketType int64) *strategies.OHLCV {
	return nil
}
