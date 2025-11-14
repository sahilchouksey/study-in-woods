package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id parameters for key derivation
	Argon2Time      uint32 = 1
	Argon2Memory    uint32 = 64 * 1024 // 64 MB
	Argon2Threads   uint8  = 4
	Argon2KeyLength uint32 = 32 // 256 bits for AES-256

	// Salt length for key derivation
	SaltLength = 32
)

var (
	ErrInvalidKeyLength = errors.New("invalid key length")
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

// DeriveKey derives an encryption key from a password and salt using Argon2id
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		Argon2Time,
		Argon2Memory,
		Argon2Threads,
		Argon2KeyLength,
	)
}

// EncryptAPIKey encrypts an API key using AES-256-GCM
// Returns the encrypted data and nonce
func EncryptAPIKey(apiKey string, encryptionKey []byte) (encrypted []byte, nonce []byte, err error) {
	if len(encryptionKey) != 32 {
		return nil, nil, ErrInvalidKeyLength
	}

	// Create AES cipher
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the API key
	encrypted = gcm.Seal(nil, nonce, []byte(apiKey), nil)

	return encrypted, nonce, nil
}

// DecryptAPIKey decrypts an encrypted API key using AES-256-GCM
func DecryptAPIKey(encrypted []byte, nonce []byte, encryptionKey []byte) (string, error) {
	if len(encryptionKey) != 32 {
		return "", ErrInvalidKeyLength
	}

	// Create AES cipher
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt the API key
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(plaintext), nil
}

// EncryptData encrypts arbitrary data using AES-256-GCM (generic version)
func EncryptData(data []byte, encryptionKey []byte) (encrypted []byte, nonce []byte, err error) {
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

	encrypted = gcm.Seal(nil, nonce, data, nil)
	return encrypted, nonce, nil
}

// DecryptData decrypts arbitrary data using AES-256-GCM (generic version)
func DecryptData(encrypted []byte, nonce []byte, encryptionKey []byte) ([]byte, error) {
	if len(encryptionKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return plaintext, nil
}
