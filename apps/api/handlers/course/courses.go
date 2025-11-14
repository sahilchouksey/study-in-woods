package course

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"github.com/sahilchouksey/go-init-setup/utils/validation"
	"gorm.io/gorm"
)

// CourseHandler handles course-related requests
type CourseHandler struct {
	db        *gorm.DB
	validator *validation.Validator
}

// NewCourseHandler creates a new course handler
func NewCourseHandler(db *gorm.DB) *CourseHandler {
	return &CourseHandler{
		db:        db,
		validator: validation.NewValidator(),
	}
}

// CreateCourseRequest represents the request body for creating a course
type CreateCourseRequest struct {
	UniversityID uint   `json:"university_id" validate:"required,min=1"`
	Name         string `json:"name" validate:"required,min=3,max=255"`
	Code         string `json:"code" validate:"required,min=2,max=50"`
	Description  string `json:"description" validate:"omitempty,max=1000"`
	Duration     int    `json:"duration" validate:"required,min=1,max=20"`
}

// UpdateCourseRequest represents the request body for updating a course
type UpdateCourseRequest struct {
	UniversityID *uint  `json:"university_id" validate:"omitempty,min=1"`
	Name         string `json:"name" validate:"omitempty,min=3,max=255"`
	Code         string `json:"code" validate:"omitempty,min=2,max=50"`
	Description  string `json:"description" validate:"omitempty,max=1000"`
	Duration     *int   `json:"duration" validate:"omitempty,min=1,max=20"`
}

// ListCourses handles GET /api/v1/courses
func (h *CourseHandler) ListCourses(c *fiber.Ctx) error {
	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")
	universityID := c.Query("university_id", "")

	// Build query
	query := h.db.Model(&model.Course{})

	// Apply filters
	if search != "" {
		query = query.Where("name ILIKE ? OR code ILIKE ? OR description ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if universityID != "" {
		query = query.Where("university_id = ?", universityID)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count courses")
	}

	// Calculate pagination
	offset := (page - 1) * limit
	pagination := response.CalculatePagination(page, limit, total)

	// Get courses with pagination and preload university
	var courses []model.Course
	if err := query.Preload("University").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&courses).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch courses")
	}

	return response.Paginated(c, courses, pagination)
}

// GetCourse handles GET /api/v1/courses/:id
func (h *CourseHandler) GetCourse(c *fiber.Ctx) error {
	id := c.Params("id")

	var course model.Course
	if err := h.db.Preload("University").
		Preload("Semesters").
		First(&course, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalServerError(c, "Failed to fetch course")
	}

	return response.Success(c, course)
}

// CreateCourse handles POST /api/v1/courses
func (h *CourseHandler) CreateCourse(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse request body
	var req CreateCourseRequest
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
	req.Description = validation.SanitizeString(req.Description)

	// Check if university exists
	var university model.University
	if err := h.db.First(&university, req.UniversityID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "University not found")
		}
		return response.InternalServerError(c, "Failed to verify university")
	}

	// Check if course with same code already exists
	var existingCourse model.Course
	if err := h.db.Where("code = ?", req.Code).First(&existingCourse).Error; err == nil {
		return response.Conflict(c, "Course with this code already exists")
	}

	// Create course
	course := model.Course{
		UniversityID: req.UniversityID,
		Name:         req.Name,
		Code:         req.Code,
		Description:  req.Description,
		Duration:     req.Duration,
	}

	if err := h.db.Create(&course).Error; err != nil {
		return response.InternalServerError(c, "Failed to create course")
	}

	// Preload university for response
	h.db.Preload("University").First(&course, course.ID)

	return response.Created(c, course)
}

// UpdateCourse handles PUT /api/v1/courses/:id
func (h *CourseHandler) UpdateCourse(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse request body
	var req UpdateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate request
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Check if course exists
	var course model.Course
	if err := h.db.First(&course, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalServerError(c, "Failed to fetch course")
	}

	// Update fields if provided
	if req.UniversityID != nil {
		// Check if university exists
		var university model.University
		if err := h.db.First(&university, *req.UniversityID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return response.NotFound(c, "University not found")
			}
			return response.InternalServerError(c, "Failed to verify university")
		}
		course.UniversityID = *req.UniversityID
	}

	if req.Name != "" {
		course.Name = validation.SanitizeString(req.Name)
	}

	if req.Code != "" {
		// Check if code is already used by another course
		var existingCourse model.Course
		if err := h.db.Where("code = ? AND id != ?", req.Code, id).First(&existingCourse).Error; err == nil {
			return response.Conflict(c, "Course with this code already exists")
		}
		course.Code = validation.SanitizeString(req.Code)
	}

	if req.Description != "" {
		course.Description = validation.SanitizeString(req.Description)
	}

	if req.Duration != nil {
		course.Duration = *req.Duration
	}

	// Save changes
	if err := h.db.Save(&course).Error; err != nil {
		return response.InternalServerError(c, "Failed to update course")
	}

	// Preload university for response
	h.db.Preload("University").First(&course, course.ID)

	return response.SuccessWithMessage(c, "Course updated successfully", course)
}

// DeleteCourse handles DELETE /api/v1/courses/:id
func (h *CourseHandler) DeleteCourse(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Check if course exists
	var course model.Course
	if err := h.db.First(&course, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalServerError(c, "Failed to fetch course")
	}

	// Check if course has semesters
	var semesterCount int64
	if err := h.db.Model(&model.Semester{}).Where("course_id = ?", id).Count(&semesterCount).Error; err != nil {
		return response.InternalServerError(c, "Failed to check course dependencies")
	}

	if semesterCount > 0 {
		return response.BadRequest(c, "Cannot delete course with existing semesters")
	}

	// Delete course (soft delete)
	if err := h.db.Delete(&course).Error; err != nil {
		return response.InternalServerError(c, "Failed to delete course")
	}

	return response.SuccessWithMessage(c, "Course deleted successfully", nil)
}
