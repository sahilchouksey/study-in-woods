package digitalocean

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	// BaseURL is the DigitalOcean API base URL
	BaseURL = "https://api.digitalocean.com"
	// DefaultTimeout is the default HTTP client timeout for regular API calls
	DefaultTimeout = 30 * time.Second
	// DefaultStreamingTimeout is the timeout for streaming requests (AI responses can take several minutes)
	// NOTE: This is now used for connection/header timeouts only, NOT body reading
	DefaultStreamingTimeout = 5 * time.Minute
	// DefaultDialTimeout is the timeout for establishing TCP connections
	DefaultDialTimeout = 10 * time.Second
	// DefaultTLSTimeout is the timeout for TLS handshake
	DefaultTLSTimeout = 10 * time.Second
	// DefaultHeaderTimeout is the timeout for waiting for response headers
	DefaultHeaderTimeout = 30 * time.Second
	// DefaultIdleTimeout is the timeout for idle connections
	DefaultIdleTimeout = 90 * time.Second
)

// Client handles all DigitalOcean API interactions
type Client struct {
	apiToken        string
	baseURL         string
	httpClient      *http.Client // For regular API calls
	streamingClient *http.Client // For streaming requests (longer timeout)
	retryConfig     RetryConfig
	rateLimiter     *RateLimiter // Rate limiter for API requests
}

// Config holds configuration for the DigitalOcean client
type Config struct {
	APIToken          string
	Timeout           time.Duration
	StreamingTimeout  time.Duration
	BaseURL           string
	RetryConfig       *RetryConfig       // Optional custom retry config
	RateLimiterConfig *RateLimiterConfig // Optional rate limiter config
}

// RetryConfig holds retry configuration for failed requests
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retry attempts (default: 2)
	InitialBackoff time.Duration // Initial backoff duration (default: 500ms)
	MaxBackoff     time.Duration // Maximum backoff duration (default: 30s)
}

// DefaultRetryConfig returns the default retry configuration
// Matches DigitalOcean SDK behavior: 2 retries with exponential backoff
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
	}
}

// NewClient creates a new DigitalOcean API client
func NewClient(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = BaseURL
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.StreamingTimeout == 0 {
		config.StreamingTimeout = DefaultStreamingTimeout
	}

	retryConfig := DefaultRetryConfig()
	if config.RetryConfig != nil {
		retryConfig = *config.RetryConfig
	}

	rateLimiterConfig := DefaultRateLimiterConfig()
	if config.RateLimiterConfig != nil {
		rateLimiterConfig = *config.RateLimiterConfig
	}
	rateLimiter := NewRateLimiter(rateLimiterConfig)

	// Create streaming transport with proper timeouts for SSE
	// IMPORTANT: Do NOT set http.Client.Timeout for streaming - it kills long-running streams!
	// Instead, use Transport-level timeouts for connection establishment only
	streamingTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   DefaultDialTimeout, // TCP connection timeout
			KeepAlive: DefaultIdleTimeout, // Keep-alive probe interval
		}).DialContext,
		TLSHandshakeTimeout:   DefaultTLSTimeout,    // TLS handshake timeout
		ResponseHeaderTimeout: DefaultHeaderTimeout, // Time to wait for response headers
		// NO IdleConnTimeout - we want to keep streaming connections alive
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		// DisableKeepAlives: false, // Keep connections alive for streaming
	}

	return &Client{
		apiToken: config.APIToken,
		baseURL:  config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		// Streaming client: NO Timeout set on client level!
		// This allows SSE streams to run indefinitely (or until server closes)
		// Connection/header timeouts are handled by Transport
		streamingClient: &http.Client{
			Transport: streamingTransport,
			// Timeout: 0, // Explicitly NOT setting this - streams can run as long as needed
		},
		retryConfig: retryConfig,
		rateLimiter: rateLimiter,
	}
}

// GetStreamingClient returns the streaming HTTP client (for use in streaming methods)
func (c *Client) GetStreamingClient() *http.Client {
	return c.streamingClient
}

