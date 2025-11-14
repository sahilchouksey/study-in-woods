package apikey

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// APIKeyHandler handles API key management requests
type APIKeyHandler struct {
	db            *gorm.DB
	apiKeyService *services.APIKeyService
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(db *gorm.DB, apiKeyService *services.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{
		db:            db,
		apiKeyService: apiKeyService,
	}
}

// CreateAPIKey handles POST /api/v1/api-keys
func (h *APIKeyHandler) CreateAPIKey(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse request
	var req struct {
		Name         string   `json:"name" validate:"required,min=3,max=100"`
		Scopes       []string `json:"scopes"`
		RateLimit    int      `json:"rate_limit"`    // Optional, defaults to 100
		MonthlyQuota int      `json:"monthly_quota"` // Optional, defaults to 10000
	}

	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Set defaults
	if req.RateLimit == 0 {
		req.RateLimit = 100
	}
	if req.MonthlyQuota == 0 {
		req.MonthlyQuota = 10000
	}

	// Create API key
	apiKey, err := h.apiKeyService.CreateAPIKey(c.Context(), user.ID, req.Name, req.Scopes, req.RateLimit, req.MonthlyQuota)
	if err != nil {
		return response.InternalServerError(c, "Failed to create API key: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "API key created successfully. Save this key securely - it will not be shown again.",
		"api_key": apiKey,
	})
}

// ListAPIKeys handles GET /api/v1/api-keys
func (h *APIKeyHandler) ListAPIKeys(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Get API keys
	keys, err := h.apiKeyService.ListAPIKeys(c.Context(), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to list API keys: "+err.Error())
	}

	return response.Success(c, keys)
}

// GetAPIKey handles GET /api/v1/api-keys/:id
func (h *APIKeyHandler) GetAPIKey(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse key ID
	id := c.Params("id")
	keyID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid API key ID")
	}

	// Get API key
	apiKey, err := h.apiKeyService.GetAPIKey(c.Context(), uint(keyID), user.ID)
	if err != nil {
		return response.NotFound(c, "API key not found")
	}

	return response.Success(c, apiKey)
}

// UpdateAPIKey handles PUT /api/v1/api-keys/:id
func (h *APIKeyHandler) UpdateAPIKey(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse key ID
	id := c.Params("id")
	keyID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid API key ID")
	}

	// Parse request
	var req struct {
		Name         *string  `json:"name"`
		Scopes       []string `json:"scopes"`
		RateLimit    *int     `json:"rate_limit"`
		MonthlyQuota *int     `json:"monthly_quota"`
		IsActive     *bool    `json:"is_active"`
	}

	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if len(req.Scopes) > 0 {
		// Convert scopes to JSON string
		scopesJSON := "["
		for i, scope := range req.Scopes {
			if i > 0 {
				scopesJSON += ","
			}
			scopesJSON += "\"" + scope + "\""
		}
		scopesJSON += "]"
		updates["scopes"] = scopesJSON
	}
	if req.RateLimit != nil {
		updates["rate_limit"] = *req.RateLimit
	}
	if req.MonthlyQuota != nil {
		updates["monthly_quota"] = *req.MonthlyQuota
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	// Update API key
	if err := h.apiKeyService.UpdateAPIKey(c.Context(), uint(keyID), user.ID, updates); err != nil {
		return response.InternalServerError(c, "Failed to update API key: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "API key updated successfully",
	})
}

// RevokeAPIKey handles POST /api/v1/api-keys/:id/revoke
func (h *APIKeyHandler) RevokeAPIKey(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse key ID
	id := c.Params("id")
	keyID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid API key ID")
	}

	// Revoke API key
	if err := h.apiKeyService.RevokeAPIKey(c.Context(), uint(keyID), user.ID); err != nil {
		return response.InternalServerError(c, "Failed to revoke API key: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "API key revoked successfully",
	})
}

// DeleteAPIKey handles DELETE /api/v1/api-keys/:id
func (h *APIKeyHandler) DeleteAPIKey(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse key ID
	id := c.Params("id")
	keyID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid API key ID")
	}

	// Delete API key
	if err := h.apiKeyService.DeleteAPIKey(c.Context(), uint(keyID), user.ID); err != nil {
		return response.InternalServerError(c, "Failed to delete API key: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "API key deleted successfully",
	})
}

// GetUsageStats handles GET /api/v1/api-keys/:id/usage
func (h *APIKeyHandler) GetUsageStats(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse key ID
	id := c.Params("id")
	keyID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid API key ID")
	}

	// Get usage stats
	stats, err := h.apiKeyService.GetUsageStats(c.Context(), uint(keyID), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to get usage stats: "+err.Error())
	}

	return response.Success(c, stats)
}

// ExtendExpiry handles POST /api/v1/api-keys/:id/extend
func (h *APIKeyHandler) ExtendExpiry(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse key ID
	id := c.Params("id")
	keyID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid API key ID")
	}

	// Parse request
	var req struct {
		Duration string `json:"duration"` // e.g., "30d", "6m", "1y"
	}

	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Parse duration
	duration, err := parseDuration(req.Duration)
	if err != nil {
		return response.BadRequest(c, "Invalid duration format. Use format like: 30d, 6m, 1y")
	}

	// Extend expiry
	if err := h.apiKeyService.ExtendExpiry(c.Context(), uint(keyID), user.ID, duration); err != nil {
		return response.InternalServerError(c, "Failed to extend API key expiry: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "API key expiry extended successfully",
	})
}

// parseDuration parses duration strings like "30d", "6m", "1y"
func parseDuration(duration string) (time.Duration, error) {
	duration = strings.ToLower(strings.TrimSpace(duration))

	if len(duration) < 2 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid duration format")
	}

	value, err := strconv.Atoi(duration[:len(duration)-1])
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid duration value")
	}

	unit := duration[len(duration)-1]
	switch unit {
	case 'd':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'm':
		return time.Duration(value) * 30 * 24 * time.Hour, nil // Approximate month
	case 'y':
		return time.Duration(value) * 365 * 24 * time.Hour, nil // Approximate year
	default:
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid duration unit. Use d (days), m (months), or y (years)")
	}
}
