package cron

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
)

// CheckDocumentIndexingStatus checks the status of documents being indexed in knowledge bases
// Runs every 15 minutes to update document indexing statuses from DigitalOcean
func (m *CronManager) CheckDocumentIndexingStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	jobName := "check_document_indexing"

	// Get all documents with pending or in-progress indexing
	var documents []model.Document
	err := m.db.Where("indexing_status IN ?", []string{
		string(model.IndexingStatusPending),
		string(model.IndexingStatusInProgress),
	}).Find(&documents).Error

	if err != nil {
		m.logJobError(jobName, fmt.Errorf("failed to query documents: %w", err))
		return
	}

	if len(documents) == 0 {
		m.logJobComplete(jobName, "No documents to check")
		return
	}

	// Initialize DigitalOcean client
	doClient := digitalocean.NewClient(digitalocean.Config{
		APIToken: os.Getenv("DIGITALOCEAN_TOKEN"),
	})

	updated := 0
	failed := 0

	// Check each document's indexing status
	for _, doc := range documents {
		if doc.DataSourceID == "" {
			continue
		}

		// Get subject to access knowledge base UUID
		var subject model.Subject
		if err := m.db.First(&subject, doc.SubjectID).Error; err != nil {
			log.Printf("[CRON] Failed to get subject for document %d: %v", doc.ID, err)
			continue
		}

		if subject.KnowledgeBaseUUID == "" {
			continue
		}

		// Get data source status from DigitalOcean
		dataSource, err := doClient.GetDataSource(ctx, subject.KnowledgeBaseUUID, doc.DataSourceID)
		if err != nil {
			log.Printf("[CRON] Failed to get data source status for document %d: %v", doc.ID, err)
			failed++
			continue
		}

		// Update document status based on data source status
		var newStatus model.IndexingStatus
		var errorMsg string

		switch dataSource.Status {
		case "pending":
			newStatus = model.IndexingStatusPending
		case "processing":
			newStatus = model.IndexingStatusInProgress
		case "indexed":
			newStatus = model.IndexingStatusCompleted
		case "failed":
			newStatus = model.IndexingStatusFailed
			errorMsg = "Indexing failed on DigitalOcean"
		case "partially_indexed":
			newStatus = model.IndexingStatusPartial
			errorMsg = "Some chunks failed to index"
		default:
			newStatus = model.IndexingStatusPending
		}

		// Update database if status changed
		if doc.IndexingStatus != newStatus || (errorMsg != "" && doc.IndexingError != errorMsg) {
			updateData := map[string]interface{}{
				"indexing_status": newStatus,
			}
			if errorMsg != "" {
				updateData["indexing_error"] = errorMsg
			}
			if dataSource.ChunkCount > 0 {
				updateData["page_count"] = dataSource.ChunkCount
			}

			if err := m.db.Model(&doc).Updates(updateData).Error; err != nil {
				log.Printf("[CRON] Failed to update document %d: %v", doc.ID, err)
				failed++
				continue
			}

			updated++
		}
	}

	m.logJobComplete(jobName, fmt.Sprintf("Checked %d documents, updated %d, failed %d", len(documents), updated, failed))
}

