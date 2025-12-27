package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AISetupService handles AI resource setup (KB + Agent + API Key) for subjects
type AISetupService struct {
	db                  *gorm.DB
	subjectService      *SubjectService
	notificationService *NotificationService
}

// NewAISetupService creates a new AI setup service
func NewAISetupService(db *gorm.DB, subjectService *SubjectService, notificationService *NotificationService) *AISetupService {
	return &AISetupService{
		db:                  db,
		subjectService:      subjectService,
		notificationService: notificationService,
	}
}

// AISetupRequest represents a request to setup AI for subjects
type AISetupRequest struct {
	SubjectIDs []uint
	UserID     uint
}

// AISetupResult represents the result of starting AI setup
type AISetupResult struct {
	JobID      uint   `json:"job_id"`
	TotalItems int    `json:"total_items"`
	Message    string `json:"message"`
	IsNewJob   bool   `json:"is_new_job"` // true if new job created, false if subjects added to existing job
}

// StartOrQueueAISetup creates a new job OR adds subjects to an existing active job
func (s *AISetupService) StartOrQueueAISetup(ctx context.Context, req AISetupRequest) (*AISetupResult, error) {
	if len(req.SubjectIDs) == 0 {
		return nil, fmt.Errorf("no subjects provided for AI setup")
	}

	// Check if subjects exist and don't already have AI resources
	var subjects []model.Subject
	if err := s.db.Where("id IN ?", req.SubjectIDs).Find(&subjects).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch subjects: %w", err)
	}

	// Filter out subjects that already have AI resources
	var subjectsNeedingSetup []model.Subject
	for _, subject := range subjects {
		if subject.KnowledgeBaseUUID == "" || subject.AgentUUID == "" || subject.AgentAPIKeyEncrypted == "" {
			subjectsNeedingSetup = append(subjectsNeedingSetup, subject)
		} else {
			log.Printf("AISetupService: Skipping subject %d (%s) - already has AI resources", subject.ID, subject.Name)
		}
	}

	if len(subjectsNeedingSetup) == 0 {
		return &AISetupResult{
			JobID:      0,
			TotalItems: 0,
			Message:    "All subjects already have AI resources",
			IsNewJob:   false,
		}, nil
	}

	// Check for existing active job for this user
	var existingJob model.IndexingJob
	err := s.db.Where("created_by_user_id = ? AND job_type = ? AND status IN ?",
		req.UserID,
		model.IndexingJobTypeAISetup,
		[]model.IndexingJobStatus{model.IndexingJobStatusPending, model.IndexingJobStatusProcessing, model.IndexingJobStatusKBIndexing},
	).First(&existingJob).Error

	if err == nil {
		// Found existing job - add items to it
		return s.addSubjectsToJob(ctx, &existingJob, subjectsNeedingSetup)
	}

	// No existing job - create new one
	return s.createNewJob(ctx, req.UserID, subjectsNeedingSetup)
}

