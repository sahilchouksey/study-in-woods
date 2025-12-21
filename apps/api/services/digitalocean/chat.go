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

// RetrievalInfo represents citation/retrieval information from knowledge base
type RetrievalInfo struct {
	ID         string                 `json:"id,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Source     string                 `json:"source,omitempty"`
	SourceName string                 `json:"source_name,omitempty"`
	FileName   string                 `json:"file_name,omitempty"`
	Page       int                    `json:"page,omitempty"`
	Score      float64                `json:"score,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// StreamResult contains the result of a streaming request including any partial content on error
type StreamResult struct {
	PartialContent string          // Content accumulated before error occurred
	ChunkCount     int             // Number of chunks processed
	Retrievals     []RetrievalInfo // Any retrievals captured before error
	Error          error           // The error that occurred, nil if successful
	IsComplete     bool            // True if stream completed normally
	ErrorType      string          // Type of error: "timeout", "connection", "unknown"
}

// IsTimeoutError checks if the error is a timeout-related error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "client.timeout")
}

// NonStreamRetrieval represents the retrieval section in DO AI Agent non-streaming response
type NonStreamRetrieval struct {
	RetrievedData []RetrievedData `json:"retrieved_data,omitempty"`
}

// NonStreamCitations represents the citations section in DO AI Agent non-streaming response
type NonStreamCitations struct {
	Citations []RetrievedData `json:"citations,omitempty"`
}

// ChatCompletionResponse represents a response from chat completion
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"` // "stop", "tool_calls", etc.
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	// DO AI Agent format (nested objects)
	Retrieval *NonStreamRetrieval `json:"retrieval,omitempty"`
	Citations *NonStreamCitations `json:"citations,omitempty"`
	// Legacy flat array format
	Retrievals      []RetrievalInfo `json:"retrievals,omitempty"`
	LegacyCitations []RetrievalInfo `json:"-"` // Will be populated from various sources
	Sources         []RetrievalInfo `json:"sources,omitempty"`
	Context         []RetrievalInfo `json:"context,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// GetAllRetrievals returns all retrievals merged and deduplicated for non-streaming response
func (r *ChatCompletionResponse) GetAllRetrievals() []RetrievalInfo {
	var all []RetrievalInfo
	seen := make(map[string]bool)

	addUnique := func(info RetrievalInfo) {
		key := info.FileName
		if key == "" {
			key = info.ID
		}
		if key == "" && len(info.Content) > 0 {
			key = info.Content[:min(100, len(info.Content))]
		}
		if key != "" && !seen[key] {
			seen[key] = true
			all = append(all, info)
		}
	}

	// Add from new DO AI Agent format (nested)
	if r.Retrieval != nil {
		for _, rd := range r.Retrieval.RetrievedData {
			addUnique(RetrievalInfo{
				ID:       rd.ID,
				Content:  rd.PageContent,
				FileName: rd.Filename,
				Score:    rd.Score,
				Metadata: rd.Metadata,
			})
		}
	}

	if r.Citations != nil {
		for _, rd := range r.Citations.Citations {
			addUnique(RetrievalInfo{
				ID:       rd.ID,
				Content:  rd.PageContent,
				FileName: rd.Filename,
				Score:    rd.Score,
				Metadata: rd.Metadata,
			})
		}
	}

	// Add from legacy flat arrays
	for _, ri := range r.Retrievals {
		addUnique(ri)
	}
	for _, ri := range r.Sources {
		addUnique(ri)
	}
	for _, ri := range r.Context {
		addUnique(ri)
	}

	return all
}

// HasToolCalls checks if the response contains tool calls
func (r *ChatCompletionResponse) HasToolCalls() bool {
	if len(r.Choices) == 0 {
		return false
	}
	return len(r.Choices[0].Message.ToolCalls) > 0
}

// GetToolCalls returns tool calls from the response
func (r *ChatCompletionResponse) GetToolCalls() []ToolCall {
	if len(r.Choices) == 0 {
		return nil
	}
	return r.Choices[0].Message.ToolCalls
}

// StreamChunkDelta represents the delta content in a streaming chunk
type StreamChunkDelta struct {
	Role             string     `json:"role,omitempty"`
	Content          string     `json:"content,omitempty"`           // Actual response content (may be null during reasoning)
	ReasoningContent string     `json:"reasoning_content,omitempty"` // AI's thinking/reasoning process
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

// StreamChunkChoice represents a choice in a streaming chunk
type StreamChunkChoice struct {
	Index        int              `json:"index"`
	Delta        StreamChunkDelta `json:"delta"`
	FinishReason string           `json:"finish_reason,omitempty"` // "stop", "tool_calls", etc.
}

// RetrievedData represents citation data from the knowledge base (DO AI Agent format)
type RetrievedData struct {
	ID           string                 `json:"id,omitempty"`
	Index        string                 `json:"index,omitempty"`
	PageContent  string                 `json:"page_content,omitempty"`
	Score        float64                `json:"score,omitempty"`
	Filename     string                 `json:"filename,omitempty"`
	DataSourceID string                 `json:"data_source_id,omitempty"`
	ChunkID      int                    `json:"chunk_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// StreamRetrieval represents the retrieval section in DO AI Agent response
