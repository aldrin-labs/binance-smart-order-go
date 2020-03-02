package backtest

import (
	"testing"

	"github.com/joho/godotenv"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/eventDriven/mysql"
)

func TestBacktest(t *testing.T) {

	err := godotenv.Load("../../.env")
	if err != nil {
		println("Error loading .env file")
	}

	sqlConn := mysql.SQLConn{}
	sqlConn.Initialize()

	sqlConn.GetOhlcv("binance", "BTC", "USDT", 86400, 1, 1578405936, 1578476936)
}
