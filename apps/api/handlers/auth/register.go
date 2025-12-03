package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	authutil "github.com/sahilchouksey/go-init-setup/utils/auth"
	"github.com/sahilchouksey/go-init-setup/utils/crypto"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	db                   *gorm.DB
	jwtManager           *authutil.JWTManager
	blacklistService     *authutil.BlacklistService
	bruteForceProtection *middleware.BruteForceProtection
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *gorm.DB, jwtManager *authutil.JWTManager, bruteForceProtection *middleware.BruteForceProtection) *AuthHandler {
	return &AuthHandler{
		db:                   db,
		jwtManager:           jwtManager,
		blacklistService:     authutil.NewBlacklistService(db),
		bruteForceProtection: bruteForceProtection,
	}
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=2"`
	Role     string `json:"role,omitempty"` // Optional, defaults to "student"
	Semester int    `json:"semester,omitempty"`
}

// RegisterResponse represents a successful registration response
type RegisterResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"` // in seconds
}

// UserResponse represents user data in responses
type UserResponse struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Semester  int       `json:"semester"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Register handles user registration
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate request
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return response.BadRequest(c, "Email, password, and name are required")
	}

	// Validate password strength
	if !authutil.IsPasswordValid(req.Password) {
		return response.BadRequest(c, "Password must be at least 8 characters long")
	}

	// Set default role if not provided
	if req.Role == "" {
		req.Role = "student"
	}

	// Validate role
	if req.Role != "student" && req.Role != "admin" {
		return response.BadRequest(c, "Invalid role. Must be 'student' or 'admin'")
	}

	// Check if user already exists
	var existingUser model.User
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return response.Conflict(c, "User with this email already exists")
	}

	// Hash password
	hashedPassword, err := authutil.HashPassword(req.Password)
	if err != nil {
		return response.InternalServerError(c, "Failed to process password")
	}

	// Generate password salt
	passwordSalt, err := crypto.GenerateSalt()
	if err != nil {
		return response.InternalServerError(c, "Failed to generate password salt")
	}

	// Create user
	user := model.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		PasswordSalt: passwordSalt,
		Name:         req.Name,
		Role:         req.Role,
		Semester:     req.Semester,
		TokenVersion: 0, // Initialize token version
	}

	if err := h.db.Create(&user).Error; err != nil {
		return response.InternalServerError(c, "Failed to create user")
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
	res := RegisterResponse{
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

	return response.Created(c, res)
}