type StreamRetrieval struct {
	RetrievedData []RetrievedData `json:"retrieved_data,omitempty"`
}

// StreamCitations represents the citations section in DO AI Agent response
type StreamCitations struct {
	Citations []RetrievedData `json:"citations,omitempty"`
}

// StreamGuardrails represents guardrails info in DO AI Agent response
type StreamGuardrails struct {
	TriggeredGuardrails []string `json:"triggered_guardrails,omitempty"`
}

// StreamUsage represents token usage in DO AI Agent response
type StreamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a chunk in a streaming response (DO AI Agent format)
type StreamChunk struct {
	ID         string              `json:"id"`
	Object     string              `json:"object,omitempty"`
	Created    int                 `json:"created"`
	Model      string              `json:"model"`
	Choices    []StreamChunkChoice `json:"choices"`
	Retrieval  *StreamRetrieval    `json:"retrieval,omitempty"`  // DO AI Agent uses this for KB sources
	Citations  *StreamCitations    `json:"citations,omitempty"`  // DO AI Agent uses this for inline citations
	Guardrails *StreamGuardrails   `json:"guardrails,omitempty"` // Safety guardrails
	Usage      *StreamUsage        `json:"usage,omitempty"`      // Token usage (in final chunk)
	// Legacy field names for compatibility
	Retrievals      []RetrievalInfo `json:"retrievals,omitempty"`
	LegacyCitations []RetrievalInfo `json:"-"` // Will be populated from various sources
	Sources         []RetrievalInfo `json:"sources,omitempty"`
	Context         []RetrievalInfo `json:"context,omitempty"`
}

// GetReasoningContent returns the reasoning content from the first choice
func (c *StreamChunk) GetReasoningContent() string {
	if len(c.Choices) == 0 {
		return ""
	}
	return c.Choices[0].Delta.ReasoningContent
}

// GetContent returns the actual content from the first choice
func (c *StreamChunk) GetContent() string {
	if len(c.Choices) == 0 {
		return ""
	}
	return c.Choices[0].Delta.Content
}

// GetFinishReason returns the finish reason from the first choice
func (c *StreamChunk) GetFinishReason() string {
	if len(c.Choices) == 0 {
		return ""
	}
	return c.Choices[0].FinishReason
}

// IsDone returns true if the stream is done
func (c *StreamChunk) IsDone() bool {
	return c.GetFinishReason() == "stop"
}

// GetAllRetrievals returns all retrievals from various sources merged and deduplicated
func (c *StreamChunk) GetAllRetrievals() []RetrievalInfo {
	var all []RetrievalInfo
	seen := make(map[string]bool)

	// Helper to add with deduplication by filename or ID
	addUnique := func(r RetrievalInfo) {
		// Create a unique key based on filename (primary) or ID (fallback)
		key := r.FileName
		if key == "" {
			key = r.ID
		}
		if key == "" {
			key = r.Content[:min(100, len(r.Content))] // Use content prefix as fallback
		}
		if key != "" && !seen[key] {
			seen[key] = true
			all = append(all, r)
		}
	}

	// Add from new DO AI Agent format
	if c.Retrieval != nil {
		for _, r := range c.Retrieval.RetrievedData {
			addUnique(RetrievalInfo{
				ID:       r.ID,
				Content:  r.PageContent,
				FileName: r.Filename,
				Score:    r.Score,
				Metadata: r.Metadata,
			})
		}
	}

	// Add from citations field
	if c.Citations != nil {
		for _, r := range c.Citations.Citations {
			addUnique(RetrievalInfo{
				ID:       r.ID,
				Content:  r.PageContent,
				FileName: r.Filename,
				Score:    r.Score,
				Metadata: r.Metadata,
			})
		}
	}

	// Add from legacy fields
	for _, r := range c.Retrievals {
		addUnique(r)
	}
	for _, r := range c.Sources {
		addUnique(r)
	}
	for _, r := range c.Context {
		addUnique(r)
	}

	return all
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

// StreamOptions represents options for streaming responses
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// Tool represents a tool/function the AI can call
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a function the AI can call
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema for parameters
}

