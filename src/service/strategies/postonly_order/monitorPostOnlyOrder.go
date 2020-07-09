package postonly_order

import "time"

func(po *PostOnlyOrder) monitorPostOnlyOrder() {
	monitorFrequency := po.OrderParams.Frequency
	if monitorFrequency == 0 {
		monitorFrequency = 1000.0
	}
	prevPrice := po.OrderParams.Price
	for {
		bestPrice := po.getBestAskOrBidPrice()
		if bestPrice == prevPrice {
			time.Sleep(time.Duration(monitorFrequency))
			continue
		}
		// в канселе мы так же записываем в монгу последний зафилленый ордер

		orderStatus := po.cancelPreviousPostOnlyOrder()
	}
}