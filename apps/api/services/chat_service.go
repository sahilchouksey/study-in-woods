package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"github.com/sahilchouksey/go-init-setup/utils/crypto"
	"gorm.io/gorm"
)

// ChatService handles chat operations with AI agents
type ChatService struct {
	db             *gorm.DB
	doClient       *digitalocean.Client
	contextService *ChatContextService
	memoryService  *ChatMemoryService
	toolsRegistry  *ChatToolsRegistry
	enableAI       bool
}

// NewChatService creates a new chat service
func NewChatService(db *gorm.DB) *ChatService {
	service := &ChatService{
		db:       db,
		enableAI: false,
	}

	// Initialize DigitalOcean client for AI features
	apiToken := os.Getenv("DIGITALOCEAN_TOKEN")
	if apiToken != "" {
		service.doClient = digitalocean.NewClient(digitalocean.Config{
			APIToken: apiToken,
		})
		service.enableAI = true
	} else {
		log.Println("Warning: DIGITALOCEAN_TOKEN not set. Chat features will be disabled.")
	}

	return service
}

// SetContextService sets the chat context service for syllabus injection
func (s *ChatService) SetContextService(contextService *ChatContextService) {
	s.contextService = contextService
}

// SetMemoryService sets the chat memory service for conversation memory
func (s *ChatService) SetMemoryService(memoryService *ChatMemoryService) {
	s.memoryService = memoryService
	// Initialize or update tools registry with memory service
	s.toolsRegistry = NewChatToolsRegistry(memoryService)
}

// GetMemoryService returns the memory service
func (s *ChatService) GetMemoryService() *ChatMemoryService {
	return s.memoryService
}

// GetToolsRegistry returns the tools registry
func (s *ChatService) GetToolsRegistry() *ChatToolsRegistry {
	return s.toolsRegistry
}

// AgentCredentials holds the deployment URL and API key for an agent
type AgentCredentials struct {
	DeploymentURL string
	APIKey        string
}

// getOrFetchAgentCredentials gets the deployment URL and API key for a subject's agent,
// fetching from DigitalOcean and caching if not already stored
func (s *ChatService) getOrFetchAgentCredentials(ctx context.Context, subject *model.Subject) (*AgentCredentials, error) {
	creds := &AgentCredentials{}

	// Get or fetch deployment URL
	if subject.AgentDeploymentURL != "" {
		creds.DeploymentURL = subject.AgentDeploymentURL
	} else {
		// Try to fetch from DigitalOcean
		log.Printf("Deployment URL not cached for subject %d, fetching from DigitalOcean...", subject.ID)
		fetchedURL, fetchErr := s.doClient.GetAgentDeploymentURL(ctx, subject.AgentUUID)
		if fetchErr != nil {
			return nil, fmt.Errorf("subject has no deployment URL and failed to fetch from DigitalOcean: %w", fetchErr)
		}
		creds.DeploymentURL = fetchedURL

		// Cache it in the database for future requests
		if cacheErr := s.db.Model(&model.Subject{}).Where("id = ?", subject.ID).
			Update("agent_deployment_url", creds.DeploymentURL).Error; cacheErr != nil {
			log.Printf("Warning: failed to cache deployment URL for subject %d: %v", subject.ID, cacheErr)
		} else {
			log.Printf("Cached deployment URL for subject %d", subject.ID)
			subject.AgentDeploymentURL = creds.DeploymentURL // Update in-memory object
		}
	}

	// Get or create API key
	if subject.AgentAPIKeyEncrypted != "" {
		// Decrypt the existing API key
		decryptedKey, decryptErr := crypto.DecryptAPIKeyFromStorage(subject.AgentAPIKeyEncrypted)
		if decryptErr != nil {
			return nil, fmt.Errorf("failed to decrypt agent API key: %w", decryptErr)
		}
		creds.APIKey = decryptedKey
	} else {
		// No API key stored - try to create one from DigitalOcean
		log.Printf("No API key stored for subject %d, creating new one from DigitalOcean...", subject.ID)
		apiKeyName := fmt.Sprintf("chat-service-%d-%d", subject.ID, time.Now().Unix())
		apiKeyResult, createErr := s.doClient.CreateAgentAPIKey(ctx, subject.AgentUUID, apiKeyName)
		if createErr != nil {
			return nil, fmt.Errorf("subject has no API key and failed to create from DigitalOcean: %w", createErr)
		}
		if apiKeyResult.SecretKey == "" {
			return nil, fmt.Errorf("DigitalOcean returned empty API key")
		}
		creds.APIKey = apiKeyResult.SecretKey

		// Encrypt and cache it in the database
		encryptedKey, encryptErr := crypto.EncryptAPIKeyForStorage(creds.APIKey)
		if encryptErr != nil {
			log.Printf("Warning: failed to encrypt API key for caching: %v", encryptErr)
		} else {
			if cacheErr := s.db.Model(&model.Subject{}).Where("id = ?", subject.ID).
				Update("agent_api_key_encrypted", encryptedKey).Error; cacheErr != nil {
				log.Printf("Warning: failed to cache API key for subject %d: %v", subject.ID, cacheErr)
			} else {
				log.Printf("Created and cached new API key for subject %d", subject.ID)
				subject.AgentAPIKeyEncrypted = encryptedKey // Update in-memory object
			}
		}
	}

	return creds, nil
}

