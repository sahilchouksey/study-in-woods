package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"time"
)

// SecurityConfig holds security middleware configuration
type SecurityConfig struct {
	AllowedOrigins    string
	RateLimitRequests int
	RateLimitWindow   time.Duration
	TrustedProxies    []string
}

// SetupSecurity applies all security middleware
func SetupSecurity(app *fiber.App, config SecurityConfig) {
	// Request ID middleware - add unique ID to each request
	app.Use(requestid.New())

	// Logger middleware - log all requests
	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} ${path} | ${ip}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

	// Recover middleware - recover from panics
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	// Helmet middleware - secure HTTP headers
	app.Use(helmet.New(helmet.Config{
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "SAMEORIGIN",
		HSTSMaxAge:         31536000,
		ReferrerPolicy:     "no-referrer",
	}))

	// CORS middleware
	origins := strings.Split(config.AllowedOrigins, ",")
	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(origins, ","),
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Tavily-Api-Key,X-Exa-Api-Key,X-Firecrawl-Api-Key",
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	// Rate limiting middleware
	if config.RateLimitRequests > 0 {
		app.Use(limiter.New(limiter.Config{
			Max:        config.RateLimitRequests,
			Expiration: config.RateLimitWindow,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    "RATE_LIMIT_EXCEEDED",
						"message": "Too many requests. Please try again later.",
					},
				})
			},
		}))
	}
}

// TrustedProxyConfig configures trusted proxies
func TrustedProxyConfig(app *fiber.App, trustedProxies []string) {
	// Note: TrustedProxyRanges configuration depends on Fiber version
	// For Fiber v2, this is typically handled through app configuration
	// You may need to configure this when creating the Fiber app instance
	_ = trustedProxies // Avoid unused variable warning
}
