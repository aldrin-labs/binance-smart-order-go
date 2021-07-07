package makeronly_order

import (
	"gitlab.com/crypto_project/core/strategy_service/src/trading/orders"
	"log"
)

func (po *MakerOnlyOrder) run() {
	//pair := po.Strategy.GetModel().Conditions.Pair
	//marketType := po.Strategy.GetModel().Conditions.MarketType
	//exchange := "binance"
	log.Println("run maker-only")
	price, err := po.getBestAskOrBidPrice()

	if err == nil || po.MakerOnlyOrder == nil {
		return
	}

	po.OrderParams.Price = price
	response := po.ExchangeApi.CreateOrder(orders.CreateOrderRequest{
		KeyId:     po.KeyId,
		KeyParams: po.OrderParams,
	})

	if response.Data.OrderId == "" {
		log.Print("ERROR", response.Data.Msg)
		// log.Print(response)
	}
}
