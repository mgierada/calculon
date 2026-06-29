package db

import (
	"database/sql"
	"fmt"
	_ "github.com/glebarez/go-sqlite"
)

// PrintVersion opens the local SQLite database and prints its engine version.
func PrintVersion() error {
	conn, err := sql.Open("sqlite", "./db.go")
	if err != nil {
		return err
	}
	defer conn.Close()
	fmt.Println("Connected to the SQLite database successfully.")

	var sqliteVersion string
	if err := conn.QueryRow("select sqlite_version()").Scan(&sqliteVersion); err != nil {
		return err
	}

	fmt.Println(sqliteVersion)
	return nil
}
