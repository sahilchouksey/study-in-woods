package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/auth"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"` // in seconds
}

// Login handles user login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate request
	if req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "Email and password are required")
	}

	ip := c.IP()

	// Find user by email
	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Record failed attempt even if user not found
		if h.bruteForceProtection != nil {
			h.bruteForceProtection.RecordFailedAttempt(c, ip, req.Email)
		}
		return response.Unauthorized(c, "Invalid email or password")
	}

	// Verify password
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		// Record failed attempt
		if h.bruteForceProtection != nil {
			h.bruteForceProtection.RecordFailedAttempt(c, ip, req.Email)
		}
		return response.Unauthorized(c, "Invalid email or password")
	}

	// Clear failed attempts on successful login
	if h.bruteForceProtection != nil {
		h.bruteForceProtection.RecordSuccessfulAttempt(c, ip)
	}

	// Generate tokens with token version
	accessToken, accessJTI, err := h.jwtManager.GenerateAccessToken(user.ID, user.Email, user.Role, user.TokenVersion)
	if err != nil {
		return response.InternalServerError(c, "Failed to generate access token")
	}

	refreshToken, refreshJTI, err := h.jwtManager.GenerateRefreshToken(user.ID, user.Email, user.Role, user.TokenVersion)
	if err != nil {
		return response.InternalServerError(c, "Failed to generate refresh token")
	}

	// Store JTI for potential tracking (optional)
	_ = accessJTI
	_ = refreshJTI

	// Prepare response
	res := LoginResponse{
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Role:      user.Role,
			Semester:  user.Semester,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    24 * 60 * 60, // 24 hours in seconds
	}

	return response.Success(c, res)
}
