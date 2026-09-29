package db

import (
	"errors"
	"testing"
)

func TestUserByKey(t *testing.T) {
	conn := openTestDB(t)
	alice := createTestUser(t, conn, "alice")
	key := UserKey{Fingerprint: "SHA256:abc", PublicKey: "ssh-ed25519 AAAA", Comment: "laptop"}
	if err := AddUserKey(conn, alice.ID, key); err != nil {
		t.Fatalf("AddUserKey returned error: %v", err)
	}

	user, err := UserByKey(conn, "SHA256:abc")
	if err != nil {
		t.Fatalf("UserByKey returned error: %v", err)
	}
	if user.ID != alice.ID || user.Name != "alice" || user.Keys != 1 {
		t.Errorf("UserByKey = %+v, want alice with one key", user)
	}
}

func TestUserByKeyUnknown(t *testing.T) {
	conn := openTestDB(t)
	createTestUser(t, conn, "alice")

	if _, err := UserByKey(conn, "SHA256:nope"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("UserByKey with an unknown key returned %v, want ErrUserNotFound", err)
	}
}

// A key identifies exactly one user, so it cannot be registered twice.
func TestAddUserKeyRejectsSharedKey(t *testing.T) {
	conn := openTestDB(t)
	alice := createTestUser(t, conn, "alice")
	bob := createTestUser(t, conn, "bob")
	key := UserKey{Fingerprint: "SHA256:abc", PublicKey: "ssh-ed25519 AAAA"}

	if err := AddUserKey(conn, alice.ID, key); err != nil {
		t.Fatalf("AddUserKey returned error: %v", err)
	}
	if err := AddUserKey(conn, bob.ID, key); err == nil {
		t.Fatal("AddUserKey gave alice's key to bob too")
	}
}

func TestUsersListsByName(t *testing.T) {
	conn := openTestDB(t)
	createTestUser(t, conn, "zoe")
	createTestUser(t, conn, "adam")

	users, err := Users(conn)
	if err != nil {
		t.Fatalf("Users returned error: %v", err)
	}
	if len(users) != 2 || users[0].Name != "adam" || users[1].Name != "zoe" {
		t.Errorf("Users = %+v, want adam then zoe", users)
	}
}

func TestCreateUserRejectsDuplicateName(t *testing.T) {
	conn := openTestDB(t)
	createTestUser(t, conn, "alice")

	if _, err := CreateUser(conn, "alice"); err == nil {
		t.Fatal("CreateUser accepted a duplicate name")
	}
}
