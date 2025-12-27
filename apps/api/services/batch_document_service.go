package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// BatchDocumentService handles batch document uploads
type BatchDocumentService struct {
	db                  *gorm.DB
	doClient            *digitalocean.Client
	spacesClient        *digitalocean.SpacesClient
	notificationService *NotificationService
	ocrClient           *OCRClient
	enableAI            bool
	enableSpaces        bool

	// Active jobs tracking
	activeJobs   map[uint]context.CancelFunc
	activeJobsMu sync.RWMutex
}

// NewBatchDocumentService creates a new batch document service
func NewBatchDocumentService(db *gorm.DB, notificationService *NotificationService) *BatchDocumentService {
	service := &BatchDocumentService{
		db:                  db,
		notificationService: notificationService,
		enableAI:            false,
		enableSpaces:        false,
		activeJobs:          make(map[uint]context.CancelFunc),
	}

	// Initialize DigitalOcean client for AI features
	apiToken := os.Getenv("DIGITALOCEAN_TOKEN")
	if apiToken != "" {
		service.doClient = digitalocean.NewClient(digitalocean.Config{
			APIToken: apiToken,
		})
		service.enableAI = true
	} else {
		log.Println("Warning: DIGITALOCEAN_TOKEN not set. AI indexing will be disabled for batch document uploads.")
	}

	// Initialize Spaces client
	spacesClient, err := digitalocean.NewSpacesClientFromGlobalConfig()
	if err != nil {
		log.Printf("Warning: Failed to initialize Spaces client: %v. File storage will be disabled.", err)
	} else {
		service.spacesClient = spacesClient
		service.enableSpaces = true
	}

	// Initialize OCR client (availability is checked dynamically at runtime)
	service.ocrClient = NewOCRClient()
	log.Println("OCR client initialized for batch document service (availability will be checked at runtime)")

	return service
}

// BatchDocumentRequest represents a single document to upload
type BatchDocumentRequest struct {
	FileHeader   *multipart.FileHeader `json:"-"`
	DocumentType model.DocumentType    `json:"document_type"`
}

// BatchUploadRequest represents a batch upload request
type BatchUploadRequest struct {
	SubjectID uint                   `json:"subject_id"`
	UserID    uint                   `json:"user_id"`
	Documents []BatchDocumentRequest `json:"documents"`
}

// BatchUploadResult represents the result of starting a batch upload
type BatchUploadResult struct {
	JobID      uint   `json:"job_id"`
	Status     string `json:"status"`
	TotalItems int    `json:"total_items"`
	Message    string `json:"message"`
}

// FileData is used internally to pass file data between functions
type FileData struct {
	Filename     string
	Content      []byte
	DocumentType model.DocumentType
	FileSize     int64
}

