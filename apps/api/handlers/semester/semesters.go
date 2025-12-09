package semester

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"github.com/sahilchouksey/go-init-setup/utils/validation"
	"gorm.io/gorm"
)

// SemesterHandler handles semester-related requests
type SemesterHandler struct {
	db        *gorm.DB
	validator *validation.Validator
}

// NewSemesterHandler creates a new semester handler
func NewSemesterHandler(db *gorm.DB) *SemesterHandler {
	return &SemesterHandler{
		db:        db,
		validator: validation.NewValidator(),
	}
}

// CreateSemesterRequest represents the request body for creating a semester
type CreateSemesterRequest struct {
	CourseID uint   `json:"course_id" validate:"required,min=1"`
	Number   int    `json:"number" validate:"required,min=1,max=20"`
	Name     string `json:"name" validate:"required,min=3,max=50"`
}

// UpdateSemesterRequest represents the request body for updating a semester
type UpdateSemesterRequest struct {
	Number *int   `json:"number" validate:"omitempty,min=1,max=20"`
	Name   string `json:"name" validate:"omitempty,min=3,max=50"`
}

// ListSemesters handles GET /api/v1/courses/:course_id/semesters
func (h *SemesterHandler) ListSemesters(c *fiber.Ctx) error {
	courseID := c.Params("course_id")

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	// Check if course exists
	var course model.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalServerError(c, "Failed to fetch course")
	}

	// Build query
	query := h.db.Model(&model.Semester{}).Where("course_id = ?", courseID)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count semesters")
	}

	// Calculate pagination
	offset := (page - 1) * limit
	pagination := response.CalculatePagination(page, limit, total)

	// Get semesters with pagination
	var semesters []model.Semester
	if err := query.Order("number ASC").
		Limit(limit).
		Offset(offset).
		Find(&semesters).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch semesters")
	}

	return response.Paginated(c, semesters, pagination)
}

// GetSemester handles GET /api/v1/courses/:course_id/semesters/:number
func (h *SemesterHandler) GetSemester(c *fiber.Ctx) error {
	courseID := c.Params("course_id")
	number := c.Params("number")

	// Check if course exists
	var course model.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalServerError(c, "Failed to fetch course")
	}

	var semester model.Semester
	if err := h.db.Preload("Course").
		Preload("Subjects").
		Where("course_id = ? AND number = ?", courseID, number).
		First(&semester).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Semester not found")
		}
		return response.InternalServerError(c, "Failed to fetch semester")
	}

	return response.Success(c, semester)
}

// CreateSemester handles POST /api/v1/courses/:course_id/semesters
func (h *SemesterHandler) CreateSemester(c *fiber.Ctx) error {
	courseID := c.Params("course_id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse request body
	var req CreateSemesterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate request
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Sanitize inputs
	req.Name = validation.SanitizeString(req.Name)

	// Verify course ID from URL matches request body
	courseIDFromURL, _ := strconv.ParseUint(courseID, 10, 64)
	if uint(courseIDFromURL) != req.CourseID {
		return response.BadRequest(c, "Course ID in URL does not match request body")
	}

	// Check if course exists
	var course model.Course
	if err := h.db.First(&course, req.CourseID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalServerError(c, "Failed to verify course")
	}

	// Check if semester number already exists for this course
	var existingSemester model.Semester
	if err := h.db.Where("course_id = ? AND number = ?", req.CourseID, req.Number).
		First(&existingSemester).Error; err == nil {
		return response.Conflict(c, "Semester with this number already exists for this course")
	}

	// Create semester
	semester := model.Semester{
		CourseID: req.CourseID,
		Number:   req.Number,
		Name:     req.Name,
	}

	if err := h.db.Create(&semester).Error; err != nil {
		return response.InternalServerError(c, "Failed to create semester")
	}

	// Preload course for response
	h.db.Preload("Course").First(&semester, semester.ID)

	return response.Created(c, semester)
}

// UpdateSemester handles PUT /api/v1/courses/:course_id/semesters/:number
func (h *SemesterHandler) UpdateSemester(c *fiber.Ctx) error {
	courseID := c.Params("course_id")
	number := c.Params("number")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse request body
	var req UpdateSemesterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate request
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Check if course exists
	var course model.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalServerError(c, "Failed to fetch course")
	}

	// Check if semester exists
	var semester model.Semester
	if err := h.db.Where("course_id = ? AND number = ?", courseID, number).
		First(&semester).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Semester not found")
		}
		return response.InternalServerError(c, "Failed to fetch semester")
	}

	// Update fields if provided
	if req.Number != nil {
		// Check if new number is already used by another semester in this course
		var existingSemester model.Semester
		if err := h.db.Where("course_id = ? AND number = ? AND id != ?",
			courseID, *req.Number, semester.ID).
			First(&existingSemester).Error; err == nil {
			return response.Conflict(c, "Semester with this number already exists for this course")
		}
		semester.Number = *req.Number
	}

	if req.Name != "" {
		semester.Name = validation.SanitizeString(req.Name)
	}

	// Save changes
	if err := h.db.Save(&semester).Error; err != nil {
		return response.InternalServerError(c, "Failed to update semester")
	}

	// Preload course for response
	h.db.Preload("Course").First(&semester, semester.ID)

	return response.SuccessWithMessage(c, "Semester updated successfully", semester)
}

// DeleteSemester handles DELETE /api/v1/courses/:course_id/semesters/:number
// Cascade deletes all subjects and documents
func (h *SemesterHandler) DeleteSemester(c *fiber.Ctx) error {
	courseID := c.Params("course_id")
	number := c.Params("number")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Check if course exists
	var course model.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalServerError(c, "Failed to fetch course")
	}

	// Check if semester exists
	var semester model.Semester
	if err := h.db.Where("course_id = ? AND number = ?", courseID, number).
		First(&semester).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Semester not found")
		}
		return response.InternalServerError(c, "Failed to fetch semester")
	}

	// Use a transaction for cascade delete
	err := h.db.Transaction(func(tx *gorm.DB) error {
		// Delete all documents for subjects in this semester
		if err := tx.Where("subject_id IN (SELECT id FROM subjects WHERE semester_id = ?)", semester.ID).
			Delete(&model.Document{}).Error; err != nil {
			return err
		}

		// Delete all subjects in this semester
		if err := tx.Where("semester_id = ?", semester.ID).Delete(&model.Subject{}).Error; err != nil {
			return err
		}

		// Delete semester (soft delete)
		if err := tx.Delete(&semester).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return response.InternalServerError(c, "Failed to delete semester: "+err.Error())
	}

	return response.SuccessWithMessage(c, "Semester and all related data deleted successfully", nil)
}
