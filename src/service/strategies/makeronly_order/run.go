package makeronly_order

import (
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"log"
)

func(po *MakerOnlyOrder) run() {
	//pair := po.Strategy.GetModel().Conditions.Pair
	//marketType := po.Strategy.GetModel().Conditions.MarketType
	//exchange := "binance"
	po.OrderParams.Price = po.getBestAskOrBidPrice()

	response := po.ExchangeApi.CreateOrder(trading.CreateOrderRequest{
		KeyId:     po.KeyId,
		KeyParams: po.OrderParams,
	})

	if response.Data.OrderId == "" {
		log.Print("ERROR", response.Data.Msg)
		// log.Print(response)
	}
}
