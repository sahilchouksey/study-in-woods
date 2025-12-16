package syllabus

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// SyllabusHandler handles syllabus-related requests
type SyllabusHandler struct {
	db              *gorm.DB
	syllabusService *services.SyllabusService
	documentService *services.DocumentService
	progressTracker *services.ProgressTracker
}

// NewSyllabusHandler creates a new syllabus handler
func NewSyllabusHandler(db *gorm.DB, syllabusService *services.SyllabusService, documentService *services.DocumentService) *SyllabusHandler {
	return &SyllabusHandler{
		db:              db,
		syllabusService: syllabusService,
		documentService: documentService,
	}
}

// NewSyllabusHandlerWithTracker creates a new syllabus handler with progress tracking support
func NewSyllabusHandlerWithTracker(
	db *gorm.DB,
	syllabusService *services.SyllabusService,
	documentService *services.DocumentService,
	progressTracker *services.ProgressTracker,
) *SyllabusHandler {
	return &SyllabusHandler{
		db:              db,
		syllabusService: syllabusService,
		documentService: documentService,
		progressTracker: progressTracker,
	}
}

// SetProgressTracker sets the progress tracker for SSE streaming support
func (h *SyllabusHandler) SetProgressTracker(tracker *services.ProgressTracker) {
	h.progressTracker = tracker
}

