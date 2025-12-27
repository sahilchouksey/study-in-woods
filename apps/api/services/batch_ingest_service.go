package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

// BatchIngestService handles batch ingestion of PYQ papers
type BatchIngestService struct {
	db                  *gorm.DB
	doClient            *digitalocean.Client
	spacesClient        *digitalocean.SpacesClient
	notificationService *NotificationService
	pyqService          *PYQService
	ocrClient           *OCRClient
	enableAI            bool
	enableSpaces        bool
	// Note: enableOCR removed - we now check OCR availability dynamically at runtime

	// Active jobs tracking
	activeJobs   map[uint]context.CancelFunc
	activeJobsMu sync.RWMutex
}

// NewBatchIngestService creates a new batch ingest service
func NewBatchIngestService(db *gorm.DB, notificationService *NotificationService, pyqService *PYQService) *BatchIngestService {
	service := &BatchIngestService{
		db:                  db,
		notificationService: notificationService,
		pyqService:          pyqService,
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
		log.Println("Warning: DIGITALOCEAN_TOKEN not set. AI indexing will be disabled for batch ingest.")
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
	log.Println("OCR client initialized (availability will be checked at runtime)")

	return service
}

// BatchIngestPaperRequest represents a single paper to ingest
type BatchIngestPaperRequest struct {
	PDFURL     string `json:"pdf_url"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	Month      string `json:"month"`
	ExamType   string `json:"exam_type,omitempty"`
	SourceName string `json:"source_name,omitempty"`
}

// BatchIngestRequest represents a batch ingest request
type BatchIngestRequest struct {
	SubjectID         uint                      `json:"subject_id"`
	UserID            uint                      `json:"user_id"`
	Papers            []BatchIngestPaperRequest `json:"papers"`
	TriggerExtraction bool                      `json:"trigger_extraction"` // If true, automatically extract questions after upload
}

// BatchIngestResult represents the result of starting a batch ingest
type BatchIngestResult struct {
	JobID      uint   `json:"job_id"`
	Status     string `json:"status"`
	TotalItems int    `json:"total_items"`
	Message    string `json:"message"`
}

// StartBatchIngest creates a new batch ingest job and starts processing
func (s *BatchIngestService) StartBatchIngest(ctx context.Context, req BatchIngestRequest) (*BatchIngestResult, error) {
	// Validate subject exists and has knowledge base
	var subject model.Subject
	if err := s.db.First(&subject, req.SubjectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("subject not found")
		}
		return nil, fmt.Errorf("failed to fetch subject: %w", err)
	}

	// Require Knowledge Base for PYQ ingestion when AI is enabled
	// PYQ papers need to be indexed in KB for AI-powered search and chat features
	if subject.KnowledgeBaseUUID == "" && s.enableAI {
		return nil, fmt.Errorf("knowledge base not configured for this subject. Please wait for AI setup to complete before ingesting PYQ papers")
	}

	// Check for duplicate papers (same year+month already ingested)
	existingPapers, _ := s.pyqService.GetPYQsBySubject(ctx, req.SubjectID)
	existingKeys := make(map[string]bool)
	for _, paper := range existingPapers {
		key := fmt.Sprintf("%d-%s", paper.Year, paper.Month)
		existingKeys[key] = true
	}

	// Filter out duplicates
	var validPapers []BatchIngestPaperRequest
	for _, paper := range req.Papers {
		key := fmt.Sprintf("%d-%s", paper.Year, paper.Month)
		if !existingKeys[key] {
			validPapers = append(validPapers, paper)
		} else {
			log.Printf("Skipping duplicate paper: %s %d", paper.Month, paper.Year)
		}
	}

	if len(validPapers) == 0 {
		return nil, fmt.Errorf("all papers already exist for this subject")
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
		JobType:         model.IndexingJobTypeBatchPYQIngest,
		Status:          model.IndexingJobStatusPending,
		TotalItems:      len(validPapers),
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
	for _, paper := range validPapers {
		metadata := model.IndexingJobItemMetadata{
			Title:      paper.Title,
			Year:       paper.Year,
			Month:      paper.Month,
			ExamType:   paper.ExamType,
			SourceName: paper.SourceName,
		}
		metadataJSON, _ := json.Marshal(metadata)

		item := &model.IndexingJobItem{
			JobID:     job.ID,
			ItemType:  model.IndexingJobItemTypeExternalPDF,
			SourceURL: paper.PDFURL,
			Status:    model.IndexingJobItemStatusPending,
			Metadata:  datatypes.JSON(metadataJSON),
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
		model.NotificationCategoryPYQIngest,
		fmt.Sprintf("Ingesting %d PYQ papers", len(validPapers)),
		fmt.Sprintf("Processing papers for %s...", subject.Name),
		&model.NotificationMetadata{
			SubjectID:      subject.ID,
			SubjectName:    subject.Name,
			SubjectCode:    subject.Code,
			TotalItems:     len(validPapers),
			CompletedItems: 0,
			Progress:       0,
		},
	)
	if err != nil {
		log.Printf("Warning: Failed to create notification for job %d: %v", job.ID, err)
	}

	// Start background processing (always extracts questions)
	go s.processJob(job.ID, req.SubjectID, req.UserID, subject)

	log.Printf("========================================")
	log.Printf("[BATCH-INGEST] JOB %d CREATED", job.ID)
	log.Printf("[BATCH-INGEST] Subject: %s (ID: %d)", subject.Name, req.SubjectID)
	log.Printf("[BATCH-INGEST] User: %d", req.UserID)
	log.Printf("[BATCH-INGEST] Total Items: %d", len(validPapers))
	log.Printf("[BATCH-INGEST] Status: %s", model.IndexingJobStatusProcessing)
	log.Printf("========================================")

	return &BatchIngestResult{
		JobID:      job.ID,
		Status:     string(model.IndexingJobStatusProcessing),
		TotalItems: len(validPapers),
		Message:    fmt.Sprintf("Batch ingestion started with %d papers", len(validPapers)),
	}, nil
}

// processJob handles the background processing of a batch ingest job
func (s *BatchIngestService) processJob(jobID uint, subjectID uint, userID uint, subject model.Subject) {
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
	log.Printf("[BATCH-INGEST] GOROUTINE STARTED for job %d", jobID)
	log.Printf("[BATCH-INGEST] Subject ID: %d", subjectID)
	log.Printf("========================================")

	// Small delay to allow frontend to start polling before we begin processing
	// This prevents race conditions where job completes before frontend can poll
	time.Sleep(500 * time.Millisecond)
	log.Printf("[BATCH-INGEST] Starting processing after initial delay for job %d", jobID)

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

		log.Printf("[BATCH-INGEST] Processing item %d/%d for job %d (URL: %s)", i+1, len(items), jobID, item.SourceURL)

		// Parse metadata
		var metadata model.IndexingJobItemMetadata
		if err := json.Unmarshal(item.Metadata, &metadata); err != nil {
			log.Printf("Warning: Failed to parse item metadata: %v", err)
		}

		// Process the item
		dataSourceUUID, err := s.processItem(ctx, &item, subjectID, userID, subject, metadata)
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
			fmt.Sprintf("Ingesting PYQ papers (%d/%d)", completedItems+failedItems, len(items)),
			fmt.Sprintf("Processing papers for %s...", subject.Name),
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
func (s *BatchIngestService) processItem(ctx context.Context, item *model.IndexingJobItem, subjectID uint, userID uint, subject model.Subject, metadata model.IndexingJobItemMetadata) (string, error) {
	// Update status to downloading
	s.updateItemStatus(item.ID, model.IndexingJobItemStatusDownloading, "")

	// Download PDF from URL
	pdfData, err := s.downloadPDF(ctx, item.SourceURL)
	if err != nil {
		return "", fmt.Errorf("failed to download PDF: %w", err)
	}

	// Generate filename
	filename := s.generateFilename(metadata)

	// OCR Processing - extract text from PDF (check availability dynamically)
	var ocrText string
	var pageCount int
	if s.ocrClient != nil {
		// Check OCR service availability at runtime
		ocrCtx, ocrCancel := context.WithTimeout(ctx, 5*time.Second)
		ocrHealthErr := s.ocrClient.HealthCheck(ocrCtx)
		ocrCancel()

		if ocrHealthErr == nil {
			s.updateItemStatus(item.ID, model.IndexingJobItemStatusOCR, "")
			log.Printf("[BATCH-INGEST] Running OCR on %s", filename)

			ocrResult, err := s.ocrClient.ProcessPDFFile(ctx, pdfData, filename)
			if err != nil {
				log.Printf("Warning: OCR failed for %s: %v", filename, err)
				// Continue without OCR - it's not a fatal error
			} else {
				ocrText = ocrResult.Text
				pageCount = ocrResult.PageCount
				log.Printf("[BATCH-INGEST] OCR completed for %s: %d pages, %d chars extracted", filename, pageCount, len(ocrText))
			}
		} else {
			log.Printf("Warning: OCR service not available for %s: %v", filename, ocrHealthErr)
		}
	}

	// Update status to uploading
	s.updateItemStatus(item.ID, model.IndexingJobItemStatusUploading, "")

	// Upload to Spaces
	var spacesKey, spacesURL string
	if s.enableSpaces {
		key := digitalocean.GenerateKey(fmt.Sprintf("subjects/%d/pyqs", subjectID), filename)
		url, err := s.spacesClient.UploadBytes(ctx, key, pdfData, "application/pdf")
		if err != nil {
			return "", fmt.Errorf("failed to upload to Spaces: %w", err)
		}
		spacesKey = key
		spacesURL = url
	}

	// Create document record with OCR text
	document := &model.Document{
		SubjectID:        &subjectID,
		Type:             model.DocumentTypePYQ,
		Filename:         filename,
		OriginalURL:      item.SourceURL,
		SpacesURL:        spacesURL,
		SpacesKey:        spacesKey,
		IndexingStatus:   model.IndexingStatusPending,
		FileSize:         int64(len(pdfData)),
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

		if ocrText != "" {
			// Upload OCR text as .txt file to Spaces for KB indexing
			// This gives DO clean text to index instead of having to OCR the PDF itself
			ocrTextKey := digitalocean.GenerateKey(fmt.Sprintf("subjects/%d/pyqs/ocr", subjectID), strings.TrimSuffix(filename, ".pdf")+".txt")
			_, err := s.spacesClient.UploadBytes(ctx, ocrTextKey, []byte(ocrText), "text/plain")
			if err != nil {
				log.Printf("Warning: Failed to upload OCR text to Spaces: %v", err)
				// Fall back to using PDF
				itemPathForKB = spacesKey
			} else {
				log.Printf("[BATCH-INGEST] Uploaded OCR text to Spaces: %s (%d chars)", ocrTextKey, len(ocrText))
				itemPathForKB = ocrTextKey
				// Store the OCR text key for reference
				document.OCRSpacesKey = ocrTextKey
			}
		} else {
			// No OCR text available, use PDF (DO will do its own extraction)
			itemPathForKB = spacesKey
			log.Printf("[BATCH-INGEST] No OCR text available, using PDF for KB indexing: %s", spacesKey)
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
				log.Printf("[BATCH-INGEST] Created KB data source %s for %s", dataSource.UUID, itemPathForKB)
			}
		}
	}

	// Create PYQ paper record
	pyqPaper := &model.PYQPaper{
		SubjectID:        subjectID,
		DocumentID:       document.ID,
		Year:             metadata.Year,
		Month:            metadata.Month,
		ExamType:         metadata.ExamType,
		ExtractionStatus: model.PYQExtractionPending,
	}

	if err := s.db.Create(pyqPaper).Error; err != nil {
		log.Printf("Warning: Failed to create PYQ paper record: %v", err)
	} else {
		// Update item with PYQ paper ID
		s.db.Model(&model.IndexingJobItem{}).Where("id = ?", item.ID).Update("pyq_paper_id", pyqPaper.ID)

		// Always trigger async extraction of questions from the PYQ paper
		// This runs in background and doesn't block the batch ingest
		if s.pyqService != nil {
			log.Printf("[BATCH-INGEST] Triggering async extraction for document %d (PYQ paper %d)", document.ID, pyqPaper.ID)
			s.pyqService.TriggerExtractionAsync(document.ID)
		}
	}

	return dataSourceUUID, nil
}

// downloadPDF downloads a PDF from a URL
func (s *BatchIngestService) downloadPDF(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; StudyInWoods/1.0)")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Read response body
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return buf.Bytes(), nil
}

// generateFilename generates a filename for the PDF
func (s *BatchIngestService) generateFilename(metadata model.IndexingJobItemMetadata) string {
	// Clean title for filename
	title := strings.ReplaceAll(metadata.Title, " ", "-")
	title = strings.ReplaceAll(title, "/", "-")

	// Ensure it has .pdf extension
	if !strings.HasSuffix(strings.ToLower(title), ".pdf") {
		title = title + ".pdf"
	}

	return filepath.Base(title)
}

// updateItemStatus updates the status of a job item
func (s *BatchIngestService) updateItemStatus(itemID uint, status model.IndexingJobItemStatus, errorMsg string) {
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
func (s *BatchIngestService) updateJobProgress(jobID uint, completed, failed int) {
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"completed_items": completed,
		"failed_items":    failed,
		"updated_at":      time.Now(),
	})
}

// failJob marks a job as failed
func (s *BatchIngestService) failJob(ctx context.Context, jobID uint, userID uint, subject model.Subject, errorMsg string) {
	now := time.Now()
	s.db.Model(&model.IndexingJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":        model.IndexingJobStatusFailed,
		"error_message": errorMsg,
		"completed_at":  &now,
	})

	s.notificationService.FailNotification(ctx, jobID,
		"PYQ Ingestion Failed",
		fmt.Sprintf("Failed to ingest papers for %s: %s", subject.Name, errorMsg),
		&model.NotificationMetadata{
			SubjectID:   subject.ID,
			SubjectName: subject.Name,
			SubjectCode: subject.Code,
		},
	)
}

// completeJob marks a job as completed or waiting for KB indexing
func (s *BatchIngestService) completeJob(ctx context.Context, jobID uint, userID uint, subject model.Subject, completed, failed, total int, kbIndexingStarted bool) {
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
		// Files uploaded successfully and KB indexing was attempted (may or may not have explicit job UUID)
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
		title = "PYQ Upload Failed"
		message = fmt.Sprintf("Failed to upload papers for %s", subject.Name)
	} else if status == model.IndexingJobStatusKBIndexing {
		// Keep notification as in_progress - KB indexing is not complete
		notificationType = model.NotificationTypeInProgress
		title = "PYQ Papers Processing"
		message = fmt.Sprintf("Uploaded %d papers for %s. Indexing documents for AI search...", completed, subject.Name)
		if failed > 0 {
			message = fmt.Sprintf("Uploaded %d papers for %s (%d failed). Indexing documents for AI search...", completed, subject.Name, failed)
		}
	} else if status == model.IndexingJobStatusPartial {
		notificationType = model.NotificationTypeWarning
		title = "PYQ Upload Partially Complete"
		message = fmt.Sprintf("Uploaded %d papers for %s (%d failed).", completed, subject.Name, failed)
	} else {
		// Completed (no KB indexing)
		notificationType = model.NotificationTypeSuccess
		title = "PYQ Papers Uploaded"
		message = fmt.Sprintf("Successfully uploaded %d papers for %s.", completed, subject.Name)
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

	log.Printf("Batch ingest job %d status: %s (%d completed, %d failed)", jobID, status, completed, failed)
}

// GetJobStatus retrieves the status of an indexing job
func (s *BatchIngestService) GetJobStatus(ctx context.Context, jobID uint, userID uint) (*model.IndexingJob, error) {
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

// GetJobsBySubject retrieves all indexing jobs for a subject
func (s *BatchIngestService) GetJobsBySubject(ctx context.Context, subjectID uint, userID uint) ([]model.IndexingJob, error) {
	var jobs []model.IndexingJob

	err := s.db.WithContext(ctx).
		Where("subject_id = ? AND created_by_user_id = ?", subjectID, userID).
		Order("created_at DESC").
		Limit(20).
		Find(&jobs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", err)
	}

	return jobs, nil
}

// CancelJob cancels an active job
func (s *BatchIngestService) CancelJob(ctx context.Context, jobID uint, userID uint) error {
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
