package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// UpdateProfileRequest represents a profile update request
type UpdateProfileRequest struct {
	Name     string `json:"name,omitempty"`
	Semester int    `json:"semester,omitempty"`
}

// GetProfile retrieves the current user's profile
func (h *AuthHandler) GetProfile(c *fiber.Ctx) error {
	// Get user ID from context
	userID := c.Locals("user_id")
	if userID == nil {
		return response.Unauthorized(c, "Not authenticated")
	}

	// Get user from database
	var user model.User
	if err := h.db.First(&user, userID.(uint)).Error; err != nil {
		return response.NotFound(c, "User not found")
	}

	// Prepare response
	res := UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		Semester:  user.Semester,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return response.Success(c, res)
}

// UpdateProfile updates the current user's profile
func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	// Get user ID from context
	userID := c.Locals("user_id")
	if userID == nil {
		return response.Unauthorized(c, "Not authenticated")
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Get user from database
	var user model.User
	if err := h.db.First(&user, userID.(uint)).Error; err != nil {
		return response.NotFound(c, "User not found")
	}

	// Update fields if provided
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Semester > 0 {
		user.Semester = req.Semester
	}

	// Save updates
	if err := h.db.Save(&user).Error; err != nil {
		return response.InternalServerError(c, "Failed to update profile")
	}

	// Prepare response
	res := UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		Semester:  user.Semester,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return response.Success(c, res)
}