// createNewJob creates a new AI setup job
func (s *AISetupService) createNewJob(ctx context.Context, userID uint, subjects []model.Subject) (*AISetupResult, error) {
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	now := time.Now()
	job := &model.IndexingJob{
		JobType:         model.IndexingJobTypeAISetup,
		Status:          model.IndexingJobStatusPending,
		TotalItems:      len(subjects),
		CompletedItems:  0,
		FailedItems:     0,
		CreatedByUserID: userID,
		StartedAt:       &now,
	}

	if err := tx.Create(job).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create AI setup job: %w", err)
	}

	// Create job items for each subject
	for _, subject := range subjects {
		metadata := model.AISetupItemMetadata{
			SubjectName: subject.Name,
			SubjectCode: subject.Code,
			Phase:       "pending",
		}
		metadataJSON, _ := json.Marshal(metadata)

		item := &model.IndexingJobItem{
			JobID:     job.ID,
			ItemType:  model.IndexingJobItemTypeSubjectAI,
			SubjectID: &subject.ID,
			Status:    model.IndexingJobItemStatusPending,
			Metadata:  datatypes.JSON(metadataJSON),
		}

		if err := tx.Create(item).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create job item: %w", err)
		}

		// Update subject status to pending
		if err := tx.Model(&model.Subject{}).Where("id = ?", subject.ID).
			Update("ai_setup_status", model.AISetupStatusPending).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update subject status: %w", err)
		}
	}

	// Update job status to processing
	job.Status = model.IndexingJobStatusProcessing
	if err := tx.Save(job).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update job status: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Create notification
	subjectNames := make([]string, len(subjects))
	for i, s := range subjects {
		subjectNames[i] = s.Name
	}
	message := fmt.Sprintf("Setting up AI for: %s", strings.Join(subjectNames, ", "))
	if len(subjects) > 3 {
		message = fmt.Sprintf("Setting up AI for %d subjects...", len(subjects))
	}

	if s.notificationService != nil {
		_, err := s.notificationService.CreateNotification(ctx, CreateNotificationRequest{
			UserID:        userID,
			Type:          model.NotificationTypeInProgress,
			Category:      model.NotificationCategoryAISetup,
			Title:         "AI Setup Started",
			Message:       message,
			IndexingJobID: &job.ID,
			Metadata: &model.NotificationMetadata{
				TotalItems: len(subjects),
				Progress:   0,
			},
		})
		if err != nil {
			log.Printf("Warning: Failed to create notification for AI setup job %d: %v", job.ID, err)
		}
	}

	// Start background processing
	go s.processJob(job.ID)

	log.Printf("AISetupService: Created new job %d for %d subjects", job.ID, len(subjects))

	return &AISetupResult{
		JobID:      job.ID,
		TotalItems: len(subjects),
		Message:    fmt.Sprintf("AI setup started for %d subjects", len(subjects)),
		IsNewJob:   true,
	}, nil
}

// addSubjectsToJob adds subjects to an existing job
func (s *AISetupService) addSubjectsToJob(ctx context.Context, job *model.IndexingJob, subjects []model.Subject) (*AISetupResult, error) {
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Create job items for each subject
	for _, subject := range subjects {
		metadata := model.AISetupItemMetadata{
			SubjectName: subject.Name,
			SubjectCode: subject.Code,
			Phase:       "pending",
		}
		metadataJSON, _ := json.Marshal(metadata)

		item := &model.IndexingJobItem{
			JobID:     job.ID,
			ItemType:  model.IndexingJobItemTypeSubjectAI,
			SubjectID: &subject.ID,
			Status:    model.IndexingJobItemStatusPending,
			Metadata:  datatypes.JSON(metadataJSON),
		}

		if err := tx.Create(item).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create job item: %w", err)
		}

		// Update subject status to pending
		if err := tx.Model(&model.Subject{}).Where("id = ?", subject.ID).
			Update("ai_setup_status", model.AISetupStatusPending).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update subject status: %w", err)
		}
	}

	// Update job total items count
	newTotal := job.TotalItems + len(subjects)
	if err := tx.Model(&model.IndexingJob{}).Where("id = ?", job.ID).
		Update("total_items", newTotal).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update job total items: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("AISetupService: Added %d subjects to existing job %d (new total: %d)", len(subjects), job.ID, newTotal)

	return &AISetupResult{
		JobID:      job.ID,
		TotalItems: newTotal,
		Message:    fmt.Sprintf("Added %d subjects to existing AI setup job", len(subjects)),
		IsNewJob:   false,
	}, nil
}

