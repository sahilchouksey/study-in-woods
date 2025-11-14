package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ExternalAPIKey represents an API key for third-party developers to access our API
// This is different from UserAPIKey which is deprecated and was for storing external service keys
type ExternalAPIKey struct {
	gorm.Model
	UserID         uint       `gorm:"not null;index" json:"user_id"`
	Name           string     `gorm:"not null;type:varchar(100)" json:"name"`                  // Friendly name for the key
	KeyPrefix      string     `gorm:"not null;uniqueIndex;type:varchar(20)" json:"key_prefix"` // First 8 chars (sk_live_xxx)
	KeyHash        string     `gorm:"not null;uniqueIndex;type:varchar(64)" json:"-"`          // SHA-256 hash of full key
	Scopes         string     `gorm:"type:text" json:"scopes"`                                 // JSON array of allowed scopes
	IsActive       bool       `gorm:"default:true;index" json:"is_active"`
	ExpiresAt      *time.Time `gorm:"index" json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	UsageCount     int64      `gorm:"default:0" json:"usage_count"`
	RateLimit      int        `gorm:"default:100" json:"rate_limit"`      // Requests per minute
	MonthlyQuota   int        `gorm:"default:10000" json:"monthly_quota"` // Requests per month
	UsageThisMonth int64      `gorm:"default:0" json:"usage_this_month"`
	LastResetAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"last_reset_at"` // For monthly quota reset

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`

	// Transient field - only populated when key is first created
	// Never stored in database, never returned after creation
	PlainKey string `gorm:"-" json:"api_key,omitempty"`
}

// TableName specifies the table name for ExternalAPIKey
func (ExternalAPIKey) TableName() string {
	return "external_api_keys"
}

// GenerateAPIKey generates a new API key with format: sk_live_<32-hex-chars>
func GenerateAPIKey(prefix string) (string, error) {
	if prefix == "" {
		prefix = "sk_live"
	}

	// Generate 32 random bytes (256 bits)
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex string (64 characters)
	hexString := hex.EncodeToString(randomBytes)

	// Format: sk_live_<64-hex-chars>
	return fmt.Sprintf("%s_%s", prefix, hexString), nil
}

// BeforeCreate hook generates API key and sets initial values
func (e *ExternalAPIKey) BeforeCreate(tx *gorm.DB) error {
	// Generate API key if not provided
	if e.PlainKey == "" {
		prefix := "sk_live"
		key, err := GenerateAPIKey(prefix)
		if err != nil {
			return err
		}
		e.PlainKey = key
	}

	// Extract prefix (first 15 chars: sk_live_xxxxxxx)
	if len(e.PlainKey) >= 15 {
		e.KeyPrefix = e.PlainKey[:15]
	} else {
		return fmt.Errorf("invalid API key format")
	}

	// Hash the full key for storage
	e.KeyHash = HashAPIKey(e.PlainKey)

	// Set default expiry (1 year from now)
	if e.ExpiresAt == nil {
		expiresAt := time.Now().AddDate(1, 0, 0)
		e.ExpiresAt = &expiresAt
	}

	// Set default rate limit and quota
	if e.RateLimit == 0 {
		e.RateLimit = 100 // 100 requests per minute
	}
	if e.MonthlyQuota == 0 {
		e.MonthlyQuota = 10000 // 10,000 requests per month
	}

	// Set last reset date to now
	e.LastResetAt = time.Now()

	return nil
}

// HashAPIKey creates a SHA-256 hash of the API key for storage
func HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// IsExpired checks if the API key has expired
func (e *ExternalAPIKey) IsExpired() bool {
	if e.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*e.ExpiresAt)
}

// IsValid checks if the API key is valid (active and not expired)
func (e *ExternalAPIKey) IsValid() bool {
	return e.IsActive && !e.IsExpired()
}

// IncrementUsage atomically increments usage counters
func (e *ExternalAPIKey) IncrementUsage(db *gorm.DB) error {
	now := time.Now()

	// Check if we need to reset monthly quota
	if e.LastResetAt.Month() != now.Month() || e.LastResetAt.Year() != now.Year() {
		return db.Model(e).Updates(map[string]interface{}{
			"usage_count":      gorm.Expr("usage_count + 1"),
			"usage_this_month": 1,
			"last_used_at":     now,
			"last_reset_at":    now,
		}).Error
	}

	// Normal usage increment
	return db.Model(e).Updates(map[string]interface{}{
		"usage_count":      gorm.Expr("usage_count + 1"),
		"usage_this_month": gorm.Expr("usage_this_month + 1"),
		"last_used_at":     now,
	}).Error
}

// HasExceededQuota checks if monthly quota is exceeded
func (e *ExternalAPIKey) HasExceededQuota() bool {
	// Reset monthly counter if needed
	now := time.Now()
	if e.LastResetAt.Month() != now.Month() || e.LastResetAt.Year() != now.Year() {
		return false // Will be reset on next usage
	}

	return e.UsageThisMonth >= int64(e.MonthlyQuota)
}
