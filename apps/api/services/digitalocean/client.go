package digitalocean

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// BaseURL is the DigitalOcean API base URL
	BaseURL = "https://api.digitalocean.com"
	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
)

// Client handles all DigitalOcean API interactions
type Client struct {
	apiToken   string
	baseURL    string
	httpClient *http.Client
}

// Config holds configuration for the DigitalOcean client
type Config struct {
	APIToken string
	Timeout  time.Duration
	BaseURL  string
}

// NewClient creates a new DigitalOcean API client
func NewClient(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = BaseURL
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	return &Client{
		apiToken: config.APIToken,
		baseURL:  config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// doRequest performs an HTTP request to the DigitalOcean API
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
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
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
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