// StartBatchUpload creates a new batch upload job and starts processing
func (s *BatchDocumentService) StartBatchUpload(ctx context.Context, req BatchUploadRequest) (*BatchUploadResult, error) {
	// Validate subject exists and has knowledge base
	var subject model.Subject
	if err := s.db.First(&subject, req.SubjectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("subject not found")
		}
		return nil, fmt.Errorf("failed to fetch subject: %w", err)
	}

	if subject.KnowledgeBaseUUID == "" && s.enableAI {
		log.Printf("Warning: Subject %d has no knowledge base. Documents will be uploaded but not indexed.", req.SubjectID)
	}

	if len(req.Documents) == 0 {
		return nil, fmt.Errorf("at least one document is required")
	}

	// Read file contents into memory before starting transaction
	// This is necessary because multipart files may be closed after the request ends
	var filesData []FileData

	for _, doc := range req.Documents {
		file, err := doc.FileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", doc.FileHeader.Filename, err)
		}

		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", doc.FileHeader.Filename, err)
		}

		filesData = append(filesData, FileData{
			Filename:     doc.FileHeader.Filename,
			Content:      content,
			DocumentType: doc.DocumentType,
			FileSize:     doc.FileHeader.Size,
		})
	}

	// Start database transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Create indexing job
	now := time.Now()
	subjectID := req.SubjectID
	job := &model.IndexingJob{
		SubjectID:       &subjectID,
		JobType:         model.IndexingJobTypeDocumentUpload,
		Status:          model.IndexingJobStatusPending,
		TotalItems:      len(filesData),
		CompletedItems:  0,
		FailedItems:     0,
		CreatedByUserID: req.UserID,
		StartedAt:       &now,
	}

	if err := tx.Create(job).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create indexing job: %w", err)
	}

	// Create job items
	for _, fd := range filesData {
		metadata := model.IndexingJobItemMetadata{
			FileName: fd.Filename,
			FileSize: fd.FileSize,
		}
		metadataJSON, _ := json.Marshal(metadata)

		item := &model.IndexingJobItem{
			JobID:    job.ID,
			ItemType: model.IndexingJobItemTypeLocalPDF,
			Status:   model.IndexingJobItemStatusPending,
			Metadata: datatypes.JSON(metadataJSON),
		}

		if err := tx.Create(item).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create job item: %w", err)
		}
	}

	// Update job status to processing
	job.Status = model.IndexingJobStatusProcessing
	if err := tx.Save(job).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update job status: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Create notification for the user
	_, err := s.notificationService.CreateInProgressNotification(
		ctx,
		req.UserID,
		job.ID,
		model.NotificationCategoryDocumentUpload,
		fmt.Sprintf("Uploading %d documents", len(filesData)),
		fmt.Sprintf("Processing documents for %s...", subject.Name),
		&model.NotificationMetadata{
			SubjectID:      subject.ID,
			SubjectName:    subject.Name,
			SubjectCode:    subject.Code,
			TotalItems:     len(filesData),
			CompletedItems: 0,
			Progress:       0,
		},
	)
	if err != nil {
		log.Printf("Warning: Failed to create notification for job %d: %v", job.ID, err)
	}

	// Start background processing
	go s.processJob(job.ID, req.SubjectID, req.UserID, subject, filesData)

	log.Printf("========================================")
	log.Printf("[BATCH-UPLOAD] JOB %d CREATED", job.ID)
	log.Printf("[BATCH-UPLOAD] Subject: %s (ID: %d)", subject.Name, req.SubjectID)
	log.Printf("[BATCH-UPLOAD] User: %d", req.UserID)
	log.Printf("[BATCH-UPLOAD] Total Items: %d", len(filesData))
	log.Printf("[BATCH-UPLOAD] Status: %s", model.IndexingJobStatusProcessing)
	log.Printf("========================================")

	return &BatchUploadResult{
		JobID:      job.ID,
		Status:     string(model.IndexingJobStatusProcessing),
		TotalItems: len(filesData),
		Message:    fmt.Sprintf("Batch upload started with %d documents", len(filesData)),
	}, nil
}

