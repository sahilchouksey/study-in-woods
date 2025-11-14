package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// RefreshResponse represents a token refresh response
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.RefreshToken == "" {
		return response.BadRequest(c, "Refresh token is required")
	}

	// Validate refresh token
	claims, err := h.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil {
		return response.Unauthorized(c, "Invalid or expired refresh token")
	}

	// Check if it's a refresh token
	if claims.TokenType != "refresh" {
		return response.Unauthorized(c, "Invalid token type")
	}

	// Check if token is blacklisted
	isRevoked, err := h.blacklistService.IsTokenRevoked(c.Context(), claims.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to check token status")
	}
	if isRevoked {
		return response.Unauthorized(c, "Token has been revoked")
	}

	// Load user to get current token version
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		return response.Unauthorized(c, "User not found")
	}

	// Check token version
	if user.TokenVersion != claims.TokenVersion {
		return response.Unauthorized(c, "Token has been invalidated")
	}

	// Generate new tokens
	newAccessToken, accessJTI, err := h.jwtManager.GenerateAccessToken(user.ID, user.Email, user.Role, user.TokenVersion)
	if err != nil {
		return response.InternalServerError(c, "Failed to generate access token")
	}

	newRefreshToken, refreshJTI, err := h.jwtManager.GenerateRefreshToken(user.ID, user.Email, user.Role, user.TokenVersion)
	if err != nil {
		return response.InternalServerError(c, "Failed to generate refresh token")
	}

	// Store JTI for potential tracking (optional)
	_ = accessJTI
	_ = refreshJTI

	// Blacklist old refresh token
	expiresAt, _ := h.jwtManager.GetTokenExpiry(req.RefreshToken)
	if err := h.blacklistService.RevokeToken(c.Context(), claims.ID, user.ID, expiresAt, "token_refresh"); err != nil {
		// Log error but don't fail the request
		// Old token will expire naturally
	}

	// Prepare response
	res := RefreshResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    24 * 60 * 60, // 24 hours
	}

	return response.Success(c, res)
}

// Logout handles user logout by blacklisting tokens
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// Get user from context (set by auth middleware)
	user, ok := middleware.GetUser(c)
	if !ok {
		return response.Unauthorized(c, "Not authenticated")
	}

	// Get token JTI from context
	jti, ok := middleware.GetTokenJTI(c)
	if !ok {
		return response.BadRequest(c, "No token ID found")
	}

	// Get token expiry
	authHeader := c.Get("Authorization")
	tokenString := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenString = authHeader[7:]
	}

	expiresAt := time.Now().Add(24 * time.Hour) // Default expiry
	if tokenString != "" {
		if exp, err := h.jwtManager.GetTokenExpiry(tokenString); err == nil {
			expiresAt = exp
		}
	}

	// Blacklist the token
	if err := h.blacklistService.RevokeToken(c.Context(), jti, user.ID, expiresAt, "logout"); err != nil {
		return response.InternalServerError(c, "Failed to logout")
	}

	return response.Success(c, fiber.Map{
		"message": "Successfully logged out",
	})
}
