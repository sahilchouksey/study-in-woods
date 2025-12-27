package digitalocean

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter for API requests
// This helps prevent 429 rate limit errors from DigitalOcean's GenAI API
type RateLimiter struct {
	mu sync.Mutex

	// Token bucket parameters
	tokens         float64       // Current number of tokens
	maxTokens      float64       // Maximum tokens (bucket size)
	refillRate     float64       // Tokens added per second
	lastRefillTime time.Time     // Last time tokens were refilled
	minInterval    time.Duration // Minimum interval between requests

	// For GenAI-specific rate limiting (more conservative)
	genAITokens         float64
	genAIMaxTokens      float64
	genAIRefillRate     float64
	genAILastRefillTime time.Time
	genAIMinInterval    time.Duration
}

// RateLimiterConfig holds configuration for the rate limiter
type RateLimiterConfig struct {
	// General API rate limiting
	MaxTokens   float64       // Max burst capacity (default: 10)
	RefillRate  float64       // Tokens per second (default: 2)
	MinInterval time.Duration // Minimum time between requests (default: 200ms)

	// GenAI-specific rate limiting (more conservative)
	GenAIMaxTokens   float64       // Max burst for GenAI calls (default: 3)
	GenAIRefillRate  float64       // Tokens per second for GenAI (default: 0.1 = 1 per 10s)
	GenAIMinInterval time.Duration // Minimum time between GenAI requests (default: 10s)
}

// DefaultRateLimiterConfig returns sensible defaults for DigitalOcean API
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		// General API: 10 burst, 2/sec refill, 200ms min interval
		MaxTokens:   10,
		RefillRate:  2,
		MinInterval: 200 * time.Millisecond,

		// GenAI API: 3 burst, 1 per 10s refill, 30s min interval
		// This is very conservative to avoid 429 errors
		GenAIMaxTokens:   3,
		GenAIRefillRate:  0.033, // ~1 token per 30 seconds
		GenAIMinInterval: 30 * time.Second,
	}
}

// NewRateLimiter creates a new rate limiter with the given config
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	now := time.Now()
	return &RateLimiter{
		tokens:              config.MaxTokens,
		maxTokens:           config.MaxTokens,
		refillRate:          config.RefillRate,
		lastRefillTime:      now,
		minInterval:         config.MinInterval,
		genAITokens:         config.GenAIMaxTokens,
		genAIMaxTokens:      config.GenAIMaxTokens,
		genAIRefillRate:     config.GenAIRefillRate,
		genAILastRefillTime: now,
		genAIMinInterval:    config.GenAIMinInterval,
	}
}

// Wait blocks until a token is available for a general API request
// Returns an error if the context is cancelled
func (r *RateLimiter) Wait(ctx context.Context) error {
	return r.waitForToken(ctx, false)
}

// WaitGenAI blocks until a token is available for a GenAI API request
// Uses more conservative rate limiting since GenAI endpoints are more restrictive
func (r *RateLimiter) WaitGenAI(ctx context.Context) error {
	return r.waitForToken(ctx, true)
}

func (r *RateLimiter) waitForToken(ctx context.Context, isGenAI bool) error {
	for {
		r.mu.Lock()
		r.refillTokens()

		var tokens *float64
		var minInterval time.Duration

		if isGenAI {
			tokens = &r.genAITokens
			minInterval = r.genAIMinInterval
		} else {
			tokens = &r.tokens
			minInterval = r.minInterval
		}

		if *tokens >= 1 {
			*tokens--
			r.mu.Unlock()

			// Enforce minimum interval between requests
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(minInterval):
				return nil
			}
		}

		// Calculate wait time for next token
		var refillRate float64
		if isGenAI {
			refillRate = r.genAIRefillRate
		} else {
			refillRate = r.refillRate
		}
		waitTime := time.Duration(float64(time.Second) / refillRate)
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Try again after waiting
		}
	}
}

// refillTokens adds tokens based on elapsed time (must be called with lock held)
func (r *RateLimiter) refillTokens() {
	now := time.Now()

	// Refill general tokens
	elapsed := now.Sub(r.lastRefillTime).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefillTime = now

	// Refill GenAI tokens
	genAIElapsed := now.Sub(r.genAILastRefillTime).Seconds()
	r.genAITokens += genAIElapsed * r.genAIRefillRate
	if r.genAITokens > r.genAIMaxTokens {
		r.genAITokens = r.genAIMaxTokens
	}
	r.genAILastRefillTime = now
}

// TryAcquire attempts to acquire a token without blocking
// Returns true if a token was acquired, false otherwise
func (r *RateLimiter) TryAcquire(isGenAI bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refillTokens()

	var tokens *float64
	if isGenAI {
		tokens = &r.genAITokens
	} else {
		tokens = &r.tokens
	}

	if *tokens >= 1 {
		*tokens--
		return true
	}
	return false
}

// AvailableTokens returns the current number of available tokens
func (r *RateLimiter) AvailableTokens(isGenAI bool) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refillTokens()

	if isGenAI {
		return r.genAITokens
	}
	return r.tokens
}

// SetBackoffMultiplier temporarily reduces the rate limit
// Useful after receiving a 429 error - call with multiplier > 1 to slow down
func (r *RateLimiter) SetBackoffMultiplier(multiplier float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reduce refill rate temporarily
	r.genAIRefillRate = r.genAIRefillRate / multiplier
	// Increase minimum interval
	r.genAIMinInterval = time.Duration(float64(r.genAIMinInterval) * multiplier)
}

// ResetToDefaults resets the rate limiter to default configuration
func (r *RateLimiter) ResetToDefaults() {
	config := DefaultRateLimiterConfig()
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refillRate = config.RefillRate
	r.minInterval = config.MinInterval
	r.genAIRefillRate = config.GenAIRefillRate
	r.genAIMinInterval = config.GenAIMinInterval
}