// processJob handles the background processing of a batch upload job
func (s *BatchDocumentService) processJob(jobID uint, subjectID uint, userID uint, subject model.Subject, filesData []FileData) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Track active job
	s.activeJobsMu.Lock()
	s.activeJobs[jobID] = cancel
	s.activeJobsMu.Unlock()

	defer func() {
		s.activeJobsMu.Lock()
		delete(s.activeJobs, jobID)
		s.activeJobsMu.Unlock()
	}()

	log.Printf("========================================")
	log.Printf("[BATCH-UPLOAD] GOROUTINE STARTED for job %d", jobID)
	log.Printf("[BATCH-UPLOAD] Subject ID: %d", subjectID)
	log.Printf("========================================")

	// Small delay to allow frontend to start polling before we begin processing
	time.Sleep(500 * time.Millisecond)
	log.Printf("[BATCH-UPLOAD] Starting processing after initial delay for job %d", jobID)

	// Get all job items
	var items []model.IndexingJobItem
	if err := s.db.Where("job_id = ?", jobID).Find(&items).Error; err != nil {
		log.Printf("Error fetching job items: %v", err)
		s.failJob(ctx, jobID, userID, subject, "Failed to fetch job items")
		return
	}

	completedItems := 0
	failedItems := 0
	var createdDataSourceUUIDs []string

	// Process each item
	for i, item := range items {
		select {
		case <-ctx.Done():
			log.Printf("Job %d cancelled or timed out", jobID)
			s.failJob(ctx, jobID, userID, subject, "Job cancelled or timed out")
			return
		default:
		}

		if i >= len(filesData) {
			log.Printf("Error: File data index out of bounds for item %d", item.ID)
			s.updateItemStatus(item.ID, model.IndexingJobItemStatusFailed, "File data not found")
			failedItems++
			continue
		}

		fd := filesData[i]
		log.Printf("[BATCH-UPLOAD] Processing item %d/%d for job %d (File: %s)", i+1, len(items), jobID, fd.Filename)

		// Process the item
		dataSourceUUID, err := s.processItem(ctx, &item, subjectID, userID, subject, fd)
		if err != nil {
			log.Printf("Error processing item %d: %v", item.ID, err)
			s.updateItemStatus(item.ID, model.IndexingJobItemStatusFailed, err.Error())
			failedItems++
		} else {
			s.updateItemStatus(item.ID, model.IndexingJobItemStatusCompleted, "")
			completedItems++
			if dataSourceUUID != "" {
				createdDataSourceUUIDs = append(createdDataSourceUUIDs, dataSourceUUID)
			}
		}

		// Update job progress
		s.updateJobProgress(jobID, completedItems, failedItems)

		// Update notification
		progress := ((completedItems + failedItems) * 100) / len(items)
		s.notificationService.UpdateNotificationForJob(ctx, jobID, model.NotificationTypeInProgress,
			fmt.Sprintf("Uploading documents (%d/%d)", completedItems+failedItems, len(items)),
			fmt.Sprintf("Processing documents for %s...", subject.Name),
			&model.NotificationMetadata{
				SubjectID:      subject.ID,
				SubjectName:    subject.Name,
				SubjectCode:    subject.Code,
				TotalItems:     len(items),
				CompletedItems: completedItems,
				FailedItems:    failedItems,
				Progress:       progress,
			},
		)
	}

	// Trigger KB indexing job for all created data sources
	kbIndexingStarted := false
	if len(createdDataSourceUUIDs) > 0 && subject.KnowledgeBaseUUID != "" && s.enableAI {
		log.Printf("Triggering indexing job for %d data sources in KB %s", len(createdDataSourceUUIDs), subject.KnowledgeBaseUUID)

		indexJob, err := s.doClient.StartIndexingJob(ctx, digitalocean.StartIndexingJobRequest{
			KnowledgeBaseUUID: subject.KnowledgeBaseUUID,
			DataSourceUUIDs:   createdDataSourceUUIDs,
		})
		if err != nil {
			log.Printf("Warning: Failed to trigger KB indexing job: %v", err)
			// Even if explicit indexing job failed, the data sources were created
			// and KB may auto-index them. Mark as KB indexing in progress.
			kbIndexingStarted = true
		} else {
			// Save DO indexing job UUID
			s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).Update("do_indexing_job_uuid", indexJob.UUID)
			kbIndexingStarted = true
			log.Printf("KB indexing job started: %s", indexJob.UUID)
		}
	}

	// Complete the job
	s.completeJob(ctx, jobID, userID, subject, completedItems, failedItems, len(items), kbIndexingStarted)
}

