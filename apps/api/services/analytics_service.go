package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/gorm"
)

// AnalyticsService handles analytics and reporting
type AnalyticsService struct {
	db *gorm.DB
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(db *gorm.DB) *AnalyticsService {
	return &AnalyticsService{
		db: db,
	}
}

// DashboardStats represents overall platform statistics
type DashboardStats struct {
	TotalUsers          int64 `json:"total_users"`
	ActiveUsers         int64 `json:"active_users_7d"`
	TotalUniversities   int64 `json:"total_universities"`
	TotalCourses        int64 `json:"total_courses"`
	TotalSubjects       int64 `json:"total_subjects"`
	TotalDocuments      int64 `json:"total_documents"`
	TotalChatSessions   int64 `json:"total_chat_sessions"`
	TotalChatMessages   int64 `json:"total_chat_messages"`
	TotalTokensUsed     int64 `json:"total_tokens_used"`
	DocumentsIndexed    int64 `json:"documents_indexed"`
	DocumentsPending    int64 `json:"documents_pending"`
	DocumentsFailed     int64 `json:"documents_failed"`
	StorageUsedBytes    int64 `json:"storage_used_bytes"`
	AvgResponseTimeMs   int   `json:"avg_response_time_ms"`
	NewUsersToday       int64 `json:"new_users_today"`
	ActiveSessionsToday int64 `json:"active_sessions_today"`
}

// GetDashboardStats retrieves overall platform statistics
func (s *AnalyticsService) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Total users
	if err := s.db.Model(&model.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Active users (last 7 days)
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	if err := s.db.Model(&model.UserActivity{}).
		Where("created_at >= ?", sevenDaysAgo).
		Distinct("user_id").
		Count(&stats.ActiveUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count active users: %w", err)
	}

	// Total universities
	if err := s.db.Model(&model.University{}).Count(&stats.TotalUniversities).Error; err != nil {
		return nil, fmt.Errorf("failed to count universities: %w", err)
	}

	// Total courses
	if err := s.db.Model(&model.Course{}).Count(&stats.TotalCourses).Error; err != nil {
		return nil, fmt.Errorf("failed to count courses: %w", err)
	}

	// Total subjects
	if err := s.db.Model(&model.Subject{}).Count(&stats.TotalSubjects).Error; err != nil {
		return nil, fmt.Errorf("failed to count subjects: %w", err)
	}

	// Total documents
	if err := s.db.Model(&model.Document{}).Count(&stats.TotalDocuments).Error; err != nil {
		return nil, fmt.Errorf("failed to count documents: %w", err)
	}

	// Document indexing stats
	if err := s.db.Model(&model.Document{}).
		Where("indexing_status = ?", model.IndexingStatusCompleted).
		Count(&stats.DocumentsIndexed).Error; err != nil {
		return nil, fmt.Errorf("failed to count indexed documents: %w", err)
	}

	if err := s.db.Model(&model.Document{}).
		Where("indexing_status IN ?", []model.IndexingStatus{
			model.IndexingStatusPending,
			model.IndexingStatusInProgress,
		}).
		Count(&stats.DocumentsPending).Error; err != nil {
		return nil, fmt.Errorf("failed to count pending documents: %w", err)
	}

	if err := s.db.Model(&model.Document{}).
		Where("indexing_status = ?", model.IndexingStatusFailed).
		Count(&stats.DocumentsFailed).Error; err != nil {
		return nil, fmt.Errorf("failed to count failed documents: %w", err)
	}

	// Total storage used
	var storageResult struct {
		Total int64
	}
	if err := s.db.Model(&model.Document{}).
		Select("COALESCE(SUM(file_size), 0) as total").
		Scan(&storageResult).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate storage: %w", err)
	}
	stats.StorageUsedBytes = storageResult.Total

	// Chat statistics
	if err := s.db.Model(&model.ChatSession{}).Count(&stats.TotalChatSessions).Error; err != nil {
		return nil, fmt.Errorf("failed to count chat sessions: %w", err)
	}

	if err := s.db.Model(&model.ChatMessage{}).Count(&stats.TotalChatMessages).Error; err != nil {
		return nil, fmt.Errorf("failed to count chat messages: %w", err)
	}

	// Total tokens used
	var tokensResult struct {
		Total int64
	}
	if err := s.db.Model(&model.ChatMessage{}).
		Select("COALESCE(SUM(tokens_used), 0) as total").
		Scan(&tokensResult).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate tokens: %w", err)
	}
	stats.TotalTokensUsed = tokensResult.Total

	// Average response time
	var responseTimeResult struct {
		Avg float64
	}
	if err := s.db.Model(&model.ChatMessage{}).
		Where("response_time > 0").
		Select("COALESCE(AVG(response_time), 0) as avg").
		Scan(&responseTimeResult).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate avg response time: %w", err)
	}
	stats.AvgResponseTimeMs = int(responseTimeResult.Avg)

	// New users today
	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.Model(&model.User{}).
		Where("created_at >= ?", today).
		Count(&stats.NewUsersToday).Error; err != nil {
		return nil, fmt.Errorf("failed to count new users: %w", err)
	}

	// Active sessions today
	if err := s.db.Model(&model.ChatSession{}).
		Where("last_message_at >= ?", today).
		Count(&stats.ActiveSessionsToday).Error; err != nil {
		return nil, fmt.Errorf("failed to count active sessions: %w", err)
	}

	return stats, nil
}

