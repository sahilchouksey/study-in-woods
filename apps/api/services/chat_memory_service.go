package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"gorm.io/gorm"
)

const (
	// BatchSize is the number of messages per batch
	BatchSize = 20
	// CompactionTrigger is when the current batch reaches this count, compact the previous batch
	CompactionTrigger = 10
	// MaxAIMessagePreviewLength is the max chars for AI messages in context
	MaxAIMessagePreviewLength = 200
	// MaxContextsInPrompt is how many compacted contexts to include in the prompt
	MaxContextsInPrompt = 2
)

// ChatMemoryService manages chat history, batching, and context compaction
type ChatMemoryService struct {
	db              *gorm.DB
	inferenceClient *digitalocean.InferenceClient
	enableAI        bool
}

// NewChatMemoryService creates a new chat memory service
func NewChatMemoryService(db *gorm.DB) *ChatMemoryService {
	service := &ChatMemoryService{
		db:       db,
		enableAI: false,
	}

	// Initialize inference client for context compaction
	inferenceAPIKey := os.Getenv("DO_INFERENCE_API_KEY")
	if inferenceAPIKey != "" {
		service.inferenceClient = digitalocean.NewInferenceClient(digitalocean.InferenceConfig{
			APIKey: inferenceAPIKey,
		})
		service.enableAI = true
	} else {
		log.Println("Warning: DO_INFERENCE_API_KEY not set. Context compaction will be disabled.")
	}

	return service
}

// GetOrCreateCurrentBatch gets or creates the current active batch for a session
func (s *ChatMemoryService) GetOrCreateCurrentBatch(ctx context.Context, sessionID uint) (*model.ChatMemoryBatch, error) {
	var batch model.ChatMemoryBatch

	// Try to find an active batch
	err := s.db.Where("session_id = ? AND status = ?", sessionID, model.BatchStatusActive).
		Order("batch_number DESC").
		First(&batch).Error

	if err == gorm.ErrRecordNotFound {
		// Create first batch
		batch = model.ChatMemoryBatch{
			SessionID:    sessionID,
			BatchNumber:  1,
			Status:       model.BatchStatusActive,
			MessageCount: 0,
		}
		if err := s.db.Create(&batch).Error; err != nil {
			return nil, fmt.Errorf("failed to create batch: %w", err)
		}
		return &batch, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch batch: %w", err)
	}

	return &batch, nil
}

// RecordMessage records a new message and handles batch management
// Returns true if a compaction was triggered
func (s *ChatMemoryService) RecordMessage(ctx context.Context, sessionID uint, messageID uint) (bool, error) {
	batch, err := s.GetOrCreateCurrentBatch(ctx, sessionID)
	if err != nil {
		return false, err
	}

	// Update batch with new message
	if batch.StartMsgID == 0 {
		batch.StartMsgID = messageID
	}
	batch.EndMsgID = messageID
	batch.MessageCount++

	compactionTriggered := false

	// Check if batch is full
	if batch.MessageCount >= BatchSize {
		batch.Status = model.BatchStatusComplete

		// Create new batch
		newBatch := model.ChatMemoryBatch{
			SessionID:    sessionID,
			BatchNumber:  batch.BatchNumber + 1,
			Status:       model.BatchStatusActive,
			MessageCount: 0,
		}
		if err := s.db.Create(&newBatch).Error; err != nil {
			return false, fmt.Errorf("failed to create new batch: %w", err)
		}
	}

	// Save current batch
	if err := s.db.Save(batch).Error; err != nil {
		return false, fmt.Errorf("failed to update batch: %w", err)
	}

	// Check if we need to compact the previous batch
	// Trigger compaction when current batch reaches CompactionTrigger messages
	if batch.MessageCount >= CompactionTrigger {
		prevBatch, err := s.GetPreviousBatch(ctx, sessionID, batch.BatchNumber)
		if err == nil && prevBatch != nil && prevBatch.Status == model.BatchStatusComplete {
			// Compact in background
			go func() {
				bgCtx := context.Background()
				if err := s.CompactBatch(bgCtx, prevBatch.ID); err != nil {
					log.Printf("Background compaction failed for batch %d: %v", prevBatch.ID, err)
				}
			}()
			compactionTriggered = true
		}
	}

	return compactionTriggered, nil
}

