package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"gorm.io/gorm"
)

// DocumentService handles document upload and management with DigitalOcean integration
type DocumentService struct {
	db           *gorm.DB
	doClient     *digitalocean.Client
	spacesClient *digitalocean.SpacesClient
	ocrClient    *OCRClient
	enableAI     bool
	enableSpaces bool
	enableOCR    bool
}

// NewDocumentService creates a new document service
func NewDocumentService(db *gorm.DB) *DocumentService {
	service := &DocumentService{
		db:           db,
		enableAI:     false,
		enableSpaces: false,
	}

	// Initialize DigitalOcean client for AI features
	apiToken := os.Getenv("DIGITALOCEAN_TOKEN")
	if apiToken != "" {
		service.doClient = digitalocean.NewClient(digitalocean.Config{
			APIToken: apiToken,
		})
		service.enableAI = true
	} else {
		log.Println("Warning: DIGITALOCEAN_TOKEN not set. AI indexing will be disabled.")
	}

	// Initialize Spaces client using global config (supports auto-generation of keys)
	spacesClient, err := digitalocean.NewSpacesClientFromGlobalConfig()
	if err != nil {
		log.Printf("Warning: Failed to initialize Spaces client: %v. File storage will be disabled.", err)
	} else {
		service.spacesClient = spacesClient
		service.enableSpaces = true
	}

	// Initialize OCR client
	ocrServiceURL := os.Getenv("OCR_SERVICE_URL")
	if ocrServiceURL != "" {
		service.ocrClient = NewOCRClient()
		service.enableOCR = true
		log.Println("OCR service enabled at:", ocrServiceURL)
	} else {
		log.Println("Warning: OCR_SERVICE_URL not set. OCR processing will be disabled.")
	}

	return service
}

// UploadDocumentRequest represents a request to upload a document
// Either SubjectID or SemesterID should be provided (not both)
// - SubjectID: for subject-specific documents (PYQs, notes, etc.)
// - SemesterID: for semester-level documents (syllabus PDFs that contain multiple subjects)
type UploadDocumentRequest struct {
	SubjectID  uint // Optional - for subject-specific documents
	SemesterID uint // Optional - for semester-level documents (e.g., syllabus PDFs)
	UserID     uint
	Type       model.DocumentType
	File       multipart.File
	FileHeader *multipart.FileHeader
}

// UploadDocumentResult represents the result of document upload
type UploadDocumentResult struct {
	Document         *model.Document
	UploadedToSpaces bool
	IndexedInKB      bool
	Error            error
}

