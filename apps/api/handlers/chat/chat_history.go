package chat

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// ChatHistoryHandler handles chat history and memory-related requests
type ChatHistoryHandler struct {
	db            *gorm.DB
	memoryService *services.ChatMemoryService
}

// NewChatHistoryHandler creates a new chat history handler
func NewChatHistoryHandler(db *gorm.DB, memoryService *services.ChatMemoryService) *ChatHistoryHandler {
	return &ChatHistoryHandler{
		db:            db,
		memoryService: memoryService,
	}
}

// GetSessionHistory handles GET /api/v1/chat/history/:id
// Returns full message history with batch and context information
func (h *ChatHistoryHandler) GetSessionHistory(c *fiber.Ctx) error {
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

	// Parse pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "50"))

	// Get history
	history, err := h.memoryService.GetSessionHistory(c.Context(), uint(sessionID), user.ID, page, pageSize)
	if err != nil {
		if err.Error() == "unauthorized: session does not belong to user" {
			return response.Unauthorized(c, err.Error())
		}
		return response.InternalServerError(c, "Failed to fetch history: "+err.Error())
	}

	return response.Success(c, history)
}

// GetAllSessions handles GET /api/v1/chat/history
// Returns all chat sessions for the user with pagination
func (h *ChatHistoryHandler) GetAllSessions(c *fiber.Ctx) error {
	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	// Get sessions
	sessions, err := h.memoryService.GetAllSessions(c.Context(), user.ID, page, pageSize)
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch sessions: "+err.Error())
	}

	return response.Success(c, sessions)
}

// SearchMemoryRequest represents the request to search memory
type SearchMemoryRequest struct {
	Query string `json:"query" validate:"required,min=1,max=500"`
	Limit int    `json:"limit" validate:"omitempty,min=1,max=50"`
}

// SearchMemory handles POST /api/v1/chat/history/:id/search
// Searches through conversation history for a specific session
func (h *ChatHistoryHandler) SearchMemory(c *fiber.Ctx) error {
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

	// Parse request
	var req SearchMemoryRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Query == "" {
		return response.BadRequest(c, "Query is required")
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	// Verify session belongs to user
	var sessionUserID uint
	if err := h.db.Table("chat_sessions").
		Select("user_id").
		Where("id = ?", sessionID).
		Scan(&sessionUserID).Error; err != nil {
		return response.NotFound(c, "Session not found")
	}
	if sessionUserID != user.ID {
		return response.Unauthorized(c, "Session does not belong to user")
	}

	// Search memory
	results, err := h.memoryService.SearchMemory(c.Context(), uint(sessionID), req.Query, req.Limit)
	if err != nil {
		return response.InternalServerError(c, "Failed to search memory: "+err.Error())
	}

	return response.Success(c, fiber.Map{
		"query":   req.Query,
		"results": results,
		"count":   len(results),
	})
}

// GetCompactedContexts handles GET /api/v1/chat/history/:id/contexts
// Returns all compacted contexts for a session
func (h *ChatHistoryHandler) GetCompactedContexts(c *fiber.Ctx) error {
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

	// Verify session belongs to user
	var sessionUserID uint
	if err := h.db.Table("chat_sessions").
		Select("user_id").
		Where("id = ?", sessionID).
		Scan(&sessionUserID).Error; err != nil {
		return response.NotFound(c, "Session not found")
	}
	if sessionUserID != user.ID {
		return response.Unauthorized(c, "Session does not belong to user")
	}

	// Get compacted contexts
	var contexts []ChatCompactedContextResponse
	if err := h.db.Table("chat_compacted_contexts").
		Where("session_id = ?", sessionID).
		Order("batch_number ASC").
		Find(&contexts).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch contexts")
	}

	return response.Success(c, contexts)
}

// ChatCompactedContextResponse is the API response for compacted context
type ChatCompactedContextResponse struct {
	ID            uint     `json:"id"`
	BatchNumber   int      `json:"batch_number"`
	Summary       string   `json:"summary"`
	KeyTopics     []string `json:"key_topics"`
	KeyEntities   []string `json:"key_entities"`
	UserIntents   []string `json:"user_intents"`
	MessageRange  string   `json:"message_range"`
	OriginalCount int      `json:"original_message_count"`
	CreatedAt     string   `json:"created_at"`
}

// GetBatches handles GET /api/v1/chat/history/:id/batches
// Returns all message batches for a session
func (h *ChatHistoryHandler) GetBatches(c *fiber.Ctx) error {
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

	// Verify session belongs to user
	var sessionUserID uint
	if err := h.db.Table("chat_sessions").
		Select("user_id").
		Where("id = ?", sessionID).
		Scan(&sessionUserID).Error; err != nil {
		return response.NotFound(c, "Session not found")
	}
	if sessionUserID != user.ID {
		return response.Unauthorized(c, "Session does not belong to user")
	}

	// Get batches
	var batches []BatchResponse
	if err := h.db.Table("chat_memory_batches").
		Where("session_id = ?", sessionID).
		Order("batch_number ASC").
		Find(&batches).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch batches")
	}

	return response.Success(c, batches)
}

// BatchResponse is the API response for a message batch
type BatchResponse struct {
	ID           uint   `json:"id"`
	BatchNumber  int    `json:"batch_number"`
	Status       string `json:"status"`
	MessageCount int    `json:"message_count"`
	StartMsgID   uint   `json:"start_msg_id"`
	EndMsgID     uint   `json:"end_msg_id"`
	CompactedAt  string `json:"compacted_at,omitempty"`
	ContextID    *uint  `json:"context_id,omitempty"`
	CreatedAt    string `json:"created_at"`
}
