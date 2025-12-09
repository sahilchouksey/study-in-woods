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
	// InferenceBaseURL is the DigitalOcean AI Inference API base URL
	InferenceBaseURL = "https://inference.do-ai.run"
	// DefaultInferenceTimeout is longer for LLM inference requests
	DefaultInferenceTimeout = 120 * time.Second
	// DefaultInferenceModel is the default model for inference
	DefaultInferenceModel = "openai-gpt-oss-120b"
)

// InferenceClient handles direct LLM inference API calls (not agent-based)
type InferenceClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	model      string
}

// InferenceConfig holds configuration for the inference client
type InferenceConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
	Model   string
}

// NewInferenceClient creates a new DigitalOcean AI Inference client
func NewInferenceClient(config InferenceConfig) *InferenceClient {
	if config.BaseURL == "" {
		config.BaseURL = InferenceBaseURL
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultInferenceTimeout
	}
	if config.Model == "" {
		config.Model = DefaultInferenceModel
	}

	return &InferenceClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		model: config.Model,
	}
}

// InferenceMessage represents a message in the inference chat completion request
type InferenceMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // The message content
}

// ResponseFormatType defines the type of response format
type ResponseFormatType string

const (
	// ResponseFormatText is for plain text responses (default)
	ResponseFormatText ResponseFormatType = "text"
	// ResponseFormatJSON requests JSON object output
	ResponseFormatJSON ResponseFormatType = "json_object"
	// ResponseFormatJSONSchema requests structured JSON with a specific schema
	ResponseFormatJSONSchema ResponseFormatType = "json_schema"
)

// JSONSchema defines the schema for structured JSON output
type JSONSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
	Strict      bool                   `json:"strict,omitempty"`
}

// ResponseFormat defines the response format for chat completions
type ResponseFormat struct {
	Type       ResponseFormatType `json:"type"`
	JSONSchema *JSONSchema        `json:"json_schema,omitempty"`
}

