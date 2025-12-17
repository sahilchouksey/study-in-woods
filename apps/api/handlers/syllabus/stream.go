package syllabus

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"github.com/sahilchouksey/go-init-setup/utils/sse"
)

// ExtractSyllabusStream handles SSE streaming extraction
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

	// Check if streaming is requested
	stream := c.Query("stream", "false") == "true"
	if !stream {
		// Fall back to regular non-streaming extraction
		return h.ExtractSyllabus(c)
	}

	// Verify document exists and is a syllabus type
	var document model.Document
	if err := h.db.Preload("Subject").First(&document, documentID).Error; err != nil {
		return response.NotFound(c, "Document not found")
	}

	if document.Type != model.DocumentTypeSyllabus {
		return response.BadRequest(c, "Document is not a syllabus type")
	}

	// Check user permission
	if user.Role != "admin" && document.UploadedByUserID != user.ID {
		return response.Forbidden(c, "You don't have permission to extract this syllabus")
	}

	// Cancel any existing active job for this user before starting a new one
	// This ensures users can start fresh even if a previous job got stuck
	if h.progressTracker != nil {
		activeJobID, _ := h.progressTracker.GetActiveJob(c.Context(), user.ID)
		if activeJobID != "" {
			// Try to properly cancel the job (this will update status and clear active job)
			if err := h.progressTracker.CancelJob(c.Context(), activeJobID); err != nil {
				// If cancel fails (job might not exist or already completed), just clear the reference
				h.progressTracker.ClearActiveJob(c.Context(), user.ID)
			}
		}
	}

	// Create extraction job (if progressTracker is available)
	var jobID string
	if h.progressTracker != nil {
		job, err := h.progressTracker.CreateJob(c.Context(), user.ID, uint(documentID))
		if err != nil {
			return response.InternalServerError(c, fmt.Sprintf("Failed to create job: %v", err))
		}
		jobID = job.JobID
	} else {
		// Generate a simple job ID if no progress tracker
		jobID = fmt.Sprintf("%d_%d", documentID, time.Now().Unix())
	}

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Note: CORS headers are handled by the CORS middleware in security.go
	// Do NOT set them here to avoid duplicate headers which breaks browsers

	// Start streaming
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Use background context for operations inside stream writer
		// The Fiber context (c.Context()) is not valid inside the goroutine
		ctx := context.Background()

		// Send initial started event
		startedEvent := services.ProgressEvent{
			Type:      "started",
			JobID:     jobID,
			Progress:  0,
			Phase:     "initializing",
			Message:   "Starting extraction...",
			Timestamp: time.Now(),
		}
		sse.Send(w, sse.Event{Event: "started", Data: startedEvent})

		// Extract with progress callback
		syllabuses, err := h.syllabusService.ExtractSyllabusWithProgress(
			ctx,
			uint(documentID),
			func(event services.ProgressEvent) error {
				// Check if job has been cancelled
				if h.progressTracker != nil && h.progressTracker.IsJobCancelled(ctx, jobID) {
					return fmt.Errorf("job cancelled by user")
				}

				// Add job ID to event
				event.JobID = jobID

				// Update job state in Redis (if tracker available)
				if h.progressTracker != nil {
					if updateErr := h.progressTracker.UpdateProgress(ctx, jobID, event); updateErr != nil {
						// Log error but don't stop stream
						fmt.Printf("Failed to update job state: %v\n", updateErr)
					}
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
			errorType, _ := services.ClassifyError(err)
			errorEvent := services.ProgressEvent{
				Type:         "error",
				JobID:        jobID,
				Progress:     0,
				Phase:        "error",
				Message:      "Extraction failed",
				ErrorType:    string(errorType),
				ErrorMessage: err.Error(),
				Recoverable:  false,
				Timestamp:    time.Now(),
			}

			// Update job state
			if h.progressTracker != nil {
				h.progressTracker.UpdateProgress(ctx, jobID, errorEvent)
			}

			sse.Send(w, sse.Event{Event: "error", Data: errorEvent})
			return
		}

		// Note: The completion event is already sent by the syllabus service via the progressCallback.
		// We just need to update the job result IDs in the progress tracker for reconnection support.
		if h.progressTracker != nil && len(syllabuses) > 0 {
			syllabusIDs := make([]uint, len(syllabuses))
			for i, s := range syllabuses {
				syllabusIDs[i] = s.ID
			}
			h.progressTracker.SetJobResult(ctx, jobID, syllabusIDs)
		}
	})

	return nil
}

