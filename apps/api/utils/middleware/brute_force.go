package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/utils/cache"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// BruteForceProtection handles brute force protection using Redis
type BruteForceProtection struct {
	redisCache *cache.RedisCache
}

// NewBruteForceProtection creates a new brute force protection instance
func NewBruteForceProtection(redisCache *cache.RedisCache) *BruteForceProtection {
	return &BruteForceProtection{
		redisCache: redisCache,
	}
}

// CheckAndRecordAttempt middleware checks if IP is locked out
func (b *BruteForceProtection) CheckAndRecordAttempt() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		lockKey := fmt.Sprintf("brute_force:lock:%s", ip)

		// Check if IP is locked
		locked, err := b.redisCache.Exists(c.Context(), lockKey)
		if err != nil {
			// If Redis is down, allow the request but log the error
			// Don't block legitimate users due to cache issues
			return c.Next()
		}

		if locked {
			// Get TTL for retry time
			ttl, _ := b.redisCache.TTL(c.Context(), lockKey)
			retryAfter := int(ttl.Seconds())
			if retryAfter < 0 {
				retryAfter = 60 // Default to 60 seconds
			}

			c.Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			return response.TooManyRequests(c, fmt.Sprintf("Too many failed attempts. Try again in %d seconds", retryAfter))
		}

		return c.Next()
	}
}

// RecordFailedAttempt records a failed login attempt and applies progressive lockouts
func (b *BruteForceProtection) RecordFailedAttempt(c *fiber.Ctx, ip, email string) error {
	ctx := c.Context()
	attemptKey := fmt.Sprintf("brute_force:attempts:%s", ip)
	lockKey := fmt.Sprintf("brute_force:lock:%s", ip)

	// Increment attempt counter
	attempts, err := b.redisCache.Increment(ctx, attemptKey)
	if err != nil {
		// If Redis is down, just return without blocking
		return nil
	}

	// Set expiry on attempts counter (15 minute window)
	if attempts == 1 {
		b.redisCache.Expire(ctx, attemptKey, 15*time.Minute)
	}

	// Apply progressive lockouts
	var lockDuration time.Duration
	switch {
	case attempts >= 25:
		// 25+ attempts: 24 hour lockout
		lockDuration = 24 * time.Hour
	case attempts >= 10:
		// 10-24 attempts: 1 hour lockout
		lockDuration = 1 * time.Hour
	case attempts >= 5:
		// 5-9 attempts: 2 minute lockout
		lockDuration = 2 * time.Minute
	default:
		// Less than 5 attempts: no lockout yet
		return nil
	}

	// Apply lockout
	return b.redisCache.Set(ctx, lockKey, "locked", lockDuration)
}

// RecordSuccessfulAttempt clears failed attempts on successful login
func (b *BruteForceProtection) RecordSuccessfulAttempt(c *fiber.Ctx, ip string) error {
	ctx := c.Context()
	attemptKey := fmt.Sprintf("brute_force:attempts:%s", ip)
	lockKey := fmt.Sprintf("brute_force:lock:%s", ip)

	// Clear attempts counter and any locks
	b.redisCache.Delete(ctx, attemptKey)
	b.redisCache.Delete(ctx, lockKey)

	return nil
}

// GetAttemptCount returns the current attempt count for an IP
func (b *BruteForceProtection) GetAttemptCount(c *fiber.Ctx, ip string) (int, error) {
	ctx := c.Context()
	attemptKey := fmt.Sprintf("brute_force:attempts:%s", ip)

	val, err := b.redisCache.Get(ctx, attemptKey)
	if err != nil {
		if err == cache.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}

	var count int
	fmt.Sscanf(val, "%d", &count)
	return count, nil
}

// IsIPLocked checks if an IP is currently locked
func (b *BruteForceProtection) IsIPLocked(c *fiber.Ctx, ip string) (bool, error) {
	ctx := c.Context()
	lockKey := fmt.Sprintf("brute_force:lock:%s", ip)
	return b.redisCache.Exists(ctx, lockKey)
}

// ClearAttempts manually clears attempts for an IP (admin function)
func (b *BruteForceProtection) ClearAttempts(c *fiber.Ctx, ip string) error {
	ctx := c.Context()
	attemptKey := fmt.Sprintf("brute_force:attempts:%s", ip)
	lockKey := fmt.Sprintf("brute_force:lock:%s", ip)

	b.redisCache.Delete(ctx, attemptKey)
	b.redisCache.Delete(ctx, lockKey)

	return nil
}
