package makeronly_order

import (
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"log"
)

func (mo *MakerOnlyOrder) waitForOrder(orderId string, orderStatus string) {
	//println("in wait for order")
	mo.StatusByOrderId.Store(orderId, orderStatus)
	_ = mo.StateMgmt.SubscribeToOrder(orderId, mo.orderCallback)
}
func (mo *MakerOnlyOrder) orderCallback(order *models.MongoOrder) {
	if order == nil || (order.OrderId == "" && order.PostOnlyInitialOrderId == "") || !(order.Status == "filled" || order.Status == "canceled")  {
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
		state := mo.Strategy.GetModel().State
		state.EntryPrice = order.Average
		state.ExecutedAmount = order.Filled
		mo.StateMgmt.UpdateEntryPrice(mo.Strategy.GetModel().ID, state)
		mo.StateMgmt.UpdateExecutedAmount(mo.Strategy.GetModel().ID, state)
		log.Println("mo.MakerOnlyOrder", mo.MakerOnlyOrder)
		mo.MakerOnlyOrder.Average = order.Average
		mo.MakerOnlyOrder.Filled = order.Filled
		mo.MakerOnlyOrder.Status = order.Status
		mo.StateMgmt.SaveOrder(*mo.MakerOnlyOrder, mo.KeyId, mo.Strategy.GetModel().Conditions.MarketType)
		mo.State.Fire(CheckExistingOrders)
	}
}