// GetRetryConfig returns the retry configuration
func (c *Client) GetRetryConfig() RetryConfig {
	return c.retryConfig
}

// GetRateLimiter returns the rate limiter
func (c *Client) GetRateLimiter() *RateLimiter {
	return c.rateLimiter
}

// IsRetryableStatusCode checks if an HTTP status code should trigger a retry
// Retryable codes: 408 (Timeout), 409 (Conflict), 429 (Rate Limit), 5xx (Server errors)
func IsRetryableStatusCode(statusCode int) bool {
	return statusCode == 408 || statusCode == 409 || statusCode == 429 || statusCode >= 500
}

// CalculateBackoff returns the backoff duration for a given retry attempt
// Uses exponential backoff: initialBackoff * 2^attempt, capped at maxBackoff
func CalculateBackoff(attempt int, config RetryConfig) time.Duration {
	backoff := config.InitialBackoff * time.Duration(1<<uint(attempt))
	if backoff > config.MaxBackoff {
		return config.MaxBackoff
	}
	return backoff
}

// ParseRetryAfter extracts the retry-after header value from a response
// Returns 0 if the header is not present or cannot be parsed
func ParseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try parsing as seconds (most common)
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date
	if t, err := http.ParseTime(retryAfter); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return duration
		}
	}

	return 0
}

// doRequest performs an HTTP request to the DigitalOcean API
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	return c.doRequestWithRateLimit(ctx, method, endpoint, body, result, false)
}

// doRequestGenAI performs an HTTP request using GenAI rate limiting (more conservative)
func (c *Client) doRequestGenAI(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	return c.doRequestWithRateLimit(ctx, method, endpoint, body, result, true)
}

// doRequestWithRateLimit performs an HTTP request with rate limiting
func (c *Client) doRequestWithRateLimit(ctx context.Context, method, endpoint string, body interface{}, result interface{}, isGenAI bool) error {
	// Apply rate limiting
	if c.rateLimiter != nil {
		if isGenAI {
			if err := c.rateLimiter.WaitGenAI(ctx); err != nil {
				return fmt.Errorf("rate limiter wait cancelled: %w", err)
			}
		} else {
			if err := c.rateLimiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter wait cancelled: %w", err)
			}
		}
	}

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Perform request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Debug: Log error response
		fmt.Printf("[DO API Debug] Error Response (status %d): %s\n", resp.StatusCode, string(respBody))

		// Handle rate limit (429) specially
		if resp.StatusCode == 429 {
			retryAfter := ParseRetryAfter(resp)
			if retryAfter > 0 {
				fmt.Printf("[DO API Debug] Rate limited. Retry-After: %v\n", retryAfter)
			}
			// Apply backoff multiplier to slow down future requests
			if c.rateLimiter != nil {
				c.rateLimiter.SetBackoffMultiplier(2.0)
			}
		}

		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		apiErr.StatusCode = resp.StatusCode
		return &apiErr
	}

	// Decode response if result is provided
	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// APIError represents a DigitalOcean API error response
type APIError struct {
	ID         string `json:"id"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id"`
	StatusCode int    `json:"-"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("DigitalOcean API error: %s (request_id: %s)", e.Message, e.RequestID)
}

// Pagination represents pagination metadata
type Pagination struct {
	Total       int   `json:"total"`
	Count       int   `json:"count"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	TotalPages  int   `json:"total_pages"`
	Links       Links `json:"links"`
}

// Links contains pagination links
type Links struct {
	First    string `json:"first"`
	Previous string `json:"prev"`
	Next     string `json:"next"`
	Last     string `json:"last"`
}

// ListOptions specifies pagination options for list requests
type ListOptions struct {
	Page    int `json:"page,omitempty"`
	PerPage int `json:"per_page,omitempty"`
}

// HealthCheck verifies the client can connect to DigitalOcean API
func (c *Client) HealthCheck(ctx context.Context) error {
	// Try to list regions as a simple health check
	endpoint := "/v2/gen-ai/regions"
	var result struct {
		Regions []interface{} `json:"regions"`
	}

	return c.doRequest(ctx, "GET", endpoint, nil, &result)
}
