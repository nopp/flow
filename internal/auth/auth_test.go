package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHashAndCheckPassword_Bcrypt(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if IsLegacyHash(hash) {
		t.Fatal("expected bcrypt hash, got legacy hash")
	}
	if !CheckPassword("secret123", hash) {
		t.Fatal("expected password to validate")
	}
	if CheckPassword("wrong", hash) {
		t.Fatal("expected wrong password to fail")
	}
}

func TestCheckPassword_LegacySHA256(t *testing.T) {
	sum := sha256.Sum256([]byte("legacy-pass"))
	legacy := sha256Prefix + hex.EncodeToString(sum[:])
	if !IsLegacyHash(legacy) {
		t.Fatal("expected legacy hash detection")
	}
	if !CheckPassword("legacy-pass", legacy) {
		t.Fatal("expected legacy password to validate")
	}
	if CheckPassword("wrong", legacy) {
		t.Fatal("expected wrong password to fail")
	}
}
