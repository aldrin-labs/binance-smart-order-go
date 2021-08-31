package tests

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"time"
)

type IDataFeed interface {
	GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV
}

const defaultWaitBetweenTicks = 150 * time.Millisecond

type MockDataFeed struct {
	tickerData                       []interfaces.OHLCV
	spreadData                       []interfaces.SpreadData
	currentTick                      int
	lastTickTime                     time.Time
	currentSpreadTick                int
	lastSpreadTickTime               time.Time
	WaitForOrderInitializationMillis int
	TickDuration                     time.Duration
	CycleLastNEntries                int
}

func NewMockedDataFeed(mockedStream []interfaces.OHLCV) *MockDataFeed {
	dataFeed := MockDataFeed{
		tickerData:         mockedStream,
		currentTick:        -1,
		CycleLastNEntries:  1,
		TickDuration:       defaultWaitBetweenTicks,
		lastSpreadTickTime: time.Unix(0,0),
		lastTickTime:       time.Unix(0,0),
	}

	return &dataFeed
}

func NewMockedSpreadDataFeed(mockedStream []interfaces.SpreadData, mockedOHLCVStream []interfaces.OHLCV) *MockDataFeed {
	dataFeed := MockDataFeed{
		spreadData:         mockedStream,
		tickerData:         mockedOHLCVStream,
		currentSpreadTick:  -1,
		currentTick:        -1,
		CycleLastNEntries:  1,
		TickDuration:       defaultWaitBetweenTicks,
		lastSpreadTickTime: time.Unix(0,0),
		lastTickTime:       time.Unix(0,0),
	}

	return &dataFeed
}

func (df *MockDataFeed) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	if df.lastTickTime == time.Unix(0,0) {
		time.Sleep(time.Duration(df.WaitForOrderInitializationMillis) * time.Millisecond)
		df.lastTickTime = time.Now()
	}

	ticksPassed := (int)(time.Since(df.lastTickTime).Milliseconds() / df.TickDuration.Milliseconds())

	if ticksPassed < len(df.tickerData) {
		df.currentTick = ticksPassed
	} else {
		df.currentTick = len(df.tickerData) - 1
	}
	return &df.tickerData[df.currentTick]
}

func (df *MockDataFeed) GetSpreadForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.SpreadData {
	if df.lastSpreadTickTime == time.Unix(0,0) {
		time.Sleep(time.Duration(df.WaitForOrderInitializationMillis) * time.Millisecond)
		df.lastSpreadTickTime = time.Now()
	}
	ticksPassed := (int)(time.Since(df.lastSpreadTickTime).Milliseconds() / df.TickDuration.Milliseconds())

	if ticksPassed < len(df.spreadData) {
		df.currentSpreadTick = ticksPassed
	} else {
		df.currentSpreadTick = len(df.spreadData) - 1
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
