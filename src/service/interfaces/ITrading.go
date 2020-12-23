package interfaces

type ITrading interface {
	CreateOrder(exchange string, pair string, price float64, amount float64, side string) string
}
