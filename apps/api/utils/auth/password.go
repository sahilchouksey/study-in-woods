package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordMismatch = errors.New("password does not match")
)

const (
	// DefaultCost is the default bcrypt cost
	DefaultCost = 12
	// MinPasswordLength is the minimum password length
	MinPasswordLength = 8
)

// HashPassword generates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	if len(password) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedBytes), nil
}

// VerifyPassword checks if the provided password matches the hash
func VerifyPassword(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrPasswordMismatch
		}
		return err
	}
	return nil
}

// IsPasswordValid checks if password meets minimum requirements
func IsPasswordValid(password string) bool {
	return len(password) >= MinPasswordLength
}
