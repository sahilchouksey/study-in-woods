package admin

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// GetOverviewAnalytics retrieves system-wide overview statistics
// GET /admin/analytics/overview
func GetOverviewAnalytics(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	var stats struct {
		TotalUsers        int64
		TotalCourses      int64
		TotalSubjects     int64
		TotalDocuments    int64
		TotalChatSessions int64
		TotalChatMessages int64
		TotalAPIKeys      int64
		ActiveUsersToday  int64
		ActiveUsersWeek   int64
	}

	// Fetch all counts
	db.Model(&model.User{}).Count(&stats.TotalUsers)
	db.Model(&model.Course{}).Count(&stats.TotalCourses)
	db.Model(&model.Subject{}).Count(&stats.TotalSubjects)
	db.Model(&model.Document{}).Count(&stats.TotalDocuments)
	db.Model(&model.ChatSession{}).Count(&stats.TotalChatSessions)
	db.Model(&model.ChatMessage{}).Count(&stats.TotalChatMessages)
	db.Model(&model.ExternalAPIKey{}).Where("revoked_at IS NULL").Count(&stats.TotalAPIKeys)

	// Active users
	db.Model(&model.UserActivity{}).
		Where("created_at >= ?", time.Now().Add(-24*time.Hour)).
		Distinct("user_id").
		Count(&stats.ActiveUsersToday)

	db.Model(&model.UserActivity{}).
		Where("created_at >= ?", time.Now().Add(-7*24*time.Hour)).
		Distinct("user_id").
		Count(&stats.ActiveUsersWeek)

	return response.SuccessWithMessage(c, "Overview analytics retrieved successfully", stats)
}

// GetUserAnalytics retrieves detailed user analytics
// GET /admin/analytics/users
func GetUserAnalytics(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	var analytics struct {
		TotalUsers  int64
		UsersByRole []struct {
			Role  string
			Count int64
		}
		UserGrowth []struct {
			Date  string
			Count int64
		}
		TopActiveUsers []struct {
			UserID     uint
			UserName   string
			Email      string
			Activities int64
		}
	}

	// Total users
	db.Model(&model.User{}).Count(&analytics.TotalUsers)

	// Users by role
	db.Model(&model.User{}).
		Select("role, COUNT(*) as count").
		Group("role").
		Scan(&analytics.UsersByRole)

	// User growth (last 30 days)
	db.Raw(`
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM users
		WHERE created_at >= NOW() - INTERVAL '30 days'
		AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`).Scan(&analytics.UserGrowth)

	// Top active users (by activity count)
	db.Raw(`
		SELECT ua.user_id, u.name as user_name, u.email, COUNT(*) as activities
		FROM user_activities ua
		JOIN users u ON ua.user_id = u.id
		WHERE ua.created_at >= NOW() - INTERVAL '30 days'
		GROUP BY ua.user_id, u.name, u.email
		ORDER BY activities DESC
		LIMIT 10
	`).Scan(&analytics.TopActiveUsers)

	return response.SuccessWithMessage(c, "User analytics retrieved successfully", analytics)
}

// GetAPIKeyAnalytics retrieves API key usage analytics
// GET /admin/analytics/api-keys
func GetAPIKeyAnalytics(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	var analytics struct {
		TotalAPIKeys     int64
		ActiveAPIKeys    int64
		RevokedAPIKeys   int64
		TotalRequests    int64
		RequestsToday    int64
		RequestsThisWeek int64
		TopAPIKeys       []struct {
			KeyID     uint
			KeyName   string
			UserEmail string
			Requests  int64
			LastUsed  *time.Time
		}
		RequestsByDay []struct {
			Date     string
			Requests int64
		}
	}

	// API key counts
	db.Model(&model.ExternalAPIKey{}).Count(&analytics.TotalAPIKeys)
	db.Model(&model.ExternalAPIKey{}).Where("revoked_at IS NULL").Count(&analytics.ActiveAPIKeys)
	db.Model(&model.ExternalAPIKey{}).Where("revoked_at IS NOT NULL").Count(&analytics.RevokedAPIKeys)

	// Request counts
	db.Model(&model.APIKeyUsageLog{}).Count(&analytics.TotalRequests)
	db.Model(&model.APIKeyUsageLog{}).
		Where("created_at >= ?", time.Now().Add(-24*time.Hour)).
		Count(&analytics.RequestsToday)
	db.Model(&model.APIKeyUsageLog{}).
		Where("created_at >= ?", time.Now().Add(-7*24*time.Hour)).
		Count(&analytics.RequestsThisWeek)

	// Top API keys by usage
	db.Raw(`
		SELECT k.id as key_id, k.name as key_name, u.email as user_email, 
			   COUNT(l.id) as requests, MAX(l.created_at) as last_used
		FROM external_api_keys k
		JOIN users u ON k.user_id = u.id
		LEFT JOIN api_key_usage_logs l ON k.id = l.api_key_id
		WHERE k.revoked_at IS NULL
		AND l.created_at >= NOW() - INTERVAL '30 days'
		GROUP BY k.id, k.name, u.email
		ORDER BY requests DESC
		LIMIT 10
	`).Scan(&analytics.TopAPIKeys)

	// Requests by day (last 30 days)
	db.Raw(`
		SELECT DATE(created_at) as date, COUNT(*) as requests
		FROM api_key_usage_logs
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`).Scan(&analytics.RequestsByDay)

	return response.SuccessWithMessage(c, "API key analytics retrieved successfully", analytics)
}

