package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NotificationType represents the type/severity of notification
type NotificationType string

const (
	NotificationTypeInfo       NotificationType = "info"
	NotificationTypeSuccess    NotificationType = "success"
	NotificationTypeWarning    NotificationType = "warning"
	NotificationTypeError      NotificationType = "error"
	NotificationTypeInProgress NotificationType = "in_progress"
)

// NotificationCategory represents the category of notification
type NotificationCategory string

const (
	NotificationCategoryPYQIngest       NotificationCategory = "pyq_ingest"
	NotificationCategoryDocumentUpload  NotificationCategory = "document_upload"
	NotificationCategorySyllabusExtract NotificationCategory = "syllabus_extraction"
	NotificationCategoryGeneral         NotificationCategory = "general"
)

// UserNotification represents a notification for a user
type UserNotification struct {
	ID            uint                 `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	DeletedAt     gorm.DeletedAt       `gorm:"index" json:"deleted_at,omitempty"`
	UserID        uint                 `gorm:"index;not null" json:"user_id"`
	Type          NotificationType     `gorm:"type:varchar(20);not null" json:"type"`
	Category      NotificationCategory `gorm:"type:varchar(30);not null" json:"category"`
	Title         string               `gorm:"type:varchar(255);not null" json:"title"`
	Message       string               `gorm:"type:text" json:"message"`
	Read          bool                 `gorm:"default:false" json:"read"`
	IndexingJobID *uint                `gorm:"index" json:"indexing_job_id,omitempty"` // Link to IndexingJob if applicable
	Metadata      datatypes.JSON       `gorm:"type:jsonb" json:"metadata,omitempty"`   // Additional context

	// Relationships
	User        User         `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	IndexingJob *IndexingJob `gorm:"foreignKey:IndexingJobID;constraint:OnDelete:SET NULL" json:"indexing_job,omitempty"`
}

// NotificationMetadata represents common metadata fields
type NotificationMetadata struct {
	SubjectID      uint   `json:"subject_id,omitempty"`
	SubjectName    string `json:"subject_name,omitempty"`
	SubjectCode    string `json:"subject_code,omitempty"`
	TotalItems     int    `json:"total_items,omitempty"`
	CompletedItems int    `json:"completed_items,omitempty"`
	FailedItems    int    `json:"failed_items,omitempty"`
	Progress       int    `json:"progress,omitempty"` // 0-100
}

// NotificationResponse represents the API response format for a notification
type NotificationResponse struct {
	ID            uint                 `json:"id"`
	Type          NotificationType     `json:"type"`
	Category      NotificationCategory `json:"category"`
	Title         string               `json:"title"`
	Message       string               `json:"message"`
	Read          bool                 `json:"read"`
	IndexingJobID *uint                `json:"indexing_job_id,omitempty"`
	Metadata      datatypes.JSON       `json:"metadata,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// ToResponse converts a UserNotification to NotificationResponse
func (n *UserNotification) ToResponse() NotificationResponse {
	return NotificationResponse{
		ID:            n.ID,
		Type:          n.Type,
		Category:      n.Category,
		Title:         n.Title,
		Message:       n.Message,
		Read:          n.Read,
		IndexingJobID: n.IndexingJobID,
		Metadata:      n.Metadata,
		CreatedAt:     n.CreatedAt,
		UpdatedAt:     n.UpdatedAt,
	}
}
