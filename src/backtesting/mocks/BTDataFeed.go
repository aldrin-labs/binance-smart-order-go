package backtest

import (
	"log"

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
	pageSize    int
	info        HistoricalParams
	sqlConn     mysql.SQLConn // maybe should move it to sqlConn singleton
}

func NewBTDataFeed(params HistoricalParams) *BTDataFeed {
	pageSize := 1000

	// connect to MySql
	sqlConn := mysql.SQLConn{}
	sqlConn.Initialize()

	ohlcvsFromMysql := sqlConn.GetOhlcvPaged(params.Exchange, params.Base, params.Quote, params.OhlcvPeriod,
		params.MarketType, params.IntervalStart, params.IntervalEnd, 0, pageSize)

	dataFeed := BTDataFeed{
		tickerData:  ohlcvsFromMysql,
		currentTick: -1,
		pageSize:    pageSize,
		info:        params,
		sqlConn:     sqlConn,
	}

	return &dataFeed
}

func (df *BTDataFeed) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	df.currentTick += 1
	dataLength := len(df.tickerData)
	if df.currentTick >= dataLength {
		// get new portion of OHLCVs
		log.Printf("Fetching more OHLCVs...")

		offset := dataLength
		ohlcvsFromMysql := df.sqlConn.GetOhlcvPaged(df.info.Exchange, df.info.Base, df.info.Quote, df.info.OhlcvPeriod,
			df.info.MarketType, df.info.IntervalStart, df.info.IntervalEnd, offset, df.pageSize)

		// check that we can fetch more OHLCV
		if len(ohlcvsFromMysql) > 0 {
			df.AddToFeed(ohlcvsFromMysql)
		} else {
			// return last available OHLCV datapoint
			return &df.tickerData[df.currentTick-1]
		}
	}

	log.Println("Current tick ", df.currentTick)
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
