package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/glebarez/go-sqlite"
)

// PrintVersion opens the SQLite database at dbPath and prints its engine version.
func PrintVersion(dbPath string) error {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("connected to SQLite database at %s", dbPath)

	var sqliteVersion string
	if err := conn.QueryRow("select sqlite_version()").Scan(&sqliteVersion); err != nil {
		return err
	}

	fmt.Println(sqliteVersion)
	return nil
}
