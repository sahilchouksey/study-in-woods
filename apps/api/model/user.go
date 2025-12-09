package model

import (
	"time"

	"gorm.io/gorm"
)

// User represents a registered user in the system
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	Email        string         `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"not null" json:"-"`            // Never expose password in JSON
	PasswordSalt []byte         `gorm:"not null;type:bytea" json:"-"` // Salt for key derivation
	Name         string         `gorm:"not null" json:"name"`
	Role         string         `gorm:"type:varchar(20);default:'student'" json:"role"` // student, admin
	Semester     int            `gorm:"default:1" json:"semester"`
	TokenVersion int            `gorm:"default:0" json:"-"` // Increment to invalidate all user tokens

	// Relationships
	APIUsageLogs   []APIKeyUsageLog    `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Courses        []UserCourse        `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"courses,omitempty"`
	ChatSessions   []ChatSession       `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	ChatMessages   []ChatMessage       `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Payments       []CoursePayment     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	AdminAuditLog  []AdminAuditLog     `gorm:"foreignKey:AdminID;constraint:OnDelete:CASCADE" json:"-"`
	TokenBlacklist []JWTTokenBlacklist `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// UserCourse represents a many-to-many relationship between users and courses
type UserCourse struct {
	UserID     uint  `gorm:"primaryKey" json:"user_id"`
	CourseID   uint  `gorm:"primaryKey" json:"course_id"`
	EnrolledAt int64 `gorm:"autoCreateTime" json:"enrolled_at"`

	// Relationships
	User   User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Course Course `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"course,omitempty"`
}
