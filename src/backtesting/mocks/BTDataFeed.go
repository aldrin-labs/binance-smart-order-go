package backtest

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/eventDriven/mysql"
)

type HistoricalParams struct {
	Exchange      string
	Base          string
	Quote         string
	OhlcvPeriod   int
	MarketType    int // 0 - spot, 1 - futures,
	IntervalStart int // unix time timestamp
	IntervalEnd   int // unix time timestamp
}

type BTDataFeed struct {
	tickerData  []interfaces.OHLCV
	currentTick int
}

func NewBTDataFeed(params HistoricalParams) *BTDataFeed {

	// connect to MySql
	sqlConn := mysql.SQLConn{}
	sqlConn.Initialize()

	ohlcvsFromMysql := sqlConn.GetOhlcv(params.Exchange, params.Base, params.Quote, params.OhlcvPeriod, params.MarketType, params.IntervalStart, params.IntervalEnd)

	dataFeed := BTDataFeed{
		tickerData:  ohlcvsFromMysql,
		currentTick: -1,
	}

	return &dataFeed
}

func (df *BTDataFeed) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	df.currentTick += 1
	len := len(df.tickerData)
	if df.currentTick >= len {
		df.currentTick = len - 1
		return &df.tickerData[df.currentTick]
	}
	println("Current tick", df.currentTick)
	return &df.tickerData[df.currentTick]
}

func (df *BTDataFeed) SubscribeToPairUpdate() {
	panic("implement me")
}

func (df *BTDataFeed) GetPrice(pair, exchange string, marketType int64) *interfaces.OHLCV {
	panic("implement me")
	return nil
}

func (df *BTDataFeed) AddToFeed(mockedStream []interfaces.OHLCV) {
	df.tickerData = append(df.tickerData, mockedStream...)
}

func (df *BTDataFeed) GetTickerData() []interfaces.OHLCV {
	return df.tickerData
}

func (df *BTDataFeed) GetCurrentTick() int {
	return df.currentTick
}
