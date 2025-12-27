package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// IndexingJobType represents the type of indexing job
type IndexingJobType string

const (
	IndexingJobTypeBatchPYQIngest  IndexingJobType = "batch_pyq_ingest"
	IndexingJobTypeDocumentUpload  IndexingJobType = "document_upload"
	IndexingJobTypeSyllabusExtract IndexingJobType = "syllabus_extraction"
	IndexingJobTypeAISetup         IndexingJobType = "ai_setup" // AI setup (KB + Agent + API Key) for subjects
)

// IndexingJobStatus represents the status of an indexing job
type IndexingJobStatus string

const (
	IndexingJobStatusPending    IndexingJobStatus = "pending"
	IndexingJobStatusProcessing IndexingJobStatus = "processing"
	IndexingJobStatusKBIndexing IndexingJobStatus = "kb_indexing" // Files uploaded to Spaces, waiting for KB indexing to complete
	IndexingJobStatusCompleted  IndexingJobStatus = "completed"
	IndexingJobStatusFailed     IndexingJobStatus = "failed"
	IndexingJobStatusPartial    IndexingJobStatus = "partially_completed"
	IndexingJobStatusCancelled  IndexingJobStatus = "cancelled"
)

// IndexingJobItemStatus represents the status of an individual item in a job
type IndexingJobItemStatus string

const (
	IndexingJobItemStatusPending     IndexingJobItemStatus = "pending"
	IndexingJobItemStatusDownloading IndexingJobItemStatus = "downloading"
	IndexingJobItemStatusOCR         IndexingJobItemStatus = "ocr" // Running OCR extraction
	IndexingJobItemStatusUploading   IndexingJobItemStatus = "uploading"
	IndexingJobItemStatusIndexing    IndexingJobItemStatus = "indexing"
	IndexingJobItemStatusCompleted   IndexingJobItemStatus = "completed"
	IndexingJobItemStatusFailed      IndexingJobItemStatus = "failed"
	IndexingJobItemStatusSkipped     IndexingJobItemStatus = "skipped"

	// AI Setup specific statuses
	IndexingJobItemStatusKBCreating     IndexingJobItemStatus = "kb_creating"     // Creating Knowledge Base
	IndexingJobItemStatusKBIndexing     IndexingJobItemStatus = "kb_indexing"     // Waiting for KB indexing
	IndexingJobItemStatusAgentCreating  IndexingJobItemStatus = "agent_creating"  // Creating Agent
	IndexingJobItemStatusAPIKeyCreating IndexingJobItemStatus = "apikey_creating" // Creating API Key
)

// IndexingJobItemType represents the type of item being processed
type IndexingJobItemType string

const (
	IndexingJobItemTypeExternalPDF IndexingJobItemType = "external_pdf"
	IndexingJobItemTypeLocalPDF    IndexingJobItemType = "local_pdf"
	IndexingJobItemTypeSubjectAI   IndexingJobItemType = "subject_ai" // AI setup for a subject
)

// IndexingJob tracks batch indexing operations
type IndexingJob struct {
	ID                uint              `gorm:"primaryKey" json:"id"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	DeletedAt         gorm.DeletedAt    `gorm:"index" json:"deleted_at,omitempty"`
	SubjectID         *uint             `gorm:"index" json:"subject_id,omitempty"` // Optional: for single-subject jobs (batch ingest, doc upload). Null for AI setup jobs.
	JobType           IndexingJobType   `gorm:"type:varchar(30);not null" json:"job_type"`
	Status            IndexingJobStatus `gorm:"type:varchar(25);default:'pending'" json:"status"`
	TotalItems        int               `gorm:"default:0" json:"total_items"`
	CompletedItems    int               `gorm:"default:0" json:"completed_items"`
	FailedItems       int               `gorm:"default:0" json:"failed_items"`
	DOIndexingJobUUID string            `gorm:"type:varchar(100)" json:"do_indexing_job_uuid,omitempty"` // DigitalOcean indexing job UUID
	CreatedByUserID   uint              `gorm:"index;not null" json:"created_by_user_id"`
	StartedAt         *time.Time        `json:"started_at,omitempty"`
	CompletedAt       *time.Time        `json:"completed_at,omitempty"`
	ErrorMessage      string            `gorm:"type:text" json:"error_message,omitempty"`

	// Relationships
	Subject   *Subject          `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	CreatedBy User              `gorm:"foreignKey:CreatedByUserID;constraint:OnDelete:CASCADE" json:"created_by,omitempty"`
	Items     []IndexingJobItem `gorm:"foreignKey:JobID;constraint:OnDelete:CASCADE" json:"items,omitempty"`
}

