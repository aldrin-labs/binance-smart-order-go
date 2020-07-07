package redis

import (
	"context"
	"encoding/json"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"strconv"
	"strings"
	"sync"
)

type RedisLoop struct {
	OhlcvMap sync.Map // <string: exchange+pair+o/h/l/c/v, OHLCV: ohlcv>
	SpreadMap sync.Map
}
var redisLoop *RedisLoop

func InitRedis() interfaces.IDataFeed {
	if redisLoop == nil {
		redisLoop = &RedisLoop{}
		redisLoop.SubscribeToPairs()
	}

	return redisLoop
}

func (rl *RedisLoop) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	if redisLoop == nil {
		redisLoop = &RedisLoop{}
		redisLoop.SubscribeToPairs()
	}
	return redisLoop.GetPrice(pair, exchange, marketType)
}

func (rl *RedisLoop) GetSpreadForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.SpreadData {
	if redisLoop == nil {
		redisLoop = &RedisLoop{}
		redisLoop.SubscribeToPairs()
	}
	return redisLoop.GetSpread(pair, exchange, marketType)
}

type OrderbookOHLCV struct {
	Open float64 `json:"open_price,float"`
	High float64 `json:"high_price,float"`
	Low float64 `json:"low_price,float"`
	MarketType int64 `json:"market_type,float"`
	Close float64 `json:"close_price,float"`
	Volume float64 `json:"volume,float"`
	Base string `json:"tsym"`
	Quote string `json:"fsym"`
	Exchange string `json:"exchange"`
}

type Spread struct {
	BestBidPrice string `json:"bestBidPrice"`
	BestAskPrice string `json:"bestAskPrice"`
	Exchange     string  `json:"exchange"`
	Symbol       string  `json:"symbol"`
	MarketType   int64  `json:"marketType"`
}

func (rl *RedisLoop) SubscribeToPairs() {
	go ListenPubSubChannels(context.TODO(), func() error {
		return nil
	}, func(channel string, data []byte) error {
		if strings.Contains(channel, "best") {
			go rl.UpdateSpread(channel, data)
			return nil
		}
		go rl.UpdateOHLCV(channel, data)
		return nil
	}, "*:60", "best:*:*:*")
}

func (rl *RedisLoop) UpdateOHLCV(channel string, data []byte) {
	var ohlcvOB OrderbookOHLCV
	_ = json.Unmarshal(data, &ohlcvOB)
	pair := ohlcvOB.Quote+"_"+ohlcvOB.Base
	exchange := "binance"
	ohlcv := interfaces.OHLCV{
		Open:   ohlcvOB.Open,
		High:   ohlcvOB.High,
		Low:    ohlcvOB.Low,
		Close:  ohlcvOB.Close,
		Volume: ohlcvOB.Volume,
	}
	rl.OhlcvMap.Store(exchange+pair+strconv.FormatInt(ohlcvOB.MarketType, 10), ohlcv)

}
func (rl *RedisLoop) FillPair(pair, exchange string) *interfaces.OHLCV {
	redisClient := GetRedisClientInstance(false, true, false)
	baseStr := pair + ":0:" + exchange + ":60:"
	ohlcvResultArr, _ := redisClient.Do("GET", baseStr+"o", baseStr+"h", baseStr+"l", baseStr+"c", baseStr+"v")

	responseArr := ohlcvResultArr.([]interface{})
	for _, value := range responseArr {
		println(value)
	}
	return nil
}

func (rl *RedisLoop) GetPrice(pair, exchange string, marketType int64) *interfaces.OHLCV {
	ohlcvRaw, ob := rl.OhlcvMap.Load(exchange+pair+strconv.FormatInt(marketType, 10))
	if ob == true {
		ohlcv := ohlcvRaw.(interfaces.OHLCV)
		return &ohlcv
	}
	return nil
}

func (rl *RedisLoop) SubscribeToSpread() {
	go ListenPubSubChannels(context.TODO(), func() error {
		return nil
	}, func(channel string, data []byte) error {
		go rl.UpdateSpread(channel, data)
		return nil
	}, "best:*:*:*")
}

func (rl *RedisLoop) UpdateSpread(channel string, data []byte) {
	var spread Spread
	err := json.Unmarshal(data, &spread)

	if err != nil {
		println("spread json err", err)
	}

	bid, _ := strconv.ParseFloat(spread.BestBidPrice, 64)
	ask, _ := strconv.ParseFloat(spread.BestAskPrice, 64)

	spreadData := interfaces.SpreadData{
		Close: bid,
		BestBid: bid,
		BestAsk: ask,
	}

	arr := strings.Split(channel, ":")
	exchange := arr[1]
	pair := arr[2]
	marketType := arr[3]

	str := exchange+pair+marketType
	rl.SpreadMap.Store(str, spreadData)
}

func (rl *RedisLoop) GetSpread(pair, exchange string, marketType int64) *interfaces.SpreadData {
	spreadRaw, ok := rl.SpreadMap.Load(exchange+pair+strconv.FormatInt(marketType, 10))
	if ok == true {
		spread := spreadRaw.(interfaces.SpreadData)
		return &spread
	}
	return nil
}
