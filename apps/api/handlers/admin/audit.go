package admin

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// ListAuditLogs retrieves admin audit logs with pagination
// GET /admin/audit-logs
func ListAuditLogs(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	// Pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Filters
	action := c.Query("action")
	resource := c.Query("resource")
	adminIDStr := c.Query("admin_id")

	// Build query
	query := db.Model(&model.AdminAuditLog{}).Preload("Admin")

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}
	if adminIDStr != "" {
		if adminID, err := strconv.ParseUint(adminIDStr, 10, 32); err == nil {
			query = query.Where("admin_id = ?", adminID)
		}
	}

	// Count
	var total int64
	query.Count(&total)

	// Fetch logs
	var logs []model.AdminAuditLog
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&logs).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch audit logs")
	}

	return response.SuccessWithMessage(c, "Audit logs retrieved successfully", fiber.Map{
		"logs": logs,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetAuditLog retrieves a specific audit log entry
// GET /admin/audit-logs/:id
func GetAuditLog(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	logID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid log ID")
	}

	var log model.AdminAuditLog
	if err := db.Preload("Admin").First(&log, logID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Audit log not found")
		}
		return response.InternalServerError(c, "Failed to fetch audit log")
	}

	return response.SuccessWithMessage(c, "Audit log retrieved successfully", log)
}
