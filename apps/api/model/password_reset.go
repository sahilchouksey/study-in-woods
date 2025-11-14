package model

import (
	"time"

	"gorm.io/gorm"
)

// PasswordResetToken stores password reset tokens
type PasswordResetToken struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	Token     string         `gorm:"uniqueIndex;not null;type:varchar(100)" json:"token"`
	ExpiresAt time.Time      `gorm:"index;not null" json:"expires_at"`
	UsedAt    *time.Time     `json:"used_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}

// TableName specifies the table name for PasswordResetToken
func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

// IsExpired checks if the reset token has expired
func (p *PasswordResetToken) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

// IsUsed checks if the reset token has been used
func (p *PasswordResetToken) IsUsed() bool {
	return p.UsedAt != nil
}

// MarkAsUsed marks the token as used
func (p *PasswordResetToken) MarkAsUsed() {
	now := time.Now()
	p.UsedAt = &now
}