// UserStats represents statistics for a specific user
type UserStats struct {
	UserID                 uint      `json:"user_id"`
	TotalChatSessions      int64     `json:"total_chat_sessions"`
	TotalChatMessages      int64     `json:"total_chat_messages"`
	TotalDocumentsUploaded int64     `json:"total_documents_uploaded"`
	TotalTokensUsed        int64     `json:"total_tokens_used"`
	LastActivityAt         time.Time `json:"last_activity_at"`
	MostActiveSubject      string    `json:"most_active_subject,omitempty"`
	AvgSessionDuration     int       `json:"avg_session_duration_minutes"`
	TotalActiveDays        int64     `json:"total_active_days"`
}

// GetUserStats retrieves statistics for a specific user
func (s *AnalyticsService) GetUserStats(ctx context.Context, userID uint) (*UserStats, error) {
	stats := &UserStats{UserID: userID}

	// Total chat sessions
	if err := s.db.Model(&model.ChatSession{}).
		Where("user_id = ?", userID).
		Count(&stats.TotalChatSessions).Error; err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	// Total chat messages
	if err := s.db.Model(&model.ChatMessage{}).
		Where("user_id = ?", userID).
		Count(&stats.TotalChatMessages).Error; err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}

	// Total documents uploaded
	if err := s.db.Model(&model.Document{}).
		Where("uploaded_by_user_id = ?", userID).
		Count(&stats.TotalDocumentsUploaded).Error; err != nil {
		return nil, fmt.Errorf("failed to count documents: %w", err)
	}

	// Total tokens used
	var tokensResult struct {
		Total int64
	}
	if err := s.db.Model(&model.ChatMessage{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(tokens_used), 0) as total").
		Scan(&tokensResult).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate tokens: %w", err)
	}
	stats.TotalTokensUsed = tokensResult.Total

	// Last activity
	var activity model.UserActivity
	if err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&activity).Error; err == nil {
		stats.LastActivityAt = activity.CreatedAt
	}

	// Most active subject (based on chat messages)
	var mostActiveResult struct {
		SubjectName string
		Count       int64
	}
	if err := s.db.Model(&model.ChatMessage{}).
		Select("subjects.name as subject_name, COUNT(*) as count").
		Joins("LEFT JOIN subjects ON chat_messages.subject_id = subjects.id").
		Where("chat_messages.user_id = ?", userID).
		Group("subjects.name").
		Order("count DESC").
		Limit(1).
		Scan(&mostActiveResult).Error; err == nil {
		stats.MostActiveSubject = mostActiveResult.SubjectName
	}

	// Total active days
	if err := s.db.Model(&model.UserActivity{}).
		Where("user_id = ?", userID).
		Select("COUNT(DISTINCT DATE(created_at))").
		Scan(&stats.TotalActiveDays).Error; err != nil {
		return nil, fmt.Errorf("failed to count active days: %w", err)
	}

	return stats, nil
}

