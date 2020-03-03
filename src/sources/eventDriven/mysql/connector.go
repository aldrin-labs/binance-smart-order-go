package mysql

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

// SQLConn - SQL connection structure
type SQLConn struct {
	db *sql.DB
}

// Initialize - establishes connection to MySQL server if not yet established
func (sc *SQLConn) Initialize() {
	if sc.db != nil {
		return
	}

	log.Println("Connecting to MySQL...")
	db, _ := sql.Open("mysql", os.Getenv("SQL_CONN_STRING"))

	// Open doesn't open a connection. Validate connection:
	err := db.Ping()
	if err != nil {
		log.Println("Error while connecting to MYSQL")
		panic(err.Error())
	}

	sc.db = db
}

// Close - closes MySQL connection
func (sc *SQLConn) Close() {
	sc.db.Close()
	sc.db = nil
}