// CleanupPendingUploads removes documents that have been pending for too long
// Runs every 30 minutes to clean up stuck uploads (older than 24 hours)
func (m *CronManager) CleanupPendingUploads() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	jobName := "cleanup_pending_uploads"

	// Calculate cutoff time (24 hours ago)
	cutoffTime := time.Now().Add(-24 * time.Hour)

	// Find documents that have been pending for more than 24 hours
	var stuckDocuments []model.Document
	err := m.db.Where("indexing_status = ? AND created_at < ?",
		model.IndexingStatusPending,
		cutoffTime,
	).Find(&stuckDocuments).Error

	if err != nil {
		m.logJobError(jobName, fmt.Errorf("failed to query stuck documents: %w", err))
		return
	}

	if len(stuckDocuments) == 0 {
		m.logJobComplete(jobName, "No stuck uploads found")
		return
	}

	// Initialize DigitalOcean clients
	doClient := digitalocean.NewClient(digitalocean.Config{
		APIToken: os.Getenv("DIGITALOCEAN_TOKEN"),
	})

	spacesClient, err := digitalocean.NewSpacesClient(digitalocean.SpacesConfig{
		Region:   os.Getenv("DO_SPACES_REGION"),
		Bucket:   os.Getenv("DO_SPACES_BUCKET"),
		Endpoint: os.Getenv("DO_SPACES_ENDPOINT"),
	})
	if err != nil {
		m.logJobError(jobName, fmt.Errorf("failed to create Spaces client: %w", err))
		return
	}

	cleaned := 0
	failed := 0

	// Clean up each stuck document
	for _, doc := range stuckDocuments {
		// Delete from DigitalOcean Knowledge Base if data source exists
		if doc.DataSourceID != "" {
			var subject model.Subject
			if err := m.db.First(&subject, doc.SubjectID).Error; err == nil && subject.KnowledgeBaseUUID != "" {
				_ = doClient.DeleteDataSource(ctx, subject.KnowledgeBaseUUID, doc.DataSourceID)
			}
		}

		// Delete from Spaces if key exists
		if doc.SpacesKey != "" {
			if err := spacesClient.DeleteFile(ctx, doc.SpacesKey); err != nil {
				log.Printf("[CRON] Failed to delete file from Spaces for document %d: %v", doc.ID, err)
			}
		}

		// Delete from database
		if err := m.db.Delete(&doc).Error; err != nil {
			log.Printf("[CRON] Failed to delete document %d: %v", doc.ID, err)
			failed++
			continue
		}

		cleaned++
	}

	m.logJobComplete(jobName, fmt.Sprintf("Cleaned up %d stuck uploads, failed %d", cleaned, failed))
}

// AggregateUsageStatistics aggregates usage statistics for analytics
// Runs every hour to calculate hourly statistics
func (m *CronManager) AggregateUsageStatistics() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	jobName := "aggregate_statistics"

	// Calculate time range (previous hour)
	now := time.Now()
	endTime := now.Truncate(time.Hour)
	startTime := endTime.Add(-time.Hour)

	// Aggregate user activities
	type ActivityStats struct {
		Action string
		Count  int64
	}

	var activityStats []ActivityStats
	err := m.db.Model(&model.UserActivity{}).
		Select("action, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ?", startTime, endTime).
		Group("action").
		Scan(&activityStats).Error

	if err != nil {
		m.logJobError(jobName, fmt.Errorf("failed to aggregate user activities: %w", err))
		return
	}

	// Aggregate API key usage
	type APIKeyStats struct {
		TotalRequests int64
		UniqueKeys    int64
	}

	var apiKeyStats APIKeyStats
	err = m.db.Model(&model.APIKeyUsageLog{}).
		Select("COUNT(*) as total_requests, COUNT(DISTINCT api_key_id) as unique_keys").
		Where("created_at >= ? AND created_at < ?", startTime, endTime).
		Scan(&apiKeyStats).Error

	if err != nil {
		m.logJobError(jobName, fmt.Errorf("failed to aggregate API key usage: %w", err))
		return
	}

	// Aggregate chat statistics
	type ChatStats struct {
		TotalSessions int64
		TotalMessages int64
	}

	var chatStats ChatStats
	err = m.db.Model(&model.ChatSession{}).
		Select("COUNT(DISTINCT id) as total_sessions").
		Where("created_at >= ? AND created_at < ?", startTime, endTime).
		Scan(&chatStats).Error

	if err != nil {
		log.Printf("[CRON] Failed to aggregate chat sessions: %v", err)
	}

	err = m.db.Model(&model.ChatMessage{}).
		Select("COUNT(*) as total_messages").
		Where("created_at >= ? AND created_at < ?", startTime, endTime).
		Scan(&chatStats.TotalMessages).Error

	if err != nil {
		log.Printf("[CRON] Failed to aggregate chat messages: %v", err)
	}

	// Store aggregated statistics in app_settings as JSON for later retrieval
	statsJSON := fmt.Sprintf(`{
		"timestamp": "%s",
		"hour_start": "%s",
		"hour_end": "%s",
		"activities": %v,
		"api_requests": %d,
		"unique_api_keys": %d,
		"chat_sessions": %d,
		"chat_messages": %d
	}`, now.Format(time.RFC3339), startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
		activityStats, apiKeyStats.TotalRequests, apiKeyStats.UniqueKeys,
		chatStats.TotalSessions, chatStats.TotalMessages)

	// Save to app_settings with a unique key for this hour
	settingKey := fmt.Sprintf("stats_hourly_%s", startTime.Format("2006010215"))
	setting := model.AppSetting{
		Key:         settingKey,
		Value:       statsJSON,
		Type:        "json",
		Description: fmt.Sprintf("Hourly statistics for %s", startTime.Format("2006-01-02 15:00")),
		Category:    "statistics",
		IsPublic:    false,
	}

	if err := m.db.WithContext(ctx).Create(&setting).Error; err != nil {
		m.logJobError(jobName, fmt.Errorf("failed to save statistics: %w", err))
		return
	}

	m.logJobComplete(jobName, fmt.Sprintf("Aggregated statistics for hour %s", startTime.Format("2006-01-02 15:00")))
}

