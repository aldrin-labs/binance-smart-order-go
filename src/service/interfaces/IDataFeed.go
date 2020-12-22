package interfaces


type IDataFeed interface {
	GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *OHLCV
	GetSpreadForPairAtExchange(pair string, exchange string, marketType int64) *SpreadData
}