// processJob processes all pending items in a job
func (s *AISetupService) processJob(jobID uint) {
	ctx := context.Background()

	log.Printf("AISetupService: Starting job %d processing", jobID)

	// Initial delay to let any previous DO API calls settle
	log.Printf("AISetupService: Waiting 5s before starting...")
	time.Sleep(5 * time.Second)

	// Exponential backoff configuration for rate limit retries
	// Base: 5s, then 10s, 20s, 40s, 80s, 160s (exponential growth with 2x multiplier)
	const (
		initialBackoff = 5 * time.Second
		maxBackoff     = 180 * time.Second // Cap at 3 minutes
		backoffFactor  = 2.0
		maxRetries     = 6
	)

	for {
		// Get next pending item
		item := s.getNextPendingItem(jobID)
		if item == nil {
			break // No more items
		}

		if item.SubjectID == nil {
			log.Printf("AISetupService: Item %d has no subject ID, skipping", item.ID)
			s.markItemFailed(item.ID, "No subject ID")
			continue
		}

		subjectID := *item.SubjectID

		// Update subject status to in_progress
		s.db.Model(&model.Subject{}).Where("id = ?", subjectID).
			Update("ai_setup_status", model.AISetupStatusInProgress)

		// Update item status
		s.updateItemStatus(item.ID, model.IndexingJobItemStatusKBCreating, "kb")

		var lastErr error

		// Retry loop with exponential backoff for rate limit errors
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				// Calculate exponential backoff: initialBackoff * (backoffFactor ^ (attempt-1))
				backoff := time.Duration(float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt-1)))
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				log.Printf("AISetupService: Retrying AI setup for subject %d (attempt %d/%d) after %v backoff",
					subjectID, attempt+1, maxRetries, backoff)
				time.Sleep(backoff)
			}

			setupCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			result, err := s.subjectService.SetupSubjectAI(setupCtx, subjectID)
			cancel()

			if err == nil {
				// Success!
				log.Printf("AISetupService: AI setup complete for subject %d (KB: %v, Agent: %v, APIKey: %v)",
					subjectID, result.KnowledgeBaseCreated, result.AgentCreated, result.APIKeyCreated)

				// Update item with success metadata
				s.updateItemSuccess(item.ID, result)

				// Update subject status
				s.db.Model(&model.Subject{}).Where("id = ?", subjectID).
					Update("ai_setup_status", model.AISetupStatusCompleted)

				// Update job progress
				s.incrementJobCompleted(jobID)

				lastErr = nil
				break
			}

			lastErr = err
			if !isRateLimitError(err) {
				// Non-retriable error, log and move on
				log.Printf("AISetupService: Non-retriable error for subject %d: %v", subjectID, err)
				break
			}
			// Rate limit error - will retry
			log.Printf("AISetupService: Rate limit hit for subject %d, will retry...", subjectID)
		}

		if lastErr != nil {
			// Failed after all retries
			log.Printf("AISetupService: Failed to setup AI for subject %d: %v", subjectID, lastErr)
			s.markItemFailed(item.ID, lastErr.Error())

			// Update subject status
			s.db.Model(&model.Subject{}).Where("id = ?", subjectID).
				Update("ai_setup_status", model.AISetupStatusFailed)

			// Update job failed count
			s.incrementJobFailed(jobID)
		}

		// Delay between subjects to avoid rate limits
		// Using a moderate delay since the rate limiter handles fine-grained throttling
		log.Printf("AISetupService: Waiting 10s before next subject...")
		time.Sleep(10 * time.Second)
	}

	// Finalize job
	s.finalizeJob(ctx, jobID)
}

// getNextPendingItem gets the next pending item for processing (FIFO)
func (s *AISetupService) getNextPendingItem(jobID uint) *model.IndexingJobItem {
	var item model.IndexingJobItem
	err := s.db.Where("job_id = ? AND status = ?", jobID, model.IndexingJobItemStatusPending).
		Order("created_at ASC").
		First(&item).Error

	if err != nil {
		return nil
	}
	return &item
}

// updateItemStatus updates an item's status and phase
func (s *AISetupService) updateItemStatus(itemID uint, status model.IndexingJobItemStatus, phase string) {
	// Get current metadata
	var item model.IndexingJobItem
	if err := s.db.First(&item, itemID).Error; err != nil {
		return
	}

	var metadata model.AISetupItemMetadata
	if len(item.Metadata) > 0 {
		json.Unmarshal(item.Metadata, &metadata)
	}
	metadata.Phase = phase
	metadataJSON, _ := json.Marshal(metadata)

	s.db.Model(&model.IndexingJobItem{}).Where("id = ?", itemID).Updates(map[string]interface{}{
		"status":   status,
		"metadata": datatypes.JSON(metadataJSON),
	})
}

// updateItemSuccess updates item with success result
func (s *AISetupService) updateItemSuccess(itemID uint, result *CreateSubjectResult) {
	var item model.IndexingJobItem
	if err := s.db.First(&item, itemID).Error; err != nil {
		return
	}

	var metadata model.AISetupItemMetadata
	if len(item.Metadata) > 0 {
		json.Unmarshal(item.Metadata, &metadata)
	}

	metadata.Phase = "complete"
	if result.Subject != nil {
		metadata.KnowledgeBaseUUID = result.Subject.KnowledgeBaseUUID
		metadata.AgentUUID = result.Subject.AgentUUID
		metadata.AgentDeploymentURL = result.Subject.AgentDeploymentURL
		metadata.HasAPIKey = result.Subject.AgentAPIKeyEncrypted != ""
	}
	metadataJSON, _ := json.Marshal(metadata)

	s.db.Model(&model.IndexingJobItem{}).Where("id = ?", itemID).Updates(map[string]interface{}{
		"status":   model.IndexingJobItemStatusCompleted,
		"metadata": datatypes.JSON(metadataJSON),
	})
}

