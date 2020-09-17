package makeronly_order

import (
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"log"
	"strings"
	"time"
)

func (mo *MakerOnlyOrder) PlaceOrder(anything float64, step string){
	log.Println("place order")
	model := mo.Strategy.GetModel()
	attemptsToPlaceOrder := 0
	mo.CancelEntryOrder()
	orderId := ""
	for orderId == "" {
		price, err := mo.getBestAskOrBidPrice()
		if err != nil || mo.MakerOnlyOrder == nil || mo.MakerOnlyOrder.Status == "filled" {
			return
		}
		positionSide := ""
		if model.Conditions.MarketType == 1  {
			if model.Conditions.EntryOrder.Side == "sell" && model.Conditions.EntryOrder.ReduceOnly == false || model.Conditions.EntryOrder.Side == "buy" && model.Conditions.EntryOrder.ReduceOnly == true {
				positionSide = "SHORT"
			} else {
				positionSide = "LONG"
			}
		}
		postOnly := true
		order := trading.Order{
			Side: model.Conditions.EntryOrder.Side,
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
			order.TimeInForce = "GTX"
		}
		response := mo.ExchangeApi.CreateOrder(trading.CreateOrderRequest{
			KeyId:     model.AccountId,
			KeyParams: order,
		})

		orderId = response.Data.OrderId
		if orderId != "" {
			mo.OrdersMux.Lock()
			mo.OrdersMap[response.Data.OrderId] = true
			mo.OrdersMux.Unlock()
			break
		}
		if len(response.Data.Msg) > 0 {
			if attemptsToPlaceOrder < 1 && strings.Contains(response.Data.Msg, "Key is processing") {
				attemptsToPlaceOrder += 1
				time.Sleep(time.Minute * 1)
				continue
			}
			if len(response.Data.Msg) > 0 && attemptsToPlaceOrder < 3 && strings.Contains(response.Data.Msg, "position side does not match") {
				attemptsToPlaceOrder += 1
				time.Sleep(time.Second * 5)
				continue
			}
			if len(response.Data.Msg) > 0 && attemptsToPlaceOrder < 3 && strings.Contains(response.Data.Msg, "invalid json") {
				attemptsToPlaceOrder += 1
				time.Sleep(2 * time.Second)
				continue
			}
			if len(response.Data.Msg) > 0 && strings.Contains(response.Data.Msg, "ReduceOnly Order is rejected") {
				model.Enabled = false
				go mo.StateMgmt.UpdateState(model.ID, model.State)
				break
			}
			if len(response.Data.Msg) > 0 {
				model.Enabled = false
				model.State.State = Error
				model.State.Msg = response.Data.Msg
				go mo.StateMgmt.UpdateState(model.ID, model.State)
				break
			}
			attemptsToPlaceOrder += 1
		}
	}
	model.State.EntryOrderId = orderId
	go mo.StateMgmt.UpdateState(model.ID, model.State)
	go mo.waitForOrder(orderId, model.State.State)
}