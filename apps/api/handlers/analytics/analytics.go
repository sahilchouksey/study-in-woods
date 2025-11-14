package analytics

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// AnalyticsHandler handles analytics and reporting requests
type AnalyticsHandler struct {
	db               *gorm.DB
	analyticsService *services.AnalyticsService
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(db *gorm.DB, analyticsService *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		db:               db,
		analyticsService: analyticsService,
	}
}

// GetDashboard handles GET /api/v1/admin/dashboard
func (h *AnalyticsHandler) GetDashboard(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Check if user is admin
	if user.Role != "admin" {
		return response.Forbidden(c, "Admin access required")
	}

	// Get dashboard statistics
	stats, err := h.analyticsService.GetDashboardStats(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch dashboard stats: "+err.Error())
	}

	return response.Success(c, stats)
}

// GetUserStats handles GET /api/v1/analytics/users/:id
func (h *AnalyticsHandler) GetUserStats(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse user ID
	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	// Check authorization: users can see their own stats, admins can see all
	if user.Role != "admin" && user.ID != uint(userID) {
		return response.Forbidden(c, "You can only view your own statistics")
	}

	// Get user statistics
	stats, err := h.analyticsService.GetUserStats(c.Context(), uint(userID))
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch user stats: "+err.Error())
	}

	return response.Success(c, stats)
}

// GetMyStats handles GET /api/v1/analytics/me
func (h *AnalyticsHandler) GetMyStats(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Get user statistics
	stats, err := h.analyticsService.GetUserStats(c.Context(), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch your stats: "+err.Error())
	}

	return response.Success(c, stats)
}

// GetSubjectStats handles GET /api/v1/analytics/subjects/:id
func (h *AnalyticsHandler) GetSubjectStats(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context (authentication required)
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse subject ID
	subjectID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Get subject statistics
	stats, err := h.analyticsService.GetSubjectStats(c.Context(), uint(subjectID))
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch subject stats: "+err.Error())
	}

	return response.Success(c, stats)
}

// GetActivityTimeSeries handles GET /api/v1/analytics/activity/timeseries
func (h *AnalyticsHandler) GetActivityTimeSeries(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Check if user is admin
	if user.Role != "admin" {
		return response.Forbidden(c, "Admin access required")
	}

	// Parse query parameters
	days, _ := strconv.Atoi(c.Query("days", "30"))
	if days < 1 || days > 365 {
		days = 30
	}

	activityType := model.ActivityType(c.Query("type", ""))

	// Get time series data
	timeSeries, err := h.analyticsService.GetActivityTimeSeries(c.Context(), days, activityType)
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch activity time series: "+err.Error())
	}

	return response.Success(c, timeSeries)
}

// GetChatUsageTimeSeries handles GET /api/v1/analytics/chat/timeseries
func (h *AnalyticsHandler) GetChatUsageTimeSeries(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Check if user is admin
	if user.Role != "admin" {
		return response.Forbidden(c, "Admin access required")
	}

	// Parse query parameters
	days, _ := strconv.Atoi(c.Query("days", "30"))
	if days < 1 || days > 365 {
		days = 30
	}

	// Get chat usage time series
	timeSeries, err := h.analyticsService.GetChatUsageTimeSeries(c.Context(), days)
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch chat usage: "+err.Error())
	}

	return response.Success(c, timeSeries)
}

// GetTopSubjects handles GET /api/v1/analytics/subjects/top
func (h *AnalyticsHandler) GetTopSubjects(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse query parameters
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	// Get top subjects
	topSubjects, err := h.analyticsService.GetTopSubjects(c.Context(), limit)
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch top subjects: "+err.Error())
	}

	return response.Success(c, topSubjects)
}

// GetAuditLogs handles GET /api/v1/admin/audit-logs
func (h *AnalyticsHandler) GetAuditLogs(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Check if user is admin
	if user.Role != "admin" {
		return response.Forbidden(c, "Admin access required")
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	action := c.Query("action", "")
	resource := c.Query("resource", "")

	// Build query
	query := h.db.Model(&model.AdminAuditLog{})

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count audit logs")
	}

	// Calculate pagination
	offset := (page - 1) * limit
	pagination := response.CalculatePagination(page, limit, total)

	// Get audit logs
	var logs []model.AdminAuditLog
	if err := query.Preload("Admin").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch audit logs")
	}

	return response.Paginated(c, logs, pagination)
}

// GetUserActivities handles GET /api/v1/analytics/activities
func (h *AnalyticsHandler) GetUserActivities(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	activityType := c.Query("type", "")
	userIDParam := c.Query("user_id", "")

	// Build query
	query := h.db.Model(&model.UserActivity{})

	// If not admin, only show own activities
	if user.Role != "admin" {
		query = query.Where("user_id = ?", user.ID)
	} else if userIDParam != "" {
		query = query.Where("user_id = ?", userIDParam)
	}

	if activityType != "" {
		query = query.Where("activity_type = ?", activityType)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count activities")
	}

	// Calculate pagination
	offset := (page - 1) * limit
	pagination := response.CalculatePagination(page, limit, total)

	// Get activities
	var activities []model.UserActivity
	if err := query.Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&activities).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch activities")
	}

	return response.Paginated(c, activities, pagination)
}

// LogActivity handles POST /api/v1/analytics/activity (for manual activity logging)
func (h *AnalyticsHandler) LogActivity(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse request
	var req struct {
		ActivityType string `json:"activity_type" validate:"required"`
		ResourceType string `json:"resource_type"`
		ResourceID   uint   `json:"resource_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Get client info
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	// Log activity
	if err := h.analyticsService.LogActivity(
		c.Context(),
		user.ID,
		model.ActivityType(req.ActivityType),
		req.ResourceType,
		req.ResourceID,
		ipAddress,
		userAgent,
	); err != nil {
		return response.InternalServerError(c, "Failed to log activity: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "Activity logged successfully",
	})
}

// GetSystemHealth handles GET /api/v1/admin/health
func (h *AnalyticsHandler) GetSystemHealth(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Check if user is admin
	if user.Role != "admin" {
		return response.Forbidden(c, "Admin access required")
	}

	// Check database connectivity
	sqlDB, err := h.db.DB()
	if err != nil {
		return response.Success(c, fiber.Map{
			"status":   "unhealthy",
			"database": "error",
			"error":    err.Error(),
		})
	}

	if err := sqlDB.Ping(); err != nil {
		return response.Success(c, fiber.Map{
			"status":   "unhealthy",
			"database": "unreachable",
			"error":    err.Error(),
		})
	}

	// Get database stats
	stats := sqlDB.Stats()

	return response.Success(c, fiber.Map{
		"status":   "healthy",
		"database": "connected",
		"db_stats": fiber.Map{
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
		},
	})
}
