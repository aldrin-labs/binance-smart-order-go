package binance

import (
	"encoding/json"
	"github.com/Cryptocurrencies-AI/go-binance"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"sync"
)

type BinanceLoop struct {
	OhlcvMap  sync.Map // <string: exchange+pair+o/h/l/c/v, OHLCV: ohlcv>
	SpreadMap sync.Map
}

var binanceLoop *BinanceLoop
var log *zap.Logger

func init() {
	log, _ = zap.NewProduction()
}

func InitBinance() interfaces.IDataFeed {
	if binanceLoop == nil {
		binanceLoop = &BinanceLoop{}
		binanceLoop.SubscribeToPairs()
	}

	return binanceLoop
}

func (rl *BinanceLoop) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	if binanceLoop == nil {
		binanceLoop = &BinanceLoop{}
		binanceLoop.SubscribeToPairs()
	}
	return binanceLoop.GetPrice(pair, exchange, marketType)
}

func (rl *BinanceLoop) GetSpreadForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.SpreadData {
	if binanceLoop == nil {
		binanceLoop = &BinanceLoop{}
		binanceLoop.SubscribeToPairs()
	}
	return binanceLoop.GetSpread(pair, exchange, marketType)
}

type RawOrderbookOHLCV []struct {
	//Open       float64 `json:"open_price,float"`
	//High       float64 `json:"high_price,float"`
	//Low        float64 `json:"low_price,float"`
	//MarketType int64   `json:"market_type,float"`
	//Close      float64 `json:"close_price,float"`
	//Volume     float64 `json:"volume,float"`
	//Base       string  `json:"tsym"`
	//Quote      string  `json:"fsym"`
	//Exchange   string  `json:"exchange"`
	Symbol string `json:"s"`
	Close  string `json:"p"`
}

type RawSpread struct {
	BestBidPrice float64 `json:"b,string"`
	BestAskPrice float64 `json:"a,string"`
	BestBidQty   float64 `json:"B,string"`
	BestAskQty   float64 `json:"A,string"`
	Symbol       string  `json:"s"`
}

func (rl *BinanceLoop) SubscribeToPairs() {
	go ListenBinanceMarkPrice(func(data *binance.MarkPriceAllStrEvent) error {
		go rl.UpdateOHLCV(data.Data)
		return nil
	})
	rl.SubscribeToSpread()
}

func (rl *BinanceLoop) UpdateOHLCV(data []byte) {
	var ohlcvOB RawOrderbookOHLCV
	_ = json.Unmarshal(data, &ohlcvOB)

	exchange := "binance"
	marketType := 1

	for _, ohlcv := range ohlcvOB {
		pair := ohlcv.Symbol
		price, _ := strconv.ParseFloat(ohlcv.Close, 10)
		ohlcvToSave := interfaces.OHLCV{
			Open:   price,
			High:   price,
			Low:    price,
			Close:  price,
			Volume: price,
		}
		rl.OhlcvMap.Store(exchange+pair+strconv.FormatInt(int64(marketType), 10), ohlcvToSave)
	}
}

func (rl *BinanceLoop) GetPrice(pair, exchange string, marketType int64) *interfaces.OHLCV {
	ohlcvRaw, ob := rl.OhlcvMap.Load(exchange + strings.Replace(pair, "_", "", -1) + strconv.FormatInt(marketType, 10))
	if ob == true {
		ohlcv := ohlcvRaw.(interfaces.OHLCV)
		return &ohlcv
	}
	return nil
}

func (rl *BinanceLoop) SubscribeToSpread() {
	go ListenBinanceSpread(func(data *binance.SpreadAllEvent) error {
		go rl.UpdateSpread(data.Data)
		return nil
	})
}

func (rl *BinanceLoop) UpdateSpread(data []byte) {
	var spread RawSpread
	tryparse := json.Unmarshal(data, &spread)
	if tryparse != nil {
		log.Info("can't parse spread data",
			zap.String("err", tryparse.Error()),
		)
	}

	exchange := "binance"
	marketType := 1

	spreadData := interfaces.SpreadData{
		Close:   spread.BestBidPrice,
		BestBid: spread.BestBidPrice,
		BestAsk: spread.BestAskPrice,
	}

	rl.SpreadMap.Store(exchange+spread.Symbol+strconv.FormatInt(int64(marketType), 10), spreadData)
}

func (rl *BinanceLoop) GetSpread(pair, exchange string, marketType int64) *interfaces.SpreadData {
	spreadRaw, ok := rl.SpreadMap.Load(exchange + strings.Replace(pair, "_", "", -1) + strconv.FormatInt(marketType, 10))
	//log.Println("spreadRaw ", spreadRaw)
	if ok == true {
		spread := spreadRaw.(interfaces.SpreadData)
		return &spread
	}
	return nil
}
