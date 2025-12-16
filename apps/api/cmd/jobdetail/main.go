package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Get job ID from args or use default
	jobID := uint(17) // Default to latest job
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &jobID)
	}

	// Connect to database
	db, err := connectDatabase()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Get job with items
	var job model.IndexingJob
	err = db.Preload("Items").Preload("Subject").First(&job, jobID).Error
	if err != nil {
		log.Fatalf("Failed to find job %d: %v", jobID, err)
	}

	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Printf("  INDEXING JOB #%d - DETAILED TIMING REPORT\n", job.ID)
	fmt.Println("══════════════════════════════════════════════════════════════")

	// Job metadata
	fmt.Printf("\n📋 JOB METADATA:\n")
	fmt.Printf("   Type:        %s\n", job.JobType)
	fmt.Printf("   Status:      %s\n", job.Status)
	fmt.Printf("   Subject:     %s (ID: %d)\n", job.Subject.Name, job.SubjectID)
	fmt.Printf("   User ID:     %d\n", job.CreatedByUserID)
	fmt.Printf("   Total Items: %d\n", job.TotalItems)
	fmt.Printf("   Completed:   %d\n", job.CompletedItems)
	fmt.Printf("   Failed:      %d\n", job.FailedItems)

	// Timing
	fmt.Printf("\n⏱️  TIMING:\n")
	fmt.Printf("   Created At:   %s\n", job.CreatedAt.Format("2006-01-02 15:04:05.000"))
	if job.StartedAt != nil {
		fmt.Printf("   Started At:   %s\n", job.StartedAt.Format("2006-01-02 15:04:05.000"))
		fmt.Printf("   Queue Time:   %s\n", job.StartedAt.Sub(job.CreatedAt))
	}
	if job.CompletedAt != nil {
		fmt.Printf("   Completed At: %s\n", job.CompletedAt.Format("2006-01-02 15:04:05.000"))
		if job.StartedAt != nil {
			processingTime := job.CompletedAt.Sub(*job.StartedAt)
			fmt.Printf("   Processing:   %s\n", processingTime)
			if job.TotalItems > 0 {
				avgPerItem := processingTime / time.Duration(job.TotalItems)
				fmt.Printf("   Avg/Item:     %s\n", avgPerItem)
			}
		}
		totalTime := job.CompletedAt.Sub(job.CreatedAt)
		fmt.Printf("   Total Time:   %s\n", totalTime)
	}

	// DO Indexing Job (if applicable)
	if job.DOIndexingJobUUID != "" {
		fmt.Printf("\n🌐 DIGITALOCEAN KB INDEXING:\n")
		fmt.Printf("   DO Job UUID:  %s\n", job.DOIndexingJobUUID)
	}

	// Items detail
	fmt.Printf("\n📦 ITEMS DETAIL (%d items):\n", len(job.Items))
	fmt.Println("   ┌──────┬────────────┬──────────────────────────────────────────────────┐")
	fmt.Println("   │  #   │   Status   │  Source URL                                      │")
	fmt.Println("   ├──────┼────────────┼──────────────────────────────────────────────────┤")

	for i, item := range job.Items {
		statusIcon := "⏳"
		switch item.Status {
		case model.IndexingJobItemStatusCompleted:
			statusIcon = "✅"
		case model.IndexingJobItemStatusFailed:
			statusIcon = "❌"
		case model.IndexingJobItemStatusDownloading:
			statusIcon = "⬇️"
		case model.IndexingJobItemStatusUploading:
			statusIcon = "⬆️"
		case model.IndexingJobItemStatusIndexing:
			statusIcon = "🔄"
		}

		// Truncate URL
		url := item.SourceURL
		if len(url) > 45 {
			url = url[:42] + "..."
		}

		fmt.Printf("   │  %d   │ %s %-8s │  %-45s │\n", i+1, statusIcon, item.Status, url)

		// Parse metadata
		if len(item.Metadata) > 0 {
			var meta model.IndexingJobItemMetadata
			if err := json.Unmarshal(item.Metadata, &meta); err == nil {
				fmt.Printf("   │      │            │  📄 %s (%d %s)                    │\n",
					truncate(meta.Title, 20), meta.Year, truncate(meta.Month, 8))
			}
		}

		// Show error if any
		if item.ErrorMessage != "" {
			fmt.Printf("   │      │            │  ⚠️  Error: %-33s │\n", truncate(item.ErrorMessage, 33))
		}

		// Show document/pyq IDs if created
		if item.DocumentID != nil || item.PYQPaperID != nil {
			docStr := "nil"
			pyqStr := "nil"
			if item.DocumentID != nil {
				docStr = fmt.Sprintf("%d", *item.DocumentID)
			}
			if item.PYQPaperID != nil {
				pyqStr = fmt.Sprintf("%d", *item.PYQPaperID)
			}
			fmt.Printf("   │      │            │  📑 Doc: %s, PYQ: %s                         │\n", docStr, pyqStr)
		}
	}
	fmt.Println("   └──────┴────────────┴──────────────────────────────────────────────────┘")

	// Summary
	fmt.Println("\n══════════════════════════════════════════════════════════════")
	if job.Status == model.IndexingJobStatusCompleted {
		fmt.Println("  ✅ JOB COMPLETED SUCCESSFULLY")
	} else if job.Status == model.IndexingJobStatusPartial {
		fmt.Println("  ⚠️  JOB PARTIALLY COMPLETED")
	} else if job.Status == model.IndexingJobStatusFailed {
		fmt.Println("  ❌ JOB FAILED")
		if job.ErrorMessage != "" {
			fmt.Printf("     Error: %s\n", job.ErrorMessage)
		}
	} else {
		fmt.Printf("  ⏳ JOB STATUS: %s\n", job.Status)
	}
	fmt.Println("══════════════════════════════════════════════════════════════")
}

func connectDatabase() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER_NAME", "postgres"),
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_NAME", "study_in_woods"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_SSL_MODE", "disable"),
	)

	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
