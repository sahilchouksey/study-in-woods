package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// UserAPIKeys holds the API keys passed from the frontend
type UserAPIKeys struct {
	TavilyKey    string
	ExaKey       string
	FirecrawlKey string
}

// HasAnyKey returns true if at least one key is available
func (k *UserAPIKeys) HasAnyKey() bool {
	return k.TavilyKey != "" || k.ExaKey != "" || k.FirecrawlKey != ""
}

// HasSearchKey returns true if a search API key is available
func (k *UserAPIKeys) HasSearchKey() bool {
	return k.TavilyKey != "" || k.ExaKey != ""
}

// HasScrapingKey returns true if a scraping API key is available
func (k *UserAPIKeys) HasScrapingKey() bool {
	return k.FirecrawlKey != ""
}

// registerWebTools registers web search and scraping tools
func (r *ChatToolsRegistry) registerWebTools(keys *UserAPIKeys) {
	if keys == nil {
		return
	}

	// Web Search Tool (uses Tavily or Exa)
	if keys.HasSearchKey() {
		r.RegisterTool(ToolDefinition{
			Name:        "web_search",
			Description: "Search the web for current information. Use this when the user asks about recent events, news, or needs up-to-date information that may not be in your training data or the course materials.",
			Parameters: []ToolParameter{
				{
					Name:        "query",
					Type:        "string",
					Description: "The search query. Be specific and include relevant keywords.",
					Required:    true,
				},
				{
					Name:        "max_results",
					Type:        "number",
					Description: "Maximum number of results to return (default: 5, max: 10)",
					Required:    false,
				},
			},
			RequiresAPIKey: true,
		})
	}

	// Web Scrape Tool (uses Firecrawl)
	if keys.HasScrapingKey() {
		r.RegisterTool(ToolDefinition{
			Name:        "web_scrape",
			Description: "Scrape content from a specific webpage URL. Use this when you need to read the full content of a webpage that the user has provided or that you found via web search.",
			Parameters: []ToolParameter{
				{
					Name:        "url",
					Type:        "string",
					Description: "The full URL of the webpage to scrape (must start with http:// or https://)",
					Required:    true,
				},
			},
			RequiresAPIKey: true,
		})
	}
}

// executeWebSearch executes the web_search tool using Tavily or Exa
func (r *ChatToolsRegistry) executeWebSearch(ctx context.Context, args map[string]interface{}, keys *UserAPIKeys) *ToolResult {
	if keys == nil || !keys.HasSearchKey() {
		return &ToolResult{
			Success: false,
			Error:   "No search API key available. Please add a Tavily or Exa API key in settings.",
		}
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &ToolResult{
			Success: false,
			Error:   "Invalid or missing 'query' parameter",
		}
	}

	maxResults := 5
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int(mr)
		if maxResults > 10 {
			maxResults = 10
		}
		if maxResults < 1 {
			maxResults = 1
		}
	}

	// Prefer Tavily if available
	if keys.TavilyKey != "" {
		return r.executeTavilySearch(ctx, query, maxResults, keys.TavilyKey)
	}

	// Fall back to Exa
	if keys.ExaKey != "" {
		return r.executeExaSearch(ctx, query, maxResults, keys.ExaKey)
	}

	return &ToolResult{
		Success: false,
		Error:   "No search API key configured",
	}
}

