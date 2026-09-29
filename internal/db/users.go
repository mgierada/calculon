package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrUserNotFound means no user matched the lookup.
var ErrUserNotFound = errors.New("user not found")

// User is someone who owns accounts and may log in.
type User struct {
	ID   int64
	Name string
	// Keys counts the SSH public keys that log in as this user.
	Keys int
}

// UserKey is an SSH public key that logs in as a user.
type UserKey struct {
	Fingerprint string
	// PublicKey is the key in authorized_keys format, without a comment.
	PublicKey string
	Comment   string
}

// CreateUser adds a user and returns it.
func CreateUser(conn *sql.DB, name string) (User, error) {
	if name == "" {
		return User{}, fmt.Errorf("user name is empty")
	}
	res, err := conn.Exec(`INSERT INTO users (name, created_at) VALUES (?, ?)`,
		name, formatTime(time.Now()))
	if err != nil {
		return User{}, fmt.Errorf("failed to create user %q: %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("failed to read id of user %q: %w", name, err)
	}
	return User{ID: id, Name: name}, nil
}

// UserByName looks a user up by name.
func UserByName(conn *sql.DB, name string) (User, error) {
	return scanUser(conn.QueryRow(userSelect+` WHERE u.name = ? GROUP BY u.id`, name), name)
}

// UserByKey looks up the user an SSH key fingerprint logs in as.
func UserByKey(conn *sql.DB, fingerprint string) (User, error) {
	return scanUser(conn.QueryRow(userSelect+`
		WHERE u.id = (SELECT user_id FROM user_keys WHERE fingerprint = ?) GROUP BY u.id`,
		fingerprint), fingerprint)
}

// Users lists every user, by name.
func Users(conn *sql.DB) ([]User, error) {
	rows, err := conn.Query(userSelect + ` GROUP BY u.id ORDER BY u.name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name, &user.Keys); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// AddUserKey lets a public key log in as the user. A key can belong to only one
// user, since it is what identifies them.
func AddUserKey(conn *sql.DB, userID int64, key UserKey) error {
	_, err := conn.Exec(`INSERT INTO user_keys (fingerprint, user_id, public_key, comment, added_at)
		VALUES (?, ?, ?, ?, ?)`,
		key.Fingerprint, userID, key.PublicKey, key.Comment, formatTime(time.Now()))
	if err != nil {
		return fmt.Errorf("failed to add key %s: %w", key.Fingerprint, err)
	}
	return nil
}

// userSelect reads users with their key count; callers add WHERE and GROUP BY.
const userSelect = `SELECT u.id, u.name, COUNT(k.fingerprint)
	FROM users u LEFT JOIN user_keys k ON k.user_id = u.id`

// scanUser reads one user, mapping no rows onto ErrUserNotFound.
func scanUser(row *sql.Row, lookup string) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Keys)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, fmt.Errorf("%w: %s", ErrUserNotFound, lookup)
	}
	if err != nil {
		return User{}, fmt.Errorf("failed to look up user %s: %w", lookup, err)
	}
	return user, nil
}
