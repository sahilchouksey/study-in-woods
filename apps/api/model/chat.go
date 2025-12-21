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

// MessageStatus represents the completion status of a message
type MessageStatus string

const (
	MessageStatusComplete MessageStatus = "complete" // Message was fully generated
	MessageStatusPartial  MessageStatus = "partial"  // Message was cut off due to timeout/error
	MessageStatusPending  MessageStatus = "pending"  // Message is still being generated
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

	// Partial response recovery fields
	Status          MessageStatus `gorm:"type:varchar(20);default:'complete'" json:"status"`
	ParentMessageID *uint         `gorm:"index" json:"parent_message_id,omitempty"` // Points to original partial message when continuing
	ErrorType       string        `gorm:"type:varchar(100)" json:"error_type,omitempty"`
	ErrorMessage    string        `gorm:"type:text" json:"error_message,omitempty"`

	// Relationships
	Session       ChatSession  `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"session,omitempty"`
	Subject       Subject      `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	User          User         `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	ParentMessage *ChatMessage `gorm:"foreignKey:ParentMessageID" json:"parent_message,omitempty"` // Self-referential for continuation
}

// TableName specifies the table name for ChatMessage
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// IsPartial returns true if this message was cut off due to timeout/error
func (m *ChatMessage) IsPartial() bool {
	return m.Status == MessageStatusPartial
}

// CanContinue returns true if this partial message can be continued
func (m *ChatMessage) CanContinue() bool {
	return m.IsPartial() && m.Role == MessageRoleAssistant
}

// MarkAsPartial sets the message status to partial with error info
func (m *ChatMessage) MarkAsPartial(errorType, errorMessage string) {
	m.Status = MessageStatusPartial
	m.ErrorType = errorType
	m.ErrorMessage = errorMessage
}

// MarkAsComplete sets the message status to complete
func (m *ChatMessage) MarkAsComplete() {
	m.Status = MessageStatusComplete
	m.ErrorType = ""
	m.ErrorMessage = ""
}
