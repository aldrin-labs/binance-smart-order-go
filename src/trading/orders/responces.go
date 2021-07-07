package orders

type OrderResponseData struct {
	OrderId string `json:"orderId, string"`
	Msg     string `json:"msg,string"`
	//OrderId string  `json:"orderId, string"`
	Status  string  `json:"status"`
	Type    string  `json:"type"`
	Price   float64 `json:"price"`
	Average float64 `json:"average"`
	Amount  float64 `json:"amount"`
	Filled  float64 `json:"filled"`
	Code    int64   `json:"code" bson:"code"`
}

type OrderResponse struct {
	Status string            `json:"status"`
	Data   OrderResponseData `json:"data"`
}

type UpdateLeverageResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}