// getDefaultSystemPrompt returns the default system prompt template
func (s *ChatService) getDefaultSystemPrompt(subjectName string) string {
	return fmt.Sprintf(`You are an expert AI tutor for the subject "%s". Your role is to help students understand concepts, answer questions, and provide explanations related to this subject.

CRITICAL INSTRUCTION - RESPONSE PRIORITIES:
1. FIRST PRIORITY: Always directly answer what the user is asking. If they ask to list, share, or show something specific (like previous year questions), DO THAT FIRST.
2. SECOND PRIORITY: Use the conversation context to understand what the user needs.
3. THIRD PRIORITY: Use knowledge base materials to support your answer.

IMPORTANT: When a user explicitly asks to "share", "list", "show", or "give me" specific items (like PYQs, questions, topics), your response should directly provide those items in a clear format - NOT solve or explain them unless asked.

Guidelines:
- Provide accurate, educational responses based on the subject matter
- Include relevant examples and explanations
- When citing information from course materials, clearly indicate the source
- Be encouraging and supportive while maintaining academic rigor
- If you're unsure about something, acknowledge it honestly
- Structure your responses clearly with appropriate formatting`, subjectName)
}

// buildSystemPromptWithSyllabus creates a system prompt that includes syllabus context
// If customPrompt is provided and non-empty, it uses that instead of the default
func (s *ChatService) buildSystemPromptWithSyllabus(ctx context.Context, subjectID uint, subjectName string, customPrompt string) string {
	var basePrompt string

	// Use custom prompt if provided, otherwise use default
	if strings.TrimSpace(customPrompt) != "" {
		// Replace {subject_name} placeholder in custom prompt if present
		basePrompt = strings.ReplaceAll(customPrompt, "{subject_name}", subjectName)
	} else {
		basePrompt = s.getDefaultSystemPrompt(subjectName)
	}

	// Try to get syllabus context if available
	if s.contextService != nil {
		syllabusCtx, err := s.contextService.GetSubjectSyllabusContext(ctx, subjectID)
		if err == nil && syllabusCtx != nil && syllabusCtx.FormattedText != "" {
			basePrompt += fmt.Sprintf(`

---
COURSE SYLLABUS CONTEXT:
%s
---

Use this syllabus information to provide more targeted and relevant responses. Reference specific units and topics when appropriate.`, syllabusCtx.FormattedText)
		}
	}

	return basePrompt
}

// CreateSessionRequest represents a request to create a chat session
type CreateSessionRequest struct {
	SubjectID   uint
	UserID      uint
	Title       string
	Description string
}

// CreateSession creates a new chat session
func (s *ChatService) CreateSession(ctx context.Context, req CreateSessionRequest) (*model.ChatSession, error) {
	// Verify subject exists and get agent UUID
	var subject model.Subject
	if err := s.db.First(&subject, req.SubjectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("subject not found")
		}
		return nil, fmt.Errorf("failed to fetch subject: %w", err)
	}

	// Auto-generate title if not provided
	title := req.Title
	if title == "" {
		title = fmt.Sprintf("Chat: %s - %s", subject.Name, time.Now().Format("Jan 2, 2006"))
	}

	// Create session
	session := model.ChatSession{
		SubjectID:   req.SubjectID,
		UserID:      req.UserID,
		Title:       title,
		Description: req.Description,
		AgentUUID:   subject.AgentUUID,
		Status:      "active",
	}

	if err := s.db.Create(&session).Error; err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Preload relationships
	if err := s.db.Preload("Subject").Preload("User").First(&session, session.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to load session details: %w", err)
	}

	return &session, nil
}

// AISettings represents user-configurable AI settings from the client
type AISettings struct {
	SystemPrompt     string // Custom system prompt (empty = use default)
	IncludeCitations *bool  // Whether to include citations (nil = default true)
	MaxTokens        *int   // Max response tokens (nil = default 2048)
}

// GetIncludeCitations returns the include citations setting with default
func (s *AISettings) GetIncludeCitations() bool {
	if s == nil || s.IncludeCitations == nil {
		return true // Default: include citations
	}
	return *s.IncludeCitations
}

// GetMaxTokens returns the max tokens setting with default
func (s *AISettings) GetMaxTokens() int {
	if s == nil || s.MaxTokens == nil {
		return 2048 // Default: 2048 tokens
	}
	// Clamp to valid range
	tokens := *s.MaxTokens
	if tokens < 256 {
		return 256
	}
	if tokens > 8192 {
		return 8192
	}
	return tokens
}

// Continuation detection patterns - keywords that indicate user wants to continue a partial response
// These are checked ONLY when there's a partial message in the session
var continuationKeywords = []string{
	// English - direct continuation requests
	"continue",
	"go on",
	"keep going",
	"carry on",
	"proceed",
	"resume",
	"continue from where you left off",
	"please continue",
	"continue please",
	"pls continue",
	"plz continue",
	"keep writing",
	"finish",
	"finish it",
	"finish this",
	"complete",
	"complete it",
	"complete this",
	"finish your response",
	"complete your response",
	"continue your response",
	"continue writing",

	// English - short prompts
	"more",
	"more please",
	"go ahead",
	"yes continue",
	"yes go on",
	"yes",
	"ok continue",
	"okay continue",
	"and",
	"and then",
	"then",
	"next",
	"what's next",
	"what next",
	"whats next",

	// English - questions about continuation
	"what were you saying",
	"you were saying",
	"you got cut off",
	"it got cut off",
	"response got cut off",
	"message got cut off",
	"timed out",
	"it timed out",

	// Hindi
	"जारी रखें", // continue
	"आगे बढ़ें", // go ahead
	"आगे",       // ahead/next
	"और",        // and/more
	"फिर",       // then
	"पूरा करें", // complete it
	"खत्म करें", // finish it

	// Common typos/variations
	"continu",
	"contineu",
	"coninue",
	"countinue",
}

