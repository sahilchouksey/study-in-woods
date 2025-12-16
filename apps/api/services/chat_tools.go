package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// ToolType represents the type of tool
type ToolType string

const (
	ToolTypeFunction ToolType = "function"
)

// ToolParameter defines a single parameter for a tool
type ToolParameter struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // string, number, boolean, array, object
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Enum        []string `json:"enum,omitempty"` // For constrained values
}

// ToolDefinition defines a tool that the AI can use
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  []ToolParameter `json:"parameters"`
	// Internal fields (not exposed to AI)
	RequiresMemoryService bool `json:"-"`
	RequiresDBAccess      bool `json:"-"`
	RequiresAPIKey        bool `json:"-"` // Requires user-provided API keys (web search/scraping)
}

// ToolCall represents a parsed tool call from AI response
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ChatToolsRegistry manages available tools for chat
type ChatToolsRegistry struct {
	tools         map[string]ToolDefinition
	memoryService *ChatMemoryService
	userAPIKeys   *UserAPIKeys // User-provided API keys for web tools
}

// NewChatToolsRegistry creates a new tools registry
func NewChatToolsRegistry(memoryService *ChatMemoryService) *ChatToolsRegistry {
	registry := &ChatToolsRegistry{
		tools:         make(map[string]ToolDefinition),
		memoryService: memoryService,
	}

	// Register default tools
	registry.registerDefaultTools()

	return registry
}

// NewChatToolsRegistryWithKeys creates a tools registry with user API keys
func NewChatToolsRegistryWithKeys(memoryService *ChatMemoryService, keys *UserAPIKeys) *ChatToolsRegistry {
	registry := &ChatToolsRegistry{
		tools:         make(map[string]ToolDefinition),
		memoryService: memoryService,
		userAPIKeys:   keys,
	}

	// Register default tools (memory-based)
	registry.registerDefaultTools()

	// Register web tools if API keys are provided
	if keys != nil && keys.HasAnyKey() {
		registry.registerWebTools(keys)
	}

	return registry
}

// SetUserAPIKeys sets the user's API keys and registers web tools
func (r *ChatToolsRegistry) SetUserAPIKeys(keys *UserAPIKeys) {
	r.userAPIKeys = keys
	if keys != nil && keys.HasAnyKey() {
		r.registerWebTools(keys)
	}
}

// registerDefaultTools registers all built-in tools
func (r *ChatToolsRegistry) registerDefaultTools() {
	// Memory Search Tool
	r.RegisterTool(ToolDefinition{
		Name:        "search_memory",
		Description: "Search through previous conversation history to recall information discussed earlier. Use this when the user asks you to 'remember', 'recall', 'what did I say about', or refers to something from a previous conversation.",
		Parameters: []ToolParameter{
			{
				Name:        "query",
				Type:        "string",
				Description: "The search query to find relevant past conversations. Be specific about what you're looking for.",
				Required:    true,
			},
			{
				Name:        "limit",
				Type:        "number",
				Description: "Maximum number of results to return (default: 5, max: 10)",
				Required:    false,
			},
		},
		RequiresMemoryService: true,
	})

	// Get Conversation Summary Tool
	r.RegisterTool(ToolDefinition{
		Name:                  "get_conversation_summary",
		Description:           "Get a summary of what has been discussed in the current conversation session. Use this to provide context or when the user asks 'what have we talked about'.",
		Parameters:            []ToolParameter{},
		RequiresMemoryService: true,
	})
}

// RegisterTool registers a new tool
func (r *ChatToolsRegistry) RegisterTool(tool ToolDefinition) {
	r.tools[tool.Name] = tool
}

// GetAvailableTools returns tools that are currently available
func (r *ChatToolsRegistry) GetAvailableTools() []ToolDefinition {
	var available []ToolDefinition
	for _, tool := range r.tools {
		// Check if required services are available
		if tool.RequiresMemoryService && r.memoryService == nil {
			continue
		}
		available = append(available, tool)
	}
	return available
}

