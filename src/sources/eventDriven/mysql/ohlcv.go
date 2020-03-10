package mysql

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/crypto_project/core/strategy_service/src/service/interfaces"
)

// GetOhlcvPaged - returns OHLCV records from MySQL Database by pages with length of AMOUNT starting with OFFSET
func (sc *SQLConn) GetOhlcvPaged(exchange string, base string, quote string, period int, marketType int, fromTs int, toTs int, offset int, amount int) []interfaces.OHLCV {

	ohlcvTableName := os.Getenv("SQL_TABLE_NAME")
	sqlRequest := fmt.Sprintf(`
		SELECT open_price, high_price, low_price, close_price, volume
		FROM %s 
		WHERE fsym = ? and tsym = ? and
		period = ? and market_type = ? and exchange = ?
		and record_date > ? and record_date < ?
		order by record_date asc
		LIMIT ?,?
	`, ohlcvTableName)
	stmtOut, err := sc.db.Prepare(sqlRequest)
	if err != nil {
		panic(err.Error())
	}

	ohlcvs := []interfaces.OHLCV{}
	var open, high, low, close, volume float64

	rows, err := stmtOut.Query(base, quote, period, marketType, exchange, fromTs, toTs, offset, amount)
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

// GetOhlcv - returns OHLCV records from MySQL Database
func (sc *SQLConn) GetOhlcv(exchange string, base string, quote string, period int, marketType int, fromTs int, toTs int) []interfaces.OHLCV {
	return sc.GetOhlcvPaged(exchange, base, quote, period, marketType, fromTs, toTs, 0, 9223372036854775807)
}
