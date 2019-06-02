package trading

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)
func Request (method string, data interface{}) interface{} {
	url := "http://"+ os.Getenv("EXCHANGESERVCE") +"/" + method
	fmt.Println("URL:>", url)

	var jsonStr, err = json.Marshal(data)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))
	return body
}


/*
{
	"keyId": "5ca48f82744e09001ac430d5",
	"keyParams": {
    "symbol": "BTC/USDT",
    "type": "limit",
    "side": "buy",
    "amount": 0.026,
    "price": 90
	}
}
*/

type OrderParams struct {
	StopPrice float64
	Type string
}

type KeyParams struct {
	Symbol string
	Type string
	Side string
	Amount float64
	Price float64
	Params OrderParams
}

type CreateOrderRequest struct {
	KeyId string
	KeyParams KeyParams
}

func CreateOrder(order CreateOrderRequest) interface{} {
	return Request("createOrder", order)
}


func CancelOrder() {

}

