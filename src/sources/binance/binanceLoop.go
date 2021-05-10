package binance

import (
	"encoding/json"
	"github.com/Cryptocurrencies-AI/go-binance"
	"github.com/joho/godotenv"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"go.uber.org/zap"
	"os"
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
	_ = godotenv.Load()
	if os.Getenv("LOCAL") == "true" {
		log, _ = zap.NewDevelopment()
	} else {
		log, _ = zap.NewDevelopment() // TODO: handle the error
	}
	log = log.With(zap.String("logger", "binance"))
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

type MiniTicker struct {
	// EventType string `json:"e,string"` // "24hrMiniTicker"
	// EventTime time.Time `json:"E,number"` // 123456789
	// Symbol string `json:"s,string"` // "BNBBTC"
	// Close float64 `json:"c,string"` // "0.0025"
	Symbol string `json:"s"` // "BNBBTC"
	Close  string `json:"c"` // "0.0025"
	// Open float64 `json:"o,string"` // "0.0010"
	// High float64 `json:"h,string"` // "0.0025"
	// Low float64 `json:"l,string"` // "0.0010"
	// Volume float64 `json:"v,string"` // "10000"
	// Quote float64 `json:"q,string"` // "18"
}

type RawSpread struct {
	BestBidPrice float64 `json:"b,string"`
	BestAskPrice float64 `json:"a,string"`
	BestBidQty   float64 `json:"B,string"`
	BestAskQty   float64 `json:"A,string"`
	Symbol       string  `json:"s"`
}

func (rl *BinanceLoop) SubscribeToPairs() {
	go ListenBinancePrice(func(data *binance.RawEvent, marketType int8) error {
		go rl.UpdateOHLCV(data.Data, marketType)
		return nil
	})
	rl.SubscribeToSpread()
}

// UpdateOHLCV decodes raw OHLCV data and writes them to OHLCVMap for future use.
func (rl *BinanceLoop) UpdateOHLCV(data []byte, marketType int8) {
	var allMarketOHLCV []MiniTicker
	err := json.Unmarshal(data, &allMarketOHLCV)
	if err != nil {
		log.Debug("decode all market mini ticker while OHLCV update",
			zap.Error(err),
		)
		return
	}
	for _, ohlcv := range allMarketOHLCV {
		pair := ohlcv.Symbol
		price, err := strconv.ParseFloat(ohlcv.Close, 10)
		if err != nil {
			log.Debug("parse close price while OHLCV update",
				zap.Error(err),
			)
			continue
		}
		ohlcvToSave := interfaces.OHLCV{
			Open:   price,
			High:   price,
			Low:    price,
			Close:  price,
			Volume: price,
		}
		rl.OhlcvMap.Store("binance"+pair+strconv.FormatInt(int64(marketType), 10), ohlcvToSave)
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
