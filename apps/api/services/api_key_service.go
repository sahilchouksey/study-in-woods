package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/gorm"
)

// APIKeyService handles external API key operations
type APIKeyService struct {
	db *gorm.DB
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(db *gorm.DB) *APIKeyService {
	return &APIKeyService{
		db: db,
	}
}

// CreateAPIKey generates a new API key for a user
func (s *APIKeyService) CreateAPIKey(ctx context.Context, userID uint, name string, scopes []string, rateLimit int, monthlyQuota int) (*model.ExternalAPIKey, error) {
	// Create API key record
	apiKey := &model.ExternalAPIKey{
		UserID:       userID,
		Name:         name,
		Scopes:       s.scopesToJSON(scopes),
		IsActive:     true,
		RateLimit:    rateLimit,
		MonthlyQuota: monthlyQuota,
	}

	// Generate the key (BeforeCreate hook will set PlainKey)
	if err := s.db.WithContext(ctx).Create(apiKey).Error; err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

// ListAPIKeys returns all API keys for a user (without plain keys)
func (s *APIKeyService) ListAPIKeys(ctx context.Context, userID uint) ([]model.ExternalAPIKey, error) {
	var keys []model.ExternalAPIKey

	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	return keys, nil
}

// GetAPIKey retrieves an API key by ID (without plain key)
func (s *APIKeyService) GetAPIKey(ctx context.Context, keyID uint, userID uint) (*model.ExternalAPIKey, error) {
	var key model.ExternalAPIKey

	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", keyID, userID).
		First(&key).Error; err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return &key, nil
}

// ValidateAPIKey checks if an API key is valid and returns the key record
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, plainKey string) (*model.ExternalAPIKey, error) {
	// Extract prefix from plain key (first 15 chars)
	if len(plainKey) < 15 {
		return nil, fmt.Errorf("invalid API key format")
	}

	keyPrefix := plainKey[:15]
	keyHash := model.HashAPIKey(plainKey)

	// Find key by prefix and hash
	var key model.ExternalAPIKey
	if err := s.db.WithContext(ctx).
		Where("key_prefix = ? AND key_hash = ?", keyPrefix, keyHash).
		First(&key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid API key")
		}
		return nil, fmt.Errorf("failed to validate API key: %w", err)
	}

	// Check if key is valid
	if !key.IsValid() {
		return nil, fmt.Errorf("API key is inactive or expired")
	}

	// Check monthly quota
	if key.HasExceededQuota() {
		return nil, fmt.Errorf("monthly quota exceeded")
	}

	return &key, nil
}

// UpdateAPIKey updates API key properties
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, keyID uint, userID uint, updates map[string]interface{}) error {
	result := s.db.WithContext(ctx).
		Model(&model.ExternalAPIKey{}).
		Where("id = ? AND user_id = ?", keyID, userID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update API key: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// RevokeAPIKey deactivates an API key
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, keyID uint, userID uint) error {
	return s.UpdateAPIKey(ctx, keyID, userID, map[string]interface{}{
		"is_active": false,
	})
}

// DeleteAPIKey permanently deletes an API key
func (s *APIKeyService) DeleteAPIKey(ctx context.Context, keyID uint, userID uint) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", keyID, userID).
		Delete(&model.ExternalAPIKey{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete API key: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// IncrementUsage increments the usage counter for an API key
func (s *APIKeyService) IncrementUsage(ctx context.Context, keyID uint) error {
	var key model.ExternalAPIKey
	if err := s.db.WithContext(ctx).First(&key, keyID).Error; err != nil {
		return fmt.Errorf("failed to find API key: %w", err)
	}

	return key.IncrementUsage(s.db)
}

// GetUsageStats returns usage statistics for an API key
func (s *APIKeyService) GetUsageStats(ctx context.Context, keyID uint, userID uint) (map[string]interface{}, error) {
	var key model.ExternalAPIKey
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", keyID, userID).
		First(&key).Error; err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	stats := map[string]interface{}{
		"total_usage":      key.UsageCount,
		"monthly_usage":    key.UsageThisMonth,
		"monthly_quota":    key.MonthlyQuota,
		"quota_remaining":  key.MonthlyQuota - int(key.UsageThisMonth),
		"quota_percentage": float64(key.UsageThisMonth) / float64(key.MonthlyQuota) * 100,
		"rate_limit":       key.RateLimit,
		"last_used_at":     key.LastUsedAt,
		"last_reset_at":    key.LastResetAt,
		"is_active":        key.IsActive,
		"is_expired":       key.IsExpired(),
	}

	return stats, nil
}

// ExtendExpiry extends the expiration date of an API key
func (s *APIKeyService) ExtendExpiry(ctx context.Context, keyID uint, userID uint, duration time.Duration) error {
	var key model.ExternalAPIKey
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", keyID, userID).
		First(&key).Error; err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}

	var newExpiry time.Time
	if key.ExpiresAt != nil {
		newExpiry = key.ExpiresAt.Add(duration)
	} else {
		newExpiry = time.Now().Add(duration)
	}

	return s.db.WithContext(ctx).
		Model(&key).
		Update("expires_at", newExpiry).Error
}

// Helper function to convert scopes array to JSON string
func (s *APIKeyService) scopesToJSON(scopes []string) string {
	if len(scopes) == 0 {
		return "[]"
	}

	// Simple JSON array serialization
	result := "["
	for i, scope := range scopes {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("\"%s\"", scope)
	}
	result += "]"

	return result
}
