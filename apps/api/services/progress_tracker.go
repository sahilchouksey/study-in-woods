package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/cache"
)

// TTL configurations for job states
const (
	JobStateTTLSuccess = 1 * time.Hour   // 1 hour for successful jobs
	JobStateTTLFailure = 24 * time.Hour  // 24 hours for failed jobs
	JobStateTTLPending = 24 * time.Hour  // 24 hours for pending/processing jobs
	JobLockTTL         = 5 * time.Minute // 5 minutes for processing lock
)

// SubjectSummary contains basic subject info for the completion event
type SubjectSummary struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Code    string `json:"code"`
	Credits int    `json:"credits"`
}

// ProgressEvent represents a progress update event sent to clients via SSE
type ProgressEvent struct {
	Type  string `json:"type"` // "started", "progress", "warning", "complete", "error", "info", "debug"
	JobID string `json:"job_id"`

	// Progress info
	Progress int    `json:"progress"` // 0-100
	Phase    string `json:"phase"`    // Current phase
	Message  string `json:"message"`  // User-friendly message

	// Chunk info (for extraction phase)
	TotalChunks     int `json:"total_chunks,omitempty"`
	CompletedChunks int `json:"completed_chunks,omitempty"`
	CurrentChunk    int `json:"current_chunk,omitempty"`

	// Detailed info for logs UI
	Detail        string `json:"detail,omitempty"`         // Additional detail (e.g., page range, chunk content preview)
	PageRange     string `json:"page_range,omitempty"`     // e.g., "pages 1-4"
	Duration      string `json:"duration,omitempty"`       // Human-readable duration
	BytesSize     int64  `json:"bytes_size,omitempty"`     // Size in bytes (for downloads)
	TokenCount    int    `json:"token_count,omitempty"`    // Token count for LLM calls
	SubjectsFound int    `json:"subjects_found,omitempty"` // Subjects found in chunk

	// Error info (for warning/error events)
	ErrorType    string `json:"error_type,omitempty"` // "network", "llm", "timeout", "database"
	ErrorMessage string `json:"error_message,omitempty"`
	RetryCount   int    `json:"retry_count,omitempty"`
	MaxRetries   int    `json:"max_retries,omitempty"`
	Recoverable  bool   `json:"recoverable,omitempty"`

	// Result info (for complete events)
	ResultSyllabusIDs []uint           `json:"result_syllabus_ids,omitempty"`
	ResultSubjects    []SubjectSummary `json:"result_subjects,omitempty"` // Actual subject data for UI

	// Database stats (for save phase)
	UnitsCreated  int `json:"units_created,omitempty"`
	TopicsCreated int `json:"topics_created,omitempty"`
	BooksCreated  int `json:"books_created,omitempty"`

	// Timing
	ElapsedMs int64     `json:"elapsed_ms,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ProgressCallback is a function that receives progress events
// Return an error to abort the extraction
type ProgressCallback func(ProgressEvent) error

// ErrorType represents the type of error that occurred
type ErrorType string

const (
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeLLM        ErrorType = "llm"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeDatabase   ErrorType = "database"
	ErrorTypePDF        ErrorType = "pdf"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeUnknown    ErrorType = "unknown"
)

// ProgressTracker manages extraction job state and progress updates
type ProgressTracker struct {
	cache *cache.RedisCache
}

// NewProgressTracker creates a new progress tracker instance
func NewProgressTracker(redisCache *cache.RedisCache) *ProgressTracker {
	return &ProgressTracker{cache: redisCache}
}

// CreateJob creates a new extraction job and marks it as active for the user
func (pt *ProgressTracker) CreateJob(ctx context.Context, userID, documentID uint) (*model.ExtractionJob, error) {
	// Generate job ID: {document_id}_{timestamp}
	jobID := fmt.Sprintf("%d_%d", documentID, time.Now().Unix())

	// Check if user has active job
	activeJobKey := fmt.Sprintf(model.RedisKeyActiveJob, userID)
	existingJobID, err := pt.cache.Get(ctx, activeJobKey)
	if err == nil && existingJobID != "" {
		// User has active job, return error with the existing job ID
		return nil, fmt.Errorf("user already has an active extraction job: %s", existingJobID)
	}

	// Create job
	job := &model.ExtractionJob{
		JobID:        jobID,
		UserID:       userID,
		DocumentID:   documentID,
		Status:       model.JobStatusPending,
		Progress:     0,
		CurrentPhase: "initializing",
		Message:      "Extraction queued",
		StartedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Save to Redis
	jobKey := fmt.Sprintf(model.RedisKeyJobState, jobID)
	if err := pt.cache.SetJSON(ctx, jobKey, job, JobStateTTLPending); err != nil {
		return nil, fmt.Errorf("failed to save job state: %w", err)
	}

	// Mark as active job for user
	if err := pt.cache.Set(ctx, activeJobKey, jobID, JobStateTTLPending); err != nil {
		// Clean up job state if we failed to set active job
		pt.cache.Delete(ctx, jobKey)
		return nil, fmt.Errorf("failed to mark job as active: %w", err)
	}

	return job, nil
}

// UpdateProgress updates the job state and returns the updated job
func (pt *ProgressTracker) UpdateProgress(ctx context.Context, jobID string, event ProgressEvent) error {
	// Get current job state
	job, err := pt.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	// Update job fields from event
	job.Progress = event.Progress
	job.CurrentPhase = event.Phase
	job.Message = event.Message
	job.UpdatedAt = time.Now()

	if event.TotalChunks > 0 {
		job.TotalChunks = event.TotalChunks
	}
	if event.CompletedChunks > 0 {
		job.CompletedChunks = event.CompletedChunks
	}

	// Update status based on event type
	switch event.Type {
	case "started":
		job.Status = model.JobStatusProcessing
	case "complete":
		job.Status = model.JobStatusCompleted
		now := time.Now()
		job.CompletedAt = &now
		if len(event.ResultSyllabusIDs) > 0 {
			job.ResultSyllabusIDs = event.ResultSyllabusIDs
		}
	case "error":
		job.Status = model.JobStatusFailed
		job.Error = event.ErrorMessage
		job.ErrorDetails = event.ErrorType
		now := time.Now()
		job.CompletedAt = &now
	case "warning":
		// Warning doesn't change status, but we track retry count
		if event.RetryCount > 0 {
			job.RetryCount = event.RetryCount
		}
	}

	// Determine TTL based on job status
	ttl := JobStateTTLPending
	if job.Status == model.JobStatusCompleted {
		ttl = JobStateTTLSuccess
	} else if job.Status == model.JobStatusFailed {
		ttl = JobStateTTLFailure
	}

	// Save updated state
	jobKey := fmt.Sprintf(model.RedisKeyJobState, jobID)
	if err := pt.cache.SetJSON(ctx, jobKey, job, ttl); err != nil {
		return fmt.Errorf("failed to update job state: %w", err)
	}

	// Clear active job if completed/failed
	if job.Status == model.JobStatusCompleted || job.Status == model.JobStatusFailed {
		activeJobKey := fmt.Sprintf(model.RedisKeyActiveJob, job.UserID)
		pt.cache.Delete(ctx, activeJobKey)
	}

	return nil
}

// GetJob retrieves job state from Redis
func (pt *ProgressTracker) GetJob(ctx context.Context, jobID string) (*model.ExtractionJob, error) {
	jobKey := fmt.Sprintf(model.RedisKeyJobState, jobID)

	var job model.ExtractionJob
	if err := pt.cache.GetJSON(ctx, jobKey, &job); err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			return nil, fmt.Errorf("job not found or expired: %s", jobID)
		}
		return nil, fmt.Errorf("failed to get job state: %w", err)
	}

	return &job, nil
}

// GetActiveJob returns the active job ID for a user (if any)
func (pt *ProgressTracker) GetActiveJob(ctx context.Context, userID uint) (string, error) {
	activeJobKey := fmt.Sprintf(model.RedisKeyActiveJob, userID)
	jobID, err := pt.cache.Get(ctx, activeJobKey)
	if err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			return "", nil // No active job
		}
		return "", err
	}
	return jobID, nil
}

// ClearActiveJob removes the active job reference for a user
func (pt *ProgressTracker) ClearActiveJob(ctx context.Context, userID uint) error {
	activeJobKey := fmt.Sprintf(model.RedisKeyActiveJob, userID)
	return pt.cache.Delete(ctx, activeJobKey)
}

// CancelJob cancels an active job
func (pt *ProgressTracker) CancelJob(ctx context.Context, jobID string) error {
	job, err := pt.GetJob(ctx, jobID)
	if err != nil {
		// Job doesn't exist, just clear the active job reference
		return err
	}

	// Only update status if job is still active
	if job.Status == model.JobStatusPending || job.Status == model.JobStatusProcessing {
		// Update job status to cancelled
		job.Status = model.JobStatusCancelled
		now := time.Now()
		job.CompletedAt = &now
		job.UpdatedAt = now
		job.Message = "Job cancelled by user"

		// Save updated state
		jobKey := fmt.Sprintf(model.RedisKeyJobState, jobID)
		if err := pt.cache.SetJSON(ctx, jobKey, job, JobStateTTLFailure); err != nil {
			return fmt.Errorf("failed to update job state: %w", err)
		}

		// Set cancellation flag so running operations can check it
		cancelKey := fmt.Sprintf("job:cancel:%s", jobID)
		pt.cache.Set(ctx, cancelKey, "1", 5*time.Minute)
	}

	// Clear active job
	activeJobKey := fmt.Sprintf(model.RedisKeyActiveJob, job.UserID)
	pt.cache.Delete(ctx, activeJobKey)

	return nil
}

// IsJobCancelled checks if a job has been cancelled
func (pt *ProgressTracker) IsJobCancelled(ctx context.Context, jobID string) bool {
	cancelKey := fmt.Sprintf("job:cancel:%s", jobID)
	val, err := pt.cache.Get(ctx, cancelKey)
	return err == nil && val == "1"
}

// SetJobResult stores the result syllabus IDs for a completed job
func (pt *ProgressTracker) SetJobResult(ctx context.Context, jobID string, syllabusIDs []uint) error {
	job, err := pt.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	job.ResultSyllabusIDs = syllabusIDs
	job.UpdatedAt = time.Now()

	jobKey := fmt.Sprintf(model.RedisKeyJobState, jobID)
	ttl := JobStateTTLSuccess
	if job.Status == model.JobStatusFailed {
		ttl = JobStateTTLFailure
	}

	return pt.cache.SetJSON(ctx, jobKey, job, ttl)
}

// ClassifyError classifies an error and determines if it's recoverable
func ClassifyError(err error) (ErrorType, bool) {
	if err == nil {
		return ErrorTypeUnknown, false
	}

	errStr := strings.ToLower(err.Error())

	// Network errors (recoverable)
	if strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dial") ||
		strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "reset by peer") {
		return ErrorTypeNetwork, true
	}

	// LLM API errors (recoverable)
	if strings.Contains(errStr, "inference api") ||
		strings.Contains(errStr, "status 429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "status 500") ||
		strings.Contains(errStr, "status 502") ||
		strings.Contains(errStr, "status 503") ||
		strings.Contains(errStr, "status 504") ||
		strings.Contains(errStr, "llm") {
		return ErrorTypeLLM, true
	}

	// Timeout errors (recoverable)
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context deadline") {
		return ErrorTypeTimeout, true
	}

	// Database errors (not recoverable)
	if strings.Contains(errStr, "database") ||
		strings.Contains(errStr, "transaction") ||
		strings.Contains(errStr, "sql") ||
		strings.Contains(errStr, "gorm") {
		return ErrorTypeDatabase, false
	}

	// PDF errors (not recoverable)
	if strings.Contains(errStr, "pdf") ||
		strings.Contains(errStr, "extract text") ||
		strings.Contains(errStr, "invalid document") {
		return ErrorTypePDF, false
	}

	// Validation errors (not recoverable)
	if strings.Contains(errStr, "validation") ||
		strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "required") {
		return ErrorTypeValidation, false
	}

	return ErrorTypeUnknown, false
}

// CalculateProgress calculates the overall progress percentage based on phase and chunk completion
func CalculateProgress(phase string, completedChunks, totalChunks int) int {
	switch phase {
	case "initializing":
		return 0
	case "download":
		return 5
	case "chunking":
		return 10
	case "extraction":
		if totalChunks == 0 {
			return 10
		}
		// Extraction phase: 10% - 70% (60% total, divided by chunk count)
		chunkIncrement := 60.0 / float64(totalChunks)
		progress := 10 + int(float64(completedChunks)*chunkIncrement)
		if progress > 70 {
			progress = 70
		}
		return progress
	case "merge":
		return 75
	case "save":
		return 95
	case "complete":
		return 100
	default:
		return 0
	}
}