// GetDocumentAnalytics retrieves document management analytics
// GET /admin/analytics/documents
func GetDocumentAnalytics(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	var analytics struct {
		TotalDocuments  int64
		DocumentsByType []struct {
			Type  string
			Count int64
		}
		DocumentsByStatus []struct {
			Status string
			Count  int64
		}
		TotalFileSize   int64
		AverageFileSize float64
		DocumentGrowth  []struct {
			Date  string
			Count int64
		}
		TopSubjects []struct {
			SubjectID   uint
			SubjectName string
			Documents   int64
		}
	}

	// Total documents
	db.Model(&model.Document{}).Count(&analytics.TotalDocuments)

	// Documents by type
	db.Model(&model.Document{}).
		Select("type, COUNT(*) as count").
		Group("type").
		Scan(&analytics.DocumentsByType)

	// Documents by indexing status
	db.Model(&model.Document{}).
		Select("indexing_status as status, COUNT(*) as count").
		Group("indexing_status").
		Scan(&analytics.DocumentsByStatus)

	// File size statistics
	db.Model(&model.Document{}).Select("COALESCE(SUM(file_size), 0)").Scan(&analytics.TotalFileSize)
	db.Model(&model.Document{}).Select("COALESCE(AVG(file_size), 0)").Scan(&analytics.AverageFileSize)

	// Document growth (last 30 days)
	db.Raw(`
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM documents
		WHERE created_at >= NOW() - INTERVAL '30 days'
		AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`).Scan(&analytics.DocumentGrowth)

	// Top subjects by document count
	db.Raw(`
		SELECT s.id as subject_id, s.name as subject_name, COUNT(d.id) as documents
		FROM subjects s
		JOIN documents d ON s.id = d.subject_id
		WHERE d.deleted_at IS NULL
		GROUP BY s.id, s.name
		ORDER BY documents DESC
		LIMIT 10
	`).Scan(&analytics.TopSubjects)

	return response.SuccessWithMessage(c, "Document analytics retrieved successfully", analytics)
}

// GetChatAnalytics retrieves chat usage analytics
// GET /admin/analytics/chats
func GetChatAnalytics(c *fiber.Ctx, store database.Storage) error {
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		return response.InternalServerError(c, "Database connection error")
	}

	var analytics struct {
		TotalSessions             int64
		TotalMessages             int64
		AverageMessagesPerSession float64
		SessionsToday             int64
		SessionsThisWeek          int64
		MessagesByDay             []struct {
			Date     string
			Messages int64
		}
		TopSubjects []struct {
			SubjectID   uint
			SubjectName string
			Sessions    int64
			Messages    int64
		}
		ResponseTimeAvg float64
		TotalTokensUsed int64
	}

	// Total counts
	db.Model(&model.ChatSession{}).Count(&analytics.TotalSessions)
	db.Model(&model.ChatMessage{}).Count(&analytics.TotalMessages)

	// Average messages per session
	if analytics.TotalSessions > 0 {
		analytics.AverageMessagesPerSession = float64(analytics.TotalMessages) / float64(analytics.TotalSessions)
	}

	// Sessions today and this week
	db.Model(&model.ChatSession{}).
		Where("created_at >= ?", time.Now().Add(-24*time.Hour)).
		Count(&analytics.SessionsToday)
	db.Model(&model.ChatSession{}).
		Where("created_at >= ?", time.Now().Add(-7*24*time.Hour)).
		Count(&analytics.SessionsThisWeek)

	// Messages by day (last 30 days)
	db.Raw(`
		SELECT DATE(created_at) as date, COUNT(*) as messages
		FROM chat_messages
		WHERE created_at >= NOW() - INTERVAL '30 days'
		AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`).Scan(&analytics.MessagesByDay)

	// Top subjects by chat activity
	db.Raw(`
		SELECT s.id as subject_id, s.name as subject_name,
			   COUNT(DISTINCT cs.id) as sessions,
			   COUNT(cm.id) as messages
		FROM subjects s
		JOIN chat_sessions cs ON s.id = cs.subject_id
		LEFT JOIN chat_messages cm ON cs.id = cm.session_id
		WHERE cs.deleted_at IS NULL
		GROUP BY s.id, s.name
		ORDER BY sessions DESC
		LIMIT 10
	`).Scan(&analytics.TopSubjects)

	// Response time and token usage
	db.Model(&model.ChatMessage{}).
		Select("COALESCE(AVG(response_time), 0)").
		Scan(&analytics.ResponseTimeAvg)
	db.Model(&model.ChatMessage{}).
		Select("COALESCE(SUM(tokens_used), 0)").
		Scan(&analytics.TotalTokensUsed)

	return response.SuccessWithMessage(c, "Chat analytics retrieved successfully", analytics)
}