// GetSyllabusBySubject handles GET /api/v1/subjects/:subject_id/syllabus
// Returns the first/primary syllabus for a subject
func (h *SyllabusHandler) GetSyllabusBySubject(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")

	// Parse subject ID
	subID, err := strconv.ParseUint(subjectID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Verify subject exists
	var subject model.Subject
	if err := h.db.First(&subject, subID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to fetch subject")
	}

	// Get syllabus
	syllabus, err := h.syllabusService.GetSyllabusBySubject(c.Context(), uint(subID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Syllabus not found for this subject. Upload a syllabus document to extract data.")
		}
		return response.InternalServerError(c, "Failed to fetch syllabus")
	}

	return response.Success(c, syllabus.ToResponse())
}

// GetAllSyllabusesBySubject handles GET /api/v1/subjects/:subject_id/syllabuses
// Returns all syllabuses for a subject (multiple may exist from different documents)
func (h *SyllabusHandler) GetAllSyllabusesBySubject(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")

	// Parse subject ID
	subID, err := strconv.ParseUint(subjectID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Verify subject exists
	var subject model.Subject
	if err := h.db.First(&subject, subID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to fetch subject")
	}

	// Get all syllabuses
	syllabuses, err := h.syllabusService.GetAllSyllabusesBySubject(c.Context(), uint(subID))
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch syllabuses")
	}

	// Convert to response format
	var syllabusResponses []*model.SyllabusResponse
	for _, syl := range syllabuses {
		syllabusResponses = append(syllabusResponses, syl.ToResponse())
	}

	return response.Success(c, fiber.Map{
		"subject_id": subID,
		"count":      len(syllabusResponses),
		"syllabuses": syllabusResponses,
	})
}

// GetSyllabusById handles GET /api/v1/syllabus/:id
func (h *SyllabusHandler) GetSyllabusById(c *fiber.Ctx) error {
	syllabusID := c.Params("id")

	// Parse syllabus ID
	sID, err := strconv.ParseUint(syllabusID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid syllabus ID")
	}

	// Get syllabus
	syllabus, err := h.syllabusService.GetSyllabusByID(c.Context(), uint(sID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Syllabus not found")
		}
		return response.InternalServerError(c, "Failed to fetch syllabus")
	}

	return response.Success(c, syllabus.ToResponse())
}

// ExtractSyllabus handles POST /api/v1/documents/:document_id/extract-syllabus
// Triggers syllabus extraction from a document
// Now supports multi-subject extraction - returns array of syllabuses
func (h *SyllabusHandler) ExtractSyllabus(c *fiber.Ctx) error {
	documentID := c.Params("document_id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse document ID
	docID, err := strconv.ParseUint(documentID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid document ID")
	}

	// Verify document exists and is a syllabus type
	var document model.Document
	if err := h.db.Preload("Subject").First(&document, docID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Document not found")
		}
		return response.InternalServerError(c, "Failed to fetch document")
	}

	// Verify document is syllabus type
	if document.Type != model.DocumentTypeSyllabus {
		return response.BadRequest(c, "Document is not a syllabus type. Only syllabus documents can be extracted.")
	}

	// Check user permission (admin or document uploader)
	if user.Role != "admin" && document.UploadedByUserID != user.ID {
		return response.Forbidden(c, "You don't have permission to extract this syllabus")
	}

	// Check if async extraction is requested
	async := c.Query("async", "false") == "true"

	if async {
		// Trigger async extraction
		h.syllabusService.TriggerExtractionAsync(uint(docID))
		return response.Success(c, fiber.Map{
			"message":    "Syllabus extraction started in background",
			"status":     "processing",
			"subject_id": document.SubjectID,
		})
	}

	// Synchronous extraction - returns multiple syllabuses
	syllabuses, err := h.syllabusService.ExtractSyllabusFromDocument(c.Context(), uint(docID))
	if err != nil {
		return response.InternalServerError(c, "Failed to extract syllabus: "+err.Error())
	}

	// Reload each syllabus with relationships and convert to response
	var syllabusResponses []*model.SyllabusResponse
	for _, syl := range syllabuses {
		reloaded, _ := h.syllabusService.GetSyllabusByID(c.Context(), syl.ID)
		if reloaded != nil {
			syllabusResponses = append(syllabusResponses, reloaded.ToResponse())
		}
	}

	return response.Success(c, fiber.Map{
		"message":        "Syllabus extracted successfully",
		"subjects_count": len(syllabusResponses),
		"syllabuses":     syllabusResponses,
	})
}

// GetExtractionStatus handles GET /api/v1/syllabus/:id/status
// Returns the current extraction status
func (h *SyllabusHandler) GetExtractionStatus(c *fiber.Ctx) error {
	syllabusID := c.Params("id")

	// Parse syllabus ID
	sID, err := strconv.ParseUint(syllabusID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid syllabus ID")
	}

	status, errMsg, err := h.syllabusService.GetExtractionStatus(c.Context(), uint(sID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Syllabus not found")
		}
		return response.InternalServerError(c, "Failed to fetch status")
	}

	responseData := fiber.Map{
		"status": status,
	}
	if errMsg != "" {
		responseData["error"] = errMsg
	}

	return response.Success(c, responseData)
}

// RetryExtraction handles POST /api/v1/syllabus/:id/retry
// Retries a failed extraction - now supports multi-subject extraction
func (h *SyllabusHandler) RetryExtraction(c *fiber.Ctx) error {
	syllabusID := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse syllabus ID
	sID, err := strconv.ParseUint(syllabusID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid syllabus ID")
	}

	// Get syllabus to check ownership
	var syllabus model.Syllabus
	if err := h.db.Preload("Document").First(&syllabus, sID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Syllabus not found")
		}
		return response.InternalServerError(c, "Failed to fetch syllabus")
	}

	// Check permission
	if user.Role != "admin" && syllabus.Document.UploadedByUserID != user.ID {
		return response.Forbidden(c, "You don't have permission to retry this extraction")
	}

	// Check if async is requested
	async := c.Query("async", "false") == "true"

	if async {
		// Trigger async retry
		h.syllabusService.TriggerExtractionAsync(syllabus.DocumentID)
		return response.Success(c, fiber.Map{
			"message": "Syllabus extraction retry started in background",
			"status":  "processing",
		})
	}

	// Synchronous retry - returns multiple syllabuses
	syllabuses, err := h.syllabusService.RetryExtraction(c.Context(), uint(sID))
	if err != nil {
		return response.InternalServerError(c, "Failed to retry extraction: "+err.Error())
	}

	// Reload each syllabus with relationships and convert to response
	var syllabusResponses []*model.SyllabusResponse
	for _, syl := range syllabuses {
		reloaded, _ := h.syllabusService.GetSyllabusByID(c.Context(), syl.ID)
		if reloaded != nil {
			syllabusResponses = append(syllabusResponses, reloaded.ToResponse())
		}
	}

	return response.Success(c, fiber.Map{
		"message":        "Syllabus extraction completed",
		"subjects_count": len(syllabusResponses),
		"syllabuses":     syllabusResponses,
	})
}

// DeleteSyllabus handles DELETE /api/v1/syllabus/:id
func (h *SyllabusHandler) DeleteSyllabus(c *fiber.Ctx) error {
	syllabusID := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Only admin can delete syllabus
	if user.Role != "admin" {
		return response.Forbidden(c, "Only administrators can delete syllabus data")
	}

	// Parse syllabus ID
	sID, err := strconv.ParseUint(syllabusID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid syllabus ID")
	}

	// Delete syllabus
	if err := h.syllabusService.DeleteSyllabus(c.Context(), uint(sID)); err != nil {
		return response.InternalServerError(c, "Failed to delete syllabus")
	}

	return response.Success(c, fiber.Map{
		"message": "Syllabus deleted successfully",
	})
}

