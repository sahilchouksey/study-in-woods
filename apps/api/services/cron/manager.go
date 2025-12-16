package cron

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/gorm"
)

// CronManager manages all scheduled cron jobs
type CronManager struct {
	cron *cron.Cron
	db   *gorm.DB
}

// NewCronManager creates a new cron manager
func NewCronManager(db *gorm.DB) *CronManager {
	// Create cron with seconds precision
	c := cron.New(cron.WithSeconds())

	return &CronManager{
		cron: c,
		db:   db,
	}
}

// Start starts all cron jobs
func (m *CronManager) Start() error {
	log.Println("Starting cron jobs...")

	// Register all jobs
	if err := m.registerJobs(); err != nil {
		return err
	}

	// Start the cron scheduler
	m.cron.Start()

	log.Println("Cron jobs started successfully")
	return nil
}

// Stop stops all cron jobs
func (m *CronManager) Stop() {
	log.Println("Stopping cron jobs...")
	ctx := m.cron.Stop()
	<-ctx.Done()
	log.Println("Cron jobs stopped")
}

// registerJobs registers all cron jobs with their schedules
func (m *CronManager) registerJobs() error {
	// 0. Every 2 minutes: Check batch ingest KB indexing status (HIGH PRIORITY)
	_, err := m.cron.AddFunc("0 */2 * * * *", func() {
		m.logJobStart("check_batch_ingest_kb_indexing")
		m.CheckBatchIngestKBIndexing()
	})
	if err != nil {
		return err
	}

	// 1. Every 15 minutes: Check document indexing status
	_, err = m.cron.AddFunc("0 */15 * * * *", func() {
		m.logJobStart("check_document_indexing")
		m.CheckDocumentIndexingStatus()
	})
	if err != nil {
		return err
	}

	// 2. Every 30 minutes: Cleanup pending uploads
	_, err = m.cron.AddFunc("0 */30 * * * *", func() {
		m.logJobStart("cleanup_pending_uploads")
		m.CleanupPendingUploads()
	})
	if err != nil {
		return err
	}

	// 3. Every hour: Aggregate usage statistics
	_, err = m.cron.AddFunc("0 0 * * * *", func() {
		m.logJobStart("aggregate_statistics")
		m.AggregateUsageStatistics()
	})
	if err != nil {
		return err
	}

	// 4. Every 6 hours: Sync DigitalOcean model UUIDs
	_, err = m.cron.AddFunc("0 0 */6 * * *", func() {
		m.logJobStart("sync_do_models")
		m.SyncDigitalOceanModels()
	})
	if err != nil {
		return err
	}

	// 5. Daily at 2 AM: Cleanup old data
	_, err = m.cron.AddFunc("0 0 2 * * *", func() {
		m.logJobStart("cleanup_old_data")
		m.CleanupOldData()
	})
	if err != nil {
		return err
	}

	// 6. Daily at 6:30 AM: Sync DigitalOcean config
	_, err = m.cron.AddFunc("0 30 6 * * *", func() {
		m.logJobStart("sync_do_config")
		m.SyncDigitalOceanConfig()
	})
	if err != nil {
		return err
	}

	log.Println("All cron jobs registered successfully")
	return nil
}

// logJobStart logs the start of a cron job
func (m *CronManager) logJobStart(jobName string) {
	log.Printf("[CRON] Starting job: %s at %s", jobName, time.Now().Format(time.RFC3339))

	// Log to database
	cronLog := model.CronJobLog{
		JobName:   jobName,
		Status:    "running",
		StartedAt: time.Now(),
		Metadata:  "{}",
	}
	m.db.Create(&cronLog)
}

// logJobComplete logs successful completion of a cron job
func (m *CronManager) logJobComplete(jobName string, message string) {
	log.Printf("[CRON] Completed job: %s - %s", jobName, message)

	// Update database log
	m.db.Model(&model.CronJobLog{}).
		Where("job_name = ? AND status = ?", jobName, "running").
		Order("started_at DESC").
		Limit(1).
		Updates(map[string]interface{}{
			"status":       "completed",
			"completed_at": time.Now(),
			"message":      message,
		})
}

// logJobError logs a cron job error
func (m *CronManager) logJobError(jobName string, err error) {
	log.Printf("[CRON] Error in job: %s - %v", jobName, err)

	// Update database log
	m.db.Model(&model.CronJobLog{}).
		Where("job_name = ? AND status = ?", jobName, "running").
		Order("started_at DESC").
		Limit(1).
		Updates(map[string]interface{}{
			"status":       "failed",
			"completed_at": time.Now(),
			"error_msg":    err.Error(),
		})
}
