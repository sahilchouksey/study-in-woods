package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ChatMemoryBatchStatus represents the status of a message batch
type ChatMemoryBatchStatus string

const (
	BatchStatusActive    ChatMemoryBatchStatus = "active"    // Currently being filled
	BatchStatusComplete  ChatMemoryBatchStatus = "complete"  // Full but not yet compacted
	BatchStatusCompacted ChatMemoryBatchStatus = "compacted" // Has been summarized into context
)

// ChatMemoryBatch represents a batch of 20 messages in a conversation
// When a batch is full and the next batch reaches 10 messages, this batch gets compacted
type ChatMemoryBatch struct {
	ID           uint                  `gorm:"primaryKey" json:"id"`
	SessionID    uint                  `gorm:"not null;index" json:"session_id"`
	BatchNumber  int                   `gorm:"not null" json:"batch_number"` // 1, 2, 3, etc.
	Status       ChatMemoryBatchStatus `gorm:"type:varchar(20);default:'active'" json:"status"`
	MessageCount int                   `gorm:"default:0" json:"message_count"`
	StartMsgID   uint                  `gorm:"index" json:"start_msg_id"` // First message ID in batch
	EndMsgID     uint                  `gorm:"index" json:"end_msg_id"`   // Last message ID in batch
	CompactedAt  *time.Time            `json:"compacted_at,omitempty"`
	ContextID    *uint                 `json:"context_id,omitempty"` // Link to compacted context
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	DeletedAt    gorm.DeletedAt        `gorm:"index" json:"-"`

	// Relationships
	Session ChatSession `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"-"`
	// Note: CompactedContext relationship removed to avoid circular FK dependency
	// Use ContextID to manually join if needed
}

func (ChatMemoryBatch) TableName() string {
	return "chat_memory_batches"
}

// ChatCompactedContext represents a summarized/compacted version of a message batch
// This is created when a batch needs to be moved out of active context
type ChatCompactedContext struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	SessionID     uint           `gorm:"not null;index" json:"session_id"`
	BatchID       uint           `gorm:"not null;index" json:"batch_id"`
	BatchNumber   int            `gorm:"not null" json:"batch_number"`
	Summary       string         `gorm:"type:text;not null" json:"summary"`       // AI-generated summary
	KeyTopics     StringArray    `gorm:"type:jsonb" json:"key_topics"`            // Main topics discussed
	KeyEntities   StringArray    `gorm:"type:jsonb" json:"key_entities"`          // Important entities (names, concepts)
	UserIntents   StringArray    `gorm:"type:jsonb" json:"user_intents"`          // What user was trying to accomplish
	AIResponses   StringArray    `gorm:"type:jsonb" json:"ai_responses"`          // Key points from AI responses
	MessageRange  string         `gorm:"type:varchar(50)" json:"message_range"`   // e.g., "1-20", "21-40"
	TokenCount    int            `gorm:"default:0" json:"token_count"`            // Approximate tokens in summary
	OriginalCount int            `gorm:"default:0" json:"original_message_count"` // Number of messages summarized
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Session ChatSession     `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"-"`
	Batch   ChatMemoryBatch `gorm:"foreignKey:BatchID;constraint:OnDelete:CASCADE" json:"-"`
}

func (ChatCompactedContext) TableName() string {
	return "chat_compacted_contexts"
}

// StringArray is a custom type for storing string arrays as JSONB
type StringArray []string

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = StringArray{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to unmarshal StringArray value")
	}

	if len(bytes) == 0 {
		*s = StringArray{}
		return nil
	}

	return json.Unmarshal(bytes, s)
}

func (s StringArray) Value() (driver.Value, error) {
	if len(s) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(s)
}

// ChatMemorySearchResult represents a search result from memory
type ChatMemorySearchResult struct {
	Type      string    `json:"type"`      // "message" or "context"
	Content   string    `json:"content"`   // The actual content
	Role      string    `json:"role"`      // For messages: user/assistant
	Timestamp time.Time `json:"timestamp"` // When it was created
	Relevance float64   `json:"relevance"` // Search relevance score
	BatchNum  int       `json:"batch_num"` // Which batch this is from
	MessageID *uint     `json:"message_id,omitempty"`
	ContextID *uint     `json:"context_id,omitempty"`
}

// MemorySearchResults is a slice of search results
type MemorySearchResults []ChatMemorySearchResult

func (m *MemorySearchResults) Scan(value interface{}) error {
	if value == nil {
		*m = MemorySearchResults{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to unmarshal MemorySearchResults")
	}
	return json.Unmarshal(bytes, m)
}

func (m MemorySearchResults) Value() (driver.Value, error) {
	if len(m) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(m)
}