// ToolCall represents a tool call made by the AI
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string of arguments
	} `json:"function"`
}

// ToolMessage represents a tool response message
type ToolMessage struct {
	Role       string `json:"role"` // "tool"
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

// ChatMessageWithToolCalls extends ChatMessage with tool calls
type ChatMessageWithToolCalls struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool response messages
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
	// Tool calling support
	Tools      []Tool `json:"tools,omitempty"`
	ToolChoice string `json:"tool_choice,omitempty"` // "auto", "none", or {"type": "function", "function": {"name": "..."}}
	// DigitalOcean specific options
	IncludeRetrievalInfo bool           `json:"include_retrieval_info,omitempty"`
	ProvideCitations     bool           `json:"provide_citations,omitempty"`
	StreamOptions        *StreamOptions `json:"stream_options,omitempty"`
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

	// Log raw response to see full structure including any citation fields
	fmt.Printf("[DO Non-Stream] Raw response (first 2000 chars): %s\n", string(body)[:min(len(body), 2000)])

	var result ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Use the new helper method to merge and deduplicate all retrievals
	result.Retrievals = result.GetAllRetrievals()

	// Log retrievals if present
	if len(result.Retrievals) > 0 {
		fmt.Printf("[DO Non-Stream] Found %d total retrievals in response\n", len(result.Retrievals))
		for i, r := range result.Retrievals {
			fmt.Printf("[DO Non-Stream]   Retrieval %d: id=%s, source=%s, file=%s, content_len=%d\n",
				i+1, r.ID, r.Source, r.FileName, len(r.Content))
		}
	} else {
		fmt.Printf("[DO Non-Stream] No retrievals found in response\n")
	}

	// Check for unknown fields
	var rawMap map[string]interface{}
	if err := json.Unmarshal(body, &rawMap); err == nil {
		for key := range rawMap {
			if key != "id" && key != "model" && key != "choices" && key != "created" && key != "retrievals" && key != "object" && key != "usage" && key != "system_fingerprint" && key != "citations" && key != "sources" && key != "context" {
				fmt.Printf("[DO Non-Stream] Unknown field: %s = %v\n", key, rawMap[key])
			}
		}
	}

	return &result, nil
}

// StreamAgentChatCompletion creates a streaming chat completion using the agent deployment URL and API key
// This method includes automatic retry logic with exponential backoff for transient failures
func (c *Client) StreamAgentChatCompletion(ctx context.Context, req AgentChatRequest, callback func(StreamChunk) error) error {
	retryConfig := c.GetRetryConfig()
	var lastErr error

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := CalculateBackoff(attempt-1, retryConfig)
			fmt.Printf("[DO Stream] Retry attempt %d/%d after %v backoff\n", attempt, retryConfig.MaxRetries, backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := c.doStreamRequest(ctx, req, callback)
		if err == nil {
			return nil
		}

		lastErr = err
		fmt.Printf("[DO Stream] Request failed (attempt %d/%d): %v\n", attempt+1, retryConfig.MaxRetries+1, err)

		// Check if error is retryable
		if !c.isStreamErrorRetryable(err) {
			fmt.Printf("[DO Stream] Error is not retryable, failing immediately\n")
			return err
		}
	}

	return fmt.Errorf("streaming failed after %d retries: %w", retryConfig.MaxRetries+1, lastErr)
}

// isStreamErrorRetryable determines if a streaming error should trigger a retry
func (c *Client) isStreamErrorRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Connection errors are retryable
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "no such host") {
		return true
	}

	// Check for HTTP status codes in error message
	for _, code := range []string{"408", "409", "429", "500", "502", "503", "504"} {
		if strings.Contains(errStr, fmt.Sprintf("status %s", code)) {
			return true
		}
	}

	return false
}

