package admin

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// ListSettings retrieves all app settings
// GET /admin/settings
func ListSettings(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	var settings []model.AppSetting
	if err := db.Find(&settings).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch settings")
	}

	return response.SuccessWithMessage(c, "Settings retrieved successfully", settings)
}

// GetSetting retrieves a specific setting by key
// GET /admin/settings/:key
func GetSetting(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	key := c.Params("key")
	var setting model.AppSetting
	if err := db.Where("key = ?", key).First(&setting).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Setting not found")
		}
		return response.InternalServerError(c, "Failed to fetch setting")
	}

	return response.SuccessWithMessage(c, "Setting retrieved successfully", setting)
}

// UpdateSetting updates an existing setting
// PUT /admin/settings/:key
func UpdateSetting(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	key := c.Params("key")

	var req struct {
		Value       string `json:"value"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var setting model.AppSetting
	if err := db.Where("key = ?", key).First(&setting).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Setting not found")
		}
		return response.InternalServerError(c, "Failed to fetch setting")
	}

	if err := db.Model(&setting).Updates(req).Error; err != nil {
		return response.InternalServerError(c, "Failed to update setting")
	}

	return response.SuccessWithMessage(c, "Setting updated successfully", setting)
}

// DeleteSetting deletes a setting
// DELETE /admin/settings/:key
func DeleteSetting(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	key := c.Params("key")
	result := db.Where("key = ?", key).Delete(&model.AppSetting{})

	if result.Error != nil {
		return response.InternalServerError(c, "Failed to delete setting")
	}
	if result.RowsAffected == 0 {
		return response.NotFound(c, "Setting not found")
	}

	return response.SuccessWithMessage(c, "Setting deleted successfully", fiber.Map{"key": key})
}
