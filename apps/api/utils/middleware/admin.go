package middleware

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// RequireAdmin middleware ensures the user has admin role
func RequireAdmin(store database.Storage) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user from context (set by RequireAuth middleware)
		userID, ok := c.Locals("userID").(uint)
		if !ok || userID == 0 {
			return response.Unauthorized(c, "Authentication required")
		}

		// Get database connection
		db, ok := store.GetDB().(*gorm.DB)
		if !ok {
			return response.InternalServerError(c, "Database connection error")
		}

		// Get user from database
		var user model.User
		if err := db.First(&user, userID).Error; err != nil {
			return response.Unauthorized(c, "User not found")
		}

		// Check if user has admin role
		if user.Role != "admin" {
			return response.Forbidden(c, "Admin access required")
		}

		// Store admin user in context for audit logging
		c.Locals("adminUser", user)

		return c.Next()
	}
}

// AdminAuditLog creates an audit log entry for admin actions
func AdminAuditLog(store database.Storage, action, resource string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get admin user from context
		adminUser, ok := c.Locals("adminUser").(model.User)
		if !ok {
			return c.Next() // Continue without logging if user not found
		}

		// Get database connection
		db, ok := store.GetDB().(*gorm.DB)
		if !ok {
			return c.Next() // Continue without logging if db error
		}

		// Parse resource ID from params if available
		var resourceID uint
		if id := c.Params("id"); id != "" {
			if parsedID, err := strconv.ParseUint(id, 10, 32); err == nil {
				resourceID = uint(parsedID)
			}
		}

		// Capture request body for "old value" tracking
		var oldValue interface{}
		var newValue interface{}

		// Get body for POST/PUT requests
		if c.Method() == "POST" || c.Method() == "PUT" {
			body := c.Body()
			if len(body) > 0 {
				json.Unmarshal(body, &newValue)
			}
		}

		// For DELETE or GET, we might want to capture the existing state
		if resourceID > 0 && (c.Method() == "DELETE" || c.Method() == "PUT") {
			// Attempt to get existing record (generic approach)
			switch resource {
			case "users":
				var user model.User
				if err := db.First(&user, resourceID).Error; err == nil {
					oldValue = user
				}
			case "settings":
				var setting model.AppSetting
				if err := db.Where("id = ?", resourceID).First(&setting).Error; err == nil {
					oldValue = setting
				}
			}
		}

		// Execute the actual handler
		err := c.Next()

		// Log the action after completion
		go func() {
			oldValueJSON, _ := json.Marshal(oldValue)
			newValueJSON, _ := json.Marshal(newValue)

			auditLog := model.AdminAuditLog{
				AdminID:     adminUser.ID,
				Action:      action,
				Resource:    resource,
				ResourceID:  resourceID,
				OldValue:    string(oldValueJSON),
				NewValue:    string(newValueJSON),
				IPAddress:   c.IP(),
				UserAgent:   c.Get("User-Agent"),
				Description: c.Method() + " " + c.Path(),
			}

			db.Create(&auditLog)
		}()

		return err
	}
}