// processItem processes a single item in the batch
func (s *BatchDocumentService) processItem(ctx context.Context, item *model.IndexingJobItem, subjectID uint, userID uint, subject model.Subject, fd FileData) (string, error) {
	// Update status to uploading
	s.updateItemStatus(item.ID, model.IndexingJobItemStatusUploading, "")

	// Clean filename
	filename := s.sanitizeFilename(fd.Filename)

	// OCR Processing - extract text from PDF if it's a PDF file (check availability dynamically)
	var ocrText string
	var pageCount int
	isPDF := strings.HasSuffix(strings.ToLower(filename), ".pdf")

	if isPDF && s.ocrClient != nil {
		// Check OCR service availability at runtime
		ocrCtx, ocrCancel := context.WithTimeout(ctx, 5*time.Second)
		ocrHealthErr := s.ocrClient.HealthCheck(ocrCtx)
		ocrCancel()

		if ocrHealthErr == nil {
			s.updateItemStatus(item.ID, model.IndexingJobItemStatusOCR, "")
			log.Printf("[BATCH-UPLOAD] Running OCR on %s", filename)

			ocrResult, err := s.ocrClient.ProcessPDFFile(ctx, fd.Content, filename)
			if err != nil {
				log.Printf("Warning: OCR failed for %s: %v", filename, err)
				// Continue without OCR - it's not a fatal error
			} else {
				ocrText = ocrResult.Text
				pageCount = ocrResult.PageCount
				log.Printf("[BATCH-UPLOAD] OCR completed for %s: %d pages, %d chars extracted", filename, pageCount, len(ocrText))
			}
		} else {
			log.Printf("Warning: OCR service not available for %s: %v", filename, ocrHealthErr)
		}
	}

	// Update status to uploading
	s.updateItemStatus(item.ID, model.IndexingJobItemStatusUploading, "")

	// Determine content type
	contentType := s.getContentType(filename)

	// Upload to Spaces
	var spacesKey, spacesURL string
	if s.enableSpaces {
		key := digitalocean.GenerateKey(fmt.Sprintf("subjects/%d/documents", subjectID), filename)
		url, err := s.spacesClient.UploadBytes(ctx, key, fd.Content, contentType)
		if err != nil {
			return "", fmt.Errorf("failed to upload to Spaces: %w", err)
		}
		spacesKey = key
		spacesURL = url
	}

	// Create document record with OCR text
	document := &model.Document{
		SubjectID:        &subjectID,
		Type:             fd.DocumentType,
		Filename:         filename,
		SpacesURL:        spacesURL,
		SpacesKey:        spacesKey,
		IndexingStatus:   model.IndexingStatusPending,
		FileSize:         fd.FileSize,
		PageCount:        pageCount,
		UploadedByUserID: userID,
		OCRText:          ocrText, // Store OCR extracted text
	}

	if err := s.db.Create(document).Error; err != nil {
		return "", fmt.Errorf("failed to create document record: %w", err)
	}

	// Update item with document ID
	s.db.Model(&model.IndexingJobItem{}).Where("id = ?", item.ID).Update("document_id", document.ID)

	// Update status to indexing
	s.updateItemStatus(item.ID, model.IndexingJobItemStatusIndexing, "")

	// Create data source in KB using OCR text (not PDF)
	// This ensures DO indexes our high-quality OCR text instead of doing its own PDF extraction
	var dataSourceUUID string
	if s.enableAI && subject.KnowledgeBaseUUID != "" && s.enableSpaces {
		spacesName := os.Getenv("DO_SPACES_NAME")
		spacesRegion := os.Getenv("DO_SPACES_REGION")
		if spacesRegion == "" {
			spacesRegion = "blr1"
		}

		// Determine what to index in KB
		var itemPathForKB string

		if isPDF && ocrText != "" {
			// Upload OCR text as .txt file to Spaces for KB indexing
			// This gives DO clean text to index instead of having to OCR the PDF itself
			ocrTextKey := digitalocean.GenerateKey(fmt.Sprintf("subjects/%d/documents/ocr", subjectID), strings.TrimSuffix(filename, ".pdf")+".txt")
			_, err := s.spacesClient.UploadBytes(ctx, ocrTextKey, []byte(ocrText), "text/plain")
			if err != nil {
				log.Printf("Warning: Failed to upload OCR text to Spaces: %v", err)
				// Fall back to using original file
				itemPathForKB = spacesKey
			} else {
				log.Printf("[BATCH-UPLOAD] Uploaded OCR text to Spaces: %s (%d chars)", ocrTextKey, len(ocrText))
				itemPathForKB = ocrTextKey
				// Store the OCR text key for reference
				document.OCRSpacesKey = ocrTextKey
			}
		} else {
			// No OCR text available or not a PDF, use original file (DO will do its own extraction)
			itemPathForKB = spacesKey
			log.Printf("[BATCH-UPLOAD] Using original file for KB indexing: %s", spacesKey)
		}

		if itemPathForKB != "" {
			dsReq := digitalocean.CreateDataSourceRequest{
				KnowledgeBaseUUID: subject.KnowledgeBaseUUID,
				SpacesDataSource: &digitalocean.SpacesDataSourceInput{
					BucketName: spacesName,
					Region:     spacesRegion,
					ItemPath:   itemPathForKB,
				},
			}

			dataSource, _, err := s.doClient.CreateDataSource(ctx, subject.KnowledgeBaseUUID, dsReq)
			if err != nil {
				log.Printf("Warning: Failed to create data source in KB: %v", err)
			} else {
				document.DataSourceID = dataSource.UUID
				document.IndexingStatus = model.IndexingStatusInProgress
				dataSourceUUID = dataSource.UUID
				s.db.Save(document)
				log.Printf("[BATCH-UPLOAD] Created KB data source %s for %s", dataSource.UUID, itemPathForKB)
			}
		}
	}

	return dataSourceUUID, nil
}