// isContinuationRequest checks if the user message is requesting continuation of a previous response
// This returns true for short messages that match continuation keywords
// The actual continuation only happens if there's a partial message in the session
func isContinuationRequest(content string) bool {
	normalizedContent := strings.ToLower(strings.TrimSpace(content))

	// Only check short messages (< 100 chars) to avoid false positives
	// Long messages are likely new questions even if they contain "continue"
	if len(normalizedContent) > 100 {
		return false
	}

	// Check for exact or near-exact matches
	for _, keyword := range continuationKeywords {
		keyword = strings.ToLower(keyword)

		// Exact match
		if normalizedContent == keyword {
			return true
		}

		// Match with punctuation
		if normalizedContent == keyword+"." ||
			normalizedContent == keyword+"?" ||
			normalizedContent == keyword+"!" ||
			normalizedContent == keyword+"..." ||
			normalizedContent == keyword+".." {
			return true
		}

		// Match at start/end with space
		if strings.HasPrefix(normalizedContent, keyword+" ") ||
			strings.HasSuffix(normalizedContent, " "+keyword) {
			return true
		}

		// Match with "please" variations
		if normalizedContent == keyword+" please" ||
			normalizedContent == "please "+keyword ||
			normalizedContent == keyword+" pls" ||
			normalizedContent == "pls "+keyword {
			return true
		}
	}

	return false
}

// getLastPartialMessage finds the most recent partial assistant message in a session
func (s *ChatService) getLastPartialMessage(sessionID uint) (*model.ChatMessage, error) {
	var partialMsg model.ChatMessage
	err := s.db.Where("session_id = ? AND role = ? AND status = ?",
		sessionID, model.MessageRoleAssistant, model.MessageStatusPartial).
		Order("created_at DESC").
		First(&partialMsg).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No partial message found
		}
		return nil, err
	}
	return &partialMsg, nil
}

// buildContinuationPrompt creates a prompt that includes the partial content for continuation
func buildContinuationPrompt(partialContent string) string {
	return fmt.Sprintf(`The previous response was cut off due to a timeout. Here is what was generated so far:

---BEGIN PARTIAL RESPONSE---
%s
---END PARTIAL RESPONSE---

Please continue from EXACTLY where this left off. Do NOT repeat any content that was already provided above. Simply continue the response naturally from the last word/sentence.`, partialContent)
}

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	SessionID uint
	UserID    uint
	Content   string
	Settings  *AISettings // Optional AI settings from client
}

// SendMessageResponse represents the response from sending a message
type SendMessageResponse struct {
	UserMessage      *model.ChatMessage
	AssistantMessage *model.ChatMessage
	Error            error
}

