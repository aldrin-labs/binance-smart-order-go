package postonly_order


func(po *PostOnlyOrder) getBestAskOrBidPrice() float64 {
	pair := po.Strategy.GetModel().Conditions.Pair
	marketType := po.Strategy.GetModel().Conditions.MarketType
	exchange := "binance"
	spread := po.DataFeed.GetSpreadForPairAtExchange(pair, exchange, marketType)
	if po.Strategy.GetModel().Conditions.EntryOrder.Side == "sell" {
		return spread.BestAsk
	} else {
		return spread.BestBid
	}
}