// markItemFailed marks an item as failed
func (s *AISetupService) markItemFailed(itemID uint, errorMsg string) {
	s.db.Model(&model.IndexingJobItem{}).Where("id = ?", itemID).Updates(map[string]interface{}{
		"status":        model.IndexingJobItemStatusFailed,
		"error_message": errorMsg,
	})
}

// incrementJobCompleted increments the completed count for a job
func (s *AISetupService) incrementJobCompleted(jobID uint) {
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).
		UpdateColumn("completed_items", gorm.Expr("completed_items + 1"))
}

// incrementJobFailed increments the failed count for a job
func (s *AISetupService) incrementJobFailed(jobID uint) {
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).
		UpdateColumn("failed_items", gorm.Expr("failed_items + 1"))
}

// finalizeJob marks a job as complete and sends notification
func (s *AISetupService) finalizeJob(ctx context.Context, jobID uint) {
	// Get final job state
	var job model.IndexingJob
	if err := s.db.First(&job, jobID).Error; err != nil {
		log.Printf("AISetupService: Failed to fetch job %d for finalization: %v", jobID, err)
		return
	}

	// Determine final status
	now := time.Now()
	var status model.IndexingJobStatus
	if job.FailedItems == 0 && job.CompletedItems == job.TotalItems {
		status = model.IndexingJobStatusCompleted
	} else if job.CompletedItems > 0 {
		status = model.IndexingJobStatusPartial
	} else {
		status = model.IndexingJobStatusFailed
	}

	// Update job
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":       status,
		"completed_at": &now,
	})

	// Send notification
	if s.notificationService != nil {
		var notificationType model.NotificationType
		var title, message string

		switch status {
		case model.IndexingJobStatusCompleted:
			notificationType = model.NotificationTypeSuccess
			title = "AI Setup Complete"
			message = fmt.Sprintf("AI is ready for %d subjects!", job.CompletedItems)
		case model.IndexingJobStatusPartial:
			notificationType = model.NotificationTypeWarning
			title = "AI Setup Partially Complete"
			message = fmt.Sprintf("AI setup completed for %d subjects, %d failed.", job.CompletedItems, job.FailedItems)
		default:
			notificationType = model.NotificationTypeError
			title = "AI Setup Failed"
			message = "Failed to setup AI for subjects. Please try uploading the syllabus again."
		}

		err := s.notificationService.UpdateNotificationForJob(ctx, jobID, notificationType, title, message, &model.NotificationMetadata{
			TotalItems:     job.TotalItems,
			CompletedItems: job.CompletedItems,
			FailedItems:    job.FailedItems,
			Progress:       100,
		})
		if err != nil {
			log.Printf("AISetupService: Failed to update notification for job %d: %v", jobID, err)
		}
	}

	log.Printf("AISetupService: Job %d finalized with status %s (completed: %d, failed: %d)",
		jobID, status, job.CompletedItems, job.FailedItems)
}

// GetJobStatus returns the current status of an AI setup job
func (s *AISetupService) GetJobStatus(ctx context.Context, jobID uint) (*model.IndexingJob, error) {
	var job model.IndexingJob
	if err := s.db.Preload("Items").First(&job, jobID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("failed to fetch job: %w", err)
	}

	if job.JobType != model.IndexingJobTypeAISetup {
		return nil, fmt.Errorf("job is not an AI setup job")
	}

	return &job, nil
}

// GetActiveJobForUser returns the active AI setup job for a user (if any)
func (s *AISetupService) GetActiveJobForUser(ctx context.Context, userID uint) (*model.IndexingJob, error) {
	var job model.IndexingJob
	err := s.db.Where("created_by_user_id = ? AND job_type = ? AND status IN ?",
		userID,
		model.IndexingJobTypeAISetup,
		[]model.IndexingJobStatus{model.IndexingJobStatusPending, model.IndexingJobStatusProcessing, model.IndexingJobStatusKBIndexing},
	).Preload("Items").First(&job).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No active job
		}
		return nil, fmt.Errorf("failed to fetch active job: %w", err)
	}

	return &job, nil
}
