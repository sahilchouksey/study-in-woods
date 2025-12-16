package model

import "time"

// ExtractionJobStatus represents the status of an extraction job
type ExtractionJobStatus string

const (
	JobStatusPending    ExtractionJobStatus = "pending"
	JobStatusProcessing ExtractionJobStatus = "processing"
	JobStatusCompleted  ExtractionJobStatus = "completed"
	JobStatusFailed     ExtractionJobStatus = "failed"
	JobStatusCancelled  ExtractionJobStatus = "cancelled"
)

// ExtractionJob represents the state of a syllabus extraction job stored in Redis
type ExtractionJob struct {
	JobID        string              `json:"job_id"`
	UserID       uint                `json:"user_id"`
	DocumentID   uint                `json:"document_id"`
	Status       ExtractionJobStatus `json:"status"`
	Progress     int                 `json:"progress"`      // 0-100
	CurrentPhase string              `json:"current_phase"` // "download", "chunking", "extraction", "merge", "save"
	Message      string              `json:"message"`

	// Chunk tracking
	TotalChunks     int `json:"total_chunks,omitempty"`
	CompletedChunks int `json:"completed_chunks,omitempty"`
	FailedChunks    int `json:"failed_chunks,omitempty"`

	// Error tracking
	Error        string `json:"error,omitempty"`
	ErrorDetails string `json:"error_details,omitempty"`
	RetryCount   int    `json:"retry_count,omitempty"`

	// Result
	ResultSyllabusIDs []uint `json:"result_syllabus_ids,omitempty"`

	// Timestamps
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Redis key patterns for extraction jobs
const (
	// RedisKeyJobState stores the full job state as JSON
	// Usage: fmt.Sprintf(RedisKeyJobState, jobID)
	RedisKeyJobState = "job:state:%s"

	// RedisKeyActiveJob tracks the active job ID for a user
	// Usage: fmt.Sprintf(RedisKeyActiveJob, userID)
	RedisKeyActiveJob = "job:active:%d"

	// RedisKeyJobLock is used for distributed locking during job processing
	// Usage: fmt.Sprintf(RedisKeyJobLock, jobID)
	RedisKeyJobLock = "job:lock:%s"
)
