package ingest

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// BatchIngestHandler handles batch ingest API endpoints
type BatchIngestHandler struct {
	batchIngestService *services.BatchIngestService
}

// NewBatchIngestHandler creates a new batch ingest handler
func NewBatchIngestHandler(batchIngestService *services.BatchIngestService) *BatchIngestHandler {
	return &BatchIngestHandler{
		batchIngestService: batchIngestService,
	}
}

// BatchIngestPYQsRequest represents the request body for batch PYQ ingestion
type BatchIngestPYQsRequest struct {
	Papers []struct {
		PDFURL     string `json:"pdf_url" validate:"required,url"`
		Title      string `json:"title" validate:"required"`
		Year       int    `json:"year" validate:"required,min=2000,max=2100"`
		Month      string `json:"month"`
		ExamType   string `json:"exam_type"`
		SourceName string `json:"source_name"`
	} `json:"papers" validate:"required,min=1,dive"`
	TriggerExtraction bool `json:"trigger_extraction"` // If true, automatically extract questions after upload
}

// BatchIngestPYQs handles POST /api/v1/subjects/:subject_id/batch-ingest
// Starts a batch ingestion job for multiple PYQ papers
func (h *BatchIngestHandler) BatchIngestPYQs(c *fiber.Ctx) error {
	log.Printf("[BATCH-INGEST] BatchIngestPYQs called for subject_id: %s", c.Params("subject_id"))

	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		log.Printf("[BATCH-INGEST] BatchIngestPYQs - User not authenticated")
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse subject ID
	subjectID, err := strconv.ParseUint(c.Params("subject_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Parse request body
	var req BatchIngestPYQsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate
	if len(req.Papers) == 0 {
		return response.BadRequest(c, "At least one paper is required")
	}

	if len(req.Papers) > 50 {
		return response.BadRequest(c, "Maximum 50 papers per batch")
	}

	// Convert to service request
	papers := make([]services.BatchIngestPaperRequest, len(req.Papers))
	for i, p := range req.Papers {
		if p.PDFURL == "" || p.Title == "" {
			return response.BadRequest(c, "PDF URL and title are required for all papers")
		}
		papers[i] = services.BatchIngestPaperRequest{
			PDFURL:     p.PDFURL,
			Title:      p.Title,
			Year:       p.Year,
			Month:      p.Month,
			ExamType:   p.ExamType,
			SourceName: p.SourceName,
		}
	}

	// Start batch ingest
	result, err := h.batchIngestService.StartBatchIngest(c.Context(), services.BatchIngestRequest{
		SubjectID:         uint(subjectID),
		UserID:            user.ID,
		Papers:            papers,
		TriggerExtraction: req.TriggerExtraction,
	})
	if err != nil {
		if err.Error() == "subject not found" {
			return response.NotFound(c, "Subject not found")
		}
		if err.Error() == "all papers already exist for this subject" {
			return response.BadRequest(c, err.Error())
		}
		return response.InternalServerError(c, "Failed to start batch ingestion: "+err.Error())
	}

	log.Printf("[BATCH-INGEST] BatchIngestPYQs - Job created: ID=%d, status=%s, total_items=%d",
		result.JobID, result.Status, result.TotalItems)

	return response.Success(c, fiber.Map{
		"job_id":      result.JobID,
		"status":      result.Status,
		"total_items": result.TotalItems,
		"message":     result.Message,
	})
}

// GetJobStatus handles GET /api/v1/indexing-jobs/:job_id
// Returns the status of an indexing job
func (h *BatchIngestHandler) GetJobStatus(c *fiber.Ctx) error {
	log.Printf("[BATCH-INGEST] GetJobStatus called for job_id: %s", c.Params("job_id"))

	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		log.Printf("[BATCH-INGEST] GetJobStatus - User not authenticated")
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse job ID
	jobID, err := strconv.ParseUint(c.Params("job_id"), 10, 32)
	if err != nil {
		log.Printf("[BATCH-INGEST] GetJobStatus - Invalid job ID: %v", err)
		return response.BadRequest(c, "Invalid job ID")
	}

	job, err := h.batchIngestService.GetJobStatus(c.Context(), uint(jobID), user.ID)
	if err != nil {
		log.Printf("[BATCH-INGEST] GetJobStatus - Error: %v", err)
		if err.Error() == "job not found" {
			return response.NotFound(c, "Job not found")
		}
		return response.InternalServerError(c, "Failed to fetch job status")
	}

	log.Printf("[BATCH-INGEST] GetJobStatus - Job %d: status=%s, progress=%d%%, completed=%d, failed=%d, total=%d",
		job.ID, job.Status, job.GetProgress(), job.CompletedItems, job.FailedItems, job.TotalItems)

	// Build items response
	var items []fiber.Map
	for _, item := range job.Items {
		items = append(items, fiber.Map{
			"id":            item.ID,
			"item_type":     item.ItemType,
			"source_url":    item.SourceURL,
			"status":        item.Status,
			"error_message": item.ErrorMessage,
			"document_id":   item.DocumentID,
			"pyq_paper_id":  item.PYQPaperID,
			"metadata":      item.Metadata,
		})
	}

	return response.Success(c, fiber.Map{
		"id":                   job.ID,
		"subject_id":           job.SubjectID,
		"job_type":             job.JobType,
		"status":               job.Status,
		"total_items":          job.TotalItems,
		"completed_items":      job.CompletedItems,
		"failed_items":         job.FailedItems,
		"progress":             job.GetProgress(),
		"do_indexing_job_uuid": job.DOIndexingJobUUID,
		"started_at":           job.StartedAt,
		"completed_at":         job.CompletedAt,
		"error_message":        job.ErrorMessage,
		"items":                items,
		"subject": fiber.Map{
			"id":   job.Subject.ID,
			"name": job.Subject.Name,
			"code": job.Subject.Code,
		},
	})
}

// GetJobsBySubject handles GET /api/v1/subjects/:subject_id/indexing-jobs
// Returns all indexing jobs for a subject
func (h *BatchIngestHandler) GetJobsBySubject(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse subject ID
	subjectID, err := strconv.ParseUint(c.Params("subject_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	jobs, err := h.batchIngestService.GetJobsBySubject(c.Context(), uint(subjectID), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch jobs")
	}

	var jobsResponse []fiber.Map
	for _, job := range jobs {
		jobsResponse = append(jobsResponse, fiber.Map{
			"id":              job.ID,
			"job_type":        job.JobType,
			"status":          job.Status,
			"total_items":     job.TotalItems,
			"completed_items": job.CompletedItems,
			"failed_items":    job.FailedItems,
			"progress":        job.GetProgress(),
			"started_at":      job.StartedAt,
			"completed_at":    job.CompletedAt,
			"created_at":      job.CreatedAt,
		})
	}

	return response.Success(c, fiber.Map{
		"jobs": jobsResponse,
	})
}

// CancelJob handles POST /api/v1/indexing-jobs/:job_id/cancel
// Cancels an active indexing job
func (h *BatchIngestHandler) CancelJob(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse job ID
	jobID, err := strconv.ParseUint(c.Params("job_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid job ID")
	}

	if err := h.batchIngestService.CancelJob(c.Context(), uint(jobID), user.ID); err != nil {
		if err.Error() == "job not found" {
			return response.NotFound(c, "Job not found")
		}
		if err.Error() == "job is already complete" {
			return response.BadRequest(c, err.Error())
		}
		return response.InternalServerError(c, "Failed to cancel job")
	}

	return response.Success(c, fiber.Map{
		"message": "Job cancelled",
	})
}
