package makeronly_order

func(po *MakerOnlyOrder) run() {
	//pair := po.Strategy.GetModel().Conditions.Pair
	//marketType := po.Strategy.GetModel().Conditions.MarketType
	//exchange := "binance"
	po.OrderParams.Price = po.getBestAskOrBidPrice()

	//response := po.ExchangeApi.CreateOrder(trading.CreateOrderRequest{
	//	KeyId:     po.KeyId,
	//	KeyParams: po.OrderParams,
	//})

	//if response.Data.Id == "" {
	//	println("ERROR", response.Data.Msg)
	//	// println(response)
	//}
}
