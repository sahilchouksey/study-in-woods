package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/auth"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// AuthMiddleware handles JWT authentication
type AuthMiddleware struct {
	jwtManager       *auth.JWTManager
	blacklistService *auth.BlacklistService
	db               *gorm.DB
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtManager *auth.JWTManager, db *gorm.DB) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager:       jwtManager,
		blacklistService: auth.NewBlacklistService(db),
		db:               db,
	}
}

// Required is middleware that requires a valid JWT token
func (m *AuthMiddleware) Required() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Missing authorization token")
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "Invalid authorization format")
		}

		tokenString := parts[1]

		// Validate token
		claims, err := m.jwtManager.ValidateToken(tokenString)
		if err != nil {
			if err == auth.ErrExpiredToken {
				return response.Unauthorized(c, "Token has expired")
			}
			return response.Unauthorized(c, "Invalid token")
		}

		// Check if it's an access token
		if claims.TokenType != "access" {
			return response.Unauthorized(c, "Invalid token type")
		}

		// Check if token is revoked (blacklisted)
		isRevoked, err := m.blacklistService.IsTokenRevoked(c.Context(), claims.ID)
		if err != nil {
			return response.InternalServerError(c, "Failed to check token status")
		}
		if isRevoked {
			return response.Unauthorized(c, "Token has been revoked")
		}

		// Load user from database and verify token version
		var user model.User
		if err := m.db.First(&user, claims.UserID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return response.Unauthorized(c, "User not found")
			}
			return response.InternalServerError(c, "Failed to load user")
		}

		// Check if token version matches
		if user.TokenVersion != claims.TokenVersion {
			return response.Unauthorized(c, "Token has been invalidated")
		}

		// Store user info and full user object in context
		c.Locals("user_id", claims.UserID)
		c.Locals("user_email", claims.Email)
		c.Locals("user_role", claims.Role)
		c.Locals("claims", claims)
		c.Locals("user", &user)
		c.Locals("token_jti", claims.ID)

		return c.Next()
	}
}

// Optional is middleware that allows requests with or without a token
func (m *AuthMiddleware) Optional() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Next()
		}

		tokenString := parts[1]
		claims, err := m.jwtManager.ValidateToken(tokenString)
		if err != nil {
			return c.Next()
		}

		if claims.TokenType != "access" {
			return c.Next()
		}

		// Check if token is revoked
		isRevoked, err := m.blacklistService.IsTokenRevoked(c.Context(), claims.ID)
		if err != nil || isRevoked {
			return c.Next()
		}

		// Load user
		var user model.User
		if err := m.db.First(&user, claims.UserID).Error; err != nil {
			return c.Next()
		}

		// Check token version
		if user.TokenVersion != claims.TokenVersion {
			return c.Next()
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("user_email", claims.Email)
		c.Locals("user_role", claims.Role)
		c.Locals("claims", claims)
		c.Locals("user", &user)
		c.Locals("token_jti", claims.ID)

		return c.Next()
	}
}

// RequireRole is middleware that requires specific user role
func (m *AuthMiddleware) RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("user_role")
		if userRole == nil {
			return response.Forbidden(c, "Access denied")
		}

		role := userRole.(string)
		for _, r := range roles {
			if role == r {
				return c.Next()
			}
		}

		return response.Forbidden(c, "Insufficient permissions")
	}
}

// RequireAdmin is middleware that requires admin role
// It validates the JWT token inline and checks for admin role
func (m *AuthMiddleware) RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Missing authorization token")
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "Invalid authorization format")
		}

		tokenString := parts[1]

		// Validate token
		claims, err := m.jwtManager.ValidateToken(tokenString)
		if err != nil {
			if err == auth.ErrExpiredToken {
				return response.Unauthorized(c, "Token has expired")
			}
			return response.Unauthorized(c, "Invalid token")
		}

		// Check if it's an access token
		if claims.TokenType != "access" {
			return response.Unauthorized(c, "Invalid token type")
		}

		// Check if token is revoked (blacklisted)
		isRevoked, err := m.blacklistService.IsTokenRevoked(c.Context(), claims.ID)
		if err != nil {
			return response.InternalServerError(c, "Failed to check token status")
		}
		if isRevoked {
			return response.Unauthorized(c, "Token has been revoked")
		}

		// Load user from database and verify token version
		var user model.User
		if err := m.db.First(&user, claims.UserID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return response.Unauthorized(c, "User not found")
			}
			return response.InternalServerError(c, "Failed to load user")
		}

		// Check if token version matches
		if user.TokenVersion != claims.TokenVersion {
			return response.Unauthorized(c, "Token has been invalidated")
		}

		// Check for admin role
		if claims.Role != "admin" && claims.Role != "super_admin" {
			return response.Forbidden(c, "Admin access required")
		}

		// Store user info in context
		c.Locals("user_id", claims.UserID)
		c.Locals("user_email", claims.Email)
		c.Locals("user_role", claims.Role)
		c.Locals("claims", claims)
		c.Locals("user", &user)
		c.Locals("token_jti", claims.ID)

		return c.Next()
	}
}

// GetUserID extracts user ID from context
func GetUserID(c *fiber.Ctx) (uint, bool) {
	userID := c.Locals("user_id")
	if userID == nil {
		return 0, false
	}
	id, ok := userID.(uint)
	return id, ok
}

// GetUserEmail extracts user email from context
func GetUserEmail(c *fiber.Ctx) (string, bool) {
	email := c.Locals("user_email")
	if email == nil {
		return "", false
	}
	e, ok := email.(string)
	return e, ok
}

// GetUserRole extracts user role from context
func GetUserRole(c *fiber.Ctx) (string, bool) {
	role := c.Locals("user_role")
	if role == nil {
		return "", false
	}
	r, ok := role.(string)
	return r, ok
}

// GetUser extracts full user object from context
func GetUser(c *fiber.Ctx) (*model.User, bool) {
	user := c.Locals("user")
	if user == nil {
		return nil, false
	}
	u, ok := user.(*model.User)
	return u, ok
}

// GetClaims extracts full claims from context
func GetClaims(c *fiber.Ctx) (*auth.Claims, bool) {
	claims := c.Locals("claims")
	if claims == nil {
		return nil, false
	}
	claimsData, ok := claims.(*auth.Claims)
	return claimsData, ok
}

// GetTokenJTI extracts the token JTI from context
func GetTokenJTI(c *fiber.Ctx) (string, bool) {
	jti := c.Locals("token_jti")
	if jti == nil {
		return "", false
	}
	j, ok := jti.(string)
	return j, ok
}
