// Package auth provides password hashing for the seed (default admin user).
// It uses bcrypt with cost 10.
package auth

import (
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 10

// HashPassword returns a bcrypt hash of the password for secure storage.
// Used by the seed when creating the default admin user.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(b), err
}