// doStreamRequest performs the actual streaming request
func (c *Client) doStreamRequest(ctx context.Context, req AgentChatRequest, callback func(StreamChunk) error) error {
	// Build the full endpoint URL
	endpoint := fmt.Sprintf("%s/api/v1/chat/completions", strings.TrimSuffix(req.DeploymentURL, "/"))

	// Force stream to true
	req.Stream = true

	// Build request body
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request - use background context to avoid cancellation from Fiber
	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - use agent API key, not account token
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	fmt.Printf("[DO Stream] Making HTTP request (streaming client with transport-level timeouts)...\n")

	// Use streaming client - NO client-level timeout, uses Transport timeouts for connection only
	resp, err := c.GetStreamingClient().Do(httpReq)
	if err != nil {
		fmt.Printf("[DO Stream] HTTP error: %v\n", err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("[DO Stream] Response status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("streaming failed with status %d: %s", resp.StatusCode, string(body))
		fmt.Printf("[DO Stream] Error body: %s\n", string(body))

		// Handle rate limiting specially - check for Retry-After header
		if resp.StatusCode == 429 {
			retryAfter := ParseRetryAfter(resp)
			if retryAfter > 0 {
				fmt.Printf("[DO Stream] Rate limited, Retry-After: %v\n", retryAfter)
				return fmt.Errorf("rate limited (status 429), retry after %v: %s", retryAfter, string(body))
			}
		}

		return fmt.Errorf("%s", errMsg)
	}

	// Read SSE stream with application-level idle timeout
	// This detects stalled streams where server stops sending data
	fmt.Printf("[DO Stream] Starting to read SSE stream (no timeout - stream can run indefinitely)...\n")
	chunkCount := 0
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
				fmt.Printf("[DO Stream] Received [DONE] after %d chunks\n", chunkCount)
				break
			}

			// Parse JSON chunk
			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				fmt.Printf("[DO Stream] JSON parse error: %v\n", err)
				continue
			}
			chunkCount++

			// Log first 3 chunks raw JSON to see full structure
			if chunkCount <= 3 {
				fmt.Printf("[DO Stream] Raw chunk #%d: %s\n", chunkCount, data)
			}

			// Log content chunks (only first 10 and then every 100th to reduce noise)
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				if chunkCount <= 10 || chunkCount%100 == 0 {
					fmt.Printf("[DO Stream] Content chunk #%d: %q\n", chunkCount, chunk.Choices[0].Delta.Content)
				}
			}

			// Log finish reason when stream ends
			if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != "" {
				fmt.Printf("[DO Stream] Finish reason at chunk #%d: %s\n", chunkCount, chunk.Choices[0].FinishReason)
				fmt.Printf("[DO Stream] Final chunk raw JSON: %s\n", data)
			}

			// Log reasoning content if present
			if reasoningContent := chunk.GetReasoningContent(); reasoningContent != "" {
				if chunkCount <= 10 || chunkCount%100 == 0 {
					fmt.Printf("[DO Stream] Reasoning chunk #%d: %q\n", chunkCount, reasoningContent)
				}
			}

			// Log retrieval info if present (using new helper method)
			allRetrievals := chunk.GetAllRetrievals()

			// Also check for DO AI Agent format (retrieval.retrieved_data)
			if chunk.Retrieval != nil && len(chunk.Retrieval.RetrievedData) > 0 {
				fmt.Printf("[DO Stream] Found 'retrieval.retrieved_data' with %d items\n", len(chunk.Retrieval.RetrievedData))
			}

			// Check for citations in new format
			if chunk.Citations != nil && len(chunk.Citations.Citations) > 0 {
				fmt.Printf("[DO Stream] Found 'citations.citations' with %d items\n", len(chunk.Citations.Citations))
			}

			if len(allRetrievals) > 0 {
				fmt.Printf("[DO Stream] Received %d total retrievals at chunk #%d\n", len(allRetrievals), chunkCount)
				for i, r := range allRetrievals {
					fmt.Printf("[DO Stream]   Retrieval %d: id=%s, source=%s, file=%s, page=%d, content_len=%d\n",
						i+1, r.ID, r.Source, r.FileName, r.Page, len(r.Content))
				}
				// Store merged retrievals in chunk for callback
				chunk.Retrievals = allRetrievals
			}

			// Log usage info if present (typically in final chunk)
			if chunk.Usage != nil {
				fmt.Printf("[DO Stream] Usage at chunk #%d: prompt=%d, completion=%d, total=%d\n",
					chunkCount, chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens, chunk.Usage.TotalTokens)
			}

			// Check for any unknown fields in the raw JSON
			var rawMap map[string]interface{}
			if err := json.Unmarshal([]byte(data), &rawMap); err == nil {
				knownFields := map[string]bool{
					"id": true, "model": true, "choices": true, "created": true, "object": true,
					"retrieval": true, "citations": true, "guardrails": true, "usage": true,
					"retrievals": true, "sources": true, "context": true, "system_fingerprint": true,
					"functions": true,
				}
				for key := range rawMap {
					if !knownFields[key] {
						fmt.Printf("[DO Stream] Unknown field in chunk #%d: %s = %v\n", chunkCount, key, rawMap[key])
					}
				}
			}

			// Call callback with chunk
			if err := callback(chunk); err != nil {
				return fmt.Errorf("callback error: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("[DO Stream] Scanner error: %v\n", err)
		return fmt.Errorf("stream reading error: %w", err)
	}

	fmt.Printf("[DO Stream] Stream completed successfully with %d chunks\n", chunkCount)
	return nil
}
