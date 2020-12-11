package makeronly_order

import (
	"context"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"log"
	"time"
)

func (mo *MakerOnlyOrder) waitForOrder(orderId string, orderStatus string) {
	//println("in wait for order")
	mo.StatusByOrderId.Store(orderId, orderStatus)
	_ = mo.StateMgmt.SubscribeToOrder(orderId, mo.orderCallback)
}

func (mo *MakerOnlyOrder) orderCallback(order *models.MongoOrder) {
	ctx := context.TODO()
	log.Println("order callback")
	if order == nil || order.OrderId == "" || !(order.Status == "filled" || order.Status == "canceled") {
		return
	}
	mo.OrdersMux.Lock()
	if _, ok := mo.OrdersMap[order.OrderId]; ok {
		delete(mo.OrdersMap, order.OrderId)
	} else {
		mo.OrdersMux.Unlock()
		return
	}
	mo.OrdersMux.Unlock()
	if order.Status == "filled" {
		log.Println("in waitOrder")
		state := mo.Strategy.GetModel().State
		state.EntryPrice = order.Average
		state.ExecutedAmount = order.Filled
		go mo.StateMgmt.UpdateEntryPrice(mo.Strategy.GetModel().ID, state)
		go mo.StateMgmt.UpdateExecutedAmount(mo.Strategy.GetModel().ID, state)

		go func() {
			for {
				if mo.MakerOnlyOrder != nil {
					mo.MakerOnlyOrder.Average = order.Average
					mo.MakerOnlyOrder.Filled = order.Filled
					mo.MakerOnlyOrder.Status = order.Status
					go mo.StateMgmt.SaveOrder(*mo.MakerOnlyOrder, mo.KeyId, mo.Strategy.GetModel().Conditions.MarketType)
					break
				} else {
					time.Sleep(300 * time.Millisecond)
					continue
				}
			}
		}()

		err := mo.State.Fire(CheckExistingOrders)
		mo.enterFilled(ctx)
		if err != nil {
			log.Println("waitOrder err ", err.Error())
		}
	}
}
