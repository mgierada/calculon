// Package db stores and queries the normalized records from internal/model in
// SQLite.
package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

// schemaVersion is stored in SQLite's user_version. Bump it whenever
// schema.sql changes in a way CREATE IF NOT EXISTS cannot apply.
const schemaVersion = 2

// timeLayout is how timestamps are stored, chosen so text ordering matches
// chronological ordering.
const timeLayout = time.RFC3339

// pragmas apply to every pooled connection. WAL and a busy timeout let several
// SSH sessions read while an import writes.
var pragmas = []string{"foreign_keys(1)", "journal_mode(WAL)", "busy_timeout(5000)"}

// ErrIncompatibleSchema means the file was written by an older, incompatible
// version of calculon.
var ErrIncompatibleSchema = errors.New("incompatible database schema")

// Conn is a handle to the portfolio database.
type Conn = sql.DB

// Open connects to the SQLite database at dbPath and applies the schema.
func Open(dbPath string) (*Conn, error) {
	// Check compatibility over a plain connection first: the pragmas below
	// would otherwise switch an old database to WAL before it is refused.
	if err := checkCompatible(dbPath); err != nil {
		return nil, fmt.Errorf("database at %q: %w", dbPath, err)
	}

	query := url.Values{"_pragma": pragmas}
	conn, err := sql.Open("sqlite", dbPath+"?"+query.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %q: %w", dbPath, err)
	}
	if err := Migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("database at %q: %w", dbPath, err)
	}
	return conn, nil
}

// checkCompatible opens the file without side effects and refuses it when it
// holds another schema version.
func checkCompatible(dbPath string) error {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	return compatible(conn)
}

// compatible accepts an empty database or one at the current schema version.
func compatible(conn *sql.DB) error {
	var version int
	if err := conn.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
	}
	if version == schemaVersion {
		return nil
	}
	empty, err := isEmpty(conn)
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("%w: found version %d, want %d; re-create it with `make reset-db`",
			ErrIncompatibleSchema, version, schemaVersion)
	}
	return nil
}

// Migrate creates any missing tables and indexes. It refuses a database from an
// older schema version rather than guessing how to convert it.
func Migrate(conn *sql.DB) error {
	if err := compatible(conn); err != nil {
		return err
	}

	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read embedded schema: %w", err)
	}
	if _, err := conn.Exec(string(schema)); err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}
	if _, err := conn.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("failed to record schema version: %w", err)
	}
	return nil
}

// isEmpty reports whether the database holds no tables yet.
func isEmpty(conn *sql.DB) (bool, error) {
	var tables int
	if err := conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'").
		Scan(&tables); err != nil {
		return false, fmt.Errorf("failed to inspect database: %w", err)
	}
	return tables == 0, nil
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

// parseRequiredTime reads a NOT NULL timestamp column.
func parseRequiredTime(stored string) (time.Time, error) {
	return time.Parse(timeLayout, stored)
}
