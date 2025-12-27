package aisetup

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// AISetupHandler handles AI setup job-related API endpoints
type AISetupHandler struct {
	aiSetupService *services.AISetupService
}

// NewAISetupHandler creates a new AI setup handler
func NewAISetupHandler(aiSetupService *services.AISetupService) *AISetupHandler {
	return &AISetupHandler{
		aiSetupService: aiSetupService,
	}
}

// GetJobStatus handles GET /api/v1/ai-setup-jobs/:job_id
// Returns the status and details of an AI setup job
func (h *AISetupHandler) GetJobStatus(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	jobID, err := strconv.ParseUint(c.Params("job_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid job ID")
	}

	job, err := h.aiSetupService.GetJobStatus(c.Context(), uint(jobID))
	if err != nil {
		if err.Error() == "job not found" {
			return response.NotFound(c, "Job not found")
		}
		if err.Error() == "job is not an AI setup job" {
			return response.BadRequest(c, "Job is not an AI setup job")
		}
		return response.InternalServerError(c, "Failed to get job status")
	}

	// Verify user owns this job
	if job.CreatedByUserID != user.ID && user.Role != "admin" {
		return response.Forbidden(c, "You do not have access to this job")
	}

	// Convert items to response format
	var items []fiber.Map
	for _, item := range job.Items {
		items = append(items, fiber.Map{
			"id":            item.ID,
			"item_type":     item.ItemType,
			"subject_id":    item.SubjectID,
			"status":        item.Status,
			"error_message": item.ErrorMessage,
			"metadata":      item.Metadata,
			"created_at":    item.CreatedAt,
			"updated_at":    item.UpdatedAt,
		})
	}

	return response.Success(c, fiber.Map{
		"id":              job.ID,
		"job_type":        job.JobType,
		"status":          job.Status,
		"total_items":     job.TotalItems,
		"completed_items": job.CompletedItems,
		"failed_items":    job.FailedItems,
		"started_at":      job.StartedAt,
		"completed_at":    job.CompletedAt,
		"items":           items,
	})
}

// GetActiveJob handles GET /api/v1/ai-setup-jobs/active
// Returns the user's currently active AI setup job (if any)
func (h *AISetupHandler) GetActiveJob(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	job, err := h.aiSetupService.GetActiveJobForUser(c.Context(), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to get active job")
	}

	if job == nil {
		return response.Success(c, fiber.Map{
			"active_job": nil,
			"message":    "No active AI setup job",
		})
	}

	// Convert items to response format
	var items []fiber.Map
	for _, item := range job.Items {
		items = append(items, fiber.Map{
			"id":            item.ID,
			"item_type":     item.ItemType,
			"subject_id":    item.SubjectID,
			"status":        item.Status,
			"error_message": item.ErrorMessage,
			"metadata":      item.Metadata,
			"created_at":    item.CreatedAt,
			"updated_at":    item.UpdatedAt,
		})
	}

	return response.Success(c, fiber.Map{
		"active_job": fiber.Map{
			"id":              job.ID,
			"job_type":        job.JobType,
			"status":          job.Status,
			"total_items":     job.TotalItems,
			"completed_items": job.CompletedItems,
			"failed_items":    job.FailedItems,
			"started_at":      job.StartedAt,
			"completed_at":    job.CompletedAt,
			"items":           items,
		},
	})
}
