package makeronly_order

import "errors"

func (po *MakerOnlyOrder) getBestAskOrBidPrice() (float64, error) {
	pair := po.Strategy.GetModel().Conditions.Pair
	marketType := po.Strategy.GetModel().Conditions.MarketType
	exchange := "binance"
	spread := po.DataFeed.GetSpreadForPairAtExchange(pair, exchange, marketType)
	if spread == nil {
		return 0.0, errors.New("nil spread")
	} else if po.Strategy.GetModel().Conditions.EntryOrder.Side == "sell" {
		return spread.BestAsk, nil
	} else {
		return spread.BestBid, nil
	}
}
