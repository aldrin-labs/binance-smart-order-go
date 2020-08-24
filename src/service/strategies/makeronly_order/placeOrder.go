package makeronly_order

import (
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
)

func (mo *MakerOnlyOrder) PlaceOrder(anything float64, step string){
	model := mo.Strategy.GetModel()
	if model.State.EntryOrderId != "" {
		response := mo.ExchangeApi.CancelOrder(trading.CancelOrderRequest{
			KeyId: mo.KeyId,
			KeyParams: trading.CancelOrderRequestParams{
				OrderId:    model.State.EntryOrderId,
				MarketType: model.Conditions.MarketType,
				Pair:       model.Conditions.Pair,
			},
		})
		if response.Data.Id == "" {
			// order was executed should be processed in other thread
			return
		}
		// we canceled prev order now time to place new one
	}
	orderId := ""
	for orderId == "" {
		price := mo.getBestAskOrBidPrice()
		positionSide := ""
		if model.Conditions.MarketType == 1 {
			if model.Conditions.EntryOrder.Side == "sell" && model.Conditions.EntryOrder.ReduceOnly == false || model.Conditions.EntryOrder.Side == "buy" && model.Conditions.EntryOrder.ReduceOnly == true {
				positionSide = "SHORT"
			} else {
				positionSide = "LONG"
			}
		}
		postOnly := true
		order := trading.Order{
			Price: price,
			Amount: model.Conditions.EntryOrder.Amount,
			PostOnly: &postOnly,
			Symbol: model.Conditions.Pair,
			MarketType: model.Conditions.MarketType,
			ReduceOnly: &model.Conditions.EntryOrder.ReduceOnly,
			PositionSide: positionSide,
			Type: "limit",
		}
		if model.Conditions.MarketType == 1 {
			order.TimeInForce = "GTC"
		}
		response := mo.ExchangeApi.CreateOrder(trading.CreateOrderRequest{
			KeyId:     model.AccountId,
			KeyParams: order,
		})
		orderId = response.Data.Id
	}
	model.State.EntryOrderId = orderId
	mo.StateMgmt.UpdateState(model.ID, model.State)
}