// BuildToolsPrompt generates the prompt section that describes available tools
func (r *ChatToolsRegistry) BuildToolsPrompt() string {
	tools := r.GetAvailableTools()
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(`
##TOOLS_AVAILABLE##
You have access to external tools. You MUST use them when appropriate.

**MANDATORY TOOL USE - You MUST call a tool when:**
- User asks about "latest", "current", "recent", "new", "today's", "2024", "2025" → use web_search
- User says "search", "look up", "find online", "google", "browse the web" → use web_search  
- User asks to "ground", "verify", "cite sources", "find references" → use web_search
- User wants news, updates, releases, version info, announcements → use web_search
- User provides a URL and wants content → use web_scrape
- User asks to "read this page", "scrape", "extract from URL" → use web_scrape

**HOW TO CALL A TOOL:**
Output EXACTLY this format (nothing before it):

##TOOL_CALL##
{"tool": "tool_name", "arguments": {"param": "value"}}
##END_TOOL_CALL##

**EXAMPLE - User asks "What's the latest Python version?":**
##TOOL_CALL##
{"tool": "web_search", "arguments": {"query": "latest Python version release 2025"}}
##END_TOOL_CALL##

**RULES:**
1. When triggers match above, ALWAYS output ##TOOL_CALL## block FIRST
2. Do NOT answer from memory for time-sensitive questions - search first
3. After receiving tool results, provide a comprehensive answer citing the sources
4. If no tool is needed, just answer normally without any ##TOOL_CALL## block

**AVAILABLE TOOLS:**
`)

	for i, tool := range tools {
		sb.WriteString(fmt.Sprintf("\n%d. **%s**\n", i+1, tool.Name))
		sb.WriteString(fmt.Sprintf("   Description: %s\n", tool.Description))

		if len(tool.Parameters) > 0 {
			sb.WriteString("   Parameters:\n")
			for _, param := range tool.Parameters {
				requiredStr := ""
				if param.Required {
					requiredStr = " (required)"
				}
				sb.WriteString(fmt.Sprintf("   - %s (%s)%s: %s\n",
					param.Name, param.Type, requiredStr, param.Description))
				if len(param.Enum) > 0 {
					sb.WriteString(fmt.Sprintf("     Allowed values: %s\n", strings.Join(param.Enum, ", ")))
				}
			}
		} else {
			sb.WriteString("   Parameters: None\n")
		}
	}

	sb.WriteString(`
---
END OF TOOLS
---
`)

	return sb.String()
}

