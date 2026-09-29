// Package auth maps SSH public keys onto calculon users.
package auth

import (
	"fmt"
	"strings"

	gossh "golang.org/x/crypto/ssh"

	"github.com/mgierada/calculon/internal/db"
)

// Fingerprint is the OpenSSH SHA256 fingerprint of a key, as `ssh-keygen -l`
// prints it.
func Fingerprint(key gossh.PublicKey) string {
	return gossh.FingerprintSHA256(key)
}

// ParseAuthorizedKey reads one key in authorized_keys format, e.g. the contents
// of ~/.ssh/id_ed25519.pub.
func ParseAuthorizedKey(text string) (db.UserKey, error) {
	key, comment, _, _, err := gossh.ParseAuthorizedKey([]byte(strings.TrimSpace(text)))
	if err != nil {
		return db.UserKey{}, fmt.Errorf("failed to parse public key: %w", err)
	}
	return db.UserKey{
		Fingerprint: Fingerprint(key),
		PublicKey:   strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))),
		Comment:     comment,
	}, nil
}
