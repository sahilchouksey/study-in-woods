package model

import (
	"gorm.io/gorm"
	"time"
)

// ChatSession represents a conversation session between a user and subject AI
type ChatSession struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	SubjectID     uint           `gorm:"not null;index" json:"subject_id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	Title         string         `gorm:"type:varchar(255)" json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	Status        string         `gorm:"type:varchar(20);default:'active'" json:"status"` // active, archived
	AgentUUID     string         `gorm:"type:varchar(100)" json:"agent_uuid"`
	MessageCount  int            `gorm:"default:0" json:"message_count"`
	TotalTokens   int            `gorm:"default:0" json:"total_tokens"`
	LastMessageAt *time.Time     `json:"last_message_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Subject  Subject       `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	User     User          `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Messages []ChatMessage `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
}

// TableName specifies the table name for ChatSession
func (ChatSession) TableName() string {
	return "chat_sessions"
}