// SyncDigitalOceanModels syncs available models from DigitalOcean
// Runs every 6 hours to keep the model list up-to-date
func (m *CronManager) SyncDigitalOceanModels() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	jobName := "sync_do_models"

	// Initialize DigitalOcean client
	doClient := digitalocean.NewClient(digitalocean.Config{
		APIToken: os.Getenv("DIGITALOCEAN_TOKEN"),
	})

	// Get available agents as a proxy for model availability
	agents, _, err := doClient.ListAgents(ctx, nil)
	if err != nil {
		m.logJobError(jobName, fmt.Errorf("failed to fetch agents from DigitalOcean: %w", err))
		return
	}

	// Store agent info which includes model references
	agentsJSON := fmt.Sprintf(`{
		"timestamp": "%s",
		"agents": [`, time.Now().Format(time.RFC3339))

	for i, agent := range agents {
		if i > 0 {
			agentsJSON += ","
		}
		agentsJSON += fmt.Sprintf(`{
			"uuid": "%s",
			"name": "%s",
			"model_id": "%s",
			"status": "%s"
		}`, agent.UUID, agent.Name, agent.ModelID, agent.Status)
	}

	agentsJSON += `]}`

	// Update or create app setting
	var setting model.AppSetting
	result := m.db.Where("key = ?", "digitalocean_agents").First(&setting)

	if result.Error != nil {
		// Create new setting
		setting = model.AppSetting{
			Key:         "digitalocean_agents",
			Value:       agentsJSON,
			Type:        "json",
			Description: "Available agents from DigitalOcean",
			Category:    "digitalocean",
			IsPublic:    true,
		}
		if err := m.db.WithContext(ctx).Create(&setting).Error; err != nil {
			m.logJobError(jobName, fmt.Errorf("failed to create agents setting: %w", err))
			return
		}
	} else {
		// Update existing setting
		if err := m.db.Model(&setting).Update("value", agentsJSON).Error; err != nil {
			m.logJobError(jobName, fmt.Errorf("failed to update agents setting: %w", err))
			return
		}
	}

	m.logJobComplete(jobName, fmt.Sprintf("Synced %d agents from DigitalOcean", len(agents)))
}