// SendMessage sends a message and gets AI response (non-streaming)
func (s *ChatService) SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error) {
	result := &SendMessageResponse{}

	// Get session with subject
	var session model.ChatSession
	if err := s.db.Preload("Subject").First(&session, req.SessionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to fetch session: %w", err)
	}

	// Verify user owns the session
	if session.UserID != req.UserID {
		return nil, fmt.Errorf("unauthorized: session does not belong to user")
	}

	// Check if AI is enabled
	if !s.enableAI || session.AgentUUID == "" {
		return nil, fmt.Errorf("AI chat is not available for this subject")
	}

	// Get or fetch agent credentials (deployment URL and API key)
	agentCreds, err := s.getOrFetchAgentCredentials(ctx, &session.Subject)
	if err != nil {
		return nil, err
	}

	// Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			result.Error = fmt.Errorf("panic during message send: %v", r)
		}
	}()

	// Save user message
	userMessage := model.ChatMessage{
		SessionID: req.SessionID,
		SubjectID: session.SubjectID,
		UserID:    req.UserID,
		Role:      model.MessageRoleUser,
		Content:   req.Content,
	}

	if err := tx.Create(&userMessage).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}
	result.UserMessage = &userMessage

	// Commit user message first to ensure it's in the history
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit user message: %w", err)
	}

	// Record user message in memory service (for batch tracking)
	if s.memoryService != nil {
		if _, err := s.memoryService.RecordMessage(ctx, req.SessionID, userMessage.ID); err != nil {
			log.Printf("Warning: failed to record user message in memory: %v", err)
		}
	}

	// Build messages for AI using memory service (includes compacted contexts)
	systemPrompt := s.buildSystemPromptWithSyllabus(ctx, session.SubjectID, session.Subject.Name, "")
	var messages []digitalocean.ChatMessage

	if s.memoryService != nil {
		// Use memory service to build messages with compacted contexts
		messages, err = s.memoryService.BuildMessagesForAI(ctx, req.SessionID, systemPrompt)
		if err != nil {
			log.Printf("Warning: failed to build messages from memory service, falling back: %v", err)
			messages = nil
		}
	}

	// Fallback to simple history if memory service not available or failed
	if messages == nil {
		var history []model.ChatMessage
		if err := s.db.Where("session_id = ?", req.SessionID).
			Order("created_at ASC").
			Limit(20).
			Find(&history).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch conversation history: %w", err)
		}

		messages = append(messages, digitalocean.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})

		for _, msg := range history {
			messages = append(messages, digitalocean.ChatMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}

	// Inject tools into system prompt if tools registry is available
	if s.toolsRegistry != nil && s.toolsRegistry.ToolsEnabled() {
		toolsPrompt := s.toolsRegistry.BuildToolsPrompt()
		// Find and update system message
		for i, msg := range messages {
			if msg.Role == "system" {
				messages[i].Content = msg.Content + "\n" + toolsPrompt
				break
			}
		}
	}

	// Call AI agent using deployment URL and API key
	startTime := time.Now()
	aiReq := digitalocean.AgentChatRequest{
		DeploymentURL:        agentCreds.DeploymentURL,
		APIKey:               agentCreds.APIKey,
		Messages:             messages,
		MaxTokens:            8192, // Increased max output tokens for longer responses
		IncludeRetrievalInfo: true,
		ProvideCitations:     true,
	}

	aiResp, err := s.doClient.CreateAgentChatCompletion(ctx, aiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI response: %w", err)
	}

	// Handle tool calls if present in response (parsed from content)
	if s.toolsRegistry != nil {
		aiResp, err = s.handlePromptBasedToolCalls(ctx, req.SessionID, aiReq, aiResp, agentCreds)
		if err != nil {
			log.Printf("Warning: tool handling failed: %v", err)
			// Continue with original response
		}
	}

	responseTime := time.Since(startTime).Milliseconds()

	// Extract content and usage
	content := aiResp.ExtractContent()
	_, _, totalTokens := aiResp.GetUsage()

	// Save assistant message in new transaction
	tx = s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	assistantMessage := model.ChatMessage{
		SessionID:    req.SessionID,
		SubjectID:    session.SubjectID,
		UserID:       req.UserID,
		Role:         model.MessageRoleAssistant,
		Content:      content,
		TokensUsed:   totalTokens,
		ModelUsed:    aiResp.Model,
		ResponseTime: int(responseTime),
		IsStreamed:   false,
	}

	if err := tx.Create(&assistantMessage).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to save assistant message: %w", err)
	}
	result.AssistantMessage = &assistantMessage

	// Update session statistics
	now := time.Now()
	if err := tx.Model(&session).Updates(map[string]interface{}{
		"message_count":   gorm.Expr("message_count + ?", 2),
		"total_tokens":    gorm.Expr("total_tokens + ?", totalTokens),
		"last_message_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Record assistant message in memory service
	if s.memoryService != nil {
		if _, err := s.memoryService.RecordMessage(ctx, req.SessionID, assistantMessage.ID); err != nil {
			log.Printf("Warning: failed to record assistant message in memory: %v", err)
		}
	}

	return result, nil
}

// SendMessageWithKeys sends a non-streaming message with user API keys for web tools
func (s *ChatService) SendMessageWithKeys(ctx context.Context, req SendMessageRequest, keys *UserAPIKeys) (*SendMessageResponse, error) {
	if keys != nil && keys.HasAnyKey() {
		log.Printf("[Tools] Creating tools registry with user keys for non-streaming - Tavily: %v, Exa: %v, Firecrawl: %v",
			keys.TavilyKey != "", keys.ExaKey != "", keys.FirecrawlKey != "")
		originalRegistry := s.toolsRegistry
		s.toolsRegistry = NewChatToolsRegistryWithKeys(s.memoryService, keys)
		defer func() { s.toolsRegistry = originalRegistry }()
	}
	return s.SendMessage(ctx, req)
}

// handlePromptBasedToolCalls parses tool calls from AI response content and executes them
// This is used for models that don't support native tool calling API
func (s *ChatService) handlePromptBasedToolCalls(ctx context.Context, sessionID uint, originalReq digitalocean.AgentChatRequest, aiResp *digitalocean.ChatCompletionResponse, creds *AgentCredentials) (*digitalocean.ChatCompletionResponse, error) {
	content := aiResp.ExtractContent()

	// Try to parse a tool call from the response
	toolCall, remainingText, err := s.toolsRegistry.ParseToolCall(content)
	if err != nil {
		log.Printf("[Tools] Warning: failed to parse tool call: %v", err)
		return aiResp, nil
	}

	if toolCall == nil {
		// No tool call found, return original response
		return aiResp, nil
	}

	log.Printf("[Tools] Detected tool call: %s with args: %v", toolCall.Name, toolCall.Arguments)

	// Execute the tool
	result := s.toolsRegistry.ExecuteTool(ctx, sessionID, toolCall)
	log.Printf("[Tools] Tool execution completed: success=%v", result.Success)

	// Format the tool result
	toolResultText := s.toolsRegistry.FormatToolResult(toolCall, result)

	// Add assistant message (with tool call) and tool result to messages
	assistantMsg := digitalocean.ChatMessage{
		Role:    "assistant",
		Content: content,
	}
	originalReq.Messages = append(originalReq.Messages, assistantMsg)

	// Add tool result as a user message (since ipython role may not be supported)
	toolResultMsg := digitalocean.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("Tool result for %s:\n%s\n\nNow please provide your final answer based on this information.", toolCall.Name, toolResultText),
	}
	originalReq.Messages = append(originalReq.Messages, toolResultMsg)

	// Make another request with tool results to get final response
	log.Printf("[Tools] Making follow-up request after tool execution")
	finalResp, err := s.doClient.CreateAgentChatCompletion(ctx, originalReq)
	if err != nil {
		// If follow-up fails, return original response with tool result appended
		log.Printf("[Tools] Warning: follow-up request failed: %v, returning partial response", err)

		// Modify the original response to include what we have
		if len(aiResp.Choices) > 0 {
			aiResp.Choices[0].Message.Content = remainingText + "\n\n**Tool Result:**\n" + toolResultText
		}
		return aiResp, nil
	}

	log.Printf("[Tools] Follow-up response received successfully")
	return finalResp, nil
}

// EnhancedStreamMessageRequest represents a request to stream with enhanced events
type EnhancedStreamMessageRequest struct {
	SessionID uint
	UserID    uint
	Content   string
	Settings  *AISettings // Optional AI settings from client
}

// ToolEvent represents a tool-related event during streaming
type ToolEvent struct {
	Type      string      `json:"type"`      // "tool_start", "tool_end", "tool_error"
	ToolName  string      `json:"tool_name"` // Name of the tool
	Arguments interface{} `json:"arguments"` // Tool arguments (for tool_start)
	Result    interface{} `json:"result"`    // Tool result (for tool_end)
	Success   bool        `json:"success"`   // Whether tool succeeded
	Error     string      `json:"error"`     // Error message if any
}

// PartialMessageInfo contains information about a partial (incomplete) message
type PartialMessageInfo struct {
	MessageID      uint   `json:"message_id"`      // ID of the saved partial message
	PartialContent string `json:"partial_content"` // Content accumulated before error
	Reason         string `json:"reason"`          // Why the message is partial (e.g., "timeout")
	ErrorType      string `json:"error_type"`      // Type of error that occurred
	ErrorMessage   string `json:"error_message"`   // Human-readable error message
	CanContinue    bool   `json:"can_continue"`    // Whether the message can be continued
	ChunkCount     int    `json:"chunk_count"`     // Number of chunks received before error
}

