# Server-Sent Events (SSE) Implementation Plan
## Syllabus Extraction Progress Streaming

**Date**: December 14, 2025  
**Version**: 1.0  
**Status**: Ready for Implementation  
**Estimated Effort**: 10-15 hours

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Architecture Overview](#architecture-overview)
3. [Technical Specifications](#technical-specifications)
4. [Implementation Phases](#implementation-phases)
5. [Code Specifications](#code-specifications)
6. [Testing Strategy](#testing-strategy)
7. [Deployment Checklist](#deployment-checklist)
8. [Future Enhancements](#future-enhancements)

---

## Executive Summary

### Problem Statement

Currently, syllabus extraction takes **60-120 seconds** with no user feedback during the process. Users experience:
- No indication of progress
- Uncertainty about whether the request is processing
- No way to estimate completion time
- Poor user experience during long waits

### Solution

Implement **Server-Sent Events (SSE)** to stream real-time progress updates to the frontend during extraction.

### Key Benefits

| Metric | Current | With SSE | Improvement |
|--------|---------|----------|-------------|
| Time to First Feedback | 60-120s | <1s | **99%** ⭐ |
| User Engagement | Low | High | **Significant** |
| Perceived Wait Time | 60-120s | ~30-40s | **50-67%** |
| Error Visibility | After completion | Real-time | **Immediate** |

### Requirements Summary

✅ **Endpoint Design**: API v2 with `?stream=true` query parameter  
✅ **Progress Granularity**: Medium (15-20 events per extraction)  
✅ **Error Handling**: Retry up to 3 times, then fail with detailed error  
✅ **State Management**: Redis for active jobs, PostgreSQL for persistence  
✅ **Reconnection**: Support client reconnection with job ID  
✅ **Concurrency**: One active extraction per user at a time  
✅ **Job Cleanup**: TTL-based (1 hour success, 24 hours failure)  

---

## Architecture Overview

### System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Frontend (Browser)                           │
│                     EventSource API                             │
│  • Connects to SSE endpoint                                     │
│  • Receives progress events                                     │
│  • Updates UI in real-time                                      │
└──────────────────────┬──────────────────────────────────────────┘
                       │ HTTP GET /api/v2/documents/:id/extract-syllabus?stream=true
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│              Fiber Handler (HTTP Layer)                         │
│  • Validate user authentication                                 │
│  • Check for existing active job                                │
│  • Create extraction job in Redis                               │
│  • Set SSE headers                                              │
│  • Stream progress events via SetBodyStreamWriter               │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│           Job Tracker Service (State Management)                │
│  • Create job with unique ID (document_id_timestamp)            │
│  • Store job state in Redis                                     │
│  • Track active jobs per user                                   │
│  • Emit progress events via callback                            │
│  • Update job state on completion/failure                       │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│        Syllabus Service (Business Logic)                        │
│  • ExtractSyllabusWithProgress(progressCallback)                │
│  • Emit progress at key checkpoints:                            │
│    - Download PDF (5%)                                          │
│    - Calculate chunks (10%)                                     │
│    - Process each chunk (10-70%)                                │
│    - Merge results (75%)                                        │
│    - Save to database (95%)                                     │
│  • Handle errors with retry logic                               │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│      Chunked Extractor (Parallel Processing)                    │
│  • Process chunks in parallel (max 10 concurrent)               │
│  • Retry failed chunks (max 3 attempts)                         │
│  • Emit progress per chunk completion                           │
│  • Exponential backoff: 5s, 7.5s, 11.25s                        │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Redis (State Storage)                          │
│  • job:state:{job_id} - Full job state (JSON)                  │
│  • job:active:{user_id} - Active job ID for user               │
│  • TTL: 1 hour (success), 24 hours (failure)                   │
└─────────────────────────────────────────────────────────────────┘
```

### Data Flow

```
1. Client → POST /api/v2/documents/123/extract-syllabus?stream=true
2. Handler → Check user has no active job
3. Handler → Create job in Redis (job_id: "123_1734181800")
4. Handler → Start SSE stream
5. Handler → Call service.ExtractWithProgress(progressCallback)
6. Service → Emit "started" event (0%)
7. Service → Download PDF → Emit "progress" (5%)
8. Service → Calculate chunks → Emit "progress" (10%)
9. Service → Process chunk 1 → Emit "progress" (22%)
10. Service → Process chunk 2 → Emit "progress" (34%)
    ... (continue for all chunks)
11. Service → Merge results → Emit "progress" (75%)
12. Service → Save to DB → Emit "progress" (95%)
13. Service → Emit "complete" event (100%)
14. Handler → Close SSE stream
15. Redis → Auto-delete job after 1 hour
```

### Component Interactions

```
┌──────────────┐
│   Frontend   │
└──────┬───────┘
       │ EventSource
       ▼
┌──────────────────────────────────────────────────────────┐
│                    Fiber Handler                         │
│  ┌────────────────────────────────────────────────┐     │
│  │ SetBodyStreamWriter(func(w *bufio.Writer) {    │     │
│  │   jobTracker.CreateJob()                       │     │
│  │   service.ExtractWithProgress(callback)        │     │
│  │ })                                              │     │
│  └────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────┐
│                  Job Tracker                             │
│  • CreateJob(userID, documentID) → jobID                │
│  • UpdateProgress(jobID, progress)                       │
│  • GetJobState(jobID) → JobState                        │
│  • CheckActiveJob(userID) → jobID or nil                │
└──────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────┐
│                     Redis                                │
│  job:state:123_1734181800 → {                           │
│    "job_id": "123_1734181800",                          │
│    "user_id": 456,                                       │
│    "document_id": 123,                                   │
│    "status": "processing",                               │
│    "progress": 45,                                       │
│    "message": "Processing chunk 3 of 6...",             │
│    "started_at": "2025-12-14T10:30:00Z"                 │
│  }                                                       │
│                                                          │
│  job:active:456 → "123_1734181800"                      │
└──────────────────────────────────────────────────────────┘
```

---

## Technical Specifications

### 3.1 Data Models

#### Job State Model

```go
// model/extraction_job.go
package model

import "time"

type ExtractionJobStatus string

const (
    JobStatusPending    ExtractionJobStatus = "pending"
    JobStatusProcessing ExtractionJobStatus = "processing"
    JobStatusCompleted  ExtractionJobStatus = "completed"
    JobStatusFailed     ExtractionJobStatus = "failed"
    JobStatusCancelled  ExtractionJobStatus = "cancelled"
)

type ExtractionJob struct {
    JobID        string              `json:"job_id"`
    UserID       uint                `json:"user_id"`
    DocumentID   uint                `json:"document_id"`
    Status       ExtractionJobStatus `json:"status"`
    Progress     int                 `json:"progress"`      // 0-100
    CurrentPhase string              `json:"current_phase"` // "download", "chunking", "extraction", "merge", "save"
    Message      string              `json:"message"`
    
    // Chunk tracking
    TotalChunks    int `json:"total_chunks,omitempty"`
    CompletedChunks int `json:"completed_chunks,omitempty"`
    FailedChunks   int `json:"failed_chunks,omitempty"`
    
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
```

#### Progress Event Model

```go
// services/progress_tracker.go
package services

type ProgressEvent struct {
    Type    string `json:"type"`    // "started", "progress", "warning", "complete", "error"
    JobID   string `json:"job_id"`
    
    // Progress info
    Progress     int    `json:"progress"`      // 0-100
    Phase        string `json:"phase"`         // Current phase
    Message      string `json:"message"`       // User-friendly message
    
    // Chunk info (for extraction phase)
    TotalChunks    int `json:"total_chunks,omitempty"`
    CompletedChunks int `json:"completed_chunks,omitempty"`
    CurrentChunk   int `json:"current_chunk,omitempty"`
    
    // Error info (for warning/error events)
    ErrorType    string `json:"error_type,omitempty"`    // "network", "llm", "timeout", "database"
    ErrorMessage string `json:"error_message,omitempty"`
    RetryCount   int    `json:"retry_count,omitempty"`
    MaxRetries   int    `json:"max_retries,omitempty"`
    Recoverable  bool   `json:"recoverable,omitempty"`
    
    // Timing
    ElapsedMs int64     `json:"elapsed_ms,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}
```

#### Redis Key Structure

```go
// Redis key naming conventions
const (
    // Job state storage (JSON)
    RedisKeyJobState = "job:state:%s"           // job:state:{job_id}
    
    // Active job tracking (string - job_id)
    RedisKeyActiveJob = "job:active:%d"         // job:active:{user_id}
    
    // Job lock (for distributed processing)
    RedisKeyJobLock = "job:lock:%s"             // job:lock:{job_id}
)

// TTL configurations
const (
    JobStateTTLSuccess = 1 * time.Hour          // 1 hour for successful jobs
    JobStateTTLFailure = 24 * time.Hour         // 24 hours for failed jobs
    JobLockTTL         = 5 * time.Minute        // 5 minutes for processing lock
)
```

### 3.2 API Specifications

#### Endpoint: Start Extraction with Streaming

**Request:**
```
POST /api/v2/documents/:document_id/extract-syllabus?stream=true
Authorization: Bearer <jwt_token>
```

**Response Headers:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
Transfer-Encoding: chunked
X-Accel-Buffering: no
```

**SSE Event Stream:**

```
event: started
data: {"type":"started","job_id":"123_1734181800","progress":0,"phase":"initializing","message":"Starting extraction...","timestamp":"2025-12-14T10:30:00Z"}

event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":5,"phase":"download","message":"Downloading PDF from storage...","timestamp":"2025-12-14T10:30:01Z"}

event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":10,"phase":"chunking","message":"Analyzing document structure...","total_chunks":6,"timestamp":"2025-12-14T10:30:03Z"}

event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":22,"phase":"extraction","message":"Processing chunk 1 of 6...","total_chunks":6,"completed_chunks":1,"current_chunk":1,"timestamp":"2025-12-14T10:30:15Z"}

event: warning
data: {"type":"warning","job_id":"123_1734181800","progress":34,"phase":"extraction","message":"Chunk 2 failed, retrying (attempt 1/3)...","error_type":"llm_timeout","error_message":"LLM request timed out after 60s","retry_count":1,"max_retries":3,"recoverable":true,"timestamp":"2025-12-14T10:31:20Z"}

event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":34,"phase":"extraction","message":"Processing chunk 2 of 6 (retry successful)...","total_chunks":6,"completed_chunks":2,"current_chunk":2,"timestamp":"2025-12-14T10:31:30Z"}

event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":75,"phase":"merge","message":"Merging extracted content...","timestamp":"2025-12-14T10:32:00Z"}

event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":95,"phase":"save","message":"Saving to database...","timestamp":"2025-12-14T10:32:05Z"}

event: complete
data: {"type":"complete","job_id":"123_1734181800","progress":100,"phase":"complete","message":"Extraction completed successfully","result_syllabus_ids":[456,457,458],"elapsed_ms":125000,"timestamp":"2025-12-14T10:32:10Z"}
```

**Error Event (Fatal):**
```
event: error
data: {"type":"error","job_id":"123_1734181800","progress":45,"phase":"extraction","message":"Extraction failed after maximum retries","error_type":"llm_timeout","error_message":"Chunk 3 failed after 3 retry attempts","recoverable":false,"timestamp":"2025-12-14T10:32:00Z"}
```

#### Endpoint: Non-Streaming (Backward Compatible)

**Request:**
```
POST /api/v2/documents/:document_id/extract-syllabus
Authorization: Bearer <jwt_token>
```

**Response:**
```json
{
  "success": true,
  "message": "Extraction started",
  "data": {
    "job_id": "123_1734181800",
    "status": "processing",
    "progress": 0,
    "message": "Extraction in progress..."
  }
}
```

#### Endpoint: Get Job Status

**Request:**
```
GET /api/v2/extraction-jobs/:job_id
Authorization: Bearer <jwt_token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "job_id": "123_1734181800",
    "user_id": 456,
    "document_id": 123,
    "status": "processing",
    "progress": 45,
    "current_phase": "extraction",
    "message": "Processing chunk 3 of 6...",
    "total_chunks": 6,
    "completed_chunks": 2,
    "started_at": "2025-12-14T10:30:00Z",
    "updated_at": "2025-12-14T10:31:00Z"
  }
}
```

#### Endpoint: Reconnect to Existing Job

**Request:**
```
GET /api/v2/extraction-jobs/:job_id/stream
Authorization: Bearer <jwt_token>
```

**Behavior:**
- If job is still processing: Resume streaming from current state
- If job is completed: Send final `complete` event immediately
- If job is failed: Send `error` event immediately
- If job not found or expired: Return 404

### 3.3 Progress Calculation Formula

#### Overall Progress Breakdown (100%)

```
Phase 1: Download PDF           →  0% -  5%  (5% total)
Phase 2: Calculate Chunks       →  5% - 10%  (5% total)
Phase 3: Extract Chunks         → 10% - 70%  (60% total, divided by chunk count)
Phase 4: Merge Results          → 70% - 75%  (5% total)
Phase 5: Save to Database       → 75% - 95%  (20% total)
Phase 6: Complete               → 95% - 100% (5% total)
```

#### Per-Chunk Progress Calculation

```go
// For N chunks, each chunk contributes: 60% / N
chunkProgressIncrement := 60.0 / float64(totalChunks)

// Progress for chunk completion:
progress := 10 + (completedChunks * chunkProgressIncrement)

// Example: 6 chunks
// Chunk 1 complete: 10 + (1 * 10) = 20%
// Chunk 2 complete: 10 + (2 * 10) = 30%
// Chunk 3 complete: 10 + (3 * 10) = 40%
// Chunk 4 complete: 10 + (4 * 10) = 50%
// Chunk 5 complete: 10 + (5 * 10) = 60%
// Chunk 6 complete: 10 + (6 * 10) = 70%
```

#### Variable Chunk Count Handling

```go
func calculateProgress(phase string, completedChunks, totalChunks int) int {
    switch phase {
    case "download":
        return 5
    case "chunking":
        return 10
    case "extraction":
        if totalChunks == 0 {
            return 10
        }
        chunkIncrement := 60.0 / float64(totalChunks)
        return 10 + int(float64(completedChunks)*chunkIncrement)
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
```

### 3.4 Error Handling & Retry Logic

#### Retry Configuration

```go
// config/config.go
type ExtractionRetryConfig struct {
    MaxRetries              int           `env:"EXTRACTION_MAX_RETRIES" envDefault:"3"`
    InitialBackoffSeconds   int           `env:"EXTRACTION_RETRY_DELAY_SECONDS" envDefault:"5"`
    BackoffMultiplier       float64       `env:"EXTRACTION_RETRY_BACKOFF_MULTIPLIER" envDefault:"1.5"`
    MaxBackoffSeconds       int           `env:"EXTRACTION_MAX_BACKOFF_SECONDS" envDefault:"30"`
    ChunkTimeoutSeconds     int           `env:"EXTRACTION_CHUNK_TIMEOUT_SECONDS" envDefault:"180"`
}
```

#### Retry Algorithm

```go
func retryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
    backoff := time.Duration(config.InitialBackoffSeconds) * time.Second
    maxBackoff := time.Duration(config.MaxBackoffSeconds) * time.Second
    
    for attempt := 1; attempt <= config.MaxRetries; attempt++ {
        // Emit retry event
        emitProgress(ProgressEvent{
            Type:         "warning",
            Message:      fmt.Sprintf("Retrying (attempt %d/%d)...", attempt, config.MaxRetries),
            RetryCount:   attempt,
            MaxRetries:   config.MaxRetries,
            Recoverable:  true,
        })
        
        // Try operation
        err := fn()
        if err == nil {
            return nil // Success
        }
        
        // Check if error is recoverable
        if !isRecoverable(err) {
            return err // Don't retry non-recoverable errors
        }
        
        // Check context cancellation
        if ctx.Err() != nil {
            return ctx.Err()
        }
        
        // Wait before retry (if not last attempt)
        if attempt < config.MaxRetries {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(backoff):
                // Calculate next backoff with exponential increase
                backoff = time.Duration(float64(backoff) * config.BackoffMultiplier)
                if backoff > maxBackoff {
                    backoff = maxBackoff
                }
            }
        }
    }
    
    // All retries exhausted
    return fmt.Errorf("operation failed after %d retries", config.MaxRetries)
}
```

#### Error Classification

```go
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

func classifyError(err error) (ErrorType, bool) {
    errStr := err.Error()
    
    // Network errors (recoverable)
    if strings.Contains(errStr, "connection") ||
       strings.Contains(errStr, "network") ||
       strings.Contains(errStr, "dial") {
        return ErrorTypeNetwork, true
    }
    
    // LLM API errors (recoverable)
    if strings.Contains(errStr, "inference API") ||
       strings.Contains(errStr, "status 429") ||
       strings.Contains(errStr, "status 500") ||
       strings.Contains(errStr, "status 503") {
        return ErrorTypeLLM, true
    }
    
    // Timeout errors (recoverable)
    if errors.Is(err, context.DeadlineExceeded) ||
       strings.Contains(errStr, "timeout") {
        return ErrorTypeTimeout, true
    }
    
    // Database errors (not recoverable)
    if strings.Contains(errStr, "database") ||
       strings.Contains(errStr, "transaction") {
        return ErrorTypeDatabase, false
    }
    
    // PDF errors (not recoverable)
    if strings.Contains(errStr, "PDF") ||
       strings.Contains(errStr, "extract text") {
        return ErrorTypePDF, false
    }
    
    // Validation errors (not recoverable)
    if strings.Contains(errStr, "validation") ||
       strings.Contains(errStr, "invalid") {
        return ErrorTypeValidation, false
    }
    
    return ErrorTypeUnknown, false
}
```

---

## Implementation Phases

### Phase 1: Foundation (3-4 hours)

#### 1.1 Create Data Models

**File**: `apps/api/model/extraction_job.go`

```go
package model

import "time"

type ExtractionJobStatus string

const (
    JobStatusPending    ExtractionJobStatus = "pending"
    JobStatusProcessing ExtractionJobStatus = "processing"
    JobStatusCompleted  ExtractionJobStatus = "completed"
    JobStatusFailed     ExtractionJobStatus = "failed"
    JobStatusCancelled  ExtractionJobStatus = "cancelled"
)

type ExtractionJob struct {
    JobID        string              `json:"job_id"`
    UserID       uint                `json:"user_id"`
    DocumentID   uint                `json:"document_id"`
    Status       ExtractionJobStatus `json:"status"`
    Progress     int                 `json:"progress"`
    CurrentPhase string              `json:"current_phase"`
    Message      string              `json:"message"`
    
    TotalChunks     int `json:"total_chunks,omitempty"`
    CompletedChunks int `json:"completed_chunks,omitempty"`
    FailedChunks    int `json:"failed_chunks,omitempty"`
    
    Error        string `json:"error,omitempty"`
    ErrorDetails string `json:"error_details,omitempty"`
    RetryCount   int    `json:"retry_count,omitempty"`
    
    ResultSyllabusIDs []uint `json:"result_syllabus_ids,omitempty"`
    
    StartedAt   time.Time  `json:"started_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
    UpdatedAt   time.Time  `json:"updated_at"`
}
```

#### 1.2 Create Progress Tracker Service

**File**: `apps/api/services/progress_tracker.go`

```go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/sahilchouksey/go-init-setup/model"
    "github.com/sahilchouksey/go-init-setup/utils/cache"
)

type ProgressEvent struct {
    Type    string `json:"type"`
    JobID   string `json:"job_id"`
    
    Progress     int    `json:"progress"`
    Phase        string `json:"phase"`
    Message      string `json:"message"`
    
    TotalChunks     int `json:"total_chunks,omitempty"`
    CompletedChunks int `json:"completed_chunks,omitempty"`
    CurrentChunk    int `json:"current_chunk,omitempty"`
    
    ErrorType    string `json:"error_type,omitempty"`
    ErrorMessage string `json:"error_message,omitempty"`
    RetryCount   int    `json:"retry_count,omitempty"`
    MaxRetries   int    `json:"max_retries,omitempty"`
    Recoverable  bool   `json:"recoverable,omitempty"`
    
    ElapsedMs int64     `json:"elapsed_ms,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

type ProgressCallback func(ProgressEvent) error

type ProgressTracker struct {
    cache *cache.RedisCache
}

func NewProgressTracker(cache *cache.RedisCache) *ProgressTracker {
    return &ProgressTracker{cache: cache}
}

// CreateJob creates a new extraction job
func (pt *ProgressTracker) CreateJob(ctx context.Context, userID, documentID uint) (*model.ExtractionJob, error) {
    // Generate job ID: {document_id}_{timestamp}
    jobID := fmt.Sprintf("%d_%d", documentID, time.Now().Unix())
    
    // Check if user has active job
    activeJobKey := fmt.Sprintf("job:active:%d", userID)
    existingJobID, err := pt.cache.Get(ctx, activeJobKey)
    if err == nil && existingJobID != "" {
        // User has active job, return error
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
    jobKey := fmt.Sprintf("job:state:%s", jobID)
    if err := pt.cache.SetJSON(ctx, jobKey, job, 24*time.Hour); err != nil {
        return nil, fmt.Errorf("failed to save job state: %w", err)
    }
    
    // Mark as active job for user
    if err := pt.cache.Set(ctx, activeJobKey, jobID, 24*time.Hour); err != nil {
        return nil, fmt.Errorf("failed to mark job as active: %w", err)
    }
    
    return job, nil
}

// UpdateProgress updates job progress and emits event
func (pt *ProgressTracker) UpdateProgress(ctx context.Context, jobID string, event ProgressEvent) error {
    // Get current job state
    job, err := pt.GetJob(ctx, jobID)
    if err != nil {
        return err
    }
    
    // Update job fields
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
    case "error":
        job.Status = model.JobStatusFailed
        job.Error = event.ErrorMessage
        now := time.Now()
        job.CompletedAt = &now
    }
    
    // Save updated state
    jobKey := fmt.Sprintf("job:state:%s", jobID)
    ttl := 1 * time.Hour
    if job.Status == model.JobStatusFailed {
        ttl = 24 * time.Hour
    }
    
    if err := pt.cache.SetJSON(ctx, jobKey, job, ttl); err != nil {
        return fmt.Errorf("failed to update job state: %w", err)
    }
    
    // Clear active job if completed/failed
    if job.Status == model.JobStatusCompleted || job.Status == model.JobStatusFailed {
        activeJobKey := fmt.Sprintf("job:active:%d", job.UserID)
        pt.cache.Delete(ctx, activeJobKey)
    }
    
    return nil
}

// GetJob retrieves job state from Redis
func (pt *ProgressTracker) GetJob(ctx context.Context, jobID string) (*model.ExtractionJob, error) {
    jobKey := fmt.Sprintf("job:state:%s", jobID)
    
    var job model.ExtractionJob
    if err := pt.cache.GetJSON(ctx, jobKey, &job); err != nil {
        if err == cache.ErrNotFound {
            return nil, fmt.Errorf("job not found or expired")
        }
        return nil, fmt.Errorf("failed to get job state: %w", err)
    }
    
    return &job, nil
}

// GetActiveJob returns the active job ID for a user (if any)
func (pt *ProgressTracker) GetActiveJob(ctx context.Context, userID uint) (string, error) {
    activeJobKey := fmt.Sprintf("job:active:%d", userID)
    jobID, err := pt.cache.Get(ctx, activeJobKey)
    if err != nil {
        if err == cache.ErrNotFound {
            return "", nil // No active job
        }
        return "", err
    }
    return jobID, nil
}
```

#### 1.3 Update Configuration

**File**: `apps/api/config/config.go`

Add retry configuration:

```go
type EnviornmentVariable struct {
    // ... existing fields ...
    
    // Extraction Retry Configuration
    EXTRACTION_MAX_RETRIES              int     `env:"EXTRACTION_MAX_RETRIES" envDefault:"3"`
    EXTRACTION_RETRY_DELAY_SECONDS      int     `env:"EXTRACTION_RETRY_DELAY_SECONDS" envDefault:"5"`
    EXTRACTION_RETRY_BACKOFF_MULTIPLIER float64 `env:"EXTRACTION_RETRY_BACKOFF_MULTIPLIER" envDefault:"1.5"`
    EXTRACTION_MAX_BACKOFF_SECONDS      int     `env:"EXTRACTION_MAX_BACKOFF_SECONDS" envDefault:"30"`
    EXTRACTION_CHUNK_TIMEOUT_SECONDS    int     `env:"EXTRACTION_CHUNK_TIMEOUT_SECONDS" envDefault:"180"`
    
    // Job State Configuration
    EXTRACTION_JOB_TTL_SUCCESS_HOURS int `env:"EXTRACTION_JOB_TTL_SUCCESS_HOURS" envDefault:"1"`
    EXTRACTION_JOB_TTL_FAILURE_HOURS int `env:"EXTRACTION_JOB_TTL_FAILURE_HOURS" envDefault:"24"`
}
```

**Success Criteria:**
- [ ] `model/extraction_job.go` created with all fields
- [ ] `services/progress_tracker.go` created with CRUD operations
- [ ] Configuration updated with retry settings
- [ ] Unit tests pass: `go test ./services -run TestProgressTracker`
- [ ] Redis integration tested manually

---

### Phase 2: Service Layer Refactoring (4-5 hours)

#### 2.1 Add Progress Callback to Syllabus Service

**File**: `apps/api/services/syllabus_service.go`

Add new method with progress callback:

```go
// ExtractSyllabusWithProgress extracts syllabus with real-time progress updates
func (s *SyllabusService) ExtractSyllabusWithProgress(
    ctx context.Context,
    documentID uint,
    progressCallback ProgressCallback,
) ([]*model.Syllabus, error) {
    startTime := time.Now()
    
    // Emit started event
    if err := progressCallback(ProgressEvent{
        Type:      "started",
        Progress:  0,
        Phase:     "initializing",
        Message:   "Starting syllabus extraction...",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    // Check AI enabled
    if s.doInferenceClient == nil {
        return nil, fmt.Errorf("AI extraction not enabled")
    }
    
    // Get document
    var document model.Document
    if err := s.db.Preload("Subject").First(&document, documentID).Error; err != nil {
        return nil, fmt.Errorf("failed to fetch document: %w", err)
    }
    
    if document.Type != model.DocumentTypeSyllabus {
        return nil, fmt.Errorf("document is not a syllabus type")
    }
    
    // Delete existing syllabus data
    if err := s.deleteExistingSyllabusDataForSubject(document.SubjectID); err != nil {
        return nil, fmt.Errorf("failed to delete existing syllabus: %w", err)
    }
    
    // Download PDF
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  2,
        Phase:     "download",
        Message:   "Downloading PDF from storage...",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    pdfContent, err := s.spacesClient.DownloadFile(ctx, document.FileURL)
    if err != nil {
        return nil, fmt.Errorf("failed to download PDF: %w", err)
    }
    
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  5,
        Phase:     "download",
        Message:   "PDF downloaded successfully",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    // Get page count
    pageCount, err := s.pdfExtractor.GetPageCount(pdfContent)
    if err != nil {
        return nil, fmt.Errorf("failed to get page count: %w", err)
    }
    
    // Emit chunking progress
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  10,
        Phase:     "chunking",
        Message:   fmt.Sprintf("Analyzing document (%d pages)...", pageCount),
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    var syllabuses []*model.Syllabus
    
    // Choose extraction strategy
    if pageCount <= SmallPDFThreshold {
        // Direct extraction (small PDFs)
        syllabuses, err = s.extractDirectlyWithProgress(ctx, &document, pdfContent, progressCallback)
    } else {
        // Chunked extraction (large PDFs)
        syllabuses, err = s.chunkedExtractor.ExtractSyllabusChunkedWithProgress(
            ctx, 
            &document, 
            pdfContent,
            progressCallback,
        )
    }
    
    if err != nil {
        return nil, err
    }
    
    // Emit completion
    elapsed := time.Since(startTime).Milliseconds()
    if err := progressCallback(ProgressEvent{
        Type:      "complete",
        Progress:  100,
        Phase:     "complete",
        Message:   fmt.Sprintf("Extraction completed successfully (%d subjects)", len(syllabuses)),
        ElapsedMs: elapsed,
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    return syllabuses, nil
}

// extractDirectlyWithProgress - direct extraction with progress
func (s *SyllabusService) extractDirectlyWithProgress(
    ctx context.Context,
    document *model.Document,
    pdfContent []byte,
    progressCallback ProgressCallback,
) ([]*model.Syllabus, error) {
    // Extract text
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  15,
        Phase:     "extraction",
        Message:   "Extracting text from PDF...",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    text, err := s.pdfExtractor.ExtractText(pdfContent)
    if err != nil {
        return nil, fmt.Errorf("failed to extract text: %w", err)
    }
    
    // Call LLM
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  30,
        Phase:     "extraction",
        Message:   "Processing with AI...",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    extractedData, err := s.extractWithLLM(ctx, text)
    if err != nil {
        return nil, err
    }
    
    // Save to database
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  75,
        Phase:     "save",
        Message:   "Saving to database...",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    syllabuses, err := s.saveMultiSubjectSyllabusData(document, extractedData)
    if err != nil {
        return nil, err
    }
    
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  95,
        Phase:     "save",
        Message:   "Database save complete",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    return syllabuses, nil
}
```

#### 2.2 Update Chunked Extractor with Progress

**File**: `apps/api/services/chunked_syllabus_extractor.go`

Add progress callback parameter:

```go
// ExtractSyllabusChunkedWithProgress - chunked extraction with progress
func (c *ChunkedSyllabusExtractor) ExtractSyllabusChunkedWithProgress(
    ctx context.Context,
    document *model.Document,
    pdfContent []byte,
    progressCallback ProgressCallback,
) ([]*model.Syllabus, error) {
    // Get page count
    pageCount, err := c.pdfExtractor.GetPageCount(pdfContent)
    if err != nil {
        return nil, fmt.Errorf("failed to get page count: %w", err)
    }
    
    // Calculate chunks
    chunks := c.calculateChunks(pageCount)
    
    if err := progressCallback(ProgressEvent{
        Type:         "progress",
        Progress:     10,
        Phase:        "chunking",
        Message:      fmt.Sprintf("Processing %d chunks in parallel...", len(chunks)),
        TotalChunks:  len(chunks),
        Timestamp:    time.Now(),
    }); err != nil {
        return nil, err
    }
    
    // Process chunks with progress updates
    chunkResults := c.processChunksParallelWithProgress(ctx, pdfContent, chunks, pageCount, progressCallback)
    
    // Check failure rate
    failedChunks := 0
    for _, result := range chunkResults {
        if result.Error != nil {
            failedChunks++
        }
    }
    
    if failedChunks == len(chunks) {
        return nil, fmt.Errorf("all chunks failed to extract")
    }
    
    failureRate := float64(failedChunks) / float64(len(chunks)) * 100
    if failureRate > 50 {
        return nil, fmt.Errorf("extraction failed for too many chunks (%.1f%%)", failureRate)
    }
    
    // Merge results
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  70,
        Phase:     "merge",
        Message:   "Merging extracted content...",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    mergedSubjects, err := c.mergeAndDeduplicate(ctx, chunkResults)
    if err != nil {
        return nil, fmt.Errorf("failed to merge chunk results: %w", err)
    }
    
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  75,
        Phase:     "merge",
        Message:   "Merge complete",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    // Save to database
    if err := progressCallback(ProgressEvent{
        Type:      "progress",
        Progress:  80,
        Phase:     "save",
        Message:   "Saving to database...",
        Timestamp: time.Now(),
    }); err != nil {
        return nil, err
    }
    
    extractedData := SyllabusExtractionResult{Subjects: mergedSubjects}
    syllabuses, err := c.saveMultiSubjectSyllabusData(document, &extractedData, progressCallback)
    if err != nil {
        return nil, fmt.Errorf("failed to save syllabus data: %w", err)
    }
    
    return syllabuses, nil
}

// processChunksParallelWithProgress - process chunks with progress updates
func (c *ChunkedSyllabusExtractor) processChunksParallelWithProgress(
    ctx context.Context,
    pdfContent []byte,
    chunks []PageRange,
    totalPages int,
    progressCallback ProgressCallback,
) []ChunkResult {
    results := make([]ChunkResult, len(chunks))
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, c.maxConcurrent)
    
    completedChunks := 0
    var mu sync.Mutex
    
    for idx, chunk := range chunks {
        wg.Add(1)
        go func(chunkIndex int, pageRange PageRange) {
            defer wg.Done()
            
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            // Extract chunk with retry
            result := c.extractChunkWithRetryAndProgress(
                ctx, 
                pdfContent, 
                pageRange, 
                totalPages, 
                chunkIndex,
                progressCallback,
            )
            
            results[chunkIndex] = result
            
            // Update progress
            mu.Lock()
            completedChunks++
            progress := 10 + int(float64(completedChunks)/float64(len(chunks))*60)
            mu.Unlock()
            
            // Emit progress event
            progressCallback(ProgressEvent{
                Type:            "progress",
                Progress:        progress,
                Phase:           "extraction",
                Message:         fmt.Sprintf("Completed chunk %d of %d", completedChunks, len(chunks)),
                TotalChunks:     len(chunks),
                CompletedChunks: completedChunks,
                CurrentChunk:    chunkIndex + 1,
                Timestamp:       time.Now(),
            })
        }(idx, chunk)
    }
    
    wg.Wait()
    return results
}

// extractChunkWithRetryAndProgress - retry with progress events
func (c *ChunkedSyllabusExtractor) extractChunkWithRetryAndProgress(
    ctx context.Context,
    pdfContent []byte,
    pageRange PageRange,
    totalPages int,
    chunkIndex int,
    progressCallback ProgressCallback,
) ChunkResult {
    var result ChunkResult
    result.ChunkIndex = chunkIndex
    result.PageRange = pageRange
    
    backoff := 5 * time.Second
    maxBackoff := 30 * time.Second
    backoffMultiplier := 1.5
    
    for attempt := 1; attempt <= c.maxRetries; attempt++ {
        result.Retries = attempt
        
        // Create timeout context
        chunkCtx, cancel := context.WithTimeout(ctx, c.chunkTimeout)
        
        // Try extraction
        subjects, rawResponse, err := c.extractChunk(chunkCtx, pdfContent, pageRange, totalPages)
        cancel()
        
        if err == nil {
            result.Subjects = subjects
            result.RawResponse = rawResponse
            result.Error = nil
            return result
        }
        
        result.Error = err
        
        // Check context cancellation
        if ctx.Err() != nil {
            result.Error = ctx.Err()
            return result
        }
        
        // Classify error
        errorType, recoverable := classifyError(err)
        
        // Emit warning event
        if attempt < c.maxRetries {
            progressCallback(ProgressEvent{
                Type:         "warning",
                Progress:     10 + int(float64(chunkIndex)/float64(6)*60), // Approximate
                Phase:        "extraction",
                Message:      fmt.Sprintf("Chunk %d failed, retrying (attempt %d/%d)...", chunkIndex+1, attempt, c.maxRetries),
                ErrorType:    string(errorType),
                ErrorMessage: err.Error(),
                RetryCount:   attempt,
                MaxRetries:   c.maxRetries,
                Recoverable:  recoverable,
                Timestamp:    time.Now(),
            })
            
            // Wait before retry
            select {
            case <-ctx.Done():
                result.Error = ctx.Err()
                return result
            case <-time.After(backoff):
                // Calculate next backoff
                backoff = time.Duration(float64(backoff) * backoffMultiplier)
                if backoff > maxBackoff {
                    backoff = maxBackoff
                }
            }
        }
    }
    
    return result
}
```

**Success Criteria:**
- [ ] `ExtractSyllabusWithProgress()` method added to syllabus service
- [ ] `ExtractSyllabusChunkedWithProgress()` method added to chunked extractor
- [ ] Progress events emitted at all key checkpoints
- [ ] Retry logic emits warning events
- [ ] Unit tests pass: `go test ./services -run TestExtractWithProgress`
- [ ] Integration test with mock callback succeeds

---

### Phase 3: SSE Handler Implementation (3-4 hours)

#### 3.1 Create SSE Helper Utilities

**File**: `apps/api/utils/sse/sse.go`

```go
package sse

import (
    "bufio"
    "encoding/json"
    "fmt"
)

// Event represents an SSE event
type Event struct {
    Event string      `json:"-"`
    Data  interface{} `json:"data"`
}

// Send sends an SSE event to the client
func Send(w *bufio.Writer, event Event) error {
    if event.Event != "" {
        fmt.Fprintf(w, "event: %s\n", event.Event)
    }
    
    var dataStr string
    switch v := event.Data.(type) {
    case string:
        dataStr = v
    default:
        data, err := json.Marshal(v)
        if err != nil {
            return fmt.Errorf("failed to marshal event data: %w", err)
        }
        dataStr = string(data)
    }
    
    fmt.Fprintf(w, "data: %s\n\n", dataStr)
    return w.Flush()
}

// SendError sends an error event
func SendError(w *bufio.Writer, err error) error {
    return Send(w, Event{
        Event: "error",
        Data: map[string]string{
            "error": err.Error(),
        },
    })
}

// SendProgress sends a progress event
func SendProgress(w *bufio.Writer, progress interface{}) error {
    return Send(w, Event{
        Event: "progress",
        Data:  progress,
    })
}

// SendComplete sends a completion event
func SendComplete(w *bufio.Writer, result interface{}) error {
    return Send(w, Event{
        Event: "complete",
        Data:  result,
    })
}
```

#### 3.2 Create SSE Handler

**File**: `apps/api/handlers/syllabus/stream.go`

```go
package syllabus

import (
    "bufio"
    "fmt"
    "strconv"
    
    "github.com/gofiber/fiber/v2"
    "github.com/sahilchouksey/go-init-setup/services"
    "github.com/sahilchouksey/go-init-setup/utils/middleware"
    "github.com/sahilchouksey/go-init-setup/utils/response"
    "github.com/sahilchouksey/go-init-setup/utils/sse"
)

// ExtractSyllabusStream handles streaming extraction with SSE
// GET /api/v2/documents/:document_id/extract-syllabus?stream=true
func (h *SyllabusHandler) ExtractSyllabusStream(c *fiber.Ctx) error {
    // Get user from context
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    // Parse document ID
    documentIDStr := c.Params("document_id")
    documentID, err := strconv.ParseUint(documentIDStr, 10, 32)
    if err != nil {
        return response.BadRequest(c, "Invalid document ID")
    }
    
    // Check for existing active job
    activeJobID, err := h.progressTracker.GetActiveJob(c.Context(), user.ID)
    if err != nil {
        return response.InternalServerError(c, "Failed to check active jobs")
    }
    
    if activeJobID != "" {
        return response.Conflict(c, fmt.Sprintf(
            "You already have an active extraction job: %s. Please wait for it to complete or reconnect to it.",
            activeJobID,
        ))
    }
    
    // Create extraction job
    job, err := h.progressTracker.CreateJob(c.Context(), user.ID, uint(documentID))
    if err != nil {
        return response.InternalServerError(c, fmt.Sprintf("Failed to create job: %v", err))
    }
    
    // Set SSE headers
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Transfer-Encoding", "chunked")
    c.Set("X-Accel-Buffering", "no") // Disable nginx buffering
    
    // Start streaming
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // Send initial started event
        sse.Send(w, sse.Event{
            Event: "started",
            Data: services.ProgressEvent{
                Type:      "started",
                JobID:     job.JobID,
                Progress:  0,
                Phase:     "initializing",
                Message:   "Starting extraction...",
                Timestamp: job.StartedAt,
            },
        })
        
        // Extract with progress callback
        syllabuses, err := h.syllabusService.ExtractSyllabusWithProgress(
            c.Context(),
            uint(documentID),
            func(event services.ProgressEvent) error {
                // Add job ID to event
                event.JobID = job.JobID
                
                // Update job state in Redis
                if updateErr := h.progressTracker.UpdateProgress(c.Context(), job.JobID, event); updateErr != nil {
                    // Log error but don't stop stream
                    fmt.Printf("Failed to update job state: %v\n", updateErr)
                }
                
                // Send event to client
                return sse.Send(w, sse.Event{
                    Event: event.Type,
                    Data:  event,
                })
            },
        )
        
        if err != nil {
            // Send error event
            errorEvent := services.ProgressEvent{
                Type:         "error",
                JobID:        job.JobID,
                Progress:     job.Progress,
                Phase:        job.CurrentPhase,
                Message:      "Extraction failed",
                ErrorMessage: err.Error(),
                Recoverable:  false,
                Timestamp:    job.UpdatedAt,
            }
            
            h.progressTracker.UpdateProgress(c.Context(), job.JobID, errorEvent)
            sse.Send(w, sse.Event{Event: "error", Data: errorEvent})
            return
        }
        
        // Send completion event with results
        syllabusIDs := make([]uint, len(syllabuses))
        for i, s := range syllabuses {
            syllabusIDs[i] = s.ID
        }
        
        completeEvent := services.ProgressEvent{
            Type:      "complete",
            JobID:     job.JobID,
            Progress:  100,
            Phase:     "complete",
            Message:   fmt.Sprintf("Extraction completed successfully (%d subjects)", len(syllabuses)),
            Timestamp: job.UpdatedAt,
        }
        
        // Update job with result IDs
        job.ResultSyllabusIDs = syllabusIDs
        h.progressTracker.UpdateProgress(c.Context(), job.JobID, completeEvent)
        
        sse.Send(w, sse.Event{
            Event: "complete",
            Data:  completeEvent,
        })
    })
    
    return nil
}

// GetJobStatus handles GET /api/v2/extraction-jobs/:job_id
func (h *SyllabusHandler) GetJobStatus(c *fiber.Ctx) error {
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    jobID := c.Params("job_id")
    
    job, err := h.progressTracker.GetJob(c.Context(), jobID)
    if err != nil {
        return response.NotFound(c, "Job not found or expired")
    }
    
    // Verify ownership
    if job.UserID != user.ID && user.Role != "admin" {
        return response.Forbidden(c, "Access denied")
    }
    
    return response.Success(c, job)
}

// ReconnectToJob handles GET /api/v2/extraction-jobs/:job_id/stream
func (h *SyllabusHandler) ReconnectToJob(c *fiber.Ctx) error {
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    jobID := c.Params("job_id")
    
    job, err := h.progressTracker.GetJob(c.Context(), jobID)
    if err != nil {
        return response.NotFound(c, "Job not found or expired")
    }
    
    // Verify ownership
    if job.UserID != user.ID && user.Role != "admin" {
        return response.Forbidden(c, "Access denied")
    }
    
    // Set SSE headers
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("X-Accel-Buffering", "no")
    
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // Send current state immediately
        currentEvent := services.ProgressEvent{
            Type:            "progress",
            JobID:           job.JobID,
            Progress:        job.Progress,
            Phase:           job.CurrentPhase,
            Message:         job.Message,
            TotalChunks:     job.TotalChunks,
            CompletedChunks: job.CompletedChunks,
            Timestamp:       job.UpdatedAt,
        }
        
        if job.Status == "completed" {
            currentEvent.Type = "complete"
            currentEvent.Progress = 100
        } else if job.Status == "failed" {
            currentEvent.Type = "error"
            currentEvent.ErrorMessage = job.Error
        }
        
        sse.Send(w, sse.Event{
            Event: currentEvent.Type,
            Data:  currentEvent,
        })
        
        // If job is still processing, we can't stream further updates
        // Client should poll or wait for completion
        // For true reconnection, we'd need a pub/sub system (future enhancement)
    })
    
    return nil
}
```

#### 3.3 Update Router

**File**: `apps/api/router/main.go`

Add v2 routes:

```go
func SetupRoutes(app *fiber.App, store database.Storage) {
    // ... existing setup ...
    
    // API v1 (existing)
    api := app.Group("/api/v1")
    // ... existing v1 routes ...
    
    // API v2 (new)
    apiv2 := app.Group("/api/v2")
    setupV2Routes(apiv2, authMiddleware, syllabusHandler, progressTracker)
}

func setupV2Routes(
    api fiber.Router,
    authMiddleware *middleware.AuthMiddleware,
    syllabusHandler *syllabus.SyllabusHandler,
    progressTracker *services.ProgressTracker,
) {
    // Protected v2 endpoints
    documents := api.Group("/documents", authMiddleware.Required())
    
    // SSE streaming extraction
    documents.Get("/:document_id/extract-syllabus", syllabusHandler.ExtractSyllabusStream)
    
    // Job management
    jobs := api.Group("/extraction-jobs", authMiddleware.Required())
    jobs.Get("/:job_id", syllabusHandler.GetJobStatus)
    jobs.Get("/:job_id/stream", syllabusHandler.ReconnectToJob)
}
```

**Success Criteria:**
- [ ] SSE helper utilities created and tested
- [ ] `ExtractSyllabusStream` handler implemented
- [ ] `GetJobStatus` handler implemented
- [ ] `ReconnectToJob` handler implemented
- [ ] Routes added to router
- [ ] Manual test with curl shows SSE events
- [ ] Integration test with EventSource succeeds

---

### Phase 4: Upload Endpoint Integration (1-2 hours)

#### 4.1 Add Streaming to Upload Endpoint

**File**: `apps/api/handlers/syllabus/syllabus.go`

Update `UploadAndExtractSyllabus` to support streaming:

```go
// UploadAndExtractSyllabus handles POST /api/v2/semesters/:semester_id/syllabus/upload
func (h *SyllabusHandler) UploadAndExtractSyllabus(c *fiber.Ctx) error {
    // Check if streaming requested
    stream := c.Query("stream", "false") == "true"
    
    // ... existing upload logic ...
    
    if stream {
        return h.uploadAndExtractWithStream(c, semesterID, file)
    }
    
    // Non-streaming (existing behavior)
    return h.uploadAndExtractSync(c, semesterID, file)
}

func (h *SyllabusHandler) uploadAndExtractWithStream(c *fiber.Ctx, semesterID uint, file *multipart.FileHeader) error {
    user, _ := middleware.GetUser(c)
    
    // Upload document first
    fileContent, _ := file.Open()
    defer fileContent.Close()
    
    document, err := h.documentService.UploadDocument(c.Context(), services.UploadDocumentRequest{
        File:        fileContent,
        FileName:    file.Filename,
        FileSize:    file.Size,
        SubjectID:   tempSubjectID,
        Type:        model.DocumentTypeSyllabus,
        UploadedBy:  user.ID,
    })
    if err != nil {
        return response.InternalServerError(c, "Failed to upload document")
    }
    
    // Create job
    job, err := h.progressTracker.CreateJob(c.Context(), user.ID, document.ID)
    if err != nil {
        return response.InternalServerError(c, "Failed to create extraction job")
    }
    
    // Set SSE headers and stream
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("X-Accel-Buffering", "no")
    
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // Stream extraction progress
        syllabuses, err := h.syllabusService.ExtractSyllabusWithProgress(
            c.Context(),
            document.ID,
            func(event services.ProgressEvent) error {
                event.JobID = job.JobID
                h.progressTracker.UpdateProgress(c.Context(), job.JobID, event)
                return sse.Send(w, sse.Event{Event: event.Type, Data: event})
            },
        )
        
        if err != nil {
            sse.SendError(w, err)
            return
        }
        
        sse.SendComplete(w, syllabuses)
    })
    
    return nil
}
```

**Success Criteria:**
- [ ] Upload endpoint supports `?stream=true` parameter
- [ ] Streaming and non-streaming paths both work
- [ ] Manual test with file upload + streaming succeeds
- [ ] Backward compatibility maintained

---

### Phase 5: Testing & Validation (2-3 hours)

#### 5.1 Unit Tests

**File**: `apps/api/services/progress_tracker_test.go`

```go
package services

import (
    "context"
    "testing"
    "time"
    
    "github.com/sahilchouksey/go-init-setup/model"
    "github.com/sahilchouksey/go-init-setup/utils/cache"
    "github.com/stretchr/testify/assert"
)

func TestProgressTracker_CreateJob(t *testing.T) {
    // Setup Redis mock
    redisCache, _ := cache.NewRedisCache("redis://localhost:6379/1")
    tracker := NewProgressTracker(redisCache)
    
    ctx := context.Background()
    
    // Create job
    job, err := tracker.CreateJob(ctx, 1, 123)
    assert.NoError(t, err)
    assert.NotEmpty(t, job.JobID)
    assert.Equal(t, uint(1), job.UserID)
    assert.Equal(t, uint(123), job.DocumentID)
    assert.Equal(t, model.JobStatusPending, job.Status)
    
    // Verify job stored in Redis
    retrieved, err := tracker.GetJob(ctx, job.JobID)
    assert.NoError(t, err)
    assert.Equal(t, job.JobID, retrieved.JobID)
    
    // Verify active job set
    activeJobID, err := tracker.GetActiveJob(ctx, 1)
    assert.NoError(t, err)
    assert.Equal(t, job.JobID, activeJobID)
}

func TestProgressTracker_UpdateProgress(t *testing.T) {
    redisCache, _ := cache.NewRedisCache("redis://localhost:6379/1")
    tracker := NewProgressTracker(redisCache)
    
    ctx := context.Background()
    
    job, _ := tracker.CreateJob(ctx, 1, 123)
    
    // Update progress
    event := ProgressEvent{
        Type:     "progress",
        Progress: 50,
        Phase:    "extraction",
        Message:  "Processing...",
    }
    
    err := tracker.UpdateProgress(ctx, job.JobID, event)
    assert.NoError(t, err)
    
    // Verify update
    updated, err := tracker.GetJob(ctx, job.JobID)
    assert.NoError(t, err)
    assert.Equal(t, 50, updated.Progress)
    assert.Equal(t, "extraction", updated.CurrentPhase)
}

func TestProgressTracker_ConcurrentJobPrevention(t *testing.T) {
    redisCache, _ := cache.NewRedisCache("redis://localhost:6379/1")
    tracker := NewProgressTracker(redisCache)
    
    ctx := context.Background()
    
    // Create first job
    job1, err := tracker.CreateJob(ctx, 1, 123)
    assert.NoError(t, err)
    
    // Try to create second job for same user
    job2, err := tracker.CreateJob(ctx, 1, 456)
    assert.Error(t, err)
    assert.Nil(t, job2)
    assert.Contains(t, err.Error(), "already has an active extraction job")
}
```

#### 5.2 Integration Tests

**File**: `apps/api/handlers/syllabus/stream_test.go`

```go
package syllabus

import (
    "context"
    "io"
    "net/http/httptest"
    "testing"
    
    "github.com/gofiber/fiber/v2"
    "github.com/stretchr/testify/assert"
)

func TestExtractSyllabusStream_Success(t *testing.T) {
    // Setup test app
    app := fiber.New()
    
    // ... setup handler with mocks ...
    
    // Make request
    req := httptest.NewRequest("GET", "/api/v2/documents/123/extract-syllabus?stream=true", nil)
    req.Header.Set("Authorization", "Bearer test-token")
    
    resp, err := app.Test(req, -1) // -1 = no timeout
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
    
    // Read SSE events
    body, _ := io.ReadAll(resp.Body)
    events := string(body)
    
    assert.Contains(t, events, "event: started")
    assert.Contains(t, events, "event: progress")
    assert.Contains(t, events, "event: complete")
}
```

#### 5.3 Manual Testing Checklist

**Test with curl:**

```bash
# Test SSE streaming
curl -N -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/v2/documents/123/extract-syllabus?stream=true"

# Expected output:
# event: started
# data: {"type":"started","job_id":"123_1734181800",...}
#
# event: progress
# data: {"type":"progress","progress":10,...}
#
# event: complete
# data: {"type":"complete","progress":100,...}
```

**Test scenarios:**
- [ ] Small PDF (≤4 pages) extraction
- [ ] Large PDF (12 pages) extraction
- [ ] Client disconnection during extraction
- [ ] Reconnection with job ID
- [ ] Multiple concurrent extractions (different users)
- [ ] Duplicate extraction attempt (same user)
- [ ] Chunk failure with retry
- [ ] Complete failure after retries
- [ ] Browser refresh during extraction
- [ ] Network timeout scenarios

**Success Criteria:**
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Manual curl tests show correct SSE format
- [ ] No memory leaks during long-running extractions
- [ ] Redis keys expire correctly (check with `redis-cli TTL`)

---

## Code Specifications

### 5.1 Environment Variables

```bash
# .env file additions

# Extraction Retry Configuration
EXTRACTION_MAX_RETRIES=3
EXTRACTION_RETRY_DELAY_SECONDS=5
EXTRACTION_RETRY_BACKOFF_MULTIPLIER=1.5
EXTRACTION_MAX_BACKOFF_SECONDS=30
EXTRACTION_CHUNK_TIMEOUT_SECONDS=180

# Job State Configuration
EXTRACTION_JOB_TTL_SUCCESS_HOURS=1
EXTRACTION_JOB_TTL_FAILURE_HOURS=24

# Redis Configuration (existing)
REDIS_URL=redis://localhost:6379/0
```

### 5.2 Redis Key Patterns

```
job:state:{job_id}           → JSON (ExtractionJob)
job:active:{user_id}         → String (job_id)
job:lock:{job_id}            → String ("locked") [future use]
```

### 5.3 HTTP Status Codes

| Code | Scenario |
|------|----------|
| 200 | SSE stream started successfully |
| 400 | Invalid document ID or parameters |
| 401 | User not authenticated |
| 403 | User doesn't own the document |
| 404 | Document or job not found |
| 409 | User already has active extraction job |
| 500 | Internal server error |

---

## Testing Strategy

### 6.1 Unit Testing

**Coverage Goals:**
- Progress Tracker: 90%+
- SSE Utilities: 95%+
- Service Layer: 85%+

**Key Test Cases:**
- Job creation and state management
- Progress calculation accuracy
- Error classification
- Retry logic with backoff
- Concurrent job prevention
- TTL expiration

### 6.2 Integration Testing

**Test Scenarios:**
- End-to-end extraction with SSE
- Reconnection to existing job
- Multiple users extracting concurrently
- Error propagation through layers
- Redis failure handling

### 6.3 Load Testing

**Tools**: Apache Bench, k6, or custom Go script

**Scenarios:**
- 10 concurrent extractions
- 50 concurrent SSE connections
- Redis memory usage under load
- Connection pool exhaustion

**Metrics to Monitor:**
- Response time (p50, p95, p99)
- Memory usage
- Redis memory
- Error rate
- Connection count

---

## Deployment Checklist

### 7.1 Pre-Deployment

- [ ] All tests passing (unit, integration)
- [ ] Code review completed
- [ ] Environment variables configured
- [ ] Redis available and tested
- [ ] Documentation updated
- [ ] API changelog updated

### 7.2 Deployment Steps

1. **Database Migration** (if any)
   ```bash
   # No database migrations needed for this feature
   ```

2. **Deploy Backend**
   ```bash
   # Build
   go build -o main .
   
   # Deploy (example with Docker)
   docker build -t study-woods-api:v2 .
   docker-compose up -d
   ```

3. **Verify Deployment**
   ```bash
   # Health check
   curl http://localhost:8080/health/detailed
   
   # Test SSE endpoint
   curl -N -H "Authorization: Bearer TOKEN" \
     "http://localhost:8080/api/v2/documents/123/extract-syllabus?stream=true"
   ```

4. **Monitor**
   - Check logs for errors
   - Monitor Redis memory usage
   - Monitor API response times
   - Check error rates

### 7.3 Rollback Plan

If issues occur:

1. **Immediate**: Revert to previous version
   ```bash
   docker-compose down
   docker-compose up -d study-woods-api:v1
   ```

2. **Redis Cleanup** (if needed)
   ```bash
   redis-cli KEYS "job:*" | xargs redis-cli DEL
   ```

3. **Verify**: Test v1 endpoints still work

---

## Future Enhancements

### 8.1 Short-term (1-2 months)

1. **WebSocket Alternative**
   - Bidirectional communication
   - Better for real-time updates
   - Client can send cancellation requests

2. **Job Cancellation**
   - `DELETE /api/v2/extraction-jobs/:job_id`
   - Cancel in-progress extraction
   - Clean up resources

3. **Batch Extraction**
   - Extract multiple documents in one job
   - Progress per document
   - Parallel processing

### 8.2 Long-term (3-6 months)

1. **Persistent Job History**
   - Store completed jobs in PostgreSQL
   - Query historical extractions
   - Analytics and reporting

2. **Admin Dashboard**
   - Monitor all active jobs
   - View system metrics
   - Manual intervention tools

3. **Redis Pub/Sub for True Reconnection**
   - Publish progress events to Redis channel
   - Multiple clients can subscribe
   - True real-time reconnection

4. **Extraction Queue System**
   - Queue jobs when system is busy
   - Priority-based processing
   - Better resource management

---

## Appendices

### A. Complete File Structure

```
apps/api/
├── model/
│   └── extraction_job.go          (NEW)
├── services/
│   ├── progress_tracker.go        (NEW)
│   ├── progress_tracker_test.go   (NEW)
│   ├── syllabus_service.go        (MODIFIED)
│   └── chunked_syllabus_extractor.go (MODIFIED)
├── handlers/
│   └── syllabus/
│       ├── stream.go              (NEW)
│       ├── stream_test.go         (NEW)
│       └── syllabus.go            (MODIFIED)
├── utils/
│   └── sse/
│       └── sse.go                 (NEW)
├── router/
│   └── main.go                    (MODIFIED)
└── config/
    └── config.go                  (MODIFIED)
```

### B. Estimated Timeline

| Phase | Tasks | Hours |
|-------|-------|-------|
| Phase 1 | Foundation | 3-4 |
| Phase 2 | Service Refactoring | 4-5 |
| Phase 3 | SSE Handlers | 3-4 |
| Phase 4 | Upload Integration | 1-2 |
| Phase 5 | Testing | 2-3 |
| **Total** | | **13-18** |

### C. Success Metrics

**Technical Metrics:**
- SSE connection success rate: >99%
- Progress update latency: <500ms
- Job state persistence: 100%
- Error recovery rate: >95%

**User Experience Metrics:**
- Time to first feedback: <1s
- Perceived wait time: 50-67% reduction
- User satisfaction: Measure via feedback

---

**Document Version**: 1.0  
**Last Updated**: December 14, 2025  
**Status**: ✅ Ready for Implementation

**Next Steps**: Proceed to Frontend Integration Guide and API Reference documents.
