package subject

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"github.com/sahilchouksey/go-init-setup/utils/validation"
	"gorm.io/gorm"
)

// SubjectHandler handles subject-related requests
type SubjectHandler struct {
	db             *gorm.DB
	validator      *validation.Validator
	subjectService *services.SubjectService
}

// NewSubjectHandler creates a new subject handler
func NewSubjectHandler(db *gorm.DB, subjectService *services.SubjectService) *SubjectHandler {
	return &SubjectHandler{
		db:             db,
		validator:      validation.NewValidator(),
		subjectService: subjectService,
	}
}

// CreateSubjectRequest represents the request body for creating a subject
type CreateSubjectRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=255"`
	Code        string `json:"code" validate:"required,min=2,max=50"`
	Credits     int    `json:"credits" validate:"required,min=0,max=20"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}

// UpdateSubjectRequest represents the request body for updating a subject
type UpdateSubjectRequest struct {
	Name        string `json:"name" validate:"omitempty,min=2,max=255"`
	Code        string `json:"code" validate:"omitempty,min=2,max=50"`
	Credits     *int   `json:"credits" validate:"omitempty,min=0,max=20"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}

// ListSubjects handles GET /api/v1/semesters/:semester_id/subjects
func (h *SubjectHandler) ListSubjects(c *fiber.Ctx) error {
	semesterID := c.Params("semester_id")

	// Verify semester exists
	var semester model.Semester
	if err := h.db.First(&semester, semesterID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Semester not found")
		}
		return response.InternalServerError(c, "Failed to fetch semester")
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	// Build query
	query := h.db.Model(&model.Subject{}).Where("semester_id = ?", semesterID)

	// Apply search filter
	if search != "" {
		query = query.Where("name ILIKE ? OR code ILIKE ? OR description ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count subjects")
	}

	// Calculate pagination
	offset := (page - 1) * limit
	pagination := response.CalculatePagination(page, limit, total)

	// Get subjects with pagination
	var subjects []model.Subject
	if err := query.Preload("Semester.Course").
		Order("code ASC").
		Limit(limit).
		Offset(offset).
		Find(&subjects).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch subjects")
	}

	return response.Paginated(c, subjects, pagination)
}

// GetSubject handles GET /api/v1/semesters/:semester_id/subjects/:id
func (h *SubjectHandler) GetSubject(c *fiber.Ctx) error {
	semesterID := c.Params("semester_id")
	id := c.Params("id")

	var subject model.Subject
	if err := h.db.Preload("Semester.Course").
		Where("semester_id = ? AND id = ?", semesterID, id).
		First(&subject).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to fetch subject")
	}

	return response.Success(c, subject)
}

// CreateSubject handles POST /api/v1/semesters/:semester_id/subjects
func (h *SubjectHandler) CreateSubject(c *fiber.Ctx) error {
	semesterID := c.Params("semester_id")

	// Parse and validate request
	var req CreateSubjectRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Convert semesterID to uint
	semID, err := strconv.ParseUint(semesterID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid semester ID")
	}

	// Check if subject code already exists in this semester
	var existingSubject model.Subject
	if err := h.db.Where("semester_id = ? AND code = ?", semID, req.Code).
		First(&existingSubject).Error; err == nil {
		return response.BadRequest(c, "Subject with this code already exists in this semester")
	}

	// Create subject with AI integration using SubjectService
	result, err := h.subjectService.CreateSubjectWithAI(c.Context(), services.CreateSubjectRequest{
		SemesterID:  uint(semID),
		Name:        req.Name,
		Code:        req.Code,
		Credits:     req.Credits,
		Description: req.Description,
	})

	if err != nil {
		return response.InternalServerError(c, "Failed to create subject: "+err.Error())
	}

	// Preload relationships for response
	if err := h.db.Preload("Semester.Course").First(result.Subject, result.Subject.ID).Error; err != nil {
		return response.InternalServerError(c, "Failed to load subject details")
	}

	// Add AI integration status to response
	responseData := fiber.Map{
		"subject":                result.Subject,
		"knowledge_base_created": result.KnowledgeBaseCreated,
		"agent_created":          result.AgentCreated,
	}

	return response.Created(c, responseData)
}

// UpdateSubject handles PUT /api/v1/semesters/:semester_id/subjects/:id
func (h *SubjectHandler) UpdateSubject(c *fiber.Ctx) error {
	semesterID := c.Params("semester_id")
	id := c.Params("id")

	// Parse and validate request
	var req UpdateSubjectRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Find subject
	var subject model.Subject
	if err := h.db.Where("semester_id = ? AND id = ?", semesterID, id).
		First(&subject).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to fetch subject")
	}

	// Authorization: Admin or subject owner can update
	// For now, allow any authenticated user (can be restricted later based on requirements)
	_ = user // Placeholder for authorization logic

	// Check if code is being changed and already exists
	if req.Code != "" && req.Code != subject.Code {
		var existingSubject model.Subject
		if err := h.db.Where("semester_id = ? AND code = ? AND id != ?", semesterID, req.Code, id).
			First(&existingSubject).Error; err == nil {
			return response.BadRequest(c, "Subject with this code already exists in this semester")
		}
	}

	// Update fields
	if req.Name != "" {
		subject.Name = req.Name
	}
	if req.Code != "" {
		subject.Code = req.Code
	}
	if req.Credits != nil {
		subject.Credits = *req.Credits
	}
	if req.Description != "" {
		subject.Description = req.Description
	}

	// Save changes
	if err := h.db.Save(&subject).Error; err != nil {
		return response.InternalServerError(c, "Failed to update subject")
	}

	// Preload relationships for response
	if err := h.db.Preload("Semester.Course").First(&subject, subject.ID).Error; err != nil {
		return response.InternalServerError(c, "Failed to load subject details")
	}

	return response.Success(c, subject)
}

// DeleteSubject handles DELETE /api/v1/semesters/:semester_id/subjects/:id
// Cascade deletes all documents before deleting the subject
func (h *SubjectHandler) DeleteSubject(c *fiber.Ctx) error {
	semesterID := c.Params("semester_id")
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Find subject
	var subject model.Subject
	if err := h.db.Where("semester_id = ? AND id = ?", semesterID, id).
		First(&subject).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to fetch subject")
	}

	// Authorization: Admin or subject owner can delete
	// For now, allow any authenticated user (can be restricted later based on requirements)
	_ = user // Placeholder for authorization logic

	// Delete all documents for this subject first (cascade delete)
	if err := h.db.Where("subject_id = ?", id).Delete(&model.Document{}).Error; err != nil {
		return response.InternalServerError(c, "Failed to delete subject documents")
	}

	// Use SubjectService for cleanup (deletes AI resources)
	subjectIDUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	if err := h.subjectService.DeleteSubjectWithCleanup(c.Context(), uint(subjectIDUint)); err != nil {
		return response.InternalServerError(c, "Failed to delete subject: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "Subject and all related data deleted successfully",
	})
}