// EnhancedStreamCallbacks holds callbacks for different event types
type EnhancedStreamCallbacks struct {
	OnReasoning func(chunk string) error                    // Called for reasoning/thinking content
	OnContent   func(chunk string) error                    // Called for actual response content
	OnCitations func(citations []model.Citation) error      // Called when citations are available
	OnUsage     func(usage *digitalocean.StreamUsage) error // Called with token usage
	OnToolEvent func(event ToolEvent) error                 // Called for tool-related events
	OnPartial   func(info PartialMessageInfo) error         // Called when a partial message is saved due to timeout/error
}

// StreamMessageEnhanced streams a message with separate callbacks for reasoning and content
func (s *ChatService) StreamMessageEnhanced(ctx context.Context, req EnhancedStreamMessageRequest, callbacks EnhancedStreamCallbacks) (*SendMessageResponse, error) {
	result := &SendMessageResponse{}

	// Get session with subject
	var session model.ChatSession
	if err := s.db.Preload("Subject").First(&session, req.SessionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to fetch session: %w", err)
	}

	// Verify user owns the session
	if session.UserID != req.UserID {
		return nil, fmt.Errorf("unauthorized: session does not belong to user")
	}

	// Check if AI is enabled
	if !s.enableAI || session.AgentUUID == "" {
		return nil, fmt.Errorf("AI chat is not available for this subject")
	}

	// Get or fetch agent credentials
	agentCreds, err := s.getOrFetchAgentCredentials(ctx, &session.Subject)
	if err != nil {
		return nil, err
	}

	// Save user message
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	userMessage := model.ChatMessage{
		SessionID: req.SessionID,
		SubjectID: session.SubjectID,
		UserID:    req.UserID,
		Role:      model.MessageRoleUser,
		Content:   req.Content,
	}

	if err := tx.Create(&userMessage).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}
	result.UserMessage = &userMessage

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit user message: %w", err)
	}

	// Record user message in memory service
	if s.memoryService != nil {
		if _, err := s.memoryService.RecordMessage(ctx, req.SessionID, userMessage.ID); err != nil {
			log.Printf("Warning: failed to record user message in memory: %v", err)
		}
	}

	// Check if this is a continuation request
	var continuationContext string
	var continuingFromMsg *model.ChatMessage
	if isContinuationRequest(req.Content) {
		partialMsg, partialErr := s.getLastPartialMessage(req.SessionID)
		if partialErr != nil {
			log.Printf("Warning: failed to check for partial message: %v", partialErr)
		} else if partialMsg != nil {
			log.Printf("[Continuation] Detected continuation request for partial message ID=%d (content: %d chars)",
				partialMsg.ID, len(partialMsg.Content))
			continuationContext = buildContinuationPrompt(partialMsg.Content)
			continuingFromMsg = partialMsg
		}
	}

	// Build messages for AI
	customPrompt := ""
	if req.Settings != nil {
		customPrompt = req.Settings.SystemPrompt
	}
	systemPrompt := s.buildSystemPromptWithSyllabus(ctx, session.SubjectID, session.Subject.Name, customPrompt)
	var messages []digitalocean.ChatMessage

	if s.memoryService != nil {
		messages, err = s.memoryService.BuildMessagesForAI(ctx, req.SessionID, systemPrompt)
		if err != nil {
			log.Printf("Warning: failed to build messages from memory service, falling back: %v", err)
			messages = nil
		}
	}

	if messages == nil {
		var history []model.ChatMessage
		if err := s.db.Where("session_id = ?", req.SessionID).
			Order("created_at ASC").
			Limit(20).
			Find(&history).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch conversation history: %w", err)
		}

		messages = append(messages, digitalocean.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})

		for _, msg := range history {
			messages = append(messages, digitalocean.ChatMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}

	// If this is a continuation, inject the continuation context
	if continuationContext != "" {
		// Replace the last user message (which is just "continue") with the continuation prompt
		if len(messages) > 0 && messages[len(messages)-1].Role == "user" {
			messages[len(messages)-1].Content = continuationContext
			log.Printf("[Continuation] Injected continuation context into user message")
		}
	}

	// Inject tools into system prompt if tools registry is available
	if s.toolsRegistry != nil && s.toolsRegistry.ToolsEnabled() {
		toolsPrompt := s.toolsRegistry.BuildToolsPrompt()
		log.Printf("[Tools] Injecting tools prompt (%d chars) into streaming request", len(toolsPrompt))
		// Find and update system message
		for i, msg := range messages {
			if msg.Role == "system" {
				messages[i].Content = msg.Content + "\n" + toolsPrompt
				break
			}
		}
	}

	// Stream AI response
	startTime := time.Now()
	var fullContent strings.Builder
	var fullReasoning strings.Builder
	var retrievals []digitalocean.RetrievalInfo
	var totalTokens int
	var chunkCount int // Track chunks for partial response info

	aiReq := digitalocean.AgentChatRequest{
		DeploymentURL:        agentCreds.DeploymentURL,
		APIKey:               agentCreds.APIKey,
		Messages:             messages,
		MaxTokens:            8192,
		IncludeRetrievalInfo: true,
		ProvideCitations:     true,
		StreamOptions:        &digitalocean.StreamOptions{IncludeUsage: true},
	}

	// State for filtering tool call chunks
	// We buffer content when we might be inside a tool call block
	var contentBuffer strings.Builder
	inToolCall := false
	const toolCallStart = "##TOOL_CALL##"
	const toolCallEnd = "##END_TOOL_CALL##"

	streamErr := s.doClient.StreamAgentChatCompletion(ctx, aiReq, func(chunk digitalocean.StreamChunk) error {
		// Handle reasoning content in real-time
		if reasoningContent := chunk.GetReasoningContent(); reasoningContent != "" {
			fullReasoning.WriteString(reasoningContent)
			if callbacks.OnReasoning != nil {
				if err := callbacks.OnReasoning(reasoningContent); err != nil {
					return err
				}
			}
		}

		// Handle actual content in real-time
		// We accumulate content and filter out ##TOOL_CALL##...##END_TOOL_CALL## blocks
		// The filtering happens by buffering until we can determine if content is part of a tool call
		if content := chunk.GetContent(); content != "" {
			// Always accumulate full content for internal processing (used for tool detection later)
			fullContent.WriteString(content)
			chunkCount++ // Track chunk count for partial response info

			// Buffer content and check for tool call markers
			contentBuffer.WriteString(content)
			bufferStr := contentBuffer.String()

			// If we're inside a tool call block, accumulate until we find the end
			if inToolCall {
				if strings.Contains(bufferStr, toolCallEnd) {
					// Tool call complete - extract and send any content after the end marker
					endIdx := strings.Index(bufferStr, toolCallEnd)
					afterToolCall := bufferStr[endIdx+len(toolCallEnd):]
					contentBuffer.Reset()
					inToolCall = false

					// Send content after the tool call if any
					if len(strings.TrimSpace(afterToolCall)) > 0 {
						if callbacks.OnContent != nil {
							if err := callbacks.OnContent(afterToolCall); err != nil {
								return err
							}
						}
					}
				}
				// Still inside tool call - don't emit the tool call text
				return nil
			}

			// Check if we have the complete tool call start marker
			if strings.Contains(bufferStr, toolCallStart) {
				startIdx := strings.Index(bufferStr, toolCallStart)

				// Send any content BEFORE the tool call marker
				beforeToolCall := bufferStr[:startIdx]
				if callbacks.OnContent != nil && len(strings.TrimSpace(beforeToolCall)) > 0 {
					if err := callbacks.OnContent(beforeToolCall); err != nil {
						return err
					}
				}

				// Check if the complete tool call is in the buffer
				if strings.Contains(bufferStr, toolCallEnd) {
					// Complete tool call - extract content after it
					endIdx := strings.Index(bufferStr, toolCallEnd)
					afterToolCall := bufferStr[endIdx+len(toolCallEnd):]
					contentBuffer.Reset()

					// Send content after the tool call if any
					if len(strings.TrimSpace(afterToolCall)) > 0 {
						if callbacks.OnContent != nil {
							if err := callbacks.OnContent(afterToolCall); err != nil {
								return err
							}
						}
					}
				} else {
					// Tool call started but not complete - enter tool call mode
					inToolCall = true
					contentBuffer.Reset()
					// Keep the partial tool call in buffer for detection
					contentBuffer.WriteString(bufferStr[startIdx:])
				}
				return nil
			}

			// Check if the buffer ENDS with a partial tool call marker
			// e.g., "#", "##", "##T", "##TO", etc.
			// We need to find the LONGEST matching suffix (not shortest) to know where safe content ends
			longestMatchLen := 0
			for i := 1; i <= len(toolCallStart) && i <= len(bufferStr); i++ {
				suffix := bufferStr[len(bufferStr)-i:]
				if strings.HasPrefix(toolCallStart, suffix) {
					longestMatchLen = i // Keep updating to find longest match
				}
			}

			if longestMatchLen > 0 {
				// Buffer ends with potential start of marker
				// Send everything BEFORE the potential marker start
				safeEnd := len(bufferStr) - longestMatchLen

				if safeEnd > 0 {
					safeContent := bufferStr[:safeEnd]
					if callbacks.OnContent != nil {
						if err := callbacks.OnContent(safeContent); err != nil {
							return err
						}
					}
				}
				// Keep only the potential marker part in buffer
				contentBuffer.Reset()
				contentBuffer.WriteString(bufferStr[safeEnd:])
				return nil
			}

			// No tool call concerns - send everything in buffer
			if callbacks.OnContent != nil {
				if err := callbacks.OnContent(bufferStr); err != nil {
					return err
				}
			}
			contentBuffer.Reset()
		}

		// Capture retrievals
		allRetrievals := chunk.GetAllRetrievals()
		if len(allRetrievals) > 0 {
			retrievals = append(retrievals, allRetrievals...)
		}

		// Capture usage
		if chunk.Usage != nil {
			totalTokens = chunk.Usage.TotalTokens
			if callbacks.OnUsage != nil {
				if err := callbacks.OnUsage(chunk.Usage); err != nil {
					return err
				}
			}
		}

		return nil
	})

	// Handle streaming errors - check for timeout and save partial content if available
	if streamErr != nil {
		partialContent := fullContent.String()
		responseTime := time.Since(startTime).Milliseconds()

		// Check if we have any content to save as partial
		if len(partialContent) > 0 {
			// Determine error type
			errorType := "unknown"
			errorMessage := streamErr.Error()
			if digitalocean.IsTimeoutError(streamErr) {
				errorType = "timeout"
				errorMessage = "Response was cut off due to timeout. You can continue this response."
			} else if strings.Contains(streamErr.Error(), "connection") {
				errorType = "connection"
				errorMessage = "Connection was interrupted. You can continue this response."
			}

			log.Printf("[Chat] Stream error with partial content (%d chars, %d chunks): %v", len(partialContent), chunkCount, streamErr)

			// Convert retrievals to citations for the partial message
			var citations model.Citations
			for _, r := range retrievals {
				filename := r.FileName
				if filename == "" {
					filename = r.Source
				}
				if filename == "" {
					filename = r.SourceName
				}
				citations = append(citations, model.Citation{
					ID:          r.ID,
					Filename:    filename,
					PageContent: r.Content,
					Score:       r.Score,
				})
			}

			// Save partial assistant message
			tx := s.db.Begin()
			if tx.Error == nil {
				partialMessage := model.ChatMessage{
					SessionID:    req.SessionID,
					SubjectID:    session.SubjectID,
					UserID:       req.UserID,
					Role:         model.MessageRoleAssistant,
					Content:      partialContent,
					TokensUsed:   totalTokens,
					ResponseTime: int(responseTime),
					IsStreamed:   true,
					Citations:    citations,
					Status:       model.MessageStatusPartial,
					ErrorType:    errorType,
					ErrorMessage: errorMessage,
				}

				// Store reasoning in metadata if any
				if fullReasoning.Len() > 0 {
					partialMessage.Metadata = make(model.JSONMap)
					partialMessage.Metadata["reasoning"] = fullReasoning.String()
				}

				if err := tx.Create(&partialMessage).Error; err != nil {
					tx.Rollback()
					log.Printf("[Chat] Failed to save partial message: %v", err)
				} else {
					// Update session statistics
					now := time.Now()
					tx.Model(&session).Updates(map[string]interface{}{
						"message_count":   gorm.Expr("message_count + ?", 2), // user + partial assistant
						"total_tokens":    gorm.Expr("total_tokens + ?", totalTokens),
						"last_message_at": now,
					})

					if err := tx.Commit().Error; err != nil {
						log.Printf("[Chat] Failed to commit partial message: %v", err)
					} else {
						log.Printf("[Chat] Saved partial message ID=%d with %d chars", partialMessage.ID, len(partialContent))

						// Record in memory service
						if s.memoryService != nil {
							s.memoryService.RecordMessage(ctx, req.SessionID, partialMessage.ID)
						}

						// Call OnPartial callback to notify client
						if callbacks.OnPartial != nil {
							partialInfo := PartialMessageInfo{
								MessageID:      partialMessage.ID,
								PartialContent: partialContent,
								Reason:         errorType,
								ErrorType:      errorType,
								ErrorMessage:   errorMessage,
								CanContinue:    true,
								ChunkCount:     chunkCount,
							}
							if err := callbacks.OnPartial(partialInfo); err != nil {
								log.Printf("[Chat] OnPartial callback error: %v", err)
							}
						}

						// Return partial response (not an error, but partial success)
						result.AssistantMessage = &partialMessage
						return result, nil
					}
				}
			}
		}

		// No partial content or failed to save - return error
		return nil, fmt.Errorf("failed to stream AI response: %w", streamErr)
	}

	responseTime := time.Since(startTime).Milliseconds()
	finalContent := fullContent.String()

	// Check for tool calls in streamed response and handle them
	if s.toolsRegistry != nil && s.toolsRegistry.ToolsEnabled() {
		toolCall, _, err := s.toolsRegistry.ParseToolCall(finalContent)
		if err != nil {
			log.Printf("[Tools] Warning: failed to parse tool call: %v", err)
		} else if toolCall != nil {
			log.Printf("[Tools] Detected tool call in stream: %s with args: %v", toolCall.Name, toolCall.Arguments)

			// Send tool_start event to client
			if callbacks.OnToolEvent != nil {
				callbacks.OnToolEvent(ToolEvent{
					Type:      "tool_start",
					ToolName:  toolCall.Name,
					Arguments: toolCall.Arguments,
				})
			}

			// Execute the tool
			toolResult := s.toolsRegistry.ExecuteTool(ctx, req.SessionID, toolCall)
			toolResultText := s.toolsRegistry.FormatToolResult(toolCall, toolResult)

			log.Printf("[Tools] Tool execution result: success=%v", toolResult.Success)

			// Send tool_end event to client with results
			if callbacks.OnToolEvent != nil {
				callbacks.OnToolEvent(ToolEvent{
					Type:     "tool_end",
					ToolName: toolCall.Name,
					Result:   toolResult.Data,
					Success:  toolResult.Success,
					Error:    toolResult.Error,
				})
			}

			// Make follow-up request with tool results to get final answer
			messages = append(messages, digitalocean.ChatMessage{
				Role:    "assistant",
				Content: finalContent,
			})
			messages = append(messages, digitalocean.ChatMessage{
				Role:    "user",
				Content: fmt.Sprintf("Tool result for %s:\n%s\n\nNow please provide your final answer based on this information.", toolCall.Name, toolResultText),
			})

			// Stream the follow-up response
			followUpReq := digitalocean.AgentChatRequest{
				DeploymentURL:        agentCreds.DeploymentURL,
				APIKey:               agentCreds.APIKey,
				Messages:             messages,
				MaxTokens:            8192,
				IncludeRetrievalInfo: true,
				ProvideCitations:     true,
				StreamOptions:        &digitalocean.StreamOptions{IncludeUsage: true},
			}

			var followUpContent strings.Builder
			followUpErr := s.doClient.StreamAgentChatCompletion(ctx, followUpReq, func(chunk digitalocean.StreamChunk) error {
				if content := chunk.GetContent(); content != "" {
					followUpContent.WriteString(content)
					if callbacks.OnContent != nil {
						return callbacks.OnContent(content)
					}
				}
				if chunk.Usage != nil {
					totalTokens += chunk.Usage.TotalTokens
				}
				return nil
			})

			if followUpErr != nil {
				log.Printf("[Tools] Warning: follow-up request failed: %v", followUpErr)
				// Use tool result as final content (don't include remainingText which might have partial tool call)
				finalContent = toolResultText
			} else {
				// Use only the follow-up content (the actual answer from AI)
				// Don't prepend remainingText as it might contain partial tool call text
				finalContent = followUpContent.String()
			}
			responseTime = time.Since(startTime).Milliseconds()
		}
	}

	// Convert retrievals to citations
	var citations model.Citations
	for _, r := range retrievals {
		filename := r.FileName
		if filename == "" {
			filename = r.Source
		}
		if filename == "" {
			filename = r.SourceName
		}
		citations = append(citations, model.Citation{
			ID:          r.ID,
			Filename:    filename,
			PageContent: r.Content,
			Score:       r.Score,
		})
	}

	// Send citations callback if available
	if len(citations) > 0 && callbacks.OnCitations != nil {
		if err := callbacks.OnCitations(citations); err != nil {
			log.Printf("Warning: citations callback error: %v", err)
		}
	}

	// Save assistant message
	tx = s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	assistantMessage := model.ChatMessage{
		SessionID:    req.SessionID,
		SubjectID:    session.SubjectID,
		UserID:       req.UserID,
		Role:         model.MessageRoleAssistant,
		Content:      finalContent, // Use finalContent which has the actual answer (not raw tool call)
		TokensUsed:   totalTokens,
		ResponseTime: int(responseTime),
		IsStreamed:   true,
		Citations:    citations,
	}

	// Store reasoning in metadata
	if fullReasoning.Len() > 0 {
		assistantMessage.Metadata = make(model.JSONMap)
		assistantMessage.Metadata["reasoning"] = fullReasoning.String()
	}

	if err := tx.Create(&assistantMessage).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to save assistant message: %w", err)
	}
	result.AssistantMessage = &assistantMessage

	// If this was a continuation, mark the original partial message as complete and link them
	if continuingFromMsg != nil {
		assistantMessage.ParentMessageID = &continuingFromMsg.ID
		if err := tx.Model(&assistantMessage).Update("parent_message_id", continuingFromMsg.ID).Error; err != nil {
			log.Printf("Warning: failed to link continuation message: %v", err)
		}

		// Mark the original partial message as complete
		if err := tx.Model(continuingFromMsg).Updates(map[string]interface{}{
			"status":        model.MessageStatusComplete,
			"error_type":    "",
			"error_message": "",
		}).Error; err != nil {
			log.Printf("Warning: failed to mark partial message as complete: %v", err)
		} else {
			log.Printf("[Continuation] Marked partial message ID=%d as complete, continuation ID=%d", continuingFromMsg.ID, assistantMessage.ID)
		}
	}

	// Update session statistics
	now := time.Now()
	if err := tx.Model(&session).Updates(map[string]interface{}{
		"message_count":   gorm.Expr("message_count + ?", 2),
		"total_tokens":    gorm.Expr("total_tokens + ?", totalTokens),
		"last_message_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Record assistant message in memory service
	if s.memoryService != nil {
		if _, err := s.memoryService.RecordMessage(ctx, req.SessionID, assistantMessage.ID); err != nil {
			log.Printf("Warning: failed to record assistant message in memory: %v", err)
		}
	}

	return result, nil
}

// StreamMessageEnhancedWithKeys streams with enhanced callbacks and user API keys
func (s *ChatService) StreamMessageEnhancedWithKeys(ctx context.Context, req EnhancedStreamMessageRequest, keys *UserAPIKeys, callbacks EnhancedStreamCallbacks) (*SendMessageResponse, error) {
	if keys != nil && keys.HasAnyKey() {
		log.Printf("[Tools] Creating tools registry with user keys - Tavily: %v, Exa: %v, Firecrawl: %v",
			keys.TavilyKey != "", keys.ExaKey != "", keys.FirecrawlKey != "")
		originalRegistry := s.toolsRegistry
		s.toolsRegistry = NewChatToolsRegistryWithKeys(s.memoryService, keys)
		defer func() { s.toolsRegistry = originalRegistry }()
	} else {
		log.Printf("[Tools] No user API keys provided for streaming request")
	}
	return s.StreamMessageEnhanced(ctx, req, callbacks)
}

// GetSessionMessages retrieves all messages for a session
func (s *ChatService) GetSessionMessages(ctx context.Context, sessionID uint, userID uint, limit int, offset int) ([]model.ChatMessage, int64, error) {
	// Verify session exists and belongs to user
	var session model.ChatSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, 0, fmt.Errorf("session not found")
		}
		return nil, 0, fmt.Errorf("failed to fetch session: %w", err)
	}

	if session.UserID != userID {
		return nil, 0, fmt.Errorf("unauthorized: session does not belong to user")
	}

	// Get total count
	var total int64
	if err := s.db.Model(&model.ChatMessage{}).
		Where("session_id = ?", sessionID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	// Get messages
	var messages []model.ChatMessage
	query := s.db.Where("session_id = ?", sessionID).
		Order("created_at ASC")

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Find(&messages).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch messages: %w", err)
	}

	return messages, total, nil
}

// DeleteSession deletes a chat session and all its messages
func (s *ChatService) DeleteSession(ctx context.Context, sessionID uint, userID uint) error {
	// Verify session exists and belongs to user
	var session model.ChatSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("session not found")
		}
		return fmt.Errorf("failed to fetch session: %w", err)
	}

	if session.UserID != userID {
		return fmt.Errorf("unauthorized: session does not belong to user")
	}

	// Delete session (cascade will delete messages)
	if err := s.db.Delete(&session).Error; err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// ArchiveSession archives a chat session
func (s *ChatService) ArchiveSession(ctx context.Context, sessionID uint, userID uint) error {
	// Verify session exists and belongs to user
	var session model.ChatSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("session not found")
		}
		return fmt.Errorf("failed to fetch session: %w", err)
	}

	if session.UserID != userID {
		return fmt.Errorf("unauthorized: session does not belong to user")
	}

	// Archive session
	session.Status = "archived"
	if err := s.db.Save(&session).Error; err != nil {
		return fmt.Errorf("failed to archive session: %w", err)
	}

	return nil
}
