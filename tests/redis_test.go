package tests

//import (
//	"github.com/joho/godotenv"
//	"gitlab.com/crypto_project/core/strategy_service/src/sources/redis"
//	"log"
//	"testing"
//	"time"
//)

// looks like this is integration test and requires connection to real
// infrastructure. godotenv.Load() requires .env with real values to exist in /tests
// Maybe we should separate this test into separate test suite?
/*func TestGetPriceFromRedis(t *testing.T) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	rl := redis.InitRedis()
	time.Sleep(1 * time.Second)
	ohlcv := rl.GetPrice("BTC_USDT", "binance", 0)
	time.Sleep(800 * time.Second)
	if ohlcv == nil || ohlcv.Close == 0  {
		t.Error("no OHLCV received or empty")
	}
}*/
