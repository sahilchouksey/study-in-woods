package document

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/pdfvalidation"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"strings"
)

// BatchDocumentUploadHandler handles batch document upload API endpoints
type BatchDocumentUploadHandler struct {
	batchDocumentService *services.BatchDocumentService
}

// NewBatchDocumentUploadHandler creates a new batch document upload handler
func NewBatchDocumentUploadHandler(batchDocumentService *services.BatchDocumentService) *BatchDocumentUploadHandler {
	return &BatchDocumentUploadHandler{
		batchDocumentService: batchDocumentService,
	}
}

// BatchUploadDocuments handles POST /api/v1/subjects/:subject_id/documents/batch-upload
// Starts a batch upload job for multiple documents
func (h *BatchDocumentUploadHandler) BatchUploadDocuments(c *fiber.Ctx) error {
	log.Printf("[BATCH-UPLOAD] BatchUploadDocuments called for subject_id: %s", c.Params("subject_id"))

	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		log.Printf("[BATCH-UPLOAD] BatchUploadDocuments - User not authenticated")
		return response.Unauthorized(c, "User not authenticated")
	}

	// Only admins can upload documents
	if user.Role != "admin" {
		return response.Forbidden(c, "Only administrators can upload documents")
	}

	// Parse subject ID
	subjectID, err := strconv.ParseUint(c.Params("subject_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		return response.BadRequest(c, "Failed to parse multipart form")
	}

	// Get files from form
	files := form.File["files"]
	if len(files) == 0 {
		return response.BadRequest(c, "At least one file is required")
	}

	if len(files) > 20 {
		return response.BadRequest(c, "Maximum 20 files per batch upload")
	}

	// Get document types from form (optional - defaults to "notes")
	// Format: types[0]=notes&types[1]=book&types[2]=reference
	typeValues := form.Value["types"]

	// Build document requests
	var documents []services.BatchDocumentRequest
	for i, fileHeader := range files {
		// Validate file type
		filename := strings.ToLower(fileHeader.Filename)
		if !isValidFileType(filename) {
			return response.BadRequest(c, "Invalid file type: "+fileHeader.Filename+". Supported: PDF, DOCX, DOC, TXT, MD, CSV, XLSX, XLS, PPTX, PPT, HTML, JSON")
		}

		// Get document type for this file FIRST (needed for validation limits)
		docType := model.DocumentTypeNotes // Default to notes
		if i < len(typeValues) && typeValues[i] != "" {
			parsedType := model.DocumentType(typeValues[i])
			if isValidDocumentType(parsedType) {
				docType = parsedType
			}
		}

		// For PDF files, validate size and page count
		if strings.HasSuffix(filename, ".pdf") {
			// Select appropriate limits based on document type
			var limits pdfvalidation.PDFLimits
			switch docType {
			case model.DocumentTypeSyllabus:
				limits = pdfvalidation.SyllabusLimits
			case model.DocumentTypePYQ:
				limits = pdfvalidation.PYQLimits
			case model.DocumentTypeNotes, model.DocumentTypeBook:
				limits = pdfvalidation.NotesLimits // 2000 pages for notes/books
			default:
				limits = pdfvalidation.DefaultLimits
			}

			validation, err := pdfvalidation.ValidatePDFFile(fileHeader, limits)
			if err != nil {
				return response.BadRequest(c, "Failed to validate PDF "+fileHeader.Filename+": "+err.Error())
			}
			if !validation.Valid {
				return response.BadRequest(c, "Invalid PDF "+fileHeader.Filename+": "+validation.Error)
			}
		} else {
			// Non-PDF files: validate file size (max 100MB for notes, 50MB for others)
			maxFileSize := int64(50 * 1024 * 1024) // 50MB default
			if docType == model.DocumentTypeNotes || docType == model.DocumentTypeBook {
				maxFileSize = 100 * 1024 * 1024 // 100MB for notes/books
			}
			if fileHeader.Size > maxFileSize {
				return response.BadRequest(c, "File "+fileHeader.Filename+" exceeds maximum size")
			}
		}

		documents = append(documents, services.BatchDocumentRequest{
			FileHeader:   fileHeader,
			DocumentType: docType,
		})
	}

	// Start batch upload
	result, err := h.batchDocumentService.StartBatchUpload(c.Context(), services.BatchUploadRequest{
		SubjectID: uint(subjectID),
		UserID:    user.ID,
		Documents: documents,
	})
	if err != nil {
		if err.Error() == "subject not found" {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to start batch upload: "+err.Error())
	}

	log.Printf("[BATCH-UPLOAD] BatchUploadDocuments - Job created: ID=%d, status=%s, total_items=%d",
		result.JobID, result.Status, result.TotalItems)

	return response.Success(c, fiber.Map{
		"job_id":      result.JobID,
		"status":      result.Status,
		"total_items": result.TotalItems,
		"message":     result.Message,
	})
}

// GetDocumentUploadJobsBySubject handles GET /api/v1/subjects/:subject_id/documents/upload-jobs
// Returns all document upload jobs for a subject
func (h *BatchDocumentUploadHandler) GetDocumentUploadJobsBySubject(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse subject ID
	subjectID, err := strconv.ParseUint(c.Params("subject_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	jobs, err := h.batchDocumentService.GetJobsBySubject(c.Context(), uint(subjectID), user.ID)
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

// Helper functions

func isValidFileType(filename string) bool {
	validExtensions := []string{
		".pdf", ".docx", ".doc", ".txt", ".md",
		".csv", ".xlsx", ".xls", ".pptx", ".ppt",
		".html", ".htm", ".json",
	}
	for _, ext := range validExtensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}

func isValidDocumentType(docType model.DocumentType) bool {
	validTypes := map[model.DocumentType]bool{
		model.DocumentTypePYQ:       true,
		model.DocumentTypeBook:      true,
		model.DocumentTypeReference: true,
		model.DocumentTypeSyllabus:  true,
		model.DocumentTypeNotes:     true,
	}
	return validTypes[docType]
}
