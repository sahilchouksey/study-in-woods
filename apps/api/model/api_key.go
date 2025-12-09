package model

// ============================================================================
// DEPRECATED: This file contains the OLD server-side API key encryption model
// ============================================================================
//
// ARCHITECTURE CHANGE (2025-10-26):
// We have moved from server-side encrypted API key storage to CLIENT-SIDE storage.
//
// NEW ARCHITECTURE:
// - API keys are encrypted and stored in browser localStorage using Web Crypto API
// - Users send their encrypted keys with each request via Authorization header
// - Backend receives keys, uses them temporarily, never stores them
// - See model/api_key_usage_log.go for the new audit-only model
//
// THIS FILE IS KEPT FOR:
// 1. Reference/historical purposes
// 2. Potential future use of encryption utilities (crypto package)
// 3. Migration scripts to move existing users to new architecture
//
// DO NOT USE UserAPIKey FOR NEW FEATURES.
// Use APIKeyUsageLog instead (see model/api_key_usage_log.go)
//
// Related Documentation:
// - thoughts/shared/research/2025-10-26-client-side-api-key-architecture.md
// - thoughts/shared/research/2025-10-26-backend-architecture-research.md
// ============================================================================

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sahilchouksey/go-init-setup/utils/crypto"
	"gorm.io/gorm"
)

// ServiceType represents the external service for which an API key is stored
type ServiceType string

const (
	ServiceTypeFirecrawl ServiceType = "firecrawl"
	ServiceTypeTavily    ServiceType = "tavily"
	ServiceTypeExa       ServiceType = "exa"
)

var (
	ErrEncryptionKeyNotFound = errors.New("encryption key not found in context")
	ErrPlainAPIKeyNotFound   = errors.New("plain API key not found in context")
)

// Context keys for passing encryption data
type contextKey string

const (
	ContextKeyEncryptionKey contextKey = "encryptionKey"
	ContextKeyPlainAPIKey   contextKey = "plainAPIKey"
)

// UserAPIKey stores encrypted API keys for external services
// The encryption is done using AES-256-GCM with a key derived from the user's password
type UserAPIKey struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	UserID          uint           `gorm:"not null;index" json:"user_id"`
	Service         ServiceType    `gorm:"not null;type:varchar(50);index" json:"service"`
	EncryptedAPIKey []byte         `gorm:"not null;type:bytea" json:"-"`          // Never expose encrypted data
	Nonce           []byte         `gorm:"not null;type:bytea" json:"-"`          // GCM nonce
	KeyVersion      int            `gorm:"not null;default:1" json:"key_version"` // For key rotation
	IsActive        bool           `gorm:"default:true" json:"is_active"`

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`

	// Transient field - not stored in database
	// Used to pass decrypted key in memory after decryption
	DecryptedKey string `gorm:"-" json:"api_key,omitempty"`
}

// TableName specifies the table name for UserAPIKey
func (UserAPIKey) TableName() string {
	return "user_api_keys"
}

// BeforeCreate hook encrypts the API key before saving to database
func (u *UserAPIKey) BeforeCreate(tx *gorm.DB) error {
	return u.encryptAPIKey(tx)
}

// BeforeUpdate hook encrypts the API key before updating in database
func (u *UserAPIKey) BeforeUpdate(tx *gorm.DB) error {
	// Only encrypt if DecryptedKey is provided (meaning we're updating the key)
	if u.DecryptedKey != "" {
		return u.encryptAPIKey(tx)
	}
	return nil
}

// AfterFind hook decrypts the API key after retrieving from database
func (u *UserAPIKey) AfterFind(tx *gorm.DB) error {
	// Get encryption key from context
	encryptionKey, ok := tx.Statement.Context.Value(ContextKeyEncryptionKey).([]byte)
	if !ok || len(encryptionKey) == 0 {
		// Don't fail the query if encryption key is not provided
		// Just don't decrypt (useful for listing API keys without exposing them)
		return nil
	}

	// Decrypt the API key
	decrypted, err := crypto.DecryptAPIKey(u.EncryptedAPIKey, u.Nonce, encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt API key: %w", err)
	}

	// Store in transient field
	u.DecryptedKey = decrypted

	return nil
}

// encryptAPIKey is a helper method to encrypt the API key
func (u *UserAPIKey) encryptAPIKey(tx *gorm.DB) error {
	// Get encryption key from context
	encryptionKey, ok := tx.Statement.Context.Value(ContextKeyEncryptionKey).([]byte)
	if !ok || len(encryptionKey) == 0 {
		return ErrEncryptionKeyNotFound
	}

	// Get plain API key - either from DecryptedKey field or context
	plainAPIKey := u.DecryptedKey
	if plainAPIKey == "" {
		// Try to get from context (alternative way to pass the key)
		if contextKey, ok := tx.Statement.Context.Value(ContextKeyPlainAPIKey).(string); ok {
			plainAPIKey = contextKey
		}
	}

	if plainAPIKey == "" {
		return ErrPlainAPIKeyNotFound
	}

	// Encrypt the API key
	encrypted, nonce, err := crypto.EncryptAPIKey(plainAPIKey, encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt API key: %w", err)
	}

	u.EncryptedAPIKey = encrypted
	u.Nonce = nonce

	// Clear the plain text key from memory
	u.DecryptedKey = ""

	return nil
}

// WithEncryptionKey returns a new context with the encryption key
// This should be called when performing operations on UserAPIKey
func WithEncryptionKey(ctx context.Context, encryptionKey []byte) context.Context {
	return context.WithValue(ctx, ContextKeyEncryptionKey, encryptionKey)
}

// WithPlainAPIKey returns a new context with the plain API key
// This is an alternative to setting DecryptedKey field
func WithPlainAPIKey(ctx context.Context, apiKey string) context.Context {
	return context.WithValue(ctx, ContextKeyPlainAPIKey, apiKey)
}
