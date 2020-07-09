package postonly_order

import "gitlab.com/crypto_project/core/strategy_service/src/trading"

func(po *PostOnlyOrder) run() {
	pair := po.Strategy.GetModel().Conditions.Pair
	marketType := po.Strategy.GetModel().Conditions.MarketType
	exchange := "binance"
	po.OrderParams.Price = po.getBestAskOrBidPrice()

	response := po.ExchangeApi.CreateOrder(trading.CreateOrderRequest{
		KeyId:     po.KeyId,
		KeyParams: po.OrderParams,
	})

	if response.Data.Id == "" {
		println("ERROR", response.Data.Msg)
		println(response)
	}
}
