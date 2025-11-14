package auth

import (
	"context"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/gorm"
)

// BlacklistService handles JWT token revocation
type BlacklistService struct {
	db *gorm.DB
}

// NewBlacklistService creates a new blacklist service
func NewBlacklistService(db *gorm.DB) *BlacklistService {
	return &BlacklistService{db: db}
}

// RevokeToken adds a token to the blacklist
func (s *BlacklistService) RevokeToken(ctx context.Context, jti string, userID uint, expiresAt time.Time, reason string) error {
	blacklistEntry := model.JWTTokenBlacklist{
		Token:     jti,
		UserID:    userID,
		Reason:    reason,
		ExpiresAt: expiresAt,
	}

	return s.db.WithContext(ctx).Create(&blacklistEntry).Error
}

// IsTokenRevoked checks if a token is in the blacklist
func (s *BlacklistService) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.JWTTokenBlacklist{}).
		Where("token = ? AND expires_at > ?", jti, time.Now()).
		Count(&count).
		Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RevokeAllUserTokens increments user's token version to invalidate all tokens
func (s *BlacklistService) RevokeAllUserTokens(ctx context.Context, userID uint) error {
	return s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		UpdateColumn("token_version", gorm.Expr("token_version + ?", 1)).
		Error
}

// CleanupExpiredTokens removes expired entries from the blacklist
func (s *BlacklistService) CleanupExpiredTokens(ctx context.Context) error {
	return s.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&model.JWTTokenBlacklist{}).
		Error
}

// GetBlacklistedTokenCount returns the count of blacklisted tokens
func (s *BlacklistService) GetBlacklistedTokenCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.JWTTokenBlacklist{}).
		Where("expires_at > ?", time.Now()).
		Count(&count).
		Error
	return count, err
}

// GetUserTokenVersion returns the current token version for a user
func (s *BlacklistService) GetUserTokenVersion(ctx context.Context, userID uint) (int, error) {
	var user model.User
	err := s.db.WithContext(ctx).
		Select("token_version").
		First(&user, userID).
		Error
	if err != nil {
		return 0, err
	}
	return user.TokenVersion, nil
}
