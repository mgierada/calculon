package auth

import (
	"strings"
	"testing"
)

// A throwaway ed25519 key generated for this test.
const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl alice@laptop"

func TestParseAuthorizedKey(t *testing.T) {
	key, err := ParseAuthorizedKey(testKey + "\n")
	if err != nil {
		t.Fatalf("ParseAuthorizedKey returned error: %v", err)
	}
	if !strings.HasPrefix(key.Fingerprint, "SHA256:") {
		t.Errorf("fingerprint = %q, want the SHA256 form", key.Fingerprint)
	}
	if key.Comment != "alice@laptop" {
		t.Errorf("comment = %q, want alice@laptop", key.Comment)
	}
	if strings.Contains(key.PublicKey, "alice@laptop") {
		t.Errorf("public key %q still carries the comment", key.PublicKey)
	}
}

func TestParseAuthorizedKeyRejectsGarbage(t *testing.T) {
	if _, err := ParseAuthorizedKey("not a key"); err == nil {
		t.Fatal("ParseAuthorizedKey accepted garbage")
	}
}
