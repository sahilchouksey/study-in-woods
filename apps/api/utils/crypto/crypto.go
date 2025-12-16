package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	// SaltLength is the length of salts for password hashing
	SaltLength = 32
)

var (
	ErrInvalidKeyLength = errors.New("invalid key length: must be 32 bytes for AES-256")
	ErrDecryptionFailed = errors.New("decryption failed")
)

// GenerateSalt generates a cryptographically secure random salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GetEncryptionKey returns the 32-byte encryption key from ENCRYPTION_KEY env variable
// Supports both base64-encoded keys and raw 32-character strings
func GetEncryptionKey() ([]byte, error) {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		return nil, errors.New("ENCRYPTION_KEY environment variable not set")
	}

	// Try base64 decode first (recommended for binary keys)
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err == nil && len(key) == 32 {
		return key, nil
	}

	// Fall back to raw string (must be exactly 32 chars)
	if len(keyStr) == 32 {
		return []byte(keyStr), nil
	}

	return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (got %d)", len(keyStr))
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided key
// Returns base64-encoded string: nonce (12 bytes) + ciphertext
func Encrypt(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and combine nonce + ciphertext
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	combined := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt decrypts a base64-encoded ciphertext using AES-256-GCM
func Decrypt(encoded string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidKeyLength
	}

	combined, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(combined) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce := combined[:nonceSize]
	ciphertext := combined[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(plaintext), nil
}

// EncryptAPIKeyForStorage encrypts an API key for database storage
// Uses ENCRYPTION_KEY from environment
func EncryptAPIKeyForStorage(apiKey string) (string, error) {
	key, err := GetEncryptionKey()
	if err != nil {
		return "", err
	}
	return Encrypt(apiKey, key)
}

// DecryptAPIKeyFromStorage decrypts an API key from database storage
// Uses ENCRYPTION_KEY from environment
func DecryptAPIKeyFromStorage(stored string) (string, error) {
	if stored == "" {
		return "", errors.New("no encrypted API key stored")
	}
	key, err := GetEncryptionKey()
	if err != nil {
		return "", err
	}
	return Decrypt(stored, key)
}

// GenerateEncryptionKey generates a new random 32-byte key as base64
// Use this to create ENCRYPTION_KEY value
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// EncryptAPIKey encrypts an API key using AES-256-GCM (raw bytes version)
// Returns encrypted data and nonce separately - used by UserAPIKey model
func EncryptAPIKey(apiKey string, encryptionKey []byte) (encrypted []byte, nonce []byte, err error) {
	if len(encryptionKey) != 32 {
		return nil, nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	encrypted = gcm.Seal(nil, nonce, []byte(apiKey), nil)
	return encrypted, nonce, nil
}

// DecryptAPIKey decrypts an encrypted API key using AES-256-GCM (raw bytes version)
// Used by UserAPIKey model
func DecryptAPIKey(encrypted []byte, nonce []byte, encryptionKey []byte) (string, error) {
	if len(encryptionKey) != 32 {
		return "", ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(plaintext), nil
}
