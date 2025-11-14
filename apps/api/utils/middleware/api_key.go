package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/cache"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// APIKeyMiddleware handles API key authentication and rate limiting
type APIKeyMiddleware struct {
	apiKeyService *services.APIKeyService
	cache         *cache.RedisCache
}

// NewAPIKeyMiddleware creates a new API key middleware
func NewAPIKeyMiddleware(db *gorm.DB, cache *cache.RedisCache) *APIKeyMiddleware {
	return &APIKeyMiddleware{
		apiKeyService: services.NewAPIKeyService(db),
		cache:         cache,
	}
}

// Authenticate validates API key and sets it in context
func (m *APIKeyMiddleware) Authenticate() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract API key from Authorization header
		// Format: "Bearer sk_live_..."
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			authHeader = c.Get("X-API-Key") // Alternative header
		}

		if authHeader == "" {
			return response.Unauthorized(c, "API key required")
		}

		// Extract key from "Bearer <key>" format
		apiKey := strings.TrimSpace(authHeader)
		if strings.HasPrefix(apiKey, "Bearer ") {
			apiKey = strings.TrimSpace(strings.TrimPrefix(apiKey, "Bearer "))
		}

		if apiKey == "" {
			return response.Unauthorized(c, "Invalid API key format")
		}

		// Validate API key
		key, err := m.apiKeyService.ValidateAPIKey(c.Context(), apiKey)
		if err != nil {
			return response.Unauthorized(c, err.Error())
		}

		// Check rate limit
		if err := m.checkRateLimit(c, key); err != nil {
			return response.TooManyRequests(c, err.Error())
		}

		// Store key in context for later use
		c.Locals("api_key", key)
		c.Locals("user_id", key.UserID)

		// Increment usage counter (async)
		go func() {
			_ = m.apiKeyService.IncrementUsage(c.Context(), key.ID)
		}()

		return c.Next()
	}
}

// checkRateLimit checks if the API key has exceeded its rate limit
func (m *APIKeyMiddleware) checkRateLimit(c *fiber.Ctx, key *model.ExternalAPIKey) error {
	if m.cache == nil {
		// No Redis available, skip rate limiting
		return nil
	}

	// Rate limit key format: "api_key:ratelimit:<key_id>"
	rateLimitKey := "api_key:ratelimit:" + string(rune(key.ID))

	// Get current request count
	count, err := m.cache.Increment(c.Context(), rateLimitKey)
	if err != nil {
		// Log error but don't fail the request
		return nil
	}

	// Set expiry on first request (1 minute window)
	if count == 1 {
		_ = m.cache.Set(c.Context(), rateLimitKey, "1", time.Minute)
	}

	// Check if rate limit exceeded
	if count > int64(key.RateLimit) {
		return fiber.NewError(fiber.StatusTooManyRequests, "Rate limit exceeded. Try again in 1 minute.")
	}

	return nil
}

// GetAPIKey retrieves the API key from context
func GetAPIKey(c *fiber.Ctx) (*model.ExternalAPIKey, bool) {
	key, ok := c.Locals("api_key").(*model.ExternalAPIKey)
	return key, ok
}