// ValidateFileType checks if the file type is supported
func ValidateFileType(filename string) (bool, string) {
	allowedExtensions := map[string]bool{
		".pdf":  true,
		".docx": true,
		".doc":  true,
		".txt":  true,
		".md":   true,
		".csv":  true,
		".xlsx": true,
		".xls":  true,
		".pptx": true,
		".ppt":  true,
		".html": true,
		".htm":  true,
		".json": true,
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExtensions[ext] {
		return false, fmt.Sprintf("File type %s is not supported", ext)
	}
	return true, ""
}

// UploadDocument handles the complete document upload flow
// Supports two modes:
// 1. Subject-based upload (SubjectID provided): for subject-specific documents
// 2. Semester-based upload (SemesterID provided): for semester-level documents like syllabus PDFs
func (s *DocumentService) UploadDocument(ctx context.Context, req UploadDocumentRequest) (*UploadDocumentResult, error) {
	result := &UploadDocumentResult{}

	// Validate that either SubjectID or SemesterID is provided (but not both required)
	if req.SubjectID == 0 && req.SemesterID == 0 {
		return nil, fmt.Errorf("either SubjectID or SemesterID must be provided")
	}

	// Validate file type
	if valid, errMsg := ValidateFileType(req.FileHeader.Filename); !valid {
		return nil, fmt.Errorf("%s", errMsg)
	}

	// Start database transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			result.Error = fmt.Errorf("panic during document upload: %v", r)
		}
	}()

	// Variables for optional relationships
	var subject *model.Subject
	var spacesKeyPrefix string

	// 1. Verify subject or semester exists
	if req.SubjectID > 0 {
		// Subject-based upload
		var subj model.Subject
		if err := tx.First(&subj, req.SubjectID).Error; err != nil {
			tx.Rollback()
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("subject not found")
			}
			return nil, fmt.Errorf("failed to fetch subject: %w", err)
		}
		subject = &subj
		spacesKeyPrefix = fmt.Sprintf("subjects/%d", req.SubjectID)
	} else {
		// Semester-based upload (for syllabus PDFs)
		var semester model.Semester
		if err := tx.First(&semester, req.SemesterID).Error; err != nil {
			tx.Rollback()
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("semester not found")
			}
			return nil, fmt.Errorf("failed to fetch semester: %w", err)
		}
		log.Printf("DocumentService: Semester-based upload for semester ID=%d", semester.ID)
		spacesKeyPrefix = fmt.Sprintf("semesters/%d/syllabus", req.SemesterID)
	}

	// 2. Read file content
	fileContent, err := io.ReadAll(req.File)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 3. Create document record in database
	document := model.Document{
		Type:             req.Type,
		Filename:         req.FileHeader.Filename,
		FileSize:         req.FileHeader.Size,
		IndexingStatus:   model.IndexingStatusPending,
		UploadedByUserID: req.UserID,
	}

	// Set the appropriate foreign key based on upload mode
	if req.SubjectID > 0 {
		document.SubjectID = &req.SubjectID
	}
	if req.SemesterID > 0 {
		document.SemesterID = &req.SemesterID
	}

	// If Spaces is not enabled, we'll still create the record but mark it appropriately
	if !s.enableSpaces {
		document.SpacesURL = "disabled"
		document.SpacesKey = "disabled"
	}

	if err := tx.Create(&document).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create document record: %w", err)
	}

	result.Document = &document

	// 4. Upload to Spaces (if enabled)
	if s.enableSpaces {
		// Generate unique key based on upload mode
		key := digitalocean.GenerateKey(spacesKeyPrefix, req.FileHeader.Filename)
		contentType := digitalocean.GetContentType(req.FileHeader.Filename)

		spacesURL, err := s.spacesClient.UploadBytes(ctx, key, fileContent, contentType)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to upload to Spaces: %w", err)
		}

		document.SpacesKey = key
		document.SpacesURL = spacesURL
		result.UploadedToSpaces = true
	}

	// 5. Upload to Knowledge Base for AI indexing (if enabled and subject-based)
	// Note: Semester-based uploads (syllabus PDFs) don't need KB indexing as they're processed differently
	if s.enableAI && subject != nil && subject.KnowledgeBaseUUID != "" && s.enableSpaces && document.SpacesKey != "" {
		// Get Spaces config for the data source
		spacesName := os.Getenv("DO_SPACES_NAME")
		spacesRegion := os.Getenv("DO_SPACES_REGION")
		if spacesRegion == "" {
			spacesRegion = "blr1"
		}

		// Create data source in knowledge base using Spaces path
		dsReq := digitalocean.CreateDataSourceRequest{
			KnowledgeBaseUUID: subject.KnowledgeBaseUUID,
			SpacesDataSource: &digitalocean.SpacesDataSourceInput{
				BucketName: spacesName,
				Region:     spacesRegion,
				ItemPath:   document.SpacesKey, // Path to the file we already uploaded
			},
		}

		dataSource, _, err := s.doClient.CreateDataSource(ctx, subject.KnowledgeBaseUUID, dsReq)
		if err != nil {
			// Don't fail the entire upload if KB indexing fails
			log.Printf("Warning: Failed to create data source in KB: %v", err)
			document.IndexingStatus = model.IndexingStatusFailed
			document.IndexingError = err.Error()
		} else {
			document.DataSourceID = dataSource.UUID
			document.IndexingStatus = model.IndexingStatusInProgress
			result.IndexedInKB = true
			log.Printf("Created data source %s for document %s in KB %s", dataSource.UUID, document.Filename, subject.KnowledgeBaseUUID)

			// Trigger indexing job to start processing the document
			if _, err := s.doClient.StartIndexingJob(ctx, digitalocean.StartIndexingJobRequest{
				KnowledgeBaseUUID: subject.KnowledgeBaseUUID,
				DataSourceUUIDs:   []string{dataSource.UUID},
			}); err != nil {
				log.Printf("Warning: Failed to trigger indexing job: %v", err)
				// Don't fail - the document is still in KB, just not indexed yet
			}
		}
	}

	// 6. Update document with all information
	if err := tx.Save(&document).Error; err != nil {
		// Try to clean up Spaces if update fails
		if s.enableSpaces && document.SpacesKey != "" {
			s.spacesClient.DeleteFile(ctx, document.SpacesKey)
		}
		tx.Rollback()
		return nil, fmt.Errorf("failed to update document record: %w", err)
	}

	// 7. Commit transaction
	if err := tx.Commit().Error; err != nil {
		// Clean up Spaces on commit failure
		if s.enableSpaces && document.SpacesKey != "" {
			s.spacesClient.DeleteFile(ctx, document.SpacesKey)
		}
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 8. Process OCR for PDF files (synchronous - waits for completion)
	if s.enableOCR && s.enableSpaces && strings.HasSuffix(strings.ToLower(document.Filename), ".pdf") {
		if err := s.processDocumentOCR(ctx, &document); err != nil {
			log.Printf("Warning: OCR processing failed for document %d: %v", document.ID, err)
			// Don't fail the upload if OCR fails, just log the error
		}
	}

	return result, nil
}

