package document

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/pdfvalidation"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"github.com/sahilchouksey/go-init-setup/utils/validation"
	"gorm.io/gorm"
)

// DocumentHandler handles document-related requests
type DocumentHandler struct {
	db              *gorm.DB
	validator       *validation.Validator
	documentService *services.DocumentService
	syllabusService *services.SyllabusService
}

// NewDocumentHandler creates a new document handler
func NewDocumentHandler(db *gorm.DB, documentService *services.DocumentService) *DocumentHandler {
	return &DocumentHandler{
		db:              db,
		validator:       validation.NewValidator(),
		documentService: documentService,
		syllabusService: services.NewSyllabusService(db),
	}
}

// ListDocuments handles GET /api/v1/subjects/:subject_id/documents
func (h *DocumentHandler) ListDocuments(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")

	// Verify subject exists
	var subject model.Subject
	if err := h.db.First(&subject, subjectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to fetch subject")
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	docType := c.Query("type", "")
	indexingStatus := c.Query("status", "")

	// Build query
	query := h.db.Model(&model.Document{}).Where("subject_id = ?", subjectID)

	// Apply filters
	if docType != "" {
		query = query.Where("type = ?", docType)
	}
	if indexingStatus != "" {
		query = query.Where("indexing_status = ?", indexingStatus)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count documents")
	}

	// Calculate pagination
	offset := (page - 1) * limit
	pagination := response.CalculatePagination(page, limit, total)

	// Get documents with pagination
	var documents []model.Document
	if err := query.Preload("UploadedBy").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&documents).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch documents")
	}

	return response.Paginated(c, documents, pagination)
}

// GetDocument handles GET /api/v1/subjects/:subject_id/documents/:id
func (h *DocumentHandler) GetDocument(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")
	id := c.Params("id")

	var document model.Document
	if err := h.db.Preload("Subject").Preload("UploadedBy").
		Where("subject_id = ? AND id = ?", subjectID, id).
		First(&document).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Document not found")
		}
		return response.InternalServerError(c, "Failed to fetch document")
	}

	return response.Success(c, document)
}

// UploadDocument handles POST /api/v1/subjects/:subject_id/documents
func (h *DocumentHandler) UploadDocument(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")

	// Authorization: Admin only
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}
	if user.Role != "admin" {
		return response.Forbidden(c, "Only administrators can upload documents")
	}

	// Parse subject ID
	subID, err := strconv.ParseUint(subjectID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Get document type from form
	docTypeStr := c.FormValue("type")
	if docTypeStr == "" {
		return response.BadRequest(c, "Document type is required")
	}

	// Validate document type
	docType := model.DocumentType(docTypeStr)
	validTypes := map[model.DocumentType]bool{
		model.DocumentTypePYQ:       true,
		model.DocumentTypeBook:      true,
		model.DocumentTypeReference: true,
		model.DocumentTypeSyllabus:  true,
		model.DocumentTypeNotes:     true,
	}

	if !validTypes[docType] {
		return response.BadRequest(c, "Invalid document type. Must be one of: pyq, book, reference, syllabus, notes")
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "File is required")
	}

	// For PDF files, validate size and page count based on document type
	filename := strings.ToLower(file.Filename)
	if strings.HasSuffix(filename, ".pdf") {
		// Select appropriate limits based on document type
		var limits pdfvalidation.PDFLimits
		switch docType {
		case model.DocumentTypePYQ:
			limits = pdfvalidation.PYQLimits
		case model.DocumentTypeNotes:
			limits = pdfvalidation.NotesLimits
		case model.DocumentTypeSyllabus:
			limits = pdfvalidation.SyllabusLimits
		default:
			limits = pdfvalidation.DefaultLimits
		}

		validation, err := pdfvalidation.ValidatePDFFile(file, limits)
		if err != nil {
			return response.InternalServerError(c, "Failed to validate PDF: "+err.Error())
		}
		if !validation.Valid {
			return response.BadRequest(c, validation.Error)
		}
	} else {
		// Non-PDF files: just validate file size (max 50MB)
		const maxFileSize = 50 * 1024 * 1024 // 50MB
		if file.Size > maxFileSize {
			return response.BadRequest(c, "File size exceeds maximum allowed size of 50MB")
		}
	}

	// Open file
	fileContent, err := file.Open()
	if err != nil {
		return response.InternalServerError(c, "Failed to open file")
	}
	defer fileContent.Close()

	// Upload document using DocumentService
	result, err := h.documentService.UploadDocument(c.Context(), services.UploadDocumentRequest{
		SubjectID:  uint(subID),
		UserID:     user.ID,
		Type:       docType,
		File:       fileContent,
		FileHeader: file,
	})

	if err != nil {
		return response.InternalServerError(c, "Failed to upload document: "+err.Error())
	}

	// Preload relationships for response
	if err := h.db.Preload("Subject").Preload("UploadedBy").First(result.Document, result.Document.ID).Error; err != nil {
		return response.InternalServerError(c, "Failed to load document details")
	}

	// Add upload status to response
	responseData := fiber.Map{
		"document":           result.Document,
		"uploaded_to_spaces": result.UploadedToSpaces,
		"indexed_in_kb":      result.IndexedInKB,
	}

	return response.Created(c, responseData)
}