// sanitizeFilename cleans up a filename for safe storage
func (s *BatchDocumentService) sanitizeFilename(filename string) string {
	// Get the base filename
	filename = filepath.Base(filename)

	// Replace problematic characters
	filename = strings.ReplaceAll(filename, " ", "-")
	filename = strings.ReplaceAll(filename, "/", "-")
	filename = strings.ReplaceAll(filename, "\\", "-")

	return filename
}

// getContentType returns the MIME type for a file based on extension
func (s *BatchDocumentService) getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	contentTypes := map[string]string{
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".csv":  "text/csv",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".html": "text/html",
		".htm":  "text/html",
		".json": "application/json",
	}

	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

// updateItemStatus updates the status of a job item
func (s *BatchDocumentService) updateItemStatus(itemID uint, status model.IndexingJobItemStatus, errorMsg string) {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}

	s.db.Model(&model.IndexingJobItem{}).Where("id = ?", itemID).Updates(updates)
}

// updateJobProgress updates the progress of an indexing job
func (s *BatchDocumentService) updateJobProgress(jobID uint, completed, failed int) {
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"completed_items": completed,
		"failed_items":    failed,
		"updated_at":      time.Now(),
	})
}

// failJob marks a job as failed
func (s *BatchDocumentService) failJob(ctx context.Context, jobID uint, userID uint, subject model.Subject, errorMsg string) {
	now := time.Now()
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":        model.IndexingJobStatusFailed,
		"error_message": errorMsg,
		"completed_at":  &now,
	})

	s.notificationService.FailNotification(ctx, jobID,
		"Document Upload Failed",
		fmt.Sprintf("Failed to upload documents for %s: %s", subject.Name, errorMsg),
		&model.NotificationMetadata{
			SubjectID:   subject.ID,
			SubjectName: subject.Name,
			SubjectCode: subject.Code,
		},
	)
}