// CleanupOldData removes old data to keep the database clean
// Runs daily at 2 AM
func (m *CronManager) CleanupOldData() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	jobName := "cleanup_old_data"

	totalCleaned := 0

	// 1. Clean up expired JWT tokens from blacklist (older than 30 days)
	cutoffTokens := time.Now().Add(-30 * 24 * time.Hour)
	result := m.db.Where("expires_at < ?", cutoffTokens).Delete(&model.JWTTokenBlacklist{})
	if result.Error != nil {
		log.Printf("[CRON] Failed to clean token blacklist: %v", result.Error)
	} else {
		log.Printf("[CRON] Cleaned %d expired tokens", result.RowsAffected)
		totalCleaned += int(result.RowsAffected)
	}

	// 2. Clean up old password reset tokens (older than 7 days)
	cutoffResets := time.Now().Add(-7 * 24 * time.Hour)
	result = m.db.Where("created_at < ?", cutoffResets).Delete(&model.PasswordResetToken{})
	if result.Error != nil {
		log.Printf("[CRON] Failed to clean password resets: %v", result.Error)
	} else {
		log.Printf("[CRON] Cleaned %d old password resets", result.RowsAffected)
		totalCleaned += int(result.RowsAffected)
	}

	// 3. Clean up old cron job logs (keep only last 90 days)
	cutoffLogs := time.Now().Add(-90 * 24 * time.Hour)
	result = m.db.Where("created_at < ?", cutoffLogs).Delete(&model.CronJobLog{})
	if result.Error != nil {
		log.Printf("[CRON] Failed to clean cron logs: %v", result.Error)
	} else {
		log.Printf("[CRON] Cleaned %d old cron logs", result.RowsAffected)
		totalCleaned += int(result.RowsAffected)
	}

	// 4. Clean up old user activities (keep only last 180 days)
	cutoffActivity := time.Now().Add(-180 * 24 * time.Hour)
	result = m.db.Where("created_at < ?", cutoffActivity).Delete(&model.UserActivity{})
	if result.Error != nil {
		log.Printf("[CRON] Failed to clean user activities: %v", result.Error)
	} else {
		log.Printf("[CRON] Cleaned %d old user activities", result.RowsAffected)
		totalCleaned += int(result.RowsAffected)
	}

	// 5. Clean up old API key usage logs (keep only last 90 days)
	cutoffAPILogs := time.Now().Add(-90 * 24 * time.Hour)
	result = m.db.Where("created_at < ?", cutoffAPILogs).Delete(&model.APIKeyUsageLog{})
	if result.Error != nil {
		log.Printf("[CRON] Failed to clean API key logs: %v", result.Error)
	} else {
		log.Printf("[CRON] Cleaned %d old API key logs", result.RowsAffected)
		totalCleaned += int(result.RowsAffected)
	}

	// 6. Clean up old hourly statistics (keep only last 90 days)
	cutoffStats := time.Now().Add(-90 * 24 * time.Hour)
	statsPattern := fmt.Sprintf("stats_hourly_%s%%", cutoffStats.Format("200601"))
	result = m.db.Where("key LIKE ? AND created_at < ?", statsPattern, cutoffStats).
		Delete(&model.AppSetting{})
	if result.Error != nil {
		log.Printf("[CRON] Failed to clean old statistics: %v", result.Error)
	} else {
		log.Printf("[CRON] Cleaned %d old statistics", result.RowsAffected)
		totalCleaned += int(result.RowsAffected)
	}

	// 7. Clean up chat sessions with no messages (older than 7 days)
	cutoffSessions := time.Now().Add(-7 * 24 * time.Hour)
	var emptySessions []model.ChatSession
	m.db.Where("created_at < ?", cutoffSessions).Find(&emptySessions)

	cleanedSessions := 0
	for _, session := range emptySessions {
		var messageCount int64
		m.db.Model(&model.ChatMessage{}).Where("session_id = ?", session.ID).Count(&messageCount)

		if messageCount == 0 {
			if err := m.db.WithContext(ctx).Delete(&session).Error; err != nil {
				log.Printf("[CRON] Failed to delete empty session %d: %v", session.ID, err)
			} else {
				cleanedSessions++
			}
		}
	}
	log.Printf("[CRON] Cleaned %d empty chat sessions", cleanedSessions)
	totalCleaned += cleanedSessions

	m.logJobComplete(jobName, fmt.Sprintf("Cleaned up %d total records", totalCleaned))
}

