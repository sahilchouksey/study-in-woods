package model

import (
	"time"

	"gorm.io/gorm"
)

// DocumentType represents the type of document
type DocumentType string

const (
	DocumentTypePYQ       DocumentType = "pyq"       // Previous Year Questions
	DocumentTypeBook      DocumentType = "book"      // Textbooks
	DocumentTypeReference DocumentType = "reference" // Reference materials
	DocumentTypeSyllabus  DocumentType = "syllabus"  // Course syllabus
	DocumentTypeNotes     DocumentType = "notes"     // Study notes
)

// IndexingStatus represents the status of document indexing in the knowledge base
type IndexingStatus string

const (
	IndexingStatusPending    IndexingStatus = "pending"
	IndexingStatusInProgress IndexingStatus = "in_progress"
	IndexingStatusCompleted  IndexingStatus = "completed"
	IndexingStatusFailed     IndexingStatus = "failed"
	IndexingStatusPartial    IndexingStatus = "partially_completed"
)

// Document represents an uploaded file associated with a subject or semester
type Document struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	SubjectID        *uint          `gorm:"index" json:"subject_id,omitempty"`  // Optional - for subject-specific documents
	SemesterID       *uint          `gorm:"index" json:"semester_id,omitempty"` // Optional - for semester-level documents (e.g., syllabus PDFs)
	Type             DocumentType   `gorm:"type:varchar(20);not null" json:"type"`
	Filename         string         `gorm:"not null" json:"filename"`
	OriginalURL      string         `gorm:"type:text" json:"original_url"`            // URL from where it was crawled (if applicable)
	SpacesURL        string         `gorm:"not null" json:"spaces_url"`               // DigitalOcean Spaces URL
	SpacesKey        string         `gorm:"not null" json:"spaces_key"`               // S3-style key in Spaces
	DataSourceID     string         `gorm:"type:varchar(100)" json:"data_source_id"`  // Knowledge Base data source ID
	IndexingJobID    string         `gorm:"type:varchar(100)" json:"indexing_job_id"` // Knowledge Base indexing job ID
	IndexingStatus   IndexingStatus `gorm:"type:varchar(20);default:'pending'" json:"indexing_status"`
	IndexingError    string         `gorm:"type:text" json:"indexing_error,omitempty"`
	FileSize         int64          `gorm:"default:0" json:"file_size"`  // Size in bytes
	PageCount        int            `gorm:"default:0" json:"page_count"` // Number of pages (for PDFs)
	UploadedByUserID uint           `gorm:"index" json:"uploaded_by_user_id"`

	// OCR fields (simple approach - just store the extracted text)
	OCRText      string `gorm:"type:text" json:"ocr_text,omitempty"`               // Extracted text from OCR
	OCRSpacesKey string `gorm:"type:varchar(500)" json:"ocr_spaces_key,omitempty"` // Spaces key for OCR text file (used for KB indexing)

	// Relationships
	Subject    *Subject  `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	Semester   *Semester `gorm:"foreignKey:SemesterID;constraint:OnDelete:CASCADE" json:"semester,omitempty"`
	UploadedBy User      `gorm:"foreignKey:UploadedByUserID;constraint:OnDelete:SET NULL" json:"uploaded_by,omitempty"`
}
