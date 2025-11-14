package admin

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/auth"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// ListUsersRequest represents the query parameters for listing users
type ListUsersRequest struct {
	Page    int    `query:"page"`
	Limit   int    `query:"limit"`
	Role    string `query:"role"`
	Search  string `query:"search"`
	Sort    string `query:"sort"`
	SortDir string `query:"sort_dir"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Semester int    `json:"semester"`
}

// ResetPasswordRequest represents the request for admin password reset
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ListUsers retrieves all users with pagination and filters
// GET /admin/users
func ListUsers(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	// Parse query parameters
	var req ListUsersRequest
	if err := c.QueryParser(&req); err != nil {
		return response.BadRequest(c, "Invalid query parameters")
	}

	// Default pagination
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Sort == "" {
		req.Sort = "created_at"
	}
	if req.SortDir != "asc" && req.SortDir != "desc" {
		req.SortDir = "desc"
	}

	// Build query
	query := db.Model(&model.User{})

	// Filter by role
	if req.Role != "" {
		query = query.Where("role = ?", req.Role)
	}

	// Search by name or email
	if req.Search != "" {
		searchTerm := "%" + strings.ToLower(req.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ?", searchTerm, searchTerm)
	}

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count users")
	}

	// Get paginated users
	var users []model.User
	offset := (req.Page - 1) * req.Limit
	orderBy := req.Sort + " " + req.SortDir

	if err := query.Offset(offset).Limit(req.Limit).Order(orderBy).Find(&users).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch users")
	}

	// Remove sensitive data
	for i := range users {
		users[i].PasswordHash = ""
		users[i].PasswordSalt = nil
	}

	return response.SuccessWithMessage(c, "Users retrieved successfully", fiber.Map{
		"users": users,
		"pagination": fiber.Map{
			"page":        req.Page,
			"limit":       req.Limit,
			"total":       total,
			"total_pages": (total + int64(req.Limit) - 1) / int64(req.Limit),
		},
	})
}

// GetUser retrieves a specific user by ID
// GET /admin/users/:id
func GetUser(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	// Parse user ID
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	// Get user with relationships
	var user model.User
	if err := db.Preload("Courses.Course.University").
		Preload("ChatSessions").
		First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "User not found")
		}
		return response.InternalServerError(c, "Failed to fetch user")
	}

	// Get user statistics
	var stats struct {
		TotalChatSessions int64
		TotalChatMessages int64
		TotalDocuments    int64
		TotalAPIKeys      int64
	}

	db.Model(&model.ChatSession{}).Where("user_id = ?", userID).Count(&stats.TotalChatSessions)
	db.Model(&model.ChatMessage{}).Where("user_id = ?", userID).Count(&stats.TotalChatMessages)
	db.Model(&model.Document{}).Where("uploaded_by_user_id = ?", userID).Count(&stats.TotalDocuments)
	db.Model(&model.ExternalAPIKey{}).Where("user_id = ?", userID).Count(&stats.TotalAPIKeys)

	// Remove sensitive data
	user.PasswordHash = ""
	user.PasswordSalt = nil

	return response.SuccessWithMessage(c, "User retrieved successfully", fiber.Map{
		"user":  user,
		"stats": stats,
	})
}

// UpdateUser updates a user's information
// PUT /admin/users/:id
func UpdateUser(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	// Parse user ID
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	// Parse request body
	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Get existing user
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "User not found")
		}
		return response.InternalServerError(c, "Failed to fetch user")
	}

	// Update fields
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		// Check if email is already taken by another user
		var existingUser model.User
		if err := db.Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error; err == nil {
			return response.BadRequest(c, "Email already in use")
		}
		updates["email"] = req.Email
	}
	if req.Role != "" && (req.Role == "student" || req.Role == "admin") {
		updates["role"] = req.Role
	}
	if req.Semester > 0 {
		updates["semester"] = req.Semester
	}

	// Update user
	if len(updates) > 0 {
		if err := db.Model(&user).Updates(updates).Error; err != nil {
			return response.InternalServerError(c, "Failed to update user")
		}
	}

	// Refresh user data
	db.First(&user, userID)
	user.PasswordHash = ""
	user.PasswordSalt = nil

	return response.SuccessWithMessage(c, "User updated successfully", fiber.Map{
		"user": user,
	})
}

// DeleteUser soft deletes a user
// DELETE /admin/users/:id
func DeleteUser(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	// Parse user ID
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	// Get admin user ID (prevent self-deletion)
	adminUser, ok := c.Locals("adminUser").(model.User)
	if ok && adminUser.ID == uint(userID) {
		return response.BadRequest(c, "Cannot delete your own account")
	}

	// Get user
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "User not found")
		}
		return response.InternalServerError(c, "Failed to fetch user")
	}

	// Soft delete user (GORM will handle cascade via foreign keys)
	if err := db.Delete(&user).Error; err != nil {
		return response.InternalServerError(c, "Failed to delete user")
	}

	return response.SuccessWithMessage(c, "User deleted successfully", fiber.Map{
		"user_id": userID,
	})
}

// ResetUserPassword allows admin to reset a user's password
// POST /admin/users/:id/reset-password
func ResetUserPassword(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	// Parse user ID
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	// Parse request body
	var req ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate password length
	if len(req.NewPassword) < 8 {
		return response.BadRequest(c, "Password must be at least 8 characters")
	}

	// Get user
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "User not found")
		}
		return response.InternalServerError(c, "Failed to fetch user")
	}

	// Hash new password
	hashedPassword, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return response.InternalServerError(c, "Failed to hash password")
	}

	// Update user password and increment token version (invalidate all tokens)
	if err := db.Model(&user).Updates(map[string]interface{}{
		"password_hash": hashedPassword,
		"token_version": user.TokenVersion + 1,
	}).Error; err != nil {
		return response.InternalServerError(c, "Failed to update password")
	}

	return response.SuccessWithMessage(c, "Password reset successfully", fiber.Map{
		"user_id": userID,
		"message": "All user sessions have been invalidated",
	})
}

// GetUserStats retrieves overall user statistics
// GET /admin/users/stats
func GetUserStats(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	var stats struct {
		TotalUsers     int64
		AdminUsers     int64
		StudentUsers   int64
		ActiveToday    int64
		ActiveThisWeek int64
	}

	// Total users
	db.Model(&model.User{}).Count(&stats.TotalUsers)

	// Users by role
	db.Model(&model.User{}).Where("role = ?", "admin").Count(&stats.AdminUsers)
	db.Model(&model.User{}).Where("role = ?", "student").Count(&stats.StudentUsers)

	// Active users (based on user_activity)
	db.Model(&model.UserActivity{}).
		Where("created_at >= NOW() - INTERVAL '1 day'").
		Distinct("user_id").
		Count(&stats.ActiveToday)

	db.Model(&model.UserActivity{}).
		Where("created_at >= NOW() - INTERVAL '7 days'").
		Distinct("user_id").
		Count(&stats.ActiveThisWeek)

	return response.SuccessWithMessage(c, "User statistics retrieved successfully", stats)
}
