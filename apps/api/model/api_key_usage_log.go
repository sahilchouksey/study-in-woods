package model

import (
	"time"

	"gorm.io/gorm"
)

// APIKeyUsageLog tracks usage statistics for client-side stored API keys
// This model does NOT store the actual API keys - keys are stored client-side
// and sent with each request. This model only tracks usage metrics for audit purposes.
//
// ARCHITECTURE NOTE:
// - API keys are encrypted and stored in the browser's localStorage using Web Crypto API
// - Users send their encrypted keys with each request via Authorization header
// - Backend decrypts keys temporarily in memory, uses them, then discards
// - This model tracks when/how often keys are used for audit and analytics
type APIKeyUsageLog struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	UserID        uint           `gorm:"not null;index:idx_user_id" json:"user_id"`
	Service       ServiceType    `gorm:"not null;type:varchar(50);index:idx_user_service,unique" json:"service"`
	LastUsedAt    time.Time      `gorm:"not null" json:"last_used_at"`
	UsageCount    int            `gorm:"default:0" json:"usage_count"`
	LastRequestIP string         `gorm:"type:varchar(45)" json:"last_request_ip"` // IPv4/IPv6

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName specifies the table name for APIKeyUsageLog
func (APIKeyUsageLog) TableName() string {
	return "api_key_usage_logs"
}

// BeforeCreate hook sets initial values
func (a *APIKeyUsageLog) BeforeCreate(tx *gorm.DB) error {
	if a.LastUsedAt.IsZero() {
		a.LastUsedAt = time.Now()
	}
	if a.UsageCount == 0 {
		a.UsageCount = 1
	}
	return nil
}

// IncrementUsage updates usage statistics atomically
func (a *APIKeyUsageLog) IncrementUsage(db *gorm.DB, ip string) error {
	return db.Model(a).Updates(map[string]interface{}{
		"usage_count":     gorm.Expr("usage_count + 1"),
		"last_used_at":    time.Now(),
		"last_request_ip": ip,
	}).Error
}
