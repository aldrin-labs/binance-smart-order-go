package websocket

import (
"context"
"encoding/json"
"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
"log"
"strconv"
"strings"
"sync"
)

type WebsocketLoop struct {
	OhlcvMap  sync.Map // <string: exchange+pair+o/h/l/c/v, OHLCV: ohlcv>
	SpreadMap sync.Map
}

var websocketLoop *WebsocketLoop

func InitWebsocket() interfaces.IDataFeed {
	// here we get interface
	return websocketLoop
}

func (rl *WebsocketLoop) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	if websocketLoop == nil {
		websocketLoop = &WebsocketLoop{}
		websocketLoop.SubscribeToPairs()
	}
	return websocketLoop.GetPrice(pair, exchange, marketType)
}

func (rl *WebsocketLoop) GetSpreadForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.SpreadData {
	if websocketLoop == nil {
		websocketLoop = &WebsocketLoop{}
		websocketLoop.SubscribeToPairs()
	}
	return websocketLoop.GetSpread(pair, exchange, marketType)
}

type OrderbookOHLCV struct {
	Open       float64 `json:"open_price,float"`
	High       float64 `json:"high_price,float"`
	Low        float64 `json:"low_price,float"`
	MarketType int64   `json:"market_type,float"`
	Close      float64 `json:"close_price,float"`
	Volume     float64 `json:"volume,float"`
	Base       string  `json:"tsym"`
	Quote      string  `json:"fsym"`
	Exchange   string  `json:"exchange"`
}

// {"id":41082715216,"exchange":"binance","symbol":"ALGO_USDT","bestBidPrice":"0.2800","bestBidQuantity":"1368.2","bestAskPrice":"0.2801","bestAskQuantity":"3.3","marketType":1}
type Spread struct {
	BestBidPrice float64 `json:"bestBidPrice,float"`
	BestAskPrice float64 `json:"bestAskPrice,float"`
	Exchange     string  `json:"exchange"`
	Symbol       string  `json:"symbol"`
	MarketType   int64   `json:"marketType"`
}

func (rl *WebsocketLoop) SubscribeToPairs() {
	// subscribe to ohlcv

	//go ListenPubSubChannels(context.TODO(), func() error {
	//	return nil
	//}, func(channel string, data []byte) error {
	//	if strings.Contains(channel, "best") {
	//		go rl.UpdateSpread(channel, data)
	//		return nil
	//	}
	//	go rl.UpdateOHLCV(channel, data)
	//	return nil
	//}, "*:60")
	//rl.SubscribeToSpread()
}

func (rl *WebsocketLoop) UpdateOHLCV(channel string, data []byte) {
	// update in sync.map value

	//var ohlcvOB OrderbookOHLCV
	//_ = json.Unmarshal(data, &ohlcvOB)
	//pair := ohlcvOB.Quote + "_" + ohlcvOB.Base
	//exchange := "binance"
	//ohlcv := interfaces.OHLCV{
	//	Open:   ohlcvOB.Open,
	//	High:   ohlcvOB.High,
	//	Low:    ohlcvOB.Low,
	//	Close:  ohlcvOB.Close,
	//	Volume: ohlcvOB.Volume,
	//}
	//rl.OhlcvMap.Store(exchange+pair+strconv.FormatInt(ohlcvOB.MarketType, 10), ohlcv)
}

func (rl *WebsocketLoop) GetPrice(pair, exchange string, marketType int64) *interfaces.OHLCV {
	// get price from sync.map

	//ohlcvRaw, ob := rl.OhlcvMap.Load(exchange + pair + strconv.FormatInt(marketType, 10))
	//if ob == true {
	//	ohlcv := ohlcvRaw.(interfaces.OHLCV)
	//	return &ohlcv
	//}
	//return nil
}

func (rl *WebsocketLoop) SubscribeToSpread() {
	// subscribe to spread

	//go ListenPubSubChannels(context.TODO(), func() error {
	//	return nil
	//}, func(channel string, data []byte) error {
	//	go rl.UpdateSpread(channel, data)
	//	return nil
	//}, "best:*:*:*")
}

func (rl *WebsocketLoop) UpdateSpread(channel string, data []byte) {
	// update spread value in sync.map

	//var spread Spread
	//tryparse := json.Unmarshal(data, &spread)
	//if tryparse != nil {
	//	log.Print(tryparse)
	//}
	//spreadData := interfaces.SpreadData{
	//	Close:   spread.BestBidPrice,
	//	BestBid: spread.BestBidPrice,
	//	BestAsk: spread.BestAskPrice,
	//}
	//
	//rl.SpreadMap.Store(spread.Exchange+spread.Symbol+strconv.FormatInt(spread.MarketType, 10), spreadData)
}

func (rl *WebsocketLoop) GetSpread(pair, exchange string, marketType int64) *interfaces.SpreadData {
	// get spread

	//spreadRaw, ok := rl.SpreadMap.Load(exchange + pair + strconv.FormatInt(marketType, 10))
	////log.Println("spreadRaw ", spreadRaw)
	//if ok == true {
	//	spread := spreadRaw.(interfaces.SpreadData)
	//	return &spread
	//}
	//return nil
}
