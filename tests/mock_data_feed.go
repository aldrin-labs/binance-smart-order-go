package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
)

type MockDataFeed struct {
	tickerData  []interfaces.OHLCV
	spreadData  []interfaces.SpreadData
	currentTick int
	currentSpreadTick int
}

func NewMockedDataFeed(mockedStream []interfaces.OHLCV) *MockDataFeed {
	dataFeed := MockDataFeed{
		tickerData:  mockedStream,
		currentTick: -1,
	}

	return &dataFeed
}

func NewMockedSpreadDataFeed(mockedStream []interfaces.SpreadData, mockedOHLCVStream []interfaces.OHLCV) *MockDataFeed {
	dataFeed := MockDataFeed{
		spreadData:  mockedStream,
		tickerData: mockedOHLCVStream,
		currentSpreadTick: -1,
		currentTick: -1,
	}

	return &dataFeed
}

func (df *MockDataFeed) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	df.currentTick += 1
	len := len(df.tickerData)
	if df.currentTick >= len && len > 0 {
		df.currentTick = len - 1
		return &df.tickerData[df.currentTick]
		// df.currentTick = len - 1 // ok we wont stop everything, just keep returning last price
	}

	return &df.tickerData[df.currentTick]
}

func (df *MockDataFeed) GetSpreadForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.SpreadData {
	df.currentSpreadTick += 1
	length := len(df.spreadData)
	// log.Print(len, df.currentTick)
	if df.currentSpreadTick >= length {
		df.currentSpreadTick = length - 1
		return &df.spreadData[df.currentSpreadTick]
		// df.currentTick = len - 1 // ok we wont stop everything, just keep returning last price
	}

	return &df.spreadData[df.currentSpreadTick]
}

func (df *MockDataFeed) SubscribeToPairUpdate() {

}

func (df *MockDataFeed) GetPrice(pair, exchange string, marketType int64) *interfaces.OHLCV {
	return nil
}

func (df *MockDataFeed) AddToFeed(mockedStream []interfaces.OHLCV) {
	df.tickerData = append(df.tickerData, mockedStream...)
}