// completeJob marks a job as completed or waiting for KB indexing
func (s *BatchDocumentService) completeJob(ctx context.Context, jobID uint, userID uint, subject model.Subject, completed, failed, total int, kbIndexingStarted bool) {
	now := time.Now()

	// Check if we have a DO indexing job UUID - if so, we're waiting for KB indexing
	var job model.IndexingJob
	s.db.First(&job, jobID)

	var status model.IndexingJobStatus
	var completedAt *time.Time

	if failed > 0 && completed == 0 {
		// All items failed - mark as failed immediately
		status = model.IndexingJobStatusFailed
		completedAt = &now
	} else if kbIndexingStarted && s.enableAI {
		// Files uploaded successfully and KB indexing was attempted
		// Keep status as "kb_indexing" until cron job confirms completion
		status = model.IndexingJobStatusKBIndexing
		// Don't set completedAt - job isn't truly complete yet
	} else if failed > 0 && completed > 0 {
		// Some items failed, no KB indexing
		status = model.IndexingJobStatusPartial
		completedAt = &now
	} else {
		// All items completed, no KB indexing
		status = model.IndexingJobStatusCompleted
		completedAt = &now
	}

	updates := map[string]interface{}{
		"status":          status,
		"completed_items": completed,
		"failed_items":    failed,
	}
	if completedAt != nil {
		updates["completed_at"] = completedAt
	}
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).Updates(updates)

	// Update notification based on status
	var title, message string
	var notificationType model.NotificationType

	if status == model.IndexingJobStatusFailed {
		notificationType = model.NotificationTypeError
		title = "Document Upload Failed"
		message = fmt.Sprintf("Failed to upload documents for %s", subject.Name)
	} else if status == model.IndexingJobStatusKBIndexing {
		// Keep notification as in_progress - KB indexing is not complete
		notificationType = model.NotificationTypeInProgress
		title = "Documents Processing"
		message = fmt.Sprintf("Uploaded %d documents for %s. Indexing for AI search...", completed, subject.Name)
		if failed > 0 {
			message = fmt.Sprintf("Uploaded %d documents for %s (%d failed). Indexing for AI search...", completed, subject.Name, failed)
		}
	} else if status == model.IndexingJobStatusPartial {
		notificationType = model.NotificationTypeWarning
		title = "Document Upload Partially Complete"
		message = fmt.Sprintf("Uploaded %d documents for %s (%d failed).", completed, subject.Name, failed)
	} else {
		// Completed (no KB indexing)
		notificationType = model.NotificationTypeSuccess
		title = "Documents Uploaded"
		message = fmt.Sprintf("Successfully uploaded %d documents for %s.", completed, subject.Name)
	}

	s.notificationService.UpdateNotificationForJob(ctx, jobID, notificationType, title, message,
		&model.NotificationMetadata{
			SubjectID:      subject.ID,
			SubjectName:    subject.Name,
			SubjectCode:    subject.Code,
			TotalItems:     total,
			CompletedItems: completed,
			FailedItems:    failed,
			Progress:       100,
		},
	)

	log.Printf("Batch upload job %d status: %s (%d completed, %d failed)", jobID, status, completed, failed)
}

// GetJobStatus retrieves the status of an indexing job
func (s *BatchDocumentService) GetJobStatus(ctx context.Context, jobID uint, userID uint) (*model.IndexingJob, error) {
	var job model.IndexingJob

	err := s.db.WithContext(ctx).
		Preload("Items").
		Preload("Subject").
		Where("id = ? AND created_by_user_id = ?", jobID, userID).
		First(&job).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("failed to fetch job: %w", err)
	}

	return &job, nil
}

// GetJobsBySubject retrieves all document upload jobs for a subject
func (s *BatchDocumentService) GetJobsBySubject(ctx context.Context, subjectID uint, userID uint) ([]model.IndexingJob, error) {
	var jobs []model.IndexingJob

	err := s.db.WithContext(ctx).
		Where("subject_id = ? AND created_by_user_id = ? AND job_type = ?", subjectID, userID, model.IndexingJobTypeDocumentUpload).
		Order("created_at DESC").
		Limit(20).
		Find(&jobs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", err)
	}

	return jobs, nil
}

// CancelJob cancels an active job
func (s *BatchDocumentService) CancelJob(ctx context.Context, jobID uint, userID uint) error {
	// Check if job exists and belongs to user
	var job model.IndexingJob
	if err := s.db.Where("id = ? AND created_by_user_id = ?", jobID, userID).First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("job not found")
		}
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	if job.IsComplete() {
		return fmt.Errorf("job is already complete")
	}

	// Cancel active processing
	s.activeJobsMu.RLock()
	cancel, exists := s.activeJobs[jobID]
	s.activeJobsMu.RUnlock()

	if exists {
		cancel()
	}

	// Update job status
	now := time.Now()
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":       model.IndexingJobStatusCancelled,
		"completed_at": &now,
	})

	return nil
}