// SubjectStats represents statistics for a specific subject
type SubjectStats struct {
	SubjectID         uint   `json:"subject_id"`
	SubjectName       string `json:"subject_name"`
	TotalDocuments    int64  `json:"total_documents"`
	TotalChatSessions int64  `json:"total_chat_sessions"`
	TotalChatMessages int64  `json:"total_chat_messages"`
	TotalTokensUsed   int64  `json:"total_tokens_used"`
	UniqueUsers       int64  `json:"unique_users"`
	AvgResponseTimeMs int    `json:"avg_response_time_ms"`
	DocumentsIndexed  int64  `json:"documents_indexed"`
	DocumentsPending  int64  `json:"documents_pending"`
	MostActiveUser    string `json:"most_active_user,omitempty"`
}

// GetSubjectStats retrieves statistics for a specific subject
func (s *AnalyticsService) GetSubjectStats(ctx context.Context, subjectID uint) (*SubjectStats, error) {
	stats := &SubjectStats{SubjectID: subjectID}

	// Get subject name
	var subject model.Subject
	if err := s.db.First(&subject, subjectID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch subject: %w", err)
	}
	stats.SubjectName = subject.Name

	// Total documents
	if err := s.db.Model(&model.Document{}).
		Where("subject_id = ?", subjectID).
		Count(&stats.TotalDocuments).Error; err != nil {
		return nil, fmt.Errorf("failed to count documents: %w", err)
	}

	// Documents indexed
	if err := s.db.Model(&model.Document{}).
		Where("subject_id = ? AND indexing_status = ?", subjectID, model.IndexingStatusCompleted).
		Count(&stats.DocumentsIndexed).Error; err != nil {
		return nil, fmt.Errorf("failed to count indexed documents: %w", err)
	}

	// Documents pending
	if err := s.db.Model(&model.Document{}).
		Where("subject_id = ? AND indexing_status IN ?", subjectID, []model.IndexingStatus{
			model.IndexingStatusPending,
			model.IndexingStatusInProgress,
		}).
		Count(&stats.DocumentsPending).Error; err != nil {
		return nil, fmt.Errorf("failed to count pending documents: %w", err)
	}

	// Chat statistics
	if err := s.db.Model(&model.ChatSession{}).
		Where("subject_id = ?", subjectID).
		Count(&stats.TotalChatSessions).Error; err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	if err := s.db.Model(&model.ChatMessage{}).
		Where("subject_id = ?", subjectID).
		Count(&stats.TotalChatMessages).Error; err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}

	// Total tokens
	var tokensResult struct {
		Total int64
	}
	if err := s.db.Model(&model.ChatMessage{}).
		Where("subject_id = ?", subjectID).
		Select("COALESCE(SUM(tokens_used), 0) as total").
		Scan(&tokensResult).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate tokens: %w", err)
	}
	stats.TotalTokensUsed = tokensResult.Total

	// Unique users
	if err := s.db.Model(&model.ChatMessage{}).
		Where("subject_id = ?", subjectID).
		Distinct("user_id").
		Count(&stats.UniqueUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count unique users: %w", err)
	}

	// Average response time
	var responseTimeResult struct {
		Avg float64
	}
	if err := s.db.Model(&model.ChatMessage{}).
		Where("subject_id = ? AND response_time > 0", subjectID).
		Select("COALESCE(AVG(response_time), 0) as avg").
		Scan(&responseTimeResult).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate avg response time: %w", err)
	}
	stats.AvgResponseTimeMs = int(responseTimeResult.Avg)

	// Most active user
	var mostActiveResult struct {
		Username string
		Count    int64
	}
	if err := s.db.Model(&model.ChatMessage{}).
		Select("users.username as username, COUNT(*) as count").
		Joins("LEFT JOIN users ON chat_messages.user_id = users.id").
		Where("chat_messages.subject_id = ?", subjectID).
		Group("users.username").
		Order("count DESC").
		Limit(1).
		Scan(&mostActiveResult).Error; err == nil {
		stats.MostActiveUser = mostActiveResult.Username
	}

	return stats, nil
}

