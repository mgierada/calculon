// Package db stores and queries the normalized records from internal/model in
// SQLite.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

// timeLayout is how timestamps are stored, chosen so text ordering matches
// chronological ordering.
const timeLayout = time.RFC3339

// Conn is a handle to the portfolio database.
type Conn = sql.DB

// Open connects to the SQLite database at dbPath and applies the schema.
func Open(dbPath string) (*Conn, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %q: %w", dbPath, err)
	}
	if err := Migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Migrate creates any missing tables and indexes.
func Migrate(conn *sql.DB) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read embedded schema: %w", err)
	}
	if _, err := conn.Exec(string(schema)); err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}
	return nil
}

// formatTime renders a timestamp for storage.
func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

// nullTime renders a timestamp for storage, or NULL when it is the zero time.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return formatTime(t)
}

// parseTime reads a stored timestamp, treating NULL as the zero time.
func parseTime(stored sql.NullString) (time.Time, error) {
	if !stored.Valid || stored.String == "" {
		return time.Time{}, nil
	}
	return time.Parse(timeLayout, stored.String)
}
