package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const sha256Prefix = "sha256$"

// HashPassword hashes a plain text password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword validates plain text password against a stored hash.
// It supports legacy sha256-prefixed hashes for migration.
func CheckPassword(password, hash string) bool {
	if strings.HasPrefix(hash, sha256Prefix) {
		sum := sha256.Sum256([]byte(password))
		expected := sha256Prefix + hex.EncodeToString(sum[:])
		return subtle.ConstantTimeCompare([]byte(expected), []byte(hash)) == 1
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// IsLegacyHash reports whether the stored hash uses the legacy sha256 format.
func IsLegacyHash(hash string) bool {
	if !strings.HasPrefix(hash, sha256Prefix) {
		return false
	}
	return true
}
