package testing

import "gitlab.com/crypto_project/core/strategy_service/src/service/strategies"

type MockDataFeed struct {
	tickerData [] float64
	currentTick int
}

func NewMockedDataFeed(mockedStream [] float64) strategies.IDataFeed {
	dataFeed := MockDataFeed{
		tickerData: mockedStream,
		currentTick: -1,
	}

	return &dataFeed
}

func (df *MockDataFeed) GetPriceForPairAtExchange(pair string, exchange string) strategies.OHLCV {
	df.currentTick += 1
	if df.currentTick >= len(df.tickerData) {
		df.currentTick = 0
	}
	println(df.tickerData[df.currentTick])
	return df.tickerData[df.currentTick]
}

func (df *MockDataFeed) SubscribeToPairUpdate() {

}