// ParseToolCall extracts a tool call from AI response text
func (r *ChatToolsRegistry) ParseToolCall(response string) (*ToolCall, string, error) {
	var matchedPattern *regexp.Regexp
	var matches []string

	// Try new delimiter format first: ##TOOL_CALL## ... ##END_TOOL_CALL##
	newPattern := regexp.MustCompile("(?s)##TOOL_CALL##\\s*\\n?(.+?)\\n?##END_TOOL_CALL##")
	matches = newPattern.FindStringSubmatch(response)
	if len(matches) >= 2 {
		matchedPattern = newPattern
	}

	// Fallback to old format: ```tool_call ... ```
	if matchedPattern == nil {
		oldPattern := regexp.MustCompile("(?s)```tool_call\\s*\\n?(.+?)\\n?```")
		matches = oldPattern.FindStringSubmatch(response)
		if len(matches) >= 2 {
			matchedPattern = oldPattern
		}
	}

	if matchedPattern == nil {
		// No tool call found
		return nil, response, nil
	}

	jsonStr := strings.TrimSpace(matches[1])
	log.Printf("[Tools] Found tool call JSON: %s", jsonStr)

	// Parse the JSON
	var callData struct {
		Tool      string                 `json:"tool"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &callData); err != nil {
		return nil, response, fmt.Errorf("failed to parse tool call JSON: %w", err)
	}

	// Validate tool exists
	if _, exists := r.tools[callData.Tool]; !exists {
		return nil, response, fmt.Errorf("unknown tool: %s", callData.Tool)
	}

	toolCall := &ToolCall{
		Name:      callData.Tool,
		Arguments: callData.Arguments,
	}

	// Get remaining text (remove the tool call block from response)
	remainingText := strings.TrimSpace(matchedPattern.ReplaceAllString(response, ""))

	return toolCall, remainingText, nil
}

// ExecuteTool executes a tool and returns the result
func (r *ChatToolsRegistry) ExecuteTool(ctx context.Context, sessionID uint, call *ToolCall) *ToolResult {
	tool, exists := r.tools[call.Name]
	if !exists {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Unknown tool: %s", call.Name),
		}
	}

	// Validate required parameters
	for _, param := range tool.Parameters {
		if param.Required {
			if _, exists := call.Arguments[param.Name]; !exists {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("Missing required parameter: %s", param.Name),
				}
			}
		}
	}

	// Execute based on tool name
	switch call.Name {
	case "search_memory":
		return r.executeSearchMemory(ctx, sessionID, call.Arguments)
	case "get_conversation_summary":
		return r.executeGetConversationSummary(ctx, sessionID)
	case "web_search":
		return r.executeWebSearch(ctx, call.Arguments, r.userAPIKeys)
	case "web_scrape":
		return r.executeWebScrape(ctx, call.Arguments, r.userAPIKeys)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Tool execution not implemented: %s", call.Name),
		}
	}
}

// executeSearchMemory executes the search_memory tool
func (r *ChatToolsRegistry) executeSearchMemory(ctx context.Context, sessionID uint, args map[string]interface{}) *ToolResult {
	if r.memoryService == nil {
		return &ToolResult{
			Success: false,
			Error:   "Memory service not available",
		}
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &ToolResult{
			Success: false,
			Error:   "Invalid or missing 'query' parameter",
		}
	}

	limit := 5
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
		if limit > 10 {
			limit = 10
		}
		if limit < 1 {
			limit = 1
		}
	}

	results, err := r.memoryService.SearchMemory(ctx, sessionID, query, limit)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Search failed: %s", err.Error()),
		}
	}

	if len(results) == 0 {
		return &ToolResult{
			Success: true,
			Data: map[string]interface{}{
				"message": "No matching memories found for your search query.",
				"results": []interface{}{},
			},
		}
	}

	// Format results for the AI
	var formattedResults []map[string]interface{}
	for _, result := range results {
		formatted := map[string]interface{}{
			"type":      result.Type,
			"content":   result.Content,
			"timestamp": result.Timestamp.Format(time.RFC3339),
		}
		if result.Role != "" {
			formatted["role"] = result.Role
		}
		formattedResults = append(formattedResults, formatted)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"count":   len(results),
			"results": formattedResults,
		},
	}
}

// executeGetConversationSummary executes the get_conversation_summary tool
func (r *ChatToolsRegistry) executeGetConversationSummary(ctx context.Context, sessionID uint) *ToolResult {
	if r.memoryService == nil {
		return &ToolResult{
			Success: false,
			Error:   "Memory service not available",
		}
	}

	chatCtx, err := r.memoryService.GetContextForPrompt(ctx, sessionID)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to get context: %s", err.Error()),
		}
	}

	summary := map[string]interface{}{
		"recent_message_count": len(chatCtx.RecentMessages),
	}

	if len(chatCtx.CompactedContexts) > 0 {
		summary["previous_context_summaries"] = chatCtx.CompactedContexts
	}

	// Get last few messages as preview
	if len(chatCtx.RecentMessages) > 0 {
		previewCount := 5
		if len(chatCtx.RecentMessages) < previewCount {
			previewCount = len(chatCtx.RecentMessages)
		}
		var recentPreview []map[string]string
		for i := len(chatCtx.RecentMessages) - previewCount; i < len(chatCtx.RecentMessages); i++ {
			msg := chatCtx.RecentMessages[i]
			content := msg.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			recentPreview = append(recentPreview, map[string]string{
				"role":    msg.Role,
				"preview": content,
			})
		}
		summary["recent_messages_preview"] = recentPreview
	}

	return &ToolResult{
		Success: true,
		Data:    summary,
	}
}

// FormatToolResult formats a tool result for injection back into the conversation
func (r *ChatToolsRegistry) FormatToolResult(call *ToolCall, result *ToolResult) string {
	var sb strings.Builder

	sb.WriteString("\n---\n")
	sb.WriteString(fmt.Sprintf("TOOL RESULT for '%s':\n\n", call.Name))

	if result.Success {
		// Special formatting for web_search results to make citations easier
		if call.Name == "web_search" {
			if data, ok := result.Data.(map[string]interface{}); ok {
				if results, ok := data["results"].([]interface{}); ok {
					// Citation instructions FIRST (before results) so model sees them
					sb.WriteString("**IMPORTANT - CITATION FORMAT:**\n")
					sb.WriteString("When referencing these sources, you MUST use markdown links: [N](URL)\n")
					sb.WriteString("Example: The price is $100 [1](https://coinmarketcap.com/currencies/bitcoin/).\n\n")

					sb.WriteString("**Web Search Results:**\n\n")
					for i, r := range results {
						if item, ok := r.(map[string]interface{}); ok {
							title := item["title"]
							url := item["url"]
							content := item["content"]
							// Format with URL prominently for easy copy
							sb.WriteString(fmt.Sprintf("**[%d]** %v\n", i+1, title))
							sb.WriteString(fmt.Sprintf("Link: %v\n", url))
							sb.WriteString(fmt.Sprintf("Summary: %v\n\n", content))
						}
					}
					sb.WriteString("---\n")
					sb.WriteString("Cite using [N](URL) format. Example: According to [1](https://example.com)...\n")
				} else {
					// Fallback to JSON if structure is different
					resultJSON, err := json.MarshalIndent(result.Data, "", "  ")
					if err != nil {
						sb.WriteString(fmt.Sprintf("Data: %v\n", result.Data))
					} else {
						sb.WriteString(string(resultJSON))
						sb.WriteString("\n")
					}
				}
			} else {
				// Fallback to JSON
				resultJSON, err := json.MarshalIndent(result.Data, "", "  ")
				if err != nil {
					sb.WriteString(fmt.Sprintf("Data: %v\n", result.Data))
				} else {
					sb.WriteString(string(resultJSON))
					sb.WriteString("\n")
				}
			}
		} else {
			// Default JSON format for other tools
			resultJSON, err := json.MarshalIndent(result.Data, "", "  ")
			if err != nil {
				sb.WriteString(fmt.Sprintf("Data: %v\n", result.Data))
			} else {
				sb.WriteString(string(resultJSON))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString(fmt.Sprintf("ERROR: %s\n", result.Error))
	}

	sb.WriteString("---\n")
	sb.WriteString("Now use this information to respond to the user's query.\n")

	return sb.String()
}

// ToolsEnabled returns true if any tools are available
func (r *ChatToolsRegistry) ToolsEnabled() bool {
	return len(r.GetAvailableTools()) > 0
}