// GetPreviousBatch gets the batch before the given batch number
func (s *ChatMemoryService) GetPreviousBatch(ctx context.Context, sessionID uint, currentBatchNum int) (*model.ChatMemoryBatch, error) {
	if currentBatchNum <= 1 {
		return nil, nil
	}

	var batch model.ChatMemoryBatch
	err := s.db.Where("session_id = ? AND batch_number = ?", sessionID, currentBatchNum-1).
		First(&batch).Error
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

// CompactBatch compacts a batch into a summarized context
func (s *ChatMemoryService) CompactBatch(ctx context.Context, batchID uint) error {
	if !s.enableAI {
		return fmt.Errorf("AI not enabled for compaction")
	}

	var batch model.ChatMemoryBatch
	if err := s.db.First(&batch, batchID).Error; err != nil {
		return fmt.Errorf("batch not found: %w", err)
	}

	if batch.Status == model.BatchStatusCompacted {
		return nil // Already compacted
	}

	// Get messages in this batch
	var messages []model.ChatMessage
	if err := s.db.Where("session_id = ? AND id >= ? AND id <= ?",
		batch.SessionID, batch.StartMsgID, batch.EndMsgID).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	if len(messages) == 0 {
		return fmt.Errorf("no messages found in batch")
	}

	// Build conversation text for summarization
	var conversationBuilder strings.Builder
	for _, msg := range messages {
		role := strings.ToUpper(string(msg.Role))
		content := msg.Content
		if msg.Role == model.MessageRoleAssistant && len(content) > 500 {
			content = content[:500] + "..."
		}
		conversationBuilder.WriteString(fmt.Sprintf("[%s]: %s\n\n", role, content))
	}

	// Call AI to generate summary
	summary, err := s.generateContextSummary(ctx, conversationBuilder.String())
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Create compacted context
	compactedCtx := model.ChatCompactedContext{
		SessionID:     batch.SessionID,
		BatchID:       batch.ID,
		BatchNumber:   batch.BatchNumber,
		Summary:       summary.Summary,
		KeyTopics:     summary.KeyTopics,
		KeyEntities:   summary.KeyEntities,
		UserIntents:   summary.UserIntents,
		AIResponses:   summary.AIResponses,
		MessageRange:  fmt.Sprintf("%d-%d", batch.StartMsgID, batch.EndMsgID),
		OriginalCount: len(messages),
	}

	if err := s.db.Create(&compactedCtx).Error; err != nil {
		return fmt.Errorf("failed to create compacted context: %w", err)
	}

	// Update batch status
	now := time.Now()
	batch.Status = model.BatchStatusCompacted
	batch.CompactedAt = &now
	batch.ContextID = &compactedCtx.ID

	if err := s.db.Save(&batch).Error; err != nil {
		return fmt.Errorf("failed to update batch status: %w", err)
	}

	log.Printf("Compacted batch %d (messages %d-%d) into context %d",
		batch.ID, batch.StartMsgID, batch.EndMsgID, compactedCtx.ID)

	return nil
}

// ContextSummaryResult holds the AI-generated summary
type ContextSummaryResult struct {
	Summary     string   `json:"summary"`
	KeyTopics   []string `json:"key_topics"`
	KeyEntities []string `json:"key_entities"`
	UserIntents []string `json:"user_intents"`
	AIResponses []string `json:"ai_responses"`
}

// generateContextSummary uses AI to summarize a conversation batch
func (s *ChatMemoryService) generateContextSummary(ctx context.Context, conversation string) (*ContextSummaryResult, error) {
	systemPrompt := `You are a conversation summarizer. Your task is to analyze a conversation between a user and an AI assistant and create a structured summary.

Output ONLY valid JSON with this structure:
{
  "summary": "A 2-3 sentence summary of what was discussed",
  "key_topics": ["topic1", "topic2", "topic3"],
  "key_entities": ["entity1", "entity2"],
  "user_intents": ["what user wanted to learn/do"],
  "ai_responses": ["key points from AI responses"]
}

Guidelines:
- Summary should capture the main thread of conversation
- Key topics should be specific subject areas discussed
- Key entities include names, concepts, technical terms mentioned
- User intents capture what the user was trying to accomplish
- AI responses should highlight important information shared

Output ONLY the JSON object. No markdown, no explanation.`

	userPrompt := fmt.Sprintf("Summarize this conversation:\n\n%s", conversation)

	response, err := s.inferenceClient.JSONCompletion(
		ctx,
		systemPrompt,
		userPrompt,
		digitalocean.WithInferenceMaxTokens(1024),
		digitalocean.WithInferenceTemperature(0.3),
	)
	if err != nil {
		return nil, fmt.Errorf("AI summarization failed: %w", err)
	}

	var result ContextSummaryResult
	if err := parseJSONResponse(response, &result); err != nil {
		// Fallback to basic summary
		result = ContextSummaryResult{
			Summary:     "Conversation summary could not be parsed. Raw: " + truncateMemoryString(response, 200),
			KeyTopics:   []string{},
			KeyEntities: []string{},
			UserIntents: []string{},
			AIResponses: []string{},
		}
	}

	return &result, nil
}

// GetContextForPrompt builds the context to include in the AI prompt
// Returns: recent messages (current batch) + last 2 compacted contexts
func (s *ChatMemoryService) GetContextForPrompt(ctx context.Context, sessionID uint) (*ChatContext, error) {
	result := &ChatContext{
		RecentMessages:    []ChatContextMessage{},
		CompactedContexts: []string{},
	}

	// Get current batch
	batch, err := s.GetOrCreateCurrentBatch(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Get messages from current batch (last 20)
	var recentMessages []model.ChatMessage
	query := s.db.Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(BatchSize)

	if batch.StartMsgID > 0 {
		query = query.Where("id >= ?", batch.StartMsgID)
	}

	if err := query.Find(&recentMessages).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch recent messages: %w", err)
	}

	// Reverse to get chronological order
	for i := len(recentMessages) - 1; i >= 0; i-- {
		msg := recentMessages[i]
		content := msg.Content

		// Chop AI messages to MaxAIMessagePreviewLength
		if msg.Role == model.MessageRoleAssistant && len(content) > MaxAIMessagePreviewLength {
			content = content[:MaxAIMessagePreviewLength] + "..."
		}

		result.RecentMessages = append(result.RecentMessages, ChatContextMessage{
			Role:    string(msg.Role),
			Content: content,
		})
	}

	// Get last N compacted contexts
	var contexts []model.ChatCompactedContext
	if err := s.db.Where("session_id = ?", sessionID).
		Order("batch_number DESC").
		Limit(MaxContextsInPrompt).
		Find(&contexts).Error; err != nil {
		log.Printf("Failed to fetch compacted contexts: %v", err)
	} else {
		// Reverse to get chronological order
		for i := len(contexts) - 1; i >= 0; i-- {
			ctx := contexts[i]
			result.CompactedContexts = append(result.CompactedContexts,
				fmt.Sprintf("[Previous conversation summary (messages %s)]: %s",
					ctx.MessageRange, ctx.Summary))
		}
	}

	return result, nil
}

// ChatContext holds the context to be included in AI prompts
type ChatContext struct {
	RecentMessages    []ChatContextMessage `json:"recent_messages"`
	CompactedContexts []string             `json:"compacted_contexts"`
}

// ChatContextMessage is a simplified message for context
type ChatContextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BuildMessagesForAI builds the messages array for AI API calls
func (s *ChatMemoryService) BuildMessagesForAI(ctx context.Context, sessionID uint, systemPrompt string) ([]digitalocean.ChatMessage, error) {
	chatCtx, err := s.GetContextForPrompt(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var messages []digitalocean.ChatMessage

	// Add system prompt
	messages = append(messages, digitalocean.ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add compacted contexts as system context
	if len(chatCtx.CompactedContexts) > 0 {
		contextText := strings.Join(chatCtx.CompactedContexts, "\n\n")
		messages = append(messages, digitalocean.ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("CONVERSATION HISTORY SUMMARY:\n%s", contextText),
		})
	}

	// Add recent messages
	for _, msg := range chatCtx.RecentMessages {
		messages = append(messages, digitalocean.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return messages, nil
}

// SearchMemory searches through all conversation history (messages + compacted contexts)
func (s *ChatMemoryService) SearchMemory(ctx context.Context, sessionID uint, query string, limit int) ([]model.ChatMemorySearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	var results []model.ChatMemorySearchResult
	queryLower := strings.ToLower(query)

	// Search in compacted contexts first (most efficient)
	var contexts []model.ChatCompactedContext
	if err := s.db.Where("session_id = ?", sessionID).
		Order("batch_number DESC").
		Find(&contexts).Error; err != nil {
		log.Printf("Failed to search compacted contexts: %v", err)
	} else {
		for _, ctx := range contexts {
			// Simple text matching (could be enhanced with vector search)
			if strings.Contains(strings.ToLower(ctx.Summary), queryLower) ||
				containsAny(ctx.KeyTopics, queryLower) ||
				containsAny(ctx.KeyEntities, queryLower) {
				results = append(results, model.ChatMemorySearchResult{
					Type:      "context",
					Content:   ctx.Summary,
					Timestamp: ctx.CreatedAt,
					Relevance: 0.8,
					BatchNum:  ctx.BatchNumber,
					ContextID: &ctx.ID,
				})
			}
		}
	}

	// Search in messages if we need more results
	if len(results) < limit {
		var messages []model.ChatMessage
		searchPattern := "%" + query + "%"
		if err := s.db.Where("session_id = ? AND content ILIKE ?", sessionID, searchPattern).
			Order("created_at DESC").
			Limit(limit - len(results)).
			Find(&messages).Error; err != nil {
			log.Printf("Failed to search messages: %v", err)
		} else {
			for _, msg := range messages {
				// Find batch number for this message
				batchNum := 0
				var batch model.ChatMemoryBatch
				if err := s.db.Where("session_id = ? AND start_msg_id <= ? AND end_msg_id >= ?",
					sessionID, msg.ID, msg.ID).First(&batch).Error; err == nil {
					batchNum = batch.BatchNumber
				}

				results = append(results, model.ChatMemorySearchResult{
					Type:      "message",
					Content:   truncateString(msg.Content, 300),
					Role:      string(msg.Role),
					Timestamp: msg.CreatedAt,
					Relevance: 0.6,
					BatchNum:  batchNum,
					MessageID: &msg.ID,
				})
			}
		}
	}

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetSessionHistory returns paginated chat history for the history page
func (s *ChatMemoryService) GetSessionHistory(ctx context.Context, sessionID uint, userID uint, page int, pageSize int) (*SessionHistoryResponse, error) {
	// Verify session belongs to user
	var session model.ChatSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	if session.UserID != userID {
		return nil, fmt.Errorf("unauthorized: session does not belong to user")
	}

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	// Get total count
	var totalCount int64
	if err := s.db.Model(&model.ChatMessage{}).
		Where("session_id = ?", sessionID).
		Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}

	// Get messages with pagination
	var messages []model.ChatMessage
	if err := s.db.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	// Get batches info
	var batches []model.ChatMemoryBatch
	if err := s.db.Where("session_id = ?", sessionID).
		Order("batch_number ASC").
		Find(&batches).Error; err != nil {
		log.Printf("Failed to fetch batches: %v", err)
	}

	// Get compacted contexts
	var compactedContexts []model.ChatCompactedContext
	if err := s.db.Where("session_id = ?", sessionID).
		Order("batch_number ASC").
		Find(&compactedContexts).Error; err != nil {
		log.Printf("Failed to fetch compacted contexts: %v", err)
	}

	return &SessionHistoryResponse{
		Messages:          messages,
		TotalCount:        totalCount,
		Page:              page,
		PageSize:          pageSize,
		TotalPages:        (int(totalCount) + pageSize - 1) / pageSize,
		Batches:           batches,
		CompactedContexts: compactedContexts,
	}, nil
}

// SessionHistoryResponse is the response for history endpoint
type SessionHistoryResponse struct {
	Messages          []model.ChatMessage          `json:"messages"`
	TotalCount        int64                        `json:"total_count"`
	Page              int                          `json:"page"`
	PageSize          int                          `json:"page_size"`
	TotalPages        int                          `json:"total_pages"`
	Batches           []model.ChatMemoryBatch      `json:"batches"`
	CompactedContexts []model.ChatCompactedContext `json:"compacted_contexts"`
}

// GetAllSessions returns all chat sessions for a user
func (s *ChatMemoryService) GetAllSessions(ctx context.Context, userID uint, page int, pageSize int) (*AllSessionsResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count
	var totalCount int64
	if err := s.db.Model(&model.ChatSession{}).
		Where("user_id = ?", userID).
		Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	// Get sessions with subject info
	var sessions []model.ChatSession
	if err := s.db.Preload("Subject").
		Where("user_id = ?", userID).
		Order("last_message_at DESC NULLS LAST, created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch sessions: %w", err)
	}

	return &AllSessionsResponse{
		Sessions:   sessions,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (int(totalCount) + pageSize - 1) / pageSize,
	}, nil
}

// AllSessionsResponse is the response for listing all sessions
type AllSessionsResponse struct {
	Sessions   []model.ChatSession `json:"sessions"`
	TotalCount int64               `json:"total_count"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}

// Helper functions

func containsAny(arr []string, query string) bool {
	for _, item := range arr {
		if strings.Contains(strings.ToLower(item), query) {
			return true
		}
	}
	return false
}

// truncateMemoryString truncates a string to maxLen characters (for chat memory)
func truncateMemoryString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func parseJSONResponse(response string, result interface{}) error {
	// Try to find JSON in response
	response = strings.TrimSpace(response)

	// Remove markdown code blocks if present
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock || !strings.HasPrefix(line, "```") {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
	}

	// Find JSON object
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}

	return json.Unmarshal([]byte(response), result)
}