// TimeSeriesPoint represents a data point in time series
type TimeSeriesPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
	Value int64  `json:"value,omitempty"`
}

// GetActivityTimeSeries retrieves activity over time
func (s *AnalyticsService) GetActivityTimeSeries(ctx context.Context, days int, activityType model.ActivityType) ([]TimeSeriesPoint, error) {
	startDate := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)

	var results []TimeSeriesPoint
	query := s.db.Model(&model.UserActivity{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ?", startDate).
		Group("DATE(created_at)").
		Order("date ASC")

	if activityType != "" {
		query = query.Where("activity_type = ?", activityType)
	}

	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch time series: %w", err)
	}

	return results, nil
}

// GetChatUsageTimeSeries retrieves chat usage metrics over time
func (s *AnalyticsService) GetChatUsageTimeSeries(ctx context.Context, days int) ([]TimeSeriesPoint, error) {
	startDate := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)

	var results []TimeSeriesPoint
	if err := s.db.Model(&model.ChatMessage{}).
		Select("DATE(created_at) as date, COUNT(*) as count, COALESCE(SUM(tokens_used), 0) as value").
		Where("created_at >= ?", startDate).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch chat usage: %w", err)
	}

	return results, nil
}

// TopSubject represents a top performing subject
type TopSubject struct {
	SubjectID     uint   `json:"subject_id"`
	SubjectName   string `json:"subject_name"`
	CourseName    string `json:"course_name"`
	MessageCount  int64  `json:"message_count"`
	UserCount     int64  `json:"user_count"`
	DocumentCount int64  `json:"document_count"`
}

// GetTopSubjects retrieves most active subjects
func (s *AnalyticsService) GetTopSubjects(ctx context.Context, limit int) ([]TopSubject, error) {
	var results []TopSubject

	if err := s.db.Model(&model.Subject{}).
		Select(`
			subjects.id as subject_id,
			subjects.name as subject_name,
			courses.name as course_name,
			COUNT(DISTINCT chat_messages.id) as message_count,
			COUNT(DISTINCT chat_messages.user_id) as user_count,
			COUNT(DISTINCT documents.id) as document_count
		`).
		Joins("LEFT JOIN chat_messages ON subjects.id = chat_messages.subject_id").
		Joins("LEFT JOIN documents ON subjects.id = documents.subject_id").
		Joins("LEFT JOIN semesters ON subjects.semester_id = semesters.id").
		Joins("LEFT JOIN courses ON semesters.course_id = courses.id").
		Group("subjects.id, subjects.name, courses.name").
		Order("message_count DESC").
		Limit(limit).
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch top subjects: %w", err)
	}

	return results, nil
}

// LogActivity logs a user activity
func (s *AnalyticsService) LogActivity(ctx context.Context, userID uint, activityType model.ActivityType, resourceType string, resourceID uint, ipAddress string, userAgent string) error {
	activity := model.UserActivity{
		UserID:       userID,
		ActivityType: activityType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	if err := s.db.Create(&activity).Error; err != nil {
		return fmt.Errorf("failed to log activity: %w", err)
	}

	return nil
}
