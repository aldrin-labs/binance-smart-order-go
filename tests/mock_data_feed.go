package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
)

type MockDataFeed struct {
	tickerData  []interfaces.OHLCV
	currentTick int
}

func NewMockedDataFeed(mockedStream []interfaces.OHLCV) *MockDataFeed {
	dataFeed := MockDataFeed{
		tickerData:  mockedStream,
		currentTick: -1,
	}

	return &dataFeed
}

func (df *MockDataFeed) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	df.currentTick += 1
	len := len(df.tickerData)
	// println(len, df.currentTick)
	if df.currentTick >= len {
		df.currentTick = len - 1
		return &df.tickerData[df.currentTick]
		// df.currentTick = len - 1 // ok we wont stop everything, just keep returning last price
	}

	return &df.tickerData[df.currentTick]
}

func (df *MockDataFeed) SubscribeToPairUpdate() {

}

func (df *MockDataFeed) GetPrice(pair, exchange string, marketType int64) *interfaces.OHLCV {
	return nil
}

func (df *MockDataFeed) AddToFeed(mockedStream []interfaces.OHLCV) {
	df.tickerData = append(df.tickerData, mockedStream...)
}

func (df *MockDataFeed) GetTickerData() []interfaces.OHLCV {
	return df.tickerData
}

func (df *MockDataFeed) GetCurrentTick() int {
	return df.currentTick
}
