package testing

import (
	"github.com/joho/godotenv"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/redis"
	"log"
	"testing"
	"time"
)

func TestGetPriceFromRedis(t *testing.T) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	redis.InitRedis()
	time.Sleep(1 * time.Second)
	ohlcv := redis.GetPrice("BTC_USDT", "binance")
	time.Sleep(800 * time.Second)
	if ohlcv == nil || ohlcv.Close == 0  {
		t.Error("no OHLCV received or empty")
	}
}