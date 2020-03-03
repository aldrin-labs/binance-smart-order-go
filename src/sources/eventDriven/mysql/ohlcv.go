package mysql

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
)

// OhlcvFromDB wrapper for OHLCV with additional MySQL data
// type OhlcvFromDB struct {
// 	Ohlcv     interfaces.OHLCV
// 	Timestamp int
// }

// GetOhlcv - returns OHLCV records from MySQL Database
func (sc *SQLConn) GetOhlcv(exchange string, base string, quote string, period int, marketType int, fromTs int, toTs int) []interfaces.OHLCV {

	ohlcvTableName := os.Getenv("SQL_TABLE_NAME")
	sqlRequest := fmt.Sprintf(`
		SELECT open_price, high_price, low_price, close_price, volume
		FROM %s 
		WHERE fsym = ? and tsym = ? and
		period = ? and market_type = ? and exchange = ?
		and record_date > ? and record_date < ?
		order by record_date asc
	`, ohlcvTableName)
	stmtOut, err := sc.db.Prepare(sqlRequest)
	if err != nil {
		panic(err.Error())
	}

	ohlcvs := []interfaces.OHLCV{}
	var open, high, low, close, volume float64

	rows, err := stmtOut.Query(base, quote, period, marketType, exchange, fromTs, toTs)
	if err != nil {
		log.Println("Error while querying data from MySql")
		panic(err.Error())
	}

	for rows.Next() {
		err = rows.Scan(&open, &high, &low, &close, &volume)
		if err != nil {
			log.Println("Error while scaning data from MySql")
			panic(err.Error())
		}
		ohlcvs = append(ohlcvs, interfaces.OHLCV{Open: open, High: high, Low: low, Close: close, Volume: volume})
	}

	return ohlcvs
}