// ListUnits handles GET /api/v1/syllabus/:id/units
func (h *SyllabusHandler) ListUnits(c *fiber.Ctx) error {
	syllabusID := c.Params("id")

	// Parse syllabus ID
	sID, err := strconv.ParseUint(syllabusID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid syllabus ID")
	}

	// Get units
	var units []model.SyllabusUnit
	if err := h.db.Preload("Topics", func(db *gorm.DB) *gorm.DB {
		return db.Order("topic_number ASC")
	}).Where("syllabus_id = ?", sID).Order("unit_number ASC").Find(&units).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch units")
	}

	return response.Success(c, units)
}

// GetUnit handles GET /api/v1/syllabus/:id/units/:unit_number
func (h *SyllabusHandler) GetUnit(c *fiber.Ctx) error {
	syllabusID := c.Params("id")
	unitNumber := c.Params("unit_number")

	// Parse IDs
	sID, err := strconv.ParseUint(syllabusID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid syllabus ID")
	}

	unitNum, err := strconv.Atoi(unitNumber)
	if err != nil {
		return response.BadRequest(c, "Invalid unit number")
	}

	// Get unit
	var unit model.SyllabusUnit
	if err := h.db.Preload("Topics", func(db *gorm.DB) *gorm.DB {
		return db.Order("topic_number ASC")
	}).Where("syllabus_id = ? AND unit_number = ?", sID, unitNum).First(&unit).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Unit not found")
		}
		return response.InternalServerError(c, "Failed to fetch unit")
	}

	return response.Success(c, unit)
}

// ListBooks handles GET /api/v1/syllabus/:id/books
func (h *SyllabusHandler) ListBooks(c *fiber.Ctx) error {
	syllabusID := c.Params("id")

	// Parse syllabus ID
	sID, err := strconv.ParseUint(syllabusID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid syllabus ID")
	}

	// Get books
	var books []model.BookReference
	if err := h.db.Where("syllabus_id = ?", sID).Find(&books).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch books")
	}

	return response.Success(c, books)
}

// SearchTopics handles GET /api/v1/subjects/:subject_id/syllabus/search
// Search topics within a subject's syllabus by keyword
func (h *SyllabusHandler) SearchTopics(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")
	query := c.Query("q", "")

	if query == "" {
		return response.BadRequest(c, "Search query is required")
	}

	// Parse subject ID
	subID, err := strconv.ParseUint(subjectID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Find syllabus for subject
	var syllabus model.Syllabus
	if err := h.db.Where("subject_id = ?", subID).First(&syllabus).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Syllabus not found for this subject")
		}
		return response.InternalServerError(c, "Failed to fetch syllabus")
	}

	// Search topics by title or keywords
	var topics []model.SyllabusTopic
	searchPattern := "%" + query + "%"
	if err := h.db.Joins("JOIN syllabus_units ON syllabus_units.id = syllabus_topics.unit_id").
		Where("syllabus_units.syllabus_id = ?", syllabus.ID).
		Where("syllabus_topics.title ILIKE ? OR syllabus_topics.keywords ILIKE ? OR syllabus_topics.description ILIKE ?",
			searchPattern, searchPattern, searchPattern).
		Preload("Unit").
		Find(&topics).Error; err != nil {
		return response.InternalServerError(c, "Failed to search topics")
	}

	return response.Success(c, fiber.Map{
		"query":   query,
		"count":   len(topics),
		"results": topics,
	})
}

// UploadAndExtractSyllabus handles POST /api/v1/semesters/:semester_id/syllabus/upload
// Uploads a syllabus file at the semester level, extracts it, and auto-creates subjects
func (h *SyllabusHandler) UploadAndExtractSyllabus(c *fiber.Ctx) error {
	semesterID := c.Params("semester_id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Verify semester exists
	var semester model.Semester
	if err := h.db.First(&semester, semesterID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Semester not found")
		}
		return response.InternalServerError(c, "Failed to fetch semester")
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

	// Extract syllabus synchronously to create subjects
	syllabuses, err := h.syllabusService.ExtractSyllabusFromDocument(c.Context(), result.Document.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to extract syllabus: "+err.Error())
	}

	// No temp subject cleanup needed - we're using semester-based upload now

	// Return the syllabuses and created subjects
	var createdSubjects []model.Subject
	for _, syl := range syllabuses {
		var subject model.Subject
		if err := h.db.First(&subject, syl.SubjectID).Error; err == nil {
			createdSubjects = append(createdSubjects, subject)
		}
	}

	return response.Success(c, fiber.Map{
		"message":          "Syllabus uploaded and extracted successfully",
		"document_id":      result.Document.ID,
		"syllabuses_count": len(syllabuses),
		"syllabuses":       syllabuses,
		"subjects_created": createdSubjects,
	})
}
