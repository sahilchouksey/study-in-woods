package chat

import (
	"bufio"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"github.com/sahilchouksey/go-init-setup/utils/validation"
	"gorm.io/gorm"
)

// ChatHandler handles chat-related requests
type ChatHandler struct {
	db          *gorm.DB
	validator   *validation.Validator
	chatService *services.ChatService
}

// NewChatHandler creates a new chat handler
func NewChatHandler(db *gorm.DB, chatService *services.ChatService) *ChatHandler {
	return &ChatHandler{
		db:          db,
		validator:   validation.NewValidator(),
		chatService: chatService,
	}
}

// CreateSessionRequest represents the request to create a chat session
type CreateSessionRequest struct {
	SubjectID   uint   `json:"subject_id" validate:"required,min=1"`
	Title       string `json:"title" validate:"omitempty,max=255"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}

// SendMessageRequest represents the request to send a chat message
type SendMessageRequest struct {
	Content string `json:"content" validate:"required,min=1,max=10000"`
	Stream  bool   `json:"stream" validate:"omitempty"`
}

// ListSessions handles GET /api/v1/chat/sessions
func (h *ChatHandler) ListSessions(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	status := c.Query("status", "")
	subjectID := c.Query("subject_id", "")

	// Build query
	query := h.db.Model(&model.ChatSession{}).Where("user_id = ?", user.ID)

	// Apply filters
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if subjectID != "" {
		query = query.Where("subject_id = ?", subjectID)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return response.InternalServerError(c, "Failed to count sessions")
	}

	// Calculate pagination
	offset := (page - 1) * limit
	pagination := response.CalculatePagination(page, limit, total)

	// Get sessions
	var sessions []model.ChatSession
	if err := query.Preload("Subject").
		Order("last_message_at DESC NULLS LAST, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&sessions).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch sessions")
	}

	return response.Paginated(c, sessions, pagination)
}

// GetSession handles GET /api/v1/chat/sessions/:id
func (h *ChatHandler) GetSession(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	var session model.ChatSession
	if err := h.db.Preload("Subject").Preload("User").
		Where("id = ? AND user_id = ?", id, user.ID).
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Session not found")
		}
		return response.InternalServerError(c, "Failed to fetch session")
	}

	return response.Success(c, session)
}

// CreateSession handles POST /api/v1/chat/sessions
func (h *ChatHandler) CreateSession(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse and validate request
	var req CreateSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Create session using ChatService
	session, err := h.chatService.CreateSession(c.Context(), services.CreateSessionRequest{
		SubjectID:   req.SubjectID,
		UserID:      user.ID,
		Title:       req.Title,
		Description: req.Description,
	})

	if err != nil {
		return response.InternalServerError(c, "Failed to create session: "+err.Error())
	}

	return response.Created(c, session)
}

// DeleteSession handles DELETE /api/v1/chat/sessions/:id
func (h *ChatHandler) DeleteSession(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse session ID
	sessionID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid session ID")
	}

	// Delete session
	if err := h.chatService.DeleteSession(c.Context(), uint(sessionID), user.ID); err != nil {
		return response.InternalServerError(c, "Failed to delete session: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "Session deleted successfully",
	})
}

// ArchiveSession handles POST /api/v1/chat/sessions/:id/archive
func (h *ChatHandler) ArchiveSession(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse session ID
	sessionID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid session ID")
	}

	// Archive session
	if err := h.chatService.ArchiveSession(c.Context(), uint(sessionID), user.ID); err != nil {
		return response.InternalServerError(c, "Failed to archive session: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"message": "Session archived successfully",
	})
}

// GetMessages handles GET /api/v1/chat/sessions/:id/messages
func (h *ChatHandler) GetMessages(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	// Parse session ID
	sessionID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid session ID")
	}

	// Get messages
	offset := (page - 1) * limit
	messages, total, err := h.chatService.GetSessionMessages(c.Context(), uint(sessionID), user.ID, limit, offset)
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch messages: "+err.Error())
	}

	// Calculate pagination
	pagination := response.CalculatePagination(page, limit, total)

	return response.Paginated(c, messages, pagination)
}

// SendMessage handles POST /api/v1/chat/sessions/:id/messages
func (h *ChatHandler) SendMessage(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse and validate request
	var req SendMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := h.validator.ValidateStruct(req); err != nil {
		return response.ValidationError(c, err)
	}

	// Parse session ID
	sessionID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid session ID")
	}

	// Check if streaming is requested
	if req.Stream {
		return h.handleStreamMessage(c, uint(sessionID), user.ID, req.Content)
	}

	// Send non-streaming message
	result, err := h.chatService.SendMessage(c.Context(), services.SendMessageRequest{
		SessionID: uint(sessionID),
		UserID:    user.ID,
		Content:   req.Content,
	})

	if err != nil {
		return response.InternalServerError(c, "Failed to send message: "+err.Error())
	}

	return response.Created(c, fiber.Map{
		"user_message":      result.UserMessage,
		"assistant_message": result.AssistantMessage,
	})
}

// handleStreamMessage handles streaming chat responses
func (h *ChatHandler) handleStreamMessage(c *fiber.Ctx, sessionID uint, userID uint, content string) error {
	// Set headers for SSE
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	// Set response to streaming
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Send initial event
		fmt.Fprintf(w, "event: start\n")
		fmt.Fprintf(w, "data: {\"status\":\"streaming\"}\n\n")
		w.Flush()

		// Stream message
		result, err := h.chatService.StreamMessage(c.Context(), services.StreamMessageRequest{
			SessionID: sessionID,
			UserID:    userID,
			Content:   content,
		}, func(chunk string) error {
			// Send chunk as SSE
			fmt.Fprintf(w, "event: chunk\n")
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			return w.Flush()
		})

		if err != nil {
			// Send error event
			fmt.Fprintf(w, "event: error\n")
			fmt.Fprintf(w, "data: {\"error\":\"%s\"}\n\n", err.Error())
			w.Flush()
			return
		}

		// Send completion event
		fmt.Fprintf(w, "event: done\n")
		fmt.Fprintf(w, "data: {\"user_message_id\":%d,\"assistant_message_id\":%d}\n\n",
			result.UserMessage.ID, result.AssistantMessage.ID)
		w.Flush()
	})

	return nil
}
