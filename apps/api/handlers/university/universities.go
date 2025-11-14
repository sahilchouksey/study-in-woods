package university

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"github.com/sahilchouksey/go-init-setup/utils/validation"
	"gorm.io/gorm"
)

// UniversityHandler handles university-related requests
type UniversityHandler struct {
	db        *gorm.DB
	validator *validation.Validator
}

// NewUniversityHandler creates a new university handler
func NewUniversityHandler(db *gorm.DB) *UniversityHandler {
	return &UniversityHandler{
		db:        db,
		validator: validation.NewValidator(),
	}
}

// CreateUniversityRequest represents the request body for creating a university
type CreateUniversityRequest struct {
	Name     string `json:"name" validate:"required,min=3,max=255"`
	Code     string `json:"code" validate:"required,min=2,max=50"`
	Location string `json:"location" validate:"required,min=3,max=255"`
	Website  string `json:"website" validate:"omitempty,url,max=255"`
}

// UpdateUniversityRequest represents the request body for updating a university
type UpdateUniversityRequest struct {
	Name     string `json:"name" validate:"omitempty,min=3,max=255"`
	Code     string `json:"code" validate:"omitempty,min=2,max=50"`
	Location string `json:"location" validate:"omitempty,min=3,max=255"`
	Website  string `json:"website" validate:"omitempty,url,max=255"`
	IsActive *bool  `json:"is_active" validate:"omitempty"`
}

// ListUniversities handles GET /api/v1/universities
func (h *UniversityHandler) ListUniversities(c *fiber.Ctx) error {
	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")
	isActive := c.Query("is_active", "")

	// Build query
	query := h.db.Model(&model.University{})

	// Apply filters
	if search != "" {
		query = query.Where("name ILIKE ? OR code ILIKE ? OR location ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if isActive != "" {
		if isActive == "true" {
			query = query.Where("is_active = ?", true)
		} else if isActive == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count universities")
	}

	// Calculate pagination
	offset := (page - 1) * limit
	pagination := response.CalculatePagination(page, limit, total)

	// Get universities with pagination
	var universities []model.University
	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&universities).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch universities")
	}

	return response.Paginated(c, universities, pagination)
}

// GetUniversity handles GET /api/v1/universities/:id
func (h *UniversityHandler) GetUniversity(c *fiber.Ctx) error {
	id := c.Params("id")

	var university model.University
	if err := h.db.Preload("Courses").First(&university, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "University not found")
		}
		return response.InternalServerError(c, "Failed to fetch university")
	}

	return response.Success(c, university)
}

// CreateUniversity handles POST /api/v1/universities
func (h *UniversityHandler) CreateUniversity(c *fiber.Ctx) error {
	// Parse request body
	var req CreateUniversityRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate request
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Sanitize inputs
	req.Name = validation.SanitizeString(req.Name)
	req.Code = validation.SanitizeString(req.Code)
	req.Location = validation.SanitizeString(req.Location)
	req.Website = validation.SanitizeString(req.Website)

	// Check if university with same code already exists
	var existingUniversity model.University
	if err := h.db.Where("code = ?", req.Code).First(&existingUniversity).Error; err == nil {
		return response.Conflict(c, "University with this code already exists")
	}

	// Create university
	university := model.University{
		Name:     req.Name,
		Code:     req.Code,
		Location: req.Location,
		Website:  req.Website,
		IsActive: true,
	}

	if err := h.db.Create(&university).Error; err != nil {
		return response.InternalServerError(c, "Failed to create university")
	}

	return response.Created(c, university)
}

// UpdateUniversity handles PUT /api/v1/universities/:id
func (h *UniversityHandler) UpdateUniversity(c *fiber.Ctx) error {
	id := c.Params("id")

	// Parse request body
	var req UpdateUniversityRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate request
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Check if university exists
	var university model.University
	if err := h.db.First(&university, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "University not found")
		}
		return response.InternalServerError(c, "Failed to fetch university")
	}

	// Update fields if provided
	if req.Name != "" {
		university.Name = validation.SanitizeString(req.Name)
	}
	if req.Code != "" {
		// Check if code is already used by another university
		var existingUniversity model.University
		if err := h.db.Where("code = ? AND id != ?", req.Code, id).First(&existingUniversity).Error; err == nil {
			return response.Conflict(c, "University with this code already exists")
		}
		university.Code = validation.SanitizeString(req.Code)
	}
	if req.Location != "" {
		university.Location = validation.SanitizeString(req.Location)
	}
	if req.Website != "" {
		university.Website = validation.SanitizeString(req.Website)
	}
	if req.IsActive != nil {
		university.IsActive = *req.IsActive
	}

	// Save changes
	if err := h.db.Save(&university).Error; err != nil {
		return response.InternalServerError(c, "Failed to update university")
	}

	return response.SuccessWithMessage(c, "University updated successfully", university)
}

// DeleteUniversity handles DELETE /api/v1/universities/:id
func (h *UniversityHandler) DeleteUniversity(c *fiber.Ctx) error {
	id := c.Params("id")

	// Check if university exists
	var university model.University
	if err := h.db.First(&university, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "University not found")
		}
		return response.InternalServerError(c, "Failed to fetch university")
	}

	// Check if university has courses
	var courseCount int64
	if err := h.db.Model(&model.Course{}).Where("university_id = ?", id).Count(&courseCount).Error; err != nil {
		return response.InternalServerError(c, "Failed to check university dependencies")
	}

	if courseCount > 0 {
		return response.BadRequest(c, "Cannot delete university with existing courses")
	}

	// Delete university (soft delete)
	if err := h.db.Delete(&university).Error; err != nil {
		return response.InternalServerError(c, "Failed to delete university")
	}

	return response.SuccessWithMessage(c, "University deleted successfully", nil)
}