// executeTavilySearch performs a search using Tavily API
func (r *ChatToolsRegistry) executeTavilySearch(ctx context.Context, query string, maxResults int, apiKey string) *ToolResult {
	reqBody := map[string]interface{}{
		"api_key":     apiKey,
		"query":       query,
		"max_results": maxResults,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to prepare search request"}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewBuffer(jsonBody))
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to create search request"}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Tavily search error: %v", err)
		return &ToolResult{Success: false, Error: "Search request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to read search response"}
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Tavily API error: %d - %s", resp.StatusCode, string(body))
		return &ToolResult{Success: false, Error: fmt.Sprintf("Search API returned status %d", resp.StatusCode)}
	}

	var tavilyResp struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return &ToolResult{Success: false, Error: "Failed to parse search response"}
	}

	// Format results for AI
	var formattedResults []map[string]interface{}
	for _, result := range tavilyResp.Results {
		formattedResults = append(formattedResults, map[string]interface{}{
			"title":   result.Title,
			"url":     result.URL,
			"content": result.Content,
			"score":   result.Score,
		})
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"query":   query,
			"count":   len(formattedResults),
			"results": formattedResults,
		},
	}
}

// executeExaSearch performs a search using Exa API
func (r *ChatToolsRegistry) executeExaSearch(ctx context.Context, query string, maxResults int, apiKey string) *ToolResult {
	reqBody := map[string]interface{}{
		"query":      query,
		"numResults": maxResults,
		"contents": map[string]interface{}{
			"text": true,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to prepare search request"}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.exa.ai/search", bytes.NewBuffer(jsonBody))
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to create search request"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Exa search error: %v", err)
		return &ToolResult{Success: false, Error: "Search request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to read search response"}
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Exa API error: %d - %s", resp.StatusCode, string(body))
		return &ToolResult{Success: false, Error: fmt.Sprintf("Search API returned status %d", resp.StatusCode)}
	}

	var exaResp struct {
		Results []struct {
			Title string  `json:"title"`
			URL   string  `json:"url"`
			Text  string  `json:"text"`
			Score float64 `json:"score"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &exaResp); err != nil {
		return &ToolResult{Success: false, Error: "Failed to parse search response"}
	}

	// Format results for AI
	var formattedResults []map[string]interface{}
	for _, result := range exaResp.Results {
		content := result.Text
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		formattedResults = append(formattedResults, map[string]interface{}{
			"title":   result.Title,
			"url":     result.URL,
			"content": content,
			"score":   result.Score,
		})
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"query":   query,
			"count":   len(formattedResults),
			"results": formattedResults,
		},
	}
}

// executeWebScrape executes the web_scrape tool using Firecrawl
func (r *ChatToolsRegistry) executeWebScrape(ctx context.Context, args map[string]interface{}, keys *UserAPIKeys) *ToolResult {
	if keys == nil || !keys.HasScrapingKey() {
		return &ToolResult{
			Success: false,
			Error:   "No scraping API key available. Please add a Firecrawl API key in settings.",
		}
	}

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return &ToolResult{
			Success: false,
			Error:   "Invalid or missing 'url' parameter",
		}
	}

	reqBody := map[string]interface{}{
		"url":     url,
		"formats": []string{"markdown"},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to prepare scrape request"}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewBuffer(jsonBody))
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to create scrape request"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+keys.FirecrawlKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Firecrawl scrape error: %v", err)
		return &ToolResult{Success: false, Error: "Scrape request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{Success: false, Error: "Failed to read scrape response"}
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Firecrawl API error: %d - %s", resp.StatusCode, string(body))
		return &ToolResult{Success: false, Error: fmt.Sprintf("Scrape API returned status %d", resp.StatusCode)}
	}

	var firecrawlResp struct {
		Success bool `json:"success"`
		Data    struct {
			Markdown string `json:"markdown"`
			Title    string `json:"title"`
			URL      string `json:"url"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &firecrawlResp); err != nil {
		return &ToolResult{Success: false, Error: "Failed to parse scrape response"}
	}

	if !firecrawlResp.Success {
		return &ToolResult{Success: false, Error: "Scraping failed for the given URL"}
	}

	// Truncate content if too long
	content := firecrawlResp.Data.Markdown
	if len(content) > 10000 {
		content = content[:10000] + "\n\n[Content truncated due to length...]"
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"url":     firecrawlResp.Data.URL,
			"title":   firecrawlResp.Data.Title,
			"content": content,
		},
	}
}
