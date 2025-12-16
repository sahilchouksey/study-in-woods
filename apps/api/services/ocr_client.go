package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// OCRClient handles communication with the simple OCR service
type OCRClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// OCRResponse represents the response from OCR service
type OCRResponse struct {
	Text      string `json:"text"`
	PageCount int    `json:"page_count"`
	Filename  string `json:"filename,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
}

// NewOCRClient creates a new OCR client
func NewOCRClient() *OCRClient {
	baseURL := os.Getenv("OCR_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8081" // Default to localhost (port 8081 - Go API uses 8080)
	}

	return &OCRClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute, // OCR can take time for large PDFs
		},
	}
}

// ProcessPDFFile processes a PDF file and returns extracted text
func (c *OCRClient) ProcessPDFFile(ctx context.Context, pdfBytes []byte, filename string) (*OCRResponse, error) {
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(pdfBytes); err != nil {
		return nil, fmt.Errorf("failed to write file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/ocr/file", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OCR service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OCR service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var ocrResp OCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocrResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &ocrResp, nil
}

// ProcessPDFFromURL processes a PDF from a URL and returns extracted text
func (c *OCRClient) ProcessPDFFromURL(ctx context.Context, pdfURL string) (*OCRResponse, error) {
	// Create request payload
	payload := map[string]string{"url": pdfURL}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/ocr/url", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OCR service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OCR service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var ocrResp OCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocrResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &ocrResp, nil
}

// HealthCheck checks if OCR service is healthy
func (c *OCRClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OCR service unhealthy: status %d", resp.StatusCode)
	}

	return nil
}
