package notification

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// NotificationHandler handles notification-related API endpoints
type NotificationHandler struct {
	notificationService *services.NotificationService
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(notificationService *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
	}
}

// GetNotifications handles GET /api/v1/notifications
// Returns all notifications for the authenticated user
func (h *NotificationHandler) GetNotifications(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse query parameters
	unreadOnly := c.Query("unread_only") == "true"
	category := c.Query("category")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	notifications, total, err := h.notificationService.GetNotificationsByUser(c.Context(), services.ListNotificationsOptions{
		UserID:     user.ID,
		UnreadOnly: unreadOnly,
		Category:   category,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch notifications")
	}

	// Convert to response format
	var responseData []interface{}
	for _, n := range notifications {
		responseData = append(responseData, n.ToResponse())
	}

	// Get unread count
	unreadCount, _ := h.notificationService.GetUnreadCount(c.Context(), user.ID)

	return response.Success(c, fiber.Map{
		"notifications": responseData,
		"total":         total,
		"unread_count":  unreadCount,
		"limit":         limit,
		"offset":        offset,
	})
}

// GetUnreadCount handles GET /api/v1/notifications/unread-count
// Returns the count of unread notifications
func (h *NotificationHandler) GetUnreadCount(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	count, err := h.notificationService.GetUnreadCount(c.Context(), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to get unread count")
	}

	return response.Success(c, fiber.Map{
		"unread_count": count,
	})
}

// MarkAsRead handles POST /api/v1/notifications/:id/read
// Marks a single notification as read
func (h *NotificationHandler) MarkAsRead(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	notificationID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid notification ID")
	}

	if err := h.notificationService.MarkAsRead(c.Context(), uint(notificationID), user.ID); err != nil {
		if err.Error() == "notification not found" {
			return response.NotFound(c, "Notification not found")
		}
		return response.InternalServerError(c, "Failed to mark notification as read")
	}

	return response.Success(c, fiber.Map{
		"message": "Notification marked as read",
	})
}

// MarkAllAsRead handles POST /api/v1/notifications/read-all
// Marks all notifications as read for the authenticated user
func (h *NotificationHandler) MarkAllAsRead(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	count, err := h.notificationService.MarkAllAsRead(c.Context(), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to mark all notifications as read")
	}

	return response.Success(c, fiber.Map{
		"message": "All notifications marked as read",
		"count":   count,
	})
}

// DeleteNotification handles DELETE /api/v1/notifications/:id
// Deletes a single notification
func (h *NotificationHandler) DeleteNotification(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	notificationID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid notification ID")
	}

	if err := h.notificationService.DeleteNotification(c.Context(), uint(notificationID), user.ID); err != nil {
		if err.Error() == "notification not found" {
			return response.NotFound(c, "Notification not found")
		}
		return response.InternalServerError(c, "Failed to delete notification")
	}

	return response.Success(c, fiber.Map{
		"message": "Notification deleted",
	})
}

// DeleteAllNotifications handles DELETE /api/v1/notifications
// Deletes all notifications for the authenticated user
func (h *NotificationHandler) DeleteAllNotifications(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	count, err := h.notificationService.DeleteAllNotifications(c.Context(), user.ID)
	if err != nil {
		return response.InternalServerError(c, "Failed to delete all notifications")
	}

	return response.Success(c, fiber.Map{
		"message": "All notifications deleted",
		"count":   count,
	})
}
