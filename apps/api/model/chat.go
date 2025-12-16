package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// MessageRole represents the role of the message sender
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

// Citation represents a single citation from the knowledge base
type Citation struct {
	ID           string                 `json:"id"`
	PageContent  string                 `json:"page_content"`
	Score        float64                `json:"score"`
	Filename     string                 `json:"filename"`
	DataSourceID string                 `json:"data_source_id"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Citations is a custom type for storing citation data as JSONB
type Citations []Citation

// JSONMap is a custom type for storing JSON data as JSONB
type JSONMap map[string]interface{}

// Scan implements the sql.Scanner interface for reading from database
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = JSONMap{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to unmarshal JSONMap value")
	}

	if len(bytes) == 0 {
		*j = JSONMap{}
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements the driver.Valuer interface for writing to database
func (j JSONMap) Value() (driver.Value, error) {
	if len(j) == 0 {
		return []byte("{}"), nil // Return empty JSON object instead of nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for reading from database
func (c *Citations) Scan(value interface{}) error {
	if value == nil {
		*c = Citations{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to unmarshal citations value")
	}

	return json.Unmarshal(bytes, c)
}

// Value implements the driver.Valuer interface for writing to database
func (c Citations) Value() (driver.Value, error) {
	if len(c) == 0 {
		return []byte("[]"), nil // Return empty JSON array instead of nil
	}
	return json.Marshal(c)
}

// ChatMessage represents a single message in a chat conversation
type ChatMessage struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	SessionID    uint           `gorm:"not null;index" json:"session_id"`
	SubjectID    uint           `gorm:"not null;index" json:"subject_id"`
	UserID       uint           `gorm:"not null;index" json:"user_id"`
	Role         MessageRole    `gorm:"type:varchar(20);not null" json:"role"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	Citations    Citations      `gorm:"type:jsonb" json:"citations,omitempty"`
	TokensUsed   int            `gorm:"default:0" json:"tokens_used"`
	ModelUsed    string         `gorm:"type:varchar(100)" json:"model_used"`
	ResponseTime int            `gorm:"default:0" json:"response_time_ms"` // Response time in milliseconds
	IsStreamed   bool           `gorm:"default:false" json:"is_streamed"`
	Metadata     JSONMap        `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	// Relationships
	Session ChatSession `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"session,omitempty"`
	Subject Subject     `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	User    User        `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}

// TableName specifies the table name for ChatMessage
func (ChatMessage) TableName() string {
	return "chat_messages"
}