// GetJobStatus handles GET /api/v2/extraction-jobs/:job_id
func (h *SyllabusHandler) GetJobStatus(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	if h.progressTracker == nil {
		return response.InternalServerError(c, "Progress tracking not available")
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
// Allows clients to reconnect to an existing job's stream
func (h *SyllabusHandler) ReconnectToJob(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	if h.progressTracker == nil {
		return response.InternalServerError(c, "Progress tracking not available")
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

		// Adjust event type based on job status
		switch job.Status {
		case model.JobStatusCompleted:
			currentEvent.Type = "complete"
			currentEvent.Progress = 100
			currentEvent.ResultSyllabusIDs = job.ResultSyllabusIDs

			// Fetch actual subject details from database for completed jobs
			if len(job.ResultSyllabusIDs) > 0 {
				var syllabuses []model.Syllabus
				if err := h.db.Preload("Subject").Where("id IN ?", job.ResultSyllabusIDs).Find(&syllabuses).Error; err == nil {
					subjectSummaries := make([]services.SubjectSummary, len(syllabuses))
					for i, s := range syllabuses {
						subjectSummaries[i] = services.SubjectSummary{
							ID:      s.SubjectID,
							Name:    s.SubjectName,
							Code:    s.SubjectCode,
							Credits: s.TotalCredits,
						}
					}
					currentEvent.ResultSubjects = subjectSummaries
				}
			}
		case model.JobStatusFailed:
			currentEvent.Type = "error"
			currentEvent.ErrorMessage = job.Error
			currentEvent.Recoverable = false
		case model.JobStatusCancelled:
			currentEvent.Type = "error"
			currentEvent.ErrorMessage = "Job was cancelled"
			currentEvent.Recoverable = false
		}

		sse.Send(w, sse.Event{
			Event: currentEvent.Type,
			Data:  currentEvent,
		})

		// Note: For true real-time reconnection to an in-progress job,
		// we'd need a pub/sub system (Redis pub/sub or similar).
		// For now, we just send the current state.
		// The client can poll the status endpoint for updates.
	})

	return nil
}

// CancelJob handles POST /api/v2/extraction-jobs/:job_id/cancel
func (h *SyllabusHandler) CancelJob(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	if h.progressTracker == nil {
		return response.InternalServerError(c, "Progress tracking not available")
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

	// Cancel the job
	if err := h.progressTracker.CancelJob(c.Context(), jobID); err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "Job cancelled successfully",
		"job_id":  jobID,
	})
}

// GetMyActiveJob handles GET /api/v2/extraction-jobs/active
// Returns the user's currently active extraction job (if any)
func (h *SyllabusHandler) GetMyActiveJob(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	if h.progressTracker == nil {
		return response.InternalServerError(c, "Progress tracking not available")
	}

	activeJobID, err := h.progressTracker.GetActiveJob(c.Context(), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to check active jobs")
	}

	if activeJobID == "" {
		return response.Success(c, fiber.Map{
			"has_active_job": false,
			"job":            nil,
		})
	}

	// Get full job details
	job, err := h.progressTracker.GetJob(c.Context(), activeJobID)
	if err != nil {
		// Job might have expired
		return response.Success(c, fiber.Map{
			"has_active_job": false,
			"job":            nil,
		})
	}

	return response.Success(c, fiber.Map{
		"has_active_job": true,
		"job":            job,
	})
}

// UploadSyllabusForStreaming handles POST /api/v2/semesters/:semester_id/syllabus/upload
// Uploads a syllabus file and returns the document ID for subsequent SSE extraction
// This is the first step of a two-step process:
// 1. Upload file (this endpoint) -> returns document_id
// 2. Connect to GET /api/v2/documents/:document_id/extract-syllabus?stream=true for SSE
func (h *SyllabusHandler) UploadSyllabusForStreaming(c *fiber.Ctx) error {
	semesterID := c.Params("semester_id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Verify semester exists
	var semester model.Semester
	if err := h.db.First(&semester, semesterID).Error; err != nil {
		return response.NotFound(c, "Semester not found")
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "File is required")
	}

	// Validate file size (max 50MB)
	const maxFileSize = 50 * 1024 * 1024 // 50MB
	if file.Size > maxFileSize {
		return response.BadRequest(c, "File size exceeds maximum allowed size of 50MB")
	}

	// Validate file type (PDF only)
	if !isValidPDFFile(file.Filename) {
		return response.BadRequest(c, "Only PDF files are supported for syllabus upload")
	}

	// Open file
	fileContent, err := file.Open()
	if err != nil {
		return response.InternalServerError(c, "Failed to open file")
	}
	defer fileContent.Close()

	// Delete existing syllabus data for this semester (clean slate for new upload)
	if err := h.syllabusService.DeleteExistingSyllabusDataForSemester(c.Context(), semester.ID); err != nil {
		return response.InternalServerError(c, "Failed to clean existing syllabus data: "+err.Error())
	}

	// Upload document using DocumentService with semester reference (no temp subject needed)
	// The document is associated directly with the semester, and subjects will be created
	// during extraction based on the PDF content
	result, err := h.documentService.UploadDocument(c.Context(), services.UploadDocumentRequest{
		SemesterID: semester.ID, // Use semester-based upload instead of temp subject
		UserID:     user.ID,
		Type:       model.DocumentTypeSyllabus,
		File:       fileContent,
		FileHeader: file,
	})

	if err != nil {
		return response.InternalServerError(c, "Failed to upload syllabus: "+err.Error())
	}

	// Return the document ID for the frontend to connect to SSE
	return response.Success(c, fiber.Map{
		"message":     "File uploaded successfully. Connect to extraction endpoint for progress.",
		"document_id": result.Document.ID,
		"semester_id": semester.ID,
		"sse_url":     fmt.Sprintf("/api/v2/documents/%d/extract-syllabus?stream=true", result.Document.ID),
	})
}

// isValidPDFFile checks if the filename indicates a PDF file
func isValidPDFFile(filename string) bool {
	return len(filename) > 4 && filename[len(filename)-4:] == ".pdf" ||
		len(filename) > 4 && filename[len(filename)-4:] == ".PDF"
}
