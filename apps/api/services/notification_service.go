package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NotificationService handles user notifications
type NotificationService struct {
	db *gorm.DB
}

// NewNotificationService creates a new notification service
func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db}
}

// CreateNotificationRequest represents a request to create a notification
type CreateNotificationRequest struct {
	UserID        uint
	Type          model.NotificationType
	Category      model.NotificationCategory
	Title         string
	Message       string
	IndexingJobID *uint
	Metadata      *model.NotificationMetadata
}

// ListNotificationsOptions represents options for listing notifications
type ListNotificationsOptions struct {
	UserID     uint
	UnreadOnly bool
	Category   string
	Limit      int
	Offset     int
}

// CreateNotification creates a new notification for a user
func (s *NotificationService) CreateNotification(ctx context.Context, req CreateNotificationRequest) (*model.UserNotification, error) {
	notification := &model.UserNotification{
		UserID:        req.UserID,
		Type:          req.Type,
		Category:      req.Category,
		Title:         req.Title,
		Message:       req.Message,
		Read:          false,
		IndexingJobID: req.IndexingJobID,
	}

	// Serialize metadata if provided
	if req.Metadata != nil {
		metadataJSON, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		notification.Metadata = datatypes.JSON(metadataJSON)
	}

	if err := s.db.WithContext(ctx).Create(notification).Error; err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	log.Printf("Created notification %d for user %d: %s", notification.ID, req.UserID, req.Title)
	return notification, nil
}

// UpdateNotification updates an existing notification
func (s *NotificationService) UpdateNotification(ctx context.Context, notificationID uint, updates map[string]interface{}) error {
	result := s.db.WithContext(ctx).Model(&model.UserNotification{}).
		Where("id = ?", notificationID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update notification: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// UpdateNotificationForJob updates the notification linked to an indexing job
func (s *NotificationService) UpdateNotificationForJob(ctx context.Context, jobID uint, notificationType model.NotificationType, title, message string, metadata *model.NotificationMetadata) error {
	updates := map[string]interface{}{
		"type":       notificationType,
		"title":      title,
		"message":    message,
		"updated_at": time.Now(),
	}

	if metadata != nil {
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		updates["metadata"] = datatypes.JSON(metadataJSON)
	}

	result := s.db.WithContext(ctx).Model(&model.UserNotification{}).
		Where("indexing_job_id = ?", jobID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update notification for job: %w", result.Error)
	}

	return nil
}

// GetNotificationsByUser retrieves notifications for a user
func (s *NotificationService) GetNotificationsByUser(ctx context.Context, opts ListNotificationsOptions) ([]model.UserNotification, int64, error) {
	var notifications []model.UserNotification
	var total int64

	query := s.db.WithContext(ctx).Model(&model.UserNotification{}).
		Where("user_id = ?", opts.UserID)

	if opts.UnreadOnly {
		query = query.Where("read = ?", false)
	}

	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	// Apply pagination
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	} else {
		query = query.Limit(50) // Default limit
	}

	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}

	// Order by most recent first
	if err := query.Order("created_at DESC").Find(&notifications).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch notifications: %w", err)
	}

	return notifications, total, nil
}

// GetNotificationByID retrieves a single notification by ID
func (s *NotificationService) GetNotificationByID(ctx context.Context, notificationID uint, userID uint) (*model.UserNotification, error) {
	var notification model.UserNotification

	err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", notificationID, userID).
		First(&notification).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch notification: %w", err)
	}

	return &notification, nil
}

// MarkAsRead marks a notification as read
func (s *NotificationService) MarkAsRead(ctx context.Context, notificationID uint, userID uint) error {
	result := s.db.WithContext(ctx).Model(&model.UserNotification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("read", true)

	if result.Error != nil {
		return fmt.Errorf("failed to mark notification as read: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// MarkAllAsRead marks all notifications for a user as read
func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uint) (int64, error) {
	result := s.db.WithContext(ctx).Model(&model.UserNotification{}).
		Where("user_id = ? AND read = ?", userID, false).
		Update("read", true)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to mark all notifications as read: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// DeleteNotification deletes a notification
func (s *NotificationService) DeleteNotification(ctx context.Context, notificationID uint, userID uint) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Delete(&model.UserNotification{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete notification: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// DeleteAllNotifications deletes all notifications for a user
func (s *NotificationService) DeleteAllNotifications(ctx context.Context, userID uint) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.UserNotification{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete all notifications: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// GetUnreadCount returns the count of unread notifications for a user
func (s *NotificationService) GetUnreadCount(ctx context.Context, userID uint) (int64, error) {
	var count int64

	err := s.db.WithContext(ctx).Model(&model.UserNotification{}).
		Where("user_id = ? AND read = ?", userID, false).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}

	return count, nil
}

// CleanupOldNotifications removes notifications older than the specified duration
func (s *NotificationService) CleanupOldNotifications(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result := s.db.WithContext(ctx).
		Where("created_at < ? AND read = ?", cutoff, true).
		Delete(&model.UserNotification{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup old notifications: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d old notifications", result.RowsAffected)
	}

	return result.RowsAffected, nil
}

// CreateInProgressNotification is a helper to create an in-progress notification for a job
func (s *NotificationService) CreateInProgressNotification(ctx context.Context, userID uint, jobID uint, category model.NotificationCategory, title, message string, metadata *model.NotificationMetadata) (*model.UserNotification, error) {
	return s.CreateNotification(ctx, CreateNotificationRequest{
		UserID:        userID,
		Type:          model.NotificationTypeInProgress,
		Category:      category,
		Title:         title,
		Message:       message,
		IndexingJobID: &jobID,
		Metadata:      metadata,
	})
}

// CompleteNotification updates a notification to success or warning state
func (s *NotificationService) CompleteNotification(ctx context.Context, jobID uint, success bool, title, message string, metadata *model.NotificationMetadata) error {
	notificationType := model.NotificationTypeSuccess
	if !success {
		notificationType = model.NotificationTypeWarning
	}

	return s.UpdateNotificationForJob(ctx, jobID, notificationType, title, message, metadata)
}

// FailNotification updates a notification to error state
func (s *NotificationService) FailNotification(ctx context.Context, jobID uint, title, message string, metadata *model.NotificationMetadata) error {
	return s.UpdateNotificationForJob(ctx, jobID, model.NotificationTypeError, title, message, metadata)
}
