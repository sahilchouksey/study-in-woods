package model

import (
	"gorm.io/gorm"
	"time"
)

// JWTTokenBlacklist stores revoked JWT tokens
type JWTTokenBlacklist struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Token     string         `gorm:"uniqueIndex;not null;type:text" json:"token"`
	UserID    uint           `gorm:"index" json:"user_id"`
	Reason    string         `gorm:"type:varchar(100)" json:"reason"` // logout, security, manual_revoke
	ExpiresAt time.Time      `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}

// TableName specifies the table name for JWTTokenBlacklist
func (JWTTokenBlacklist) TableName() string {
	return "jwt_token_blacklist"
}
