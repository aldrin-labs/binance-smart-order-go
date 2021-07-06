package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"time"
)

type IDataFeed interface {
	GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV
}

type MockDataFeed struct {
	tickerData                 []interfaces.OHLCV
	spreadData                 []interfaces.SpreadData
	currentTick                int
	currentSpreadTick          int
	WaitForOrderInitialization int
	WaitBetweenTicks           int
	CycleLastNEntries          int
}

func NewMockedDataFeed(mockedStream []interfaces.OHLCV) *MockDataFeed {
	dataFeed := MockDataFeed{
		tickerData:        mockedStream,
		currentTick:       -1,
		CycleLastNEntries: 1,
	}

	return &dataFeed
}

func NewMockedDataFeedWithWait(mockedStream []interfaces.OHLCV, initializationWait int) *MockDataFeed {
	dataFeed := MockDataFeed{
		tickerData:                 mockedStream,
		currentTick:                -1,
		WaitForOrderInitialization: initializationWait,
		CycleLastNEntries:          1,
	}

	return &dataFeed
}

func NewMockedDataFeedWithWaitCycling(mockedStream []interfaces.OHLCV, initializationWait int, cycleEntries int) *MockDataFeed {
	if cycleEntries > len(mockedStream) {
		return nil
	}
	dataFeed := MockDataFeed{
		tickerData:                 mockedStream,
		currentTick:                -1,
		WaitForOrderInitialization: initializationWait,
		CycleLastNEntries:          cycleEntries,
	}

	return &dataFeed
}

func NewMockedSpreadDataFeed(mockedStream []interfaces.SpreadData, mockedOHLCVStream []interfaces.OHLCV) *MockDataFeed {
	dataFeed := MockDataFeed{
		spreadData:        mockedStream,
		tickerData:        mockedOHLCVStream,
		currentSpreadTick: -1,
		currentTick:       -1,
		CycleLastNEntries: 1,
	}

	return &dataFeed
}

func NewMockedSpreadDataFeedWithWait(
	mockedStream []interfaces.SpreadData,
	mockedOHLCVStream []interfaces.OHLCV,
	initializationWait int) *MockDataFeed {
	dataFeed := MockDataFeed{
		spreadData:                 mockedStream,
		tickerData:                 mockedOHLCVStream,
		currentSpreadTick:          -1,
		currentTick:                -1,
		WaitForOrderInitialization: initializationWait,
		CycleLastNEntries:          1,
	}

	return &dataFeed
}

func (df *MockDataFeed) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	if df.currentTick <= 0 {
		time.Sleep(time.Duration(df.WaitForOrderInitialization) * time.Millisecond)
	}
	time.Sleep(time.Duration(df.WaitBetweenTicks) * time.Millisecond)
	df.currentTick += 1
	len := len(df.tickerData)
	if df.currentTick >= len && len > 0 {
		df.currentTick = len - df.CycleLastNEntries
		return &df.tickerData[df.currentTick]
		// df.currentTick = len - 1 // ok we wont stop everything, just keep returning last price
	}

	return &df.tickerData[df.currentTick]
}

func (df *MockDataFeed) GetSpreadForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.SpreadData {
	if df.currentSpreadTick <= 0 {
		time.Sleep(time.Duration(df.WaitForOrderInitialization) * time.Millisecond)
	}
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
