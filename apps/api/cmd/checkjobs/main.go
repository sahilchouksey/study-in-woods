package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// IndexingJob mirrors the model for checking
type IndexingJob struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	SubjectID         *uint
	JobType           string
	Status            string
	TotalItems        int
	CompletedItems    int
	FailedItems       int
	DOIndexingJobUUID string
	CreatedByUserID   uint
	StartedAt         *time.Time
	CompletedAt       *time.Time
	ErrorMessage      string
}

func (IndexingJob) TableName() string {
	return "indexing_jobs"
}

// IndexingJobItem mirrors the model for checking
type IndexingJobItem struct {
	ID           uint `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	JobID        uint
	ItemType     string
	SourceURL    string
	DocumentID   *uint
	PYQPaperID   *uint
	Status       string
	ErrorMessage string
}

func (IndexingJobItem) TableName() string {
	return "indexing_job_items"
}

// UserNotification mirrors the model for checking
type UserNotification struct {
	ID            uint `gorm:"primaryKey"`
	UserID        uint
	Type          string
	Category      string
	Title         string
	Message       string
	Read          bool
	IndexingJobID *uint
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (UserNotification) TableName() string {
	return "user_notifications"
}

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Build database URL from individual variables
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER_NAME")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432"
	}

	dbURL := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("========================================")
	fmt.Println("INDEXING JOBS STATUS CHECK")
	fmt.Println("========================================")

	// Get all indexing jobs
	var jobs []IndexingJob
	if err := db.Order("created_at DESC").Limit(20).Find(&jobs).Error; err != nil {
		log.Fatalf("Failed to fetch jobs: %v", err)
	}

	if len(jobs) == 0 {
		fmt.Println("\n❌ No indexing jobs found in database")
	} else {
		fmt.Printf("\n📋 Found %d indexing jobs:\n\n", len(jobs))

		for _, job := range jobs {
			progress := 0
			if job.TotalItems > 0 {
				progress = ((job.CompletedItems + job.FailedItems) * 100) / job.TotalItems
			}

			statusIcon := "⏳"
			switch job.Status {
			case "completed":
				statusIcon = "✅"
			case "failed":
				statusIcon = "❌"
			case "processing":
				statusIcon = "🔄"
			case "partially_completed":
				statusIcon = "⚠️"
			case "cancelled":
				statusIcon = "🚫"
			}

			fmt.Printf("─────────────────────────────────────\n")
			fmt.Printf("%s Job ID: %d\n", statusIcon, job.ID)
			fmt.Printf("   Type: %s\n", job.JobType)
			fmt.Printf("   Status: %s\n", job.Status)
			if job.SubjectID != nil {
				fmt.Printf("   Subject ID: %d\n", *job.SubjectID)
			} else {
				fmt.Printf("   Subject ID: N/A (multi-subject job)\n")
			}
			fmt.Printf("   User ID: %d\n", job.CreatedByUserID)
			fmt.Printf("   Progress: %d%% (%d/%d completed, %d failed)\n",
				progress, job.CompletedItems, job.TotalItems, job.FailedItems)
			fmt.Printf("   Created: %s\n", job.CreatedAt.Format("2006-01-02 15:04:05"))
			if job.StartedAt != nil {
				fmt.Printf("   Started: %s\n", job.StartedAt.Format("2006-01-02 15:04:05"))
			}
			if job.CompletedAt != nil {
				fmt.Printf("   Completed: %s\n", job.CompletedAt.Format("2006-01-02 15:04:05"))
			}
			if job.ErrorMessage != "" {
				fmt.Printf("   Error: %s\n", job.ErrorMessage)
			}

			// Get items for this job
			var items []IndexingJobItem
			db.Where("job_id = ?", job.ID).Find(&items)
			if len(items) > 0 {
				fmt.Printf("   Items (%d):\n", len(items))
				for _, item := range items {
					itemIcon := "○"
					switch item.Status {
					case "completed":
						itemIcon = "●"
					case "failed":
						itemIcon = "✗"
					case "downloading", "uploading", "indexing":
						itemIcon = "◐"
					}
					fmt.Printf("     %s [%s] %s\n", itemIcon, item.Status, truncate(item.SourceURL, 60))
					if item.ErrorMessage != "" {
						fmt.Printf("       Error: %s\n", item.ErrorMessage)
					}
				}
			}
		}
	}

	// Check active jobs (pending or processing)
	var activeJobs []IndexingJob
	db.Where("status IN ?", []string{"pending", "processing"}).Find(&activeJobs)

	fmt.Println("\n========================================")
	fmt.Printf("ACTIVE JOBS: %d\n", len(activeJobs))
	fmt.Println("========================================")

	if len(activeJobs) > 0 {
		for _, job := range activeJobs {
			subjectStr := "N/A"
			if job.SubjectID != nil {
				subjectStr = fmt.Sprintf("%d", *job.SubjectID)
			}
			fmt.Printf("🔄 Job %d - %s (Subject: %s, User: %d)\n",
				job.ID, job.Status, subjectStr, job.CreatedByUserID)
		}
	} else {
		fmt.Println("No active jobs currently running")
	}

	// Check related notifications
	fmt.Println("\n========================================")
	fmt.Println("RELATED NOTIFICATIONS")
	fmt.Println("========================================")

	var notifications []UserNotification
	db.Where("category = ?", "pyq_ingest").Order("created_at DESC").Limit(10).Find(&notifications)

	if len(notifications) == 0 {
		fmt.Println("No PYQ ingest notifications found")
	} else {
		for _, n := range notifications {
			readIcon := "○"
			if n.Read {
				readIcon = "●"
			}
			jobIDStr := "N/A"
			if n.IndexingJobID != nil {
				jobIDStr = fmt.Sprintf("%d", *n.IndexingJobID)
			}
			fmt.Printf("%s [%s] Job:%s - %s: %s\n",
				readIcon, n.Type, jobIDStr, n.Title, truncate(n.Message, 50))
		}
	}

	fmt.Println("\n========================================")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