// UpdateDocument handles PUT /api/v1/subjects/:subject_id/documents/:id
func (h *DocumentHandler) UpdateDocument(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse request body
	var req struct {
		Type        string `json:"type"`
		OriginalURL string `json:"original_url"`
	}

	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Find document
	var document model.Document
	if err := h.db.Where("subject_id = ? AND id = ?", subjectID, id).
		First(&document).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Document not found")
		}
		return response.InternalServerError(c, "Failed to fetch document")
	}

	// Authorization: Admin or uploader can update
	if user.Role != "admin" && document.UploadedByUserID != user.ID {
		return response.Forbidden(c, "You don't have permission to update this document")
	}

	// Update fields
	if req.Type != "" {
		docType := model.DocumentType(req.Type)
		validTypes := map[model.DocumentType]bool{
			model.DocumentTypePYQ:       true,
			model.DocumentTypeBook:      true,
			model.DocumentTypeReference: true,
			model.DocumentTypeSyllabus:  true,
			model.DocumentTypeNotes:     true,
		}

		if !validTypes[docType] {
			return response.BadRequest(c, "Invalid document type")
		}
		document.Type = docType
	}

	if req.OriginalURL != "" {
		document.OriginalURL = req.OriginalURL
	}

	// Save changes
	if err := h.db.Save(&document).Error; err != nil {
		return response.InternalServerError(c, "Failed to update document")
	}

	// Preload relationships for response
	if err := h.db.Preload("Subject").Preload("UploadedBy").First(&document, document.ID).Error; err != nil {
		return response.InternalServerError(c, "Failed to load document details")
	}

	return response.Success(c, document)
}

// DeleteDocument handles DELETE /api/v1/subjects/:subject_id/documents/:id
func (h *DocumentHandler) DeleteDocument(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Find document
	var document model.Document
	if err := h.db.Where("subject_id = ? AND id = ?", subjectID, id).
		First(&document).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Document not found")
		}
		return response.InternalServerError(c, "Failed to fetch document")
	}

	// Authorization: Admin or uploader can delete
	if user.Role != "admin" && document.UploadedByUserID != user.ID {
		return response.Forbidden(c, "You don't have permission to delete this document")
	}

	// Parse document ID
	docID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid document ID")
	}

	// Use DocumentService for cleanup (deletes from Spaces and KB)
	if err := h.documentService.DeleteDocumentWithCleanup(c.Context(), uint(docID)); err != nil {
		return response.InternalServerError(c, "Failed to delete document: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "Document deleted successfully",
	})
}

// GetDownloadURL handles GET /api/v1/subjects/:subject_id/documents/:id/download
func (h *DocumentHandler) GetDownloadURL(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")
	id := c.Params("id")

	// Verify document exists and belongs to subject
	var document model.Document
	if err := h.db.Where("subject_id = ? AND id = ?", subjectID, id).
		First(&document).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Document not found")
		}
		return response.InternalServerError(c, "Failed to fetch document")
	}

	// Parse document ID
	docID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid document ID")
	}

	// Get expiration from query params (default: 60 minutes)
	expirationMinutes, _ := strconv.Atoi(c.Query("expiration", "60"))
	if expirationMinutes < 1 || expirationMinutes > 1440 { // Max 24 hours
		expirationMinutes = 60
	}

	// Generate download URL
	downloadURL, err := h.documentService.GetDocumentDownloadURL(c.Context(), uint(docID), expirationMinutes)
	if err != nil {
		return response.InternalServerError(c, "Failed to generate download URL: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"download_url": downloadURL,
		"expires_in":   expirationMinutes,
	})
}

// RefreshIndexingStatus handles POST /api/v1/subjects/:subject_id/documents/:id/refresh-status
func (h *DocumentHandler) RefreshIndexingStatus(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")
	id := c.Params("id")

	// Get user from context (authentication required)
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Verify document exists
	var document model.Document
	if err := h.db.Where("subject_id = ? AND id = ?", subjectID, id).
		First(&document).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Document not found")
		}
		return response.InternalServerError(c, "Failed to fetch document")
	}

	// Parse document ID
	docID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid document ID")
	}

	// Update indexing status
	if err := h.documentService.UpdateIndexingStatus(c.Context(), uint(docID)); err != nil {
		return response.InternalServerError(c, "Failed to refresh indexing status: "+err.Error())
	}

	// Reload document
	if err := h.db.Preload("Subject").Preload("UploadedBy").First(&document, docID).Error; err != nil {
		return response.InternalServerError(c, "Failed to load document details")
	}

	return response.Success(c, document)
}
