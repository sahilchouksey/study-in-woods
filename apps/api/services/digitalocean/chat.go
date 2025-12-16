package digitalocean

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatMessage represents a message in a chat conversation
type ChatMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

// ChatCompletionRequest represents a request for chat completion
type ChatCompletionRequest struct {
	AgentUUID   string        `json:"-"` // Not sent in body, used in URL
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatCompletionResponse represents a response from chat completion
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	CreatedAt time.Time `json:"created_at"`
}

// StreamChunk represents a chunk in a streaming response
type StreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Created int `json:"created"`
}

// CreateChatCompletion creates a chat completion (non-streaming)
func (c *Client) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s/chat/completions", req.AgentUUID)

	// Don't include stream in request body for non-streaming
	req.Stream = false

	var result ChatCompletionResponse
	if err := c.doRequest(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// StreamChatCompletion creates a streaming chat completion
func (c *Client) StreamChatCompletion(ctx context.Context, req ChatCompletionRequest, callback func(StreamChunk) error) error {
	endpoint := fmt.Sprintf("%s/v2/gen-ai/agents/%s/chat/completions", c.baseURL, req.AgentUUID)

	// Force stream to true
	req.Stream = true

	// Build request body
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Authorization", "Bearer "+c.apiToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	// Make request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("streaming failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read SSE stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse SSE data
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				break
			}

			// Parse JSON chunk
			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Log error but continue streaming
				continue
			}

			// Call callback with chunk
			if err := callback(chunk); err != nil {
				return fmt.Errorf("callback error: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream reading error: %w", err)
	}

	return nil
}

// ExtractContent extracts the content from a chat completion response
func (r *ChatCompletionResponse) ExtractContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

// GetUsage returns the token usage from the response
func (r *ChatCompletionResponse) GetUsage() (prompt, completion, total int) {
	return r.Usage.PromptTokens, r.Usage.CompletionTokens, r.Usage.TotalTokens
}

// AgentChatRequest represents a request for chat completion via agent deployment
type AgentChatRequest struct {
	DeploymentURL string        `json:"-"` // Agent deployment URL (e.g., https://xxx.agents.do-ai.run)
	APIKey        string        `json:"-"` // Agent-specific API key
	Messages      []ChatMessage `json:"messages"`
	MaxTokens     int           `json:"max_tokens,omitempty"`
	Temperature   float64       `json:"temperature,omitempty"`
	TopP          float64       `json:"top_p,omitempty"`
	Stream        bool          `json:"stream,omitempty"`
}

// CreateAgentChatCompletion creates a chat completion using the agent deployment URL and API key
// This is the REQUIRED method for querying agents - the standard API endpoint returns "not routed" errors
func (c *Client) CreateAgentChatCompletion(ctx context.Context, req AgentChatRequest) (*ChatCompletionResponse, error) {
	// Build the full endpoint URL
	endpoint := fmt.Sprintf("%s/api/v1/chat/completions", strings.TrimSuffix(req.DeploymentURL, "/"))

	// Don't include stream in request body for non-streaming
	req.Stream = false

	// Build request body
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - use agent API key, not account token
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat completion failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// StreamAgentChatCompletion creates a streaming chat completion using the agent deployment URL and API key
func (c *Client) StreamAgentChatCompletion(ctx context.Context, req AgentChatRequest, callback func(StreamChunk) error) error {
	// Build the full endpoint URL
	endpoint := fmt.Sprintf("%s/api/v1/chat/completions", strings.TrimSuffix(req.DeploymentURL, "/"))

	// Force stream to true
	req.Stream = true

	// Build request body
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - use agent API key, not account token
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	// Make request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("streaming failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read SSE stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse SSE data
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				break
			}

			// Parse JSON chunk
			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Log error but continue streaming
				continue
			}

			// Call callback with chunk
			if err := callback(chunk); err != nil {
				return fmt.Errorf("callback error: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream reading error: %w", err)
	}

	return nil
}
