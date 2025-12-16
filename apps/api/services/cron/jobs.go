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

// CheckBatchIngestKBIndexing checks the status of KB indexing jobs from batch ingest
// Runs every 2 minutes to update batch ingest job status, notifications, and PYQ extraction status
func (m *CronManager) CheckBatchIngestKBIndexing() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	jobName := "check_batch_ingest_kb_indexing"

	log.Printf("[CRON] ========== %s ==========", jobName)
	log.Printf("[CRON] Running at: %s (interval: every 2 minutes)", time.Now().Format("2006-01-02 15:04:05"))

	// Get all indexing jobs waiting for KB indexing (with or without DO indexing job UUID)
	var jobs []model.IndexingJob
	err := m.db.Where("status = ?", model.IndexingJobStatusKBIndexing).Find(&jobs).Error
	if err != nil {
		m.logJobError(jobName, fmt.Errorf("failed to query jobs: %w", err))
		return
	}

	log.Printf("[CRON] Found %d jobs with status 'kb_indexing' to check", len(jobs))

	if len(jobs) == 0 {
		m.logJobComplete(jobName, "No KB indexing jobs to check")
		return
	}

	// Initialize DigitalOcean client
	doClient := digitalocean.NewClient(digitalocean.Config{
		APIToken: os.Getenv("DIGITALOCEAN_TOKEN"),
	})

	updated := 0
	completed := 0
	failed := 0

	for _, job := range jobs {
		var newStatus model.IndexingJobStatus
		var isComplete bool

		if job.DOIndexingJobUUID != "" {
			// Case 1: We have an explicit indexing job UUID - check its status
			doJob, err := doClient.GetIndexingJob(ctx, job.DOIndexingJobUUID)
			if err != nil {
				log.Printf("[CRON] Failed to get DO indexing job %s: %v", job.DOIndexingJobUUID, err)
				failed++
				continue
			}

			log.Printf("[CRON] DO Indexing job %s status: Phase=%s, Status=%s", job.DOIndexingJobUUID, doJob.Phase, doJob.Status)

			// Check both Phase and Status fields from DO API
			switch {
			case doJob.Phase == "BATCH_JOB_PHASE_SUCCEEDED" || doJob.Status == "INDEX_JOB_STATUS_COMPLETED":
				if job.FailedItems > 0 {
					newStatus = model.IndexingJobStatusPartial
				} else {
					newStatus = model.IndexingJobStatusCompleted
				}
				isComplete = true
				completed++
			case doJob.Phase == "BATCH_JOB_PHASE_FAILED" || doJob.Status == "INDEX_JOB_STATUS_FAILED":
				newStatus = model.IndexingJobStatusFailed
				isComplete = true
				failed++
			default:
				// Still in progress
				continue
			}
		} else {
			// Case 2: No explicit indexing job UUID - check data source status directly
			// Get subject's KB UUID
			var subject model.Subject
			if err := m.db.First(&subject, job.SubjectID).Error; err != nil {
				log.Printf("[CRON] Failed to get subject %d: %v", job.SubjectID, err)
				continue
			}

			if subject.KnowledgeBaseUUID == "" {
				// No KB - mark as completed (no AI indexing possible)
				newStatus = model.IndexingJobStatusCompleted
				isComplete = true
				completed++
			} else {
				// Check data sources in KB
				dataSources, err := doClient.ListKnowledgeBaseDataSources(ctx, subject.KnowledgeBaseUUID)
				if err != nil {
					log.Printf("[CRON] Failed to list data sources for KB %s: %v", subject.KnowledgeBaseUUID, err)
					continue
				}

				log.Printf("[CRON] Job %d: Found %d data sources in KB %s", job.ID, len(dataSources), subject.KnowledgeBaseUUID)

				// Get all document data source IDs for this job
				var items []model.IndexingJobItem
				m.db.Where("job_id = ?", job.ID).Find(&items)

				var docIDs []uint
				for _, item := range items {
					if item.DocumentID != nil {
						docIDs = append(docIDs, *item.DocumentID)
					}
				}

				var documents []model.Document
				m.db.Where("id IN ?", docIDs).Find(&documents)

				log.Printf("[CRON] Job %d: Checking %d documents for indexing status", job.ID, len(documents))

				// First, retry adding documents that don't have data_source_id to the KB
				// This handles cases where initial KB upload failed due to rate limiting
				var docsWithoutDataSource []model.Document
				for _, doc := range documents {
					if doc.DataSourceID == "" && doc.SpacesKey != "" {
						docsWithoutDataSource = append(docsWithoutDataSource, doc)
					}
				}

				if len(docsWithoutDataSource) > 0 {
					log.Printf("[CRON] Job %d: Found %d documents without data_source_id, attempting to add to KB", job.ID, len(docsWithoutDataSource))

					spacesName := os.Getenv("DO_SPACES_NAME")
					spacesRegion := os.Getenv("DO_SPACES_REGION")
					if spacesRegion == "" {
						spacesRegion = "blr1"
					}

					for _, doc := range docsWithoutDataSource {
						// Try to add to KB using CreateDataSource
						dsReq := digitalocean.CreateDataSourceRequest{
							KnowledgeBaseUUID: subject.KnowledgeBaseUUID,
							SpacesDataSource: &digitalocean.SpacesDataSourceInput{
								BucketName: spacesName,
								Region:     spacesRegion,
								ItemPath:   doc.SpacesKey,
							},
						}
						dataSource, _, err := doClient.CreateDataSource(ctx, subject.KnowledgeBaseUUID, dsReq)
						if err != nil {
							log.Printf("[CRON] Job %d: Failed to add document %d to KB (rate limited?): %v", job.ID, doc.ID, err)
							// Continue - will retry next cron run
						} else {
							log.Printf("[CRON] Job %d: Added document %d to KB with data_source_id: %s", job.ID, doc.ID, dataSource.UUID)
							// Update document with data source ID
							m.db.Model(&model.Document{}).Where("id = ?", doc.ID).Update("data_source_id", dataSource.UUID)
							// Update local doc object for subsequent logic
							doc.DataSourceID = dataSource.UUID
						}
						// Small delay to avoid rate limiting
						time.Sleep(500 * time.Millisecond)
					}

					// Re-fetch documents to get updated data_source_ids
					m.db.Where("id IN ?", docIDs).Find(&documents)
					// Also re-fetch data sources list
					dataSources, _ = doClient.ListKnowledgeBaseDataSources(ctx, subject.KnowledgeBaseUUID)
				}

				// Check if there's a bucket-level data source that's already indexed
				// The bucket-level data source indexes ALL files in the bucket
				var bucketDataSourceIndexed bool
				for _, ds := range dataSources {
					// Bucket-level data source has empty item_path
					if (ds.SpacesDataSource != nil && ds.SpacesDataSource.ItemPath == "") || ds.ItemPath == "" {
						// Check LastDataSourceIndexingJob first (new API field), then fallback to LastIndexingJob
						if ds.LastDataSourceIndexingJob != nil {
							log.Printf("[CRON] Job %d: Found bucket-level data source %s with status: %s", job.ID, ds.UUID, ds.LastDataSourceIndexingJob.Status)
							if ds.LastDataSourceIndexingJob.Status == "INDEX_JOB_STATUS_COMPLETED" || ds.LastDataSourceIndexingJob.Status == "DATA_SOURCE_STATUS_UPDATED" {
								bucketDataSourceIndexed = true
							}
						} else if ds.LastIndexingJob != nil {
							log.Printf("[CRON] Job %d: Found bucket-level data source %s with status: %s", job.ID, ds.UUID, ds.LastIndexingJob.Status)
							if ds.LastIndexingJob.Status == "INDEX_JOB_STATUS_COMPLETED" || ds.LastIndexingJob.Status == "DATA_SOURCE_STATUS_UPDATED" {
								bucketDataSourceIndexed = true
							}
						}
					}
				}

				// Check if all data sources are indexed
				allIndexed := true
				anyFailed := false
				indexedCount := 0
				pendingCount := 0
				docsWithoutDataSourceCount := 0
				for _, doc := range documents {
					if doc.DataSourceID == "" {
						log.Printf("[CRON] Job %d: Document %d still has no data_source_id after retry", job.ID, doc.ID)
						docsWithoutDataSourceCount++
						allIndexed = false // Can't be complete if docs still missing data_source_id
						continue
					}

					// If bucket-level data source is indexed, consider all individual data sources as indexed too
					if bucketDataSourceIndexed {
						log.Printf("[CRON] Job %d: Document %d considered indexed via bucket-level data source", job.ID, doc.ID)
						indexedCount++
						continue
					}

					found := false
					for _, ds := range dataSources {
						if ds.UUID == doc.DataSourceID {
							found = true
							// Check LastDataSourceIndexingJob first (new API field), then fallback to LastIndexingJob
							var dsStatus string
							if ds.LastDataSourceIndexingJob != nil {
								dsStatus = ds.LastDataSourceIndexingJob.Status
							} else if ds.LastIndexingJob != nil {
								dsStatus = ds.LastIndexingJob.Status
							}

							if dsStatus != "" {
								log.Printf("[CRON] Job %d: Document %d (ds=%s) indexing status: %s", job.ID, doc.ID, doc.DataSourceID, dsStatus)
								switch dsStatus {
								case "INDEX_JOB_STATUS_COMPLETED", "DATA_SOURCE_STATUS_UPDATED":
									indexedCount++
								case "INDEX_JOB_STATUS_FAILED":
									anyFailed = true
								default:
									allIndexed = false
									pendingCount++
								}
							} else {
								log.Printf("[CRON] Job %d: Document %d (ds=%s) has NO indexing job status yet", job.ID, doc.ID, doc.DataSourceID)
								allIndexed = false
								pendingCount++
							}
							break
						}
					}
					if !found {
						log.Printf("[CRON] Job %d: Document %d data_source_id %s NOT FOUND in KB data sources", job.ID, doc.ID, doc.DataSourceID)
					}
				}

				log.Printf("[CRON] Job %d: indexedCount=%d, pendingCount=%d, allIndexed=%v, anyFailed=%v", job.ID, indexedCount, pendingCount, allIndexed, anyFailed)

				if allIndexed {
					if anyFailed {
						newStatus = model.IndexingJobStatusPartial
					} else if job.FailedItems > 0 {
						newStatus = model.IndexingJobStatusPartial
					} else {
						newStatus = model.IndexingJobStatusCompleted
					}
					isComplete = true
					completed++
				} else if pendingCount > 0 && job.DOIndexingJobUUID == "" {
					// Data sources exist but haven't been indexed yet
					// Try to start an indexing job for them
					var pendingDataSourceUUIDs []string
					for _, doc := range documents {
						if doc.DataSourceID != "" {
							// Check if this data source needs indexing
							for _, ds := range dataSources {
								if ds.UUID == doc.DataSourceID && ds.LastIndexingJob == nil {
									pendingDataSourceUUIDs = append(pendingDataSourceUUIDs, doc.DataSourceID)
									break
								}
							}
						}
					}

					if len(pendingDataSourceUUIDs) > 0 {
						log.Printf("[CRON] Job %d: Attempting to start indexing job for %d pending data sources", job.ID, len(pendingDataSourceUUIDs))
						indexJob, err := doClient.StartIndexingJob(ctx, digitalocean.StartIndexingJobRequest{
							KnowledgeBaseUUID: subject.KnowledgeBaseUUID,
							DataSourceUUIDs:   pendingDataSourceUUIDs,
						})
						if err != nil {
							log.Printf("[CRON] Job %d: Failed to start indexing job (may be rate limited): %v", job.ID, err)
						} else {
							log.Printf("[CRON] Job %d: Started indexing job %s", job.ID, indexJob.UUID)
							// Update job with DO indexing job UUID
							m.db.Model(&model.IndexingJob{}).Where("id = ?", job.ID).Update("do_indexing_job_uuid", indexJob.UUID)
						}
					}
					continue
				} else {
					// Still indexing
					continue
				}
			}
		}

		if !isComplete {
			continue
		}

		// Update job status
		now := time.Now()
		m.db.Model(&model.IndexingJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
			"status":       newStatus,
			"completed_at": &now,
		})

		// Update documents indexing status for this job
		var items []model.IndexingJobItem
		m.db.Where("job_id = ?", job.ID).Find(&items)

		for _, item := range items {
			if item.DocumentID != nil {
				docStatus := model.IndexingStatusCompleted
				if newStatus == model.IndexingJobStatusFailed {
					docStatus = model.IndexingStatusFailed
				}
				m.db.Model(&model.Document{}).Where("id = ?", *item.DocumentID).Update("indexing_status", docStatus)
			}

			// Update PYQ paper extraction status
			if item.PYQPaperID != nil {
				pyqStatus := model.PYQExtractionCompleted
				if newStatus == model.IndexingJobStatusFailed {
					pyqStatus = model.PYQExtractionFailed
				}
				m.db.Model(&model.PYQPaper{}).Where("id = ?", *item.PYQPaperID).Update("extraction_status", pyqStatus)
				log.Printf("[CRON] Updated PYQ paper %d extraction_status to %s", *item.PYQPaperID, pyqStatus)
			}
		}

		// Get subject for notification
		var subject model.Subject
		m.db.First(&subject, job.SubjectID)

		// Update notification to final status
		var title, message string
		var notificationType model.NotificationType

		if newStatus == model.IndexingJobStatusCompleted {
			notificationType = model.NotificationTypeSuccess
			title = "PYQ Papers Ready"
			message = fmt.Sprintf("Successfully indexed %d papers for %s. You can now chat with the AI about these papers!", job.CompletedItems, subject.Name)
		} else if newStatus == model.IndexingJobStatusPartial {
			notificationType = model.NotificationTypeWarning
			title = "PYQ Papers Partially Ready"
			message = fmt.Sprintf("Indexed %d papers for %s (%d failed). You can chat with the AI about the indexed papers.", job.CompletedItems, subject.Name, job.FailedItems)
		} else {
			notificationType = model.NotificationTypeError
			title = "PYQ Indexing Failed"
			message = fmt.Sprintf("Failed to index papers for %s. Please try uploading again.", subject.Name)
		}

		// Find and update notification
		m.db.Model(&model.UserNotification{}).
			Where("indexing_job_id = ?", job.ID).
			Updates(map[string]interface{}{
				"type":       notificationType,
				"title":      title,
				"message":    message,
				"updated_at": time.Now(),
			})

		updated++
		log.Printf("[CRON] Updated batch ingest job %d to status %s", job.ID, newStatus)
	}

	m.logJobComplete(jobName, fmt.Sprintf("Checked %d jobs, updated %d, completed %d, failed %d", len(jobs), updated, completed, failed))
}

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

		// Update document status based on last indexing job status
		var newStatus model.IndexingStatus
		var errorMsg string

		if dataSource.LastIndexingJob != nil {
			switch dataSource.LastIndexingJob.Status {
			case "INDEX_JOB_STATUS_PENDING":
				newStatus = model.IndexingStatusPending
			case "INDEX_JOB_STATUS_IN_PROGRESS":
				newStatus = model.IndexingStatusInProgress
			case "INDEX_JOB_STATUS_COMPLETED":
				newStatus = model.IndexingStatusCompleted
			case "INDEX_JOB_STATUS_FAILED":
				newStatus = model.IndexingStatusFailed
				errorMsg = "Indexing failed on DigitalOcean"
			default:
				newStatus = model.IndexingStatusPending
			}
		} else {
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