// InferenceRequest represents an OpenAI-compatible chat completion request for direct inference
type InferenceRequest struct {
	Model          string             `json:"model"`
	Messages       []InferenceMessage `json:"messages"`
	Temperature    float64            `json:"temperature,omitempty"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	TopP           float64            `json:"top_p,omitempty"`
	Stream         bool               `json:"stream,omitempty"`
	ResponseFormat *ResponseFormat    `json:"response_format,omitempty"`
}

// InferenceChoice represents a choice in the inference response
type InferenceChoice struct {
	Index        int              `json:"index"`
	Message      InferenceMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

// InferenceUsage represents token usage information
type InferenceUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// InferenceResponse represents the response from the inference API
type InferenceResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []InferenceChoice `json:"choices"`
	Usage   InferenceUsage    `json:"usage"`
}

// ChatCompletion sends a chat completion request to the inference API
func (c *InferenceClient) ChatCompletion(ctx context.Context, messages []InferenceMessage, options ...InferenceOption) (*InferenceResponse, error) {
	req := InferenceRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.3, // Default temperature for more deterministic output
		MaxTokens:   4096,
		Stream:      false,
	}

	// Apply options
	for _, opt := range options {
		opt(&req)
	}

	return c.sendChatCompletion(ctx, req)
}

// InferenceOption is a function that modifies the inference request
type InferenceOption func(*InferenceRequest)

// WithInferenceTemperature sets the temperature for the request
func WithInferenceTemperature(temp float64) InferenceOption {
	return func(req *InferenceRequest) {
		req.Temperature = temp
	}
}

// WithInferenceMaxTokens sets the max tokens for the request
func WithInferenceMaxTokens(tokens int) InferenceOption {
	return func(req *InferenceRequest) {
		req.MaxTokens = tokens
	}
}

// WithInferenceModel sets a different model for the request
func WithInferenceModel(model string) InferenceOption {
	return func(req *InferenceRequest) {
		req.Model = model
	}
}

// WithInferenceTopP sets the top_p value for the request
func WithInferenceTopP(topP float64) InferenceOption {
	return func(req *InferenceRequest) {
		req.TopP = topP
	}
}

// WithResponseFormatJSON enables JSON object output mode
func WithResponseFormatJSON() InferenceOption {
	return func(req *InferenceRequest) {
		req.ResponseFormat = &ResponseFormat{
			Type: ResponseFormatJSON,
		}
	}
}

// WithResponseFormatJSONSchema enables structured JSON output with a specific schema
func WithResponseFormatJSONSchema(name, description string, schema map[string]interface{}, strict bool) InferenceOption {
	return func(req *InferenceRequest) {
		req.ResponseFormat = &ResponseFormat{
			Type: ResponseFormatJSONSchema,
			JSONSchema: &JSONSchema{
				Name:        name,
				Description: description,
				Schema:      schema,
				Strict:      strict,
			},
		}
	}
}

// sendChatCompletion performs the actual API request
func (c *InferenceClient) sendChatCompletion(ctx context.Context, req InferenceRequest) (*InferenceResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers (OpenAI-compatible format)
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Perform request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("inference API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result InferenceResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// SimpleCompletion is a convenience method for simple single-turn completions
func (c *InferenceClient) SimpleCompletion(ctx context.Context, systemPrompt, userPrompt string, options ...InferenceOption) (string, error) {
	messages := []InferenceMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := c.ChatCompletion(ctx, messages, options...)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from inference API")
	}

	return resp.Choices[0].Message.Content, nil
}

// JSONCompletion is a convenience method for getting JSON responses
// It sets up the system prompt to request JSON output
func (c *InferenceClient) JSONCompletion(ctx context.Context, systemPrompt, userPrompt string, options ...InferenceOption) (string, error) {
	// Enhance system prompt to request JSON
	enhancedSystemPrompt := systemPrompt + "\n\nYou MUST respond with valid JSON only. Do not include any markdown formatting, code blocks, or explanatory text. Output raw JSON only."

	return c.SimpleCompletion(ctx, enhancedSystemPrompt, userPrompt, options...)
}

// StructuredCompletion is a convenience method for getting structured JSON responses using JSON schema
// This uses the response_format parameter for guaranteed valid JSON output
func (c *InferenceClient) StructuredCompletion(ctx context.Context, systemPrompt, userPrompt string, schemaName, schemaDescription string, schema map[string]interface{}, options ...InferenceOption) (string, error) {
	messages := []InferenceMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Add the JSON schema option
	options = append(options, WithResponseFormatJSONSchema(schemaName, schemaDescription, schema, true))

	resp, err := c.ChatCompletion(ctx, messages, options...)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from inference API")
	}

	return resp.Choices[0].Message.Content, nil
}

// StructuredCompletionWithResult is like StructuredCompletion but also unmarshals the result
func (c *InferenceClient) StructuredCompletionWithResult(ctx context.Context, systemPrompt, userPrompt string, schemaName, schemaDescription string, schema map[string]interface{}, result interface{}, options ...InferenceOption) error {
	response, err := c.StructuredCompletion(ctx, systemPrompt, userPrompt, schemaName, schemaDescription, schema, options...)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(response), result); err != nil {
		return fmt.Errorf("failed to unmarshal structured response: %w", err)
	}

	return nil
}

// HealthCheck verifies the inference API is accessible
func (c *InferenceClient) HealthCheck(ctx context.Context) error {
	messages := []InferenceMessage{
		{Role: "user", Content: "Say 'ok' if you can hear me."},
	}

	_, err := c.ChatCompletion(ctx, messages, WithInferenceMaxTokens(10))
	return err
}

// ExtractContent extracts the content from an inference response
func (r *InferenceResponse) ExtractContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

// GetUsage returns the token usage from the response
func (r *InferenceResponse) GetUsage() (prompt, completion, total int) {
	return r.Usage.PromptTokens, r.Usage.CompletionTokens, r.Usage.TotalTokens
}