// SyncDigitalOceanConfig syncs configuration from DigitalOcean
// Runs daily at 6:30 AM to fetch knowledge bases and agent info
func (m *CronManager) SyncDigitalOceanConfig() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	jobName := "sync_do_config"

	// Initialize DigitalOcean client
	doClient := digitalocean.NewClient(digitalocean.Config{
		APIToken: os.Getenv("DIGITALOCEAN_TOKEN"),
	})

	synced := 0

	// 1. Sync knowledge bases
	knowledgeBases, _, err := doClient.ListKnowledgeBases(ctx, nil)
	if err != nil {
		log.Printf("[CRON] Failed to fetch knowledge bases: %v", err)
	} else {
		kbJSON := fmt.Sprintf(`{
			"timestamp": "%s",
			"knowledge_bases": [`, time.Now().Format(time.RFC3339))

		for i, kb := range knowledgeBases {
			if i > 0 {
				kbJSON += ","
			}
			kbJSON += fmt.Sprintf(`{
				"uuid": "%s",
				"name": "%s",
				"status": "%s",
				"embedding_model": "%s"
			}`, kb.UUID, kb.Name, kb.Status, kb.EmbeddingModel)
		}
		kbJSON += `]}`

		var setting model.AppSetting
		result := m.db.Where("key = ?", "digitalocean_knowledge_bases").First(&setting)

		if result.Error != nil {
			setting = model.AppSetting{
				Key:         "digitalocean_knowledge_bases",
				Value:       kbJSON,
				Type:        "json",
				Description: "Available knowledge bases from DigitalOcean",
				Category:    "digitalocean",
				IsPublic:    true,
			}
			m.db.WithContext(ctx).Create(&setting)
		} else {
			m.db.Model(&setting).Update("value", kbJSON)
		}

		log.Printf("[CRON] Synced %d knowledge bases", len(knowledgeBases))
		synced++
	}

	// 2. Sync agents (as a double-check to keep them updated)
	agents, _, err := doClient.ListAgents(ctx, nil)
	if err != nil {
		log.Printf("[CRON] Failed to fetch agents: %v", err)
	} else {
		agentsJSON := fmt.Sprintf(`{
			"timestamp": "%s",
			"agents": [`, time.Now().Format(time.RFC3339))

		for i, agent := range agents {
			if i > 0 {
				agentsJSON += ","
			}
			agentsJSON += fmt.Sprintf(`{
				"uuid": "%s",
				"name": "%s",
				"model_id": "%s",
				"status": "%s"
			}`, agent.UUID, agent.Name, agent.ModelID, agent.Status)
		}
		agentsJSON += `]}`

		var setting model.AppSetting
		result := m.db.Where("key = ?", "digitalocean_agents_daily").First(&setting)

		if result.Error != nil {
			setting = model.AppSetting{
				Key:         "digitalocean_agents_daily",
				Value:       agentsJSON,
				Type:        "json",
				Description: "Daily sync of agents from DigitalOcean",
				Category:    "digitalocean",
				IsPublic:    false,
			}
			m.db.WithContext(ctx).Create(&setting)
		} else {
			m.db.Model(&setting).Update("value", agentsJSON)
		}

		log.Printf("[CRON] Synced %d agents", len(agents))
		synced++
	}

	if synced == 0 {
		m.logJobError(jobName, fmt.Errorf("failed to sync any configuration"))
		return
	}

	m.logJobComplete(jobName, fmt.Sprintf("Synced %d configuration items", synced))
}
