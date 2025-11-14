package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sahilchouksey/go-init-setup/model"
	authutil "github.com/sahilchouksey/go-init-setup/utils/auth"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// ForgotPasswordRequest represents a password reset request
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents a password reset with token
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ForgotPassword handles password reset request
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var req ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Email == "" {
		return response.BadRequest(c, "Email is required")
	}

	// Find user by email
	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal if email exists for security
		return response.Success(c, fiber.Map{
			"message": "If the email exists, a password reset link will be sent",
		})
	}

	// Generate reset token
	resetToken := uuid.New().String()
	expiresAt := time.Now().Add(1 * time.Hour) // 1 hour expiry

	// Create reset token record
	passwordReset := model.PasswordResetToken{
		UserID:    user.ID,
		Token:     resetToken,
		ExpiresAt: expiresAt,
	}

	if err := h.db.Create(&passwordReset).Error; err != nil {
		return response.InternalServerError(c, "Failed to create reset token")
	}

	// TODO: Send email with reset link
	// For now, just log it or return in development
	// resetLink := fmt.Sprintf("http://yourapp.com/reset-password?token=%s", resetToken)
	// emailService.SendPasswordResetEmail(user.Email, resetLink)

	return response.Success(c, fiber.Map{
		"message": "If the email exists, a password reset link will be sent",
		// In development, you might want to include the token
		// "token": resetToken, // Remove in production
	})
}

// ResetPassword handles password reset with token
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Token == "" || req.NewPassword == "" {
		return response.BadRequest(c, "Token and new password are required")
	}

	// Validate password strength
	if !authutil.IsPasswordValid(req.NewPassword) {
		return response.BadRequest(c, "Password must be at least 8 characters long")
	}

	// Find reset token
	var resetToken model.PasswordResetToken
	if err := h.db.Where("token = ?", req.Token).First(&resetToken).Error; err != nil {
		return response.BadRequest(c, "Invalid or expired reset token")
	}

	// Check if token is expired
	if resetToken.IsExpired() {
		return response.BadRequest(c, "Reset token has expired")
	}

	// Check if token is already used
	if resetToken.IsUsed() {
		return response.BadRequest(c, "Reset token has already been used")
	}

	// Find user
	var user model.User
	if err := h.db.First(&user, resetToken.UserID).Error; err != nil {
		return response.BadRequest(c, "User not found")
	}

	// Hash new password
	hashedPassword, err := authutil.HashPassword(req.NewPassword)
	if err != nil {
		return response.InternalServerError(c, "Failed to process password")
	}

	// Update user password and increment token version
	if err := h.db.Model(&user).Updates(map[string]interface{}{
		"password_hash": hashedPassword,
		"token_version": user.TokenVersion + 1, // Invalidate all existing tokens
	}).Error; err != nil {
		return response.InternalServerError(c, "Failed to update password")
	}

	// Mark token as used
	resetToken.MarkAsUsed()
	h.db.Save(&resetToken)

	return response.Success(c, fiber.Map{
		"message": "Password reset successfully",
	})
}

// ChangePassword handles password change for authenticated users
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return response.BadRequest(c, "Old password and new password are required")
	}

	// Validate new password strength
	if !authutil.IsPasswordValid(req.NewPassword) {
		return response.BadRequest(c, "Password must be at least 8 characters long")
	}

	// Get user from context
	userID := c.Locals("user_id")
	if userID == nil {
		return response.Unauthorized(c, "Not authenticated")
	}

	// Find user
	var user model.User
	if err := h.db.First(&user, userID.(uint)).Error; err != nil {
		return response.Unauthorized(c, "User not found")
	}

	// Verify old password
	if err := authutil.VerifyPassword(user.PasswordHash, req.OldPassword); err != nil {
		return response.BadRequest(c, "Current password is incorrect")
	}

	// Hash new password
	hashedPassword, err := authutil.HashPassword(req.NewPassword)
	if err != nil {
		return response.InternalServerError(c, "Failed to process password")
	}

	// Update password and increment token version
	if err := h.db.Model(&user).Updates(map[string]interface{}{
		"password_hash": hashedPassword,
		"token_version": user.TokenVersion + 1, // Invalidate all existing tokens
	}).Error; err != nil {
		return response.InternalServerError(c, "Failed to update password")
	}

	return response.Success(c, fiber.Map{
		"message": "Password changed successfully. Please login again with your new password",
	})
}
