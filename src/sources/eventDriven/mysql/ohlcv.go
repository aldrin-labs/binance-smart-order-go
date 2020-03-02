package mysql

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
)

// OhlcvFromDB wrapper for OHLCV with additional MySQL data
type OhlcvFromDB struct {
	Ohlcv     interfaces.OHLCV
	Timestamp int
}

// GetOhlcv - returns OHLCV records from MySQL Database
func (sc *SQLConn) GetOhlcv(exchange string, base string, quote string, period int, marketType int, fromTs int, toTs int) OhlcvFromDB {

	ohlcvTableName := os.Getenv("SQL_TABLE_NAME")
	sqlRequest := fmt.Sprintf(`
		SELECT close_price
		FROM %s 
		WHERE fsym = ? and tsym = ? and
		period = ? and market_type = ? and exchange = ?
		and record_date > ? and record_date < ?
	`, ohlcvTableName)
	stmtOut, err := sc.db.Prepare(sqlRequest)
	if err != nil {
		panic(err.Error())
	}

	var close float64

	err = stmtOut.QueryRow(base, quote, period, marketType, exchange, fromTs, toTs).Scan(&close)
	if err != nil {
		panic(err.Error())
	}

	log.Println("GOT CLOSE:", close)

	return OhlcvFromDB{}
}
