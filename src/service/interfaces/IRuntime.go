package interfaces


type IStrategyRuntime interface {
	Stop()
	Start()
	PlaceOrder(price float64, step string)
	TryCancelAllOrders(orderIds []string)
	TryCancelAllOrdersConsistently(orderIds []string)
	SetSelectedExitTarget(selectedExitTarget int)
	IsOrderExistsInMap(orderId string) bool
}