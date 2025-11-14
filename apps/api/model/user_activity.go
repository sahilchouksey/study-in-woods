package model

import (
	"time"
)

// ActivityType represents the type of user activity
type ActivityType string

const (
	ActivityTypeLogin          ActivityType = "login"
	ActivityTypeLogout         ActivityType = "logout"
	ActivityTypeDocumentUpload ActivityType = "document_upload"
	ActivityTypeDocumentView   ActivityType = "document_view"
	ActivityTypeChatStart      ActivityType = "chat_start"
	ActivityTypeChatMessage    ActivityType = "chat_message"
	ActivityTypeSubjectView    ActivityType = "subject_view"
	ActivityTypeCourseView     ActivityType = "course_view"
)

// UserActivity tracks detailed user activities for analytics
type UserActivity struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	UserID       uint         `gorm:"not null;index:idx_user_activity" json:"user_id"`
	ActivityType ActivityType `gorm:"type:varchar(50);not null;index:idx_activity_type" json:"activity_type"`
	ResourceType string       `gorm:"type:varchar(50)" json:"resource_type"` // e.g., "subject", "document", "chat"
	ResourceID   uint         `json:"resource_id"`
	Metadata     string       `gorm:"type:jsonb" json:"metadata,omitempty"` // Additional context
	IPAddress    string       `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent    string       `gorm:"type:text" json:"user_agent"`
	Duration     int          `gorm:"default:0" json:"duration_ms"` // Duration in milliseconds
	CreatedAt    time.Time    `gorm:"index:idx_created_at" json:"created_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName specifies the table name for UserActivity
func (UserActivity) TableName() string {
	return "user_activities"
}
