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
	"gorm.io/gorm"
)

// ChatService handles chat operations with AI agents
type ChatService struct {
	db       *gorm.DB
	doClient *digitalocean.Client
	enableAI bool
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

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	SessionID uint
	UserID    uint
	Content   string
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

	// Get conversation history (last 10 messages)
	var history []model.ChatMessage
	if err := tx.Where("session_id = ?", req.SessionID).
		Order("created_at ASC").
		Limit(10).
		Find(&history).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to fetch conversation history: %w", err)
	}

	// Build messages for AI
	var messages []digitalocean.ChatMessage
	for _, msg := range history {
		messages = append(messages, digitalocean.ChatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	// Call AI agent
	startTime := time.Now()
	aiReq := digitalocean.ChatCompletionRequest{
		AgentUUID: session.AgentUUID,
		Messages:  messages,
	}

	aiResp, err := s.doClient.CreateChatCompletion(ctx, aiReq)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get AI response: %w", err)
	}
	responseTime := time.Since(startTime).Milliseconds()

	// Extract content and usage
	content := aiResp.ExtractContent()
	_, _, totalTokens := aiResp.GetUsage()

	// Save assistant message
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

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// StreamMessageRequest represents a request to stream a message
type StreamMessageRequest struct {
	SessionID uint
	UserID    uint
	Content   string
}

// StreamCallback is called for each chunk of streamed content
type StreamCallback func(chunk string) error

// StreamMessage sends a message and streams AI response
func (s *ChatService) StreamMessage(ctx context.Context, req StreamMessageRequest, callback StreamCallback) (*SendMessageResponse, error) {
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

	// Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			result.Error = fmt.Errorf("panic during message stream: %v", r)
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

	// Commit user message immediately
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit user message: %w", err)
	}

	// Get conversation history (last 10 messages)
	var history []model.ChatMessage
	if err := s.db.Where("session_id = ?", req.SessionID).
		Order("created_at ASC").
		Limit(10).
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch conversation history: %w", err)
	}

	// Build messages for AI
	var messages []digitalocean.ChatMessage
	for _, msg := range history {
		messages = append(messages, digitalocean.ChatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	// Stream AI response
	startTime := time.Now()
	var fullContent strings.Builder

	aiReq := digitalocean.ChatCompletionRequest{
		AgentUUID: session.AgentUUID,
		Messages:  messages,
	}

	err := s.doClient.StreamChatCompletion(ctx, aiReq, func(chunk digitalocean.StreamChunk) error {
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)

			// Call user's callback
			if err := callback(content); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to stream AI response: %w", err)
	}

	responseTime := time.Since(startTime).Milliseconds()

	// Save complete assistant message
	tx = s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction for assistant message: %w", tx.Error)
	}

	assistantMessage := model.ChatMessage{
		SessionID:    req.SessionID,
		SubjectID:    session.SubjectID,
		UserID:       req.UserID,
		Role:         model.MessageRoleAssistant,
		Content:      fullContent.String(),
		ResponseTime: int(responseTime),
		IsStreamed:   true,
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
		"last_message_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
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