// uploadToPresignedURL uploads file to DigitalOcean presigned URL
func (s *DocumentService) uploadToPresignedURL(ctx context.Context, presignedURL *digitalocean.PresignedUploadURL, fileContent []byte, filename string) error {
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add form fields from presigned URL
	for key, value := range presignedURL.Fields {
		if err := writer.WriteField(key, value); err != nil {
			return fmt.Errorf("failed to write form field: %w", err)
		}
	}

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(fileContent); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Make POST request to presigned URL
	req, err := http.NewRequestWithContext(ctx, "POST", presignedURL.URL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteDocumentWithCleanup deletes a document and cleans up associated resources
func (s *DocumentService) DeleteDocumentWithCleanup(ctx context.Context, documentID uint) error {
	// Get document with subject relationship (if exists)
	var document model.Document
	if err := s.db.Preload("Subject").First(&document, documentID).Error; err != nil {
		return fmt.Errorf("failed to fetch document: %w", err)
	}

	// Clean up Knowledge Base data source if it exists (only for subject-based documents)
	if s.enableAI && document.Subject != nil && document.Subject.KnowledgeBaseUUID != "" && document.DataSourceID != "" {
		if err := s.doClient.DeleteDataSource(ctx, document.Subject.KnowledgeBaseUUID, document.DataSourceID); err != nil {
			log.Printf("Warning: Failed to delete data source %s: %v", document.DataSourceID, err)
		}
	}

	// Clean up Spaces file if it exists
	if s.enableSpaces && document.SpacesKey != "" && document.SpacesKey != "disabled" {
		if err := s.spacesClient.DeleteFile(ctx, document.SpacesKey); err != nil {
			log.Printf("Warning: Failed to delete file from Spaces %s: %v", document.SpacesKey, err)
		}
	}

	// Delete document from database (soft delete)
	if err := s.db.Delete(&document).Error; err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	return nil
}

// UpdateIndexingStatus updates the indexing status of a document
func (s *DocumentService) UpdateIndexingStatus(ctx context.Context, documentID uint) error {
	var document model.Document
	if err := s.db.Preload("Subject").First(&document, documentID).Error; err != nil {
		return fmt.Errorf("failed to fetch document: %w", err)
	}

	// Only update for subject-based documents with KB integration
	if !s.enableAI || document.Subject == nil || document.Subject.KnowledgeBaseUUID == "" || document.DataSourceID == "" {
		return nil // Nothing to update
	}

	// Get data source status from Knowledge Base
	dataSource, err := s.doClient.GetDataSource(ctx, document.Subject.KnowledgeBaseUUID, document.DataSourceID)
	if err != nil {
		return fmt.Errorf("failed to get data source status: %w", err)
	}

	// Update indexing status based on last indexing job status
	if dataSource.LastIndexingJob != nil {
		switch dataSource.LastIndexingJob.Status {
		case "INDEX_JOB_STATUS_COMPLETED":
			document.IndexingStatus = model.IndexingStatusCompleted
		case "INDEX_JOB_STATUS_IN_PROGRESS":
			document.IndexingStatus = model.IndexingStatusInProgress
		case "INDEX_JOB_STATUS_FAILED":
			document.IndexingStatus = model.IndexingStatusFailed
			document.IndexingError = "Indexing failed in knowledge base"
		case "INDEX_JOB_STATUS_PENDING":
			document.IndexingStatus = model.IndexingStatusPending
		default:
			document.IndexingStatus = model.IndexingStatusPending
		}
	} else {
		// No indexing job yet - still pending
		document.IndexingStatus = model.IndexingStatusPending
	}

	if err := s.db.Save(&document).Error; err != nil {
		return fmt.Errorf("failed to update document status: %w", err)
	}

	return nil
}

// GetDocumentDownloadURL returns a download URL for a document
func (s *DocumentService) GetDocumentDownloadURL(ctx context.Context, documentID uint, expirationMinutes int) (string, error) {
	var document model.Document
	if err := s.db.First(&document, documentID).Error; err != nil {
		return "", fmt.Errorf("failed to fetch document: %w", err)
	}

	if !s.enableSpaces || document.SpacesKey == "" || document.SpacesKey == "disabled" {
		return "", fmt.Errorf("document storage is not available")
	}

	// Generate presigned URL for temporary access
	expiration := time.Duration(expirationMinutes) * time.Minute
	url, err := s.spacesClient.GetPresignedURL(document.SpacesKey, expiration)
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %w", err)
	}

	return url, nil
}

// processDocumentOCR processes a PDF document with OCR (synchronous)
func (s *DocumentService) processDocumentOCR(ctx context.Context, document *model.Document) error {
	log.Printf("Starting OCR processing for document ID=%d, file=%s", document.ID, document.Filename)

	// Get presigned URL for the PDF
	presignedURL, err := s.spacesClient.GetPresignedURL(document.SpacesKey, 10*time.Minute)
	if err != nil {
		log.Printf("Failed to generate presigned URL for OCR: %v", err)
		return fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	// Process PDF from URL (OCR service will download it)
	ocrResp, err := s.ocrClient.ProcessPDFFromURL(ctx, presignedURL)
	if err != nil {
		log.Printf("OCR processing failed for document %d: %v", document.ID, err)
		return fmt.Errorf("OCR processing failed: %w", err)
	}

	// Update document with OCR text and page count
	updates := map[string]interface{}{
		"ocr_text":   ocrResp.Text,
		"page_count": ocrResp.PageCount,
	}

	if err := s.db.Model(document).Updates(updates).Error; err != nil {
		log.Printf("Failed to update document with OCR results: %v", err)
		return fmt.Errorf("failed to update document: %w", err)
	}

	log.Printf("OCR completed successfully for document %d: %d pages, %d characters",
		document.ID, ocrResp.PageCount, len(ocrResp.Text))

	return nil
}
