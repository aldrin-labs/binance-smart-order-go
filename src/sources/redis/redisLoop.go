package redis

import (
	"context"
	"encoding/json"
	"gitlab.com/crypto_project/core/strategy_service/src/service/strategies"
	"strconv"
	"sync"
)

type RedisLoop struct {
	OhlcvMap sync.Map // <string: exchange+pair+o/h/l/c/v, OHLCV: ohlcv>
}
var redisLoop *RedisLoop

func InitRedis() strategies.IDataFeed {
	if redisLoop == nil {
		redisLoop = &RedisLoop{}
		redisLoop.SubscribeToPairs()
	}

	return redisLoop
}

func (rl *RedisLoop) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *strategies.OHLCV {
	if redisLoop == nil {
		redisLoop = &RedisLoop{}
		redisLoop.SubscribeToPairs()
	}
	return redisLoop.GetPrice(pair, exchange, marketType)
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

func (rl *RedisLoop) SubscribeToPairs() {
	go ListenPubSubChannels(context.TODO(), func() error {
		return nil
	}, func(channel string, data []byte) error {
		go rl.UpdateOHLCV(channel, data)
		return nil
	}, "*:60")
}

func (rl *RedisLoop) UpdateOHLCV(channel string, data []byte) {
	var ohlcvOB OrderbookOHLCV
	_ = json.Unmarshal(data, &ohlcvOB)
	pair := ohlcvOB.Quote+"_"+ohlcvOB.Base
	exchange := "binance"
	ohlcv := strategies.OHLCV{
		Open:   ohlcvOB.Open,
		High:   ohlcvOB.High,
		Low:    ohlcvOB.Low,
		Close:  ohlcvOB.Close,
		Volume: ohlcvOB.Volume,
	}
	rl.OhlcvMap.Store(exchange+pair+strconv.FormatInt(ohlcvOB.MarketType, 10), ohlcv)

}
func (rl *RedisLoop) FillPair(pair, exchange string) *strategies.OHLCV {
	redisClient := GetRedisClientInstance(false, true, false)
	baseStr := pair + ":0:" + exchange + ":60:"
	ohlcvResultArr, _ := redisClient.Do("GET", baseStr+"o", baseStr+"h", baseStr+"l", baseStr+"c", baseStr+"v")

	responseArr := ohlcvResultArr.([]interface{})
	for _, value := range responseArr {
		println(value)
	}
	return nil
}

func (rl *RedisLoop) GetPrice(pair, exchange string, marketType int64) *strategies.OHLCV  {
	ohlcvRaw, ob := rl.OhlcvMap.Load(exchange+pair+strconv.FormatInt(marketType, 10))
	if ob == true {
		ohlcv := ohlcvRaw.(strategies.OHLCV)
		return &ohlcv
	}
	return nil
}