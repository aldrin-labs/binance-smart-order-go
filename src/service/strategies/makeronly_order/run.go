package makeronly_order

import (
	loggly_client "gitlab.com/crypto_project/core/strategy_service/src/sources/loggy"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
)

func (po *MakerOnlyOrder) run() {
	//pair := po.Strategy.GetModel().Conditions.Pair
	//marketType := po.Strategy.GetModel().Conditions.MarketType
	//exchange := "binance"
	loggly_client.GetInstance().Info("run maker-only")
	price, err := po.getBestAskOrBidPrice()

	if err == nil || po.MakerOnlyOrder == nil {
		return
	}

	po.OrderParams.Price = price
	response := po.ExchangeApi.CreateOrder(trading.CreateOrderRequest{
		KeyId:     po.KeyId,
		KeyParams: po.OrderParams,
	})

	if response.Data.OrderId == "" {
		loggly_client.GetInstance().Info("ERROR", response.Data.Msg)
		// loggly_client.GetInstance().Info(response)
	}
}
