package sources

import (
	"github.com/joho/godotenv"
	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/binance"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/redis"
	"go.uber.org/zap"
	"os"
)

type DataFeed struct {
	binanceLoop interfaces.IDataFeed
	redisLoop interfaces.IDataFeed
}

var dataFeed *DataFeed
var log *zap.Logger

func init() {
	_ = godotenv.Load()
	if os.Getenv("LOCAL") == "true" {
		log, _ = zap.NewDevelopment()
	} else {
		log, _ = zap.NewProduction() // TODO: handle the error
	}
	log = log.With(zap.String("logger", "datafeed"))
}

func InitDataFeed() interfaces.IDataFeed {
	if dataFeed.binanceLoop == nil {
		dataFeed.binanceLoop = binance.InitBinance()
	}

	if dataFeed.redisLoop == nil {
		dataFeed.redisLoop = redis.InitRedis()
	}

	return dataFeed
}

func (df *DataFeed) GetPriceForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.OHLCV {
	switch exchange {
		case "serum": {
			return df.redisLoop.GetPriceForPairAtExchange(pair, exchange, marketType)
		}
		case "binance": {
			return df.binanceLoop.GetPriceForPairAtExchange(pair, exchange, marketType)
		}
		default: {
			log.Error("unknown exchange for getting GetPriceForPairAtExchange")
			return nil
		}
	}
}

func (df *DataFeed) GetSpreadForPairAtExchange(pair string, exchange string, marketType int64) *interfaces.SpreadData {
	switch exchange {
		case "serum": {
			return df.redisLoop.GetSpreadForPairAtExchange(pair, exchange, marketType)
		}
		case "binance": {
			return df.binanceLoop.GetSpreadForPairAtExchange(pair, exchange, marketType)
		}
		default: {
			log.Error("unknown exchange for getting GetPriceForPairAtExchange")
			return nil
		}
	}
}