// IndexingJobItem tracks individual items within a batch job
type IndexingJobItem struct {
	ID           uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	JobID        uint                  `gorm:"index;not null" json:"job_id"`
	ItemType     IndexingJobItemType   `gorm:"type:varchar(20);not null" json:"item_type"`
	SourceURL    string                `gorm:"type:text" json:"source_url,omitempty"` // PDF URL (for external)
	DocumentID   *uint                 `gorm:"index" json:"document_id,omitempty"`    // Created document ID
	PYQPaperID   *uint                 `gorm:"index" json:"pyq_paper_id,omitempty"`   // Created PYQ paper ID
	SubjectID    *uint                 `gorm:"index" json:"subject_id,omitempty"`     // Subject ID (for AI setup jobs)
	Status       IndexingJobItemStatus `gorm:"type:varchar(20);default:'pending'" json:"status"`
	ErrorMessage string                `gorm:"type:text" json:"error_message,omitempty"`
	Metadata     datatypes.JSON        `gorm:"type:jsonb" json:"metadata,omitempty"` // {title, year, month, exam_type, source_name} or AISetupItemMetadata

	// Relationships
	Job      IndexingJob `gorm:"foreignKey:JobID;constraint:OnDelete:CASCADE" json:"-"`
	Document *Document   `gorm:"foreignKey:DocumentID;constraint:OnDelete:SET NULL" json:"document,omitempty"`
	PYQPaper *PYQPaper   `gorm:"foreignKey:PYQPaperID;constraint:OnDelete:SET NULL" json:"pyq_paper,omitempty"`
	Subject  *Subject    `gorm:"foreignKey:SubjectID;constraint:OnDelete:SET NULL" json:"subject,omitempty"`
}

// IndexingJobItemMetadata represents the metadata stored in IndexingJobItem.Metadata
type IndexingJobItemMetadata struct {
	Title      string `json:"title"`
	Year       int    `json:"year"`
	Month      string `json:"month"`
	ExamType   string `json:"exam_type,omitempty"`
	SourceName string `json:"source_name,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
}

// AISetupItemMetadata represents metadata for AI setup job items
type AISetupItemMetadata struct {
	SubjectName        string `json:"subject_name"`
	SubjectCode        string `json:"subject_code"`
	KnowledgeBaseUUID  string `json:"knowledge_base_uuid,omitempty"`
	AgentUUID          string `json:"agent_uuid,omitempty"`
	AgentDeploymentURL string `json:"agent_deployment_url,omitempty"`
	HasAPIKey          bool   `json:"has_api_key,omitempty"`
	Phase              string `json:"phase,omitempty"` // "kb", "agent", "apikey", "complete"
}

// GetProgress returns the progress percentage (0-100)
func (j *IndexingJob) GetProgress() int {
	if j.TotalItems == 0 {
		return 0
	}
	return ((j.CompletedItems + j.FailedItems) * 100) / j.TotalItems
}

// IsComplete returns true if the job has finished (success, failed, partial, or cancelled)
// Note: kb_indexing status is NOT complete - it's waiting for KB indexing to finish
func (j *IndexingJob) IsComplete() bool {
	return j.Status == IndexingJobStatusCompleted ||
		j.Status == IndexingJobStatusFailed ||
		j.Status == IndexingJobStatusPartial ||
		j.Status == IndexingJobStatusCancelled
}

// IsProcessing returns true if the job is still actively processing
func (j *IndexingJob) IsProcessing() bool {
	return j.Status == IndexingJobStatusPending ||
		j.Status == IndexingJobStatusProcessing ||
		j.Status == IndexingJobStatusKBIndexing
}
