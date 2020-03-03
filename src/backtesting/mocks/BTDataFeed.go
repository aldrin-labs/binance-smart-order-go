package backtest

import (
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/eventDriven/mysql"
)

type BTDataFeed struct {
	tickerData  []interfaces.OHLCV
	currentTick int
}

func NewBTDataFeed(exchange string, base string, quote string, period int, marketType int, fromTs int, toTs int) *BTDataFeed {

	// connect to MySql
	sqlConn := mysql.SQLConn{}
	sqlConn.Initialize()

	ohlcvsFromMysql := sqlConn.GetOhlcv(exchange, base, quote, period, marketType, fromTs, toTs)

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
