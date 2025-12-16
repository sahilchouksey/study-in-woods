//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Models for database queries
type IndexingJob struct {
	ID             uint `gorm:"primaryKey"`
	SubjectID      uint
	JobType        string
	Status         string
	TotalItems     int
	CompletedItems int
	FailedItems    int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	ErrorMessage   string
}

func (IndexingJob) TableName() string { return "indexing_jobs" }

type IndexingJobItem struct {
	ID           uint `gorm:"primaryKey"`
	JobID        uint
	Status       string
	SourceURL    string
	DocumentID   *uint
	PYQPaperID   *uint
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (IndexingJobItem) TableName() string { return "indexing_job_items" }

type PYQPaper struct {
	ID               uint `gorm:"primaryKey"`
	SubjectID        uint
	DocumentID       uint
	Year             int
	Month            string
	ExtractionStatus string
	TotalQuestions   int
	TotalMarks       int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (PYQPaper) TableName() string { return "pyq_papers" }

type Document struct {
	ID             uint `gorm:"primaryKey"`
	SubjectID      *uint
	Type           string
	Filename       string
	SpacesURL      string
	SpacesKey      string
	IndexingStatus string
	CreatedAt      time.Time
}

func (Document) TableName() string { return "documents" }

type UserNotification struct {
	ID            uint `gorm:"primaryKey"`
	UserID        uint
	Type          string
	Category      string
	Title         string
	Message       string
	IndexingJobID *uint
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (UserNotification) TableName() string { return "user_notifications" }

// API response types
type BatchIngestResponse struct {
	Success bool `json:"success"`
	Data    struct {
		JobID      uint   `json:"job_id"`
		Status     string `json:"status"`
		TotalItems int    `json:"total_items"`
		Message    string `json:"message"`
	} `json:"data"`
}

var db *gorm.DB

func main() {
	godotenv.Load()

	// Connect to database
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbURL := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, os.Getenv("DB_USER_NAME"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))

	var err error
	db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          BATCH INGEST MONITORING TEST                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	// Step 1: Clean up existing data
	fmt.Println("\n📋 Step 1: Cleaning up existing test data...")
	cleanup()

	// Step 2: Trigger batch ingest via API
	fmt.Println("\n📋 Step 2: Triggering batch ingest via API...")
	jobID := triggerBatchIngest()
	if jobID == 0 {
		log.Fatal("Failed to start batch ingest")
	}
	fmt.Printf("   ✅ Job created with ID: %d\n", jobID)

	// Step 3: Monitor the job until completion (max 5 minutes)
	fmt.Println("\n📋 Step 3: Monitoring job progress (max 5 minutes)...")
	monitorJob(jobID, 5*time.Minute)

	// Step 4: Final status check
	fmt.Println("\n📋 Step 4: Final database state...")
	printFinalState(jobID)
}

func cleanup() {
	db.Exec("DELETE FROM indexing_job_items")
	db.Exec("DELETE FROM indexing_jobs")
	db.Exec("DELETE FROM pyq_papers")
	db.Exec("DELETE FROM documents WHERE type = 'pyq'")
	db.Exec("DELETE FROM user_notifications WHERE category = 'pyq_ingest'")
	fmt.Println("   ✅ Cleanup complete")
}

func triggerBatchIngest() uint {
	// Use subject ID 643 (Data Mining) and user token
	// You need to provide a valid auth token
	authToken := os.Getenv("TEST_AUTH_TOKEN")
	if authToken == "" {
		fmt.Println("   ⚠️  TEST_AUTH_TOKEN not set, using hardcoded test")
		// For testing, we'll insert directly into database instead
		return triggerBatchIngestDirect()
	}

	payload := map[string]interface{}{
		"papers": []map[string]interface{}{
			{
				"pdf_url":     "https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.pdf",
				"title":       "MCA-301-Data-Mining-Dec-2024.pdf",
				"year":        2024,
				"month":       "December",
				"exam_type":   "End Semester",
				"source_name": "RGPV Online",
			},
			{
				"pdf_url":     "https://www.rgpvonline.com/papers/mca-301-data-mining-may-2024.pdf",
				"title":       "MCA-301-Data-Mining-May-2024.pdf",
				"year":        2024,
				"month":       "May",
				"exam_type":   "End Semester",
				"source_name": "RGPV Online",
			},
		},
		"trigger_extraction": false, // Set to true to test extraction
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/subjects/643/pyqs/batch-ingest", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("   ❌ API request failed: %v\n", err)
		return 0
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result BatchIngestResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("   ❌ Failed to parse response: %v\n", err)
		return 0
	}

	if !result.Success {
		fmt.Printf("   ❌ API returned error: %s\n", string(body))
		return 0
	}

	return result.Data.JobID
}

func triggerBatchIngestDirect() uint {
	// Check if there's already a recent job we can monitor
	var job IndexingJob
	if err := db.Order("created_at DESC").First(&job).Error; err == nil {
		fmt.Printf("   📌 Found existing job %d (status: %s)\n", job.ID, job.Status)
		return job.ID
	}

	fmt.Println("   ⚠️  No existing jobs found. Please trigger batch ingest from frontend.")
	fmt.Println("   📌 Waiting for a job to appear...")

	// Wait for a job to appear (from frontend trigger)
	for i := 0; i < 60; i++ { // Wait up to 60 seconds
		var newJob IndexingJob
		if err := db.Order("created_at DESC").First(&newJob).Error; err == nil {
			fmt.Printf("\n   ✅ Detected new job %d!\n", newJob.ID)
			return newJob.ID
		}
		fmt.Print(".")
		time.Sleep(1 * time.Second)
	}

	return 0
}

func monitorJob(jobID uint, maxDuration time.Duration) {
	startTime := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastStatus := ""
	lastProgress := -1

	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(startTime)
			if elapsed > maxDuration {
				fmt.Println("\n   ⏰ Timeout reached!")
				return
			}

			var job IndexingJob
			if err := db.First(&job, jobID).Error; err != nil {
				fmt.Printf("\n   ❌ Failed to fetch job: %v\n", err)
				continue
			}

			progress := 0
			if job.TotalItems > 0 {
				progress = ((job.CompletedItems + job.FailedItems) * 100) / job.TotalItems
			}

			// Only print if something changed
			if job.Status != lastStatus || progress != lastProgress {
				fmt.Printf("\n   [%s] Job %d: status=%s, progress=%d%% (%d/%d done, %d failed)",
					elapsed.Round(time.Second), jobID, job.Status, progress,
					job.CompletedItems, job.TotalItems, job.FailedItems)

				// Print item statuses
				var items []IndexingJobItem
				db.Where("job_id = ?", jobID).Find(&items)
				if len(items) > 0 {
					fmt.Println()
					for i, item := range items {
						icon := "○"
						switch item.Status {
						case "completed":
							icon = "●"
						case "failed":
							icon = "✗"
						case "downloading":
							icon = "↓"
						case "uploading":
							icon = "↑"
						case "indexing":
							icon = "⚡"
						}
						fmt.Printf("      %s Item %d: %s", icon, i+1, item.Status)
						if item.DocumentID != nil {
							fmt.Printf(" (doc:%d)", *item.DocumentID)
						}
						if item.PYQPaperID != nil {
							fmt.Printf(" (pyq:%d)", *item.PYQPaperID)
						}
						if item.ErrorMessage != "" {
							fmt.Printf(" ERROR: %s", item.ErrorMessage)
						}
						fmt.Println()
					}
				}

				lastStatus = job.Status
				lastProgress = progress
			} else {
				fmt.Print(".")
			}

			// Check if job is complete
			if job.Status == "completed" || job.Status == "failed" ||
				job.Status == "partially_completed" || job.Status == "cancelled" {
				fmt.Printf("\n   ✅ Job finished with status: %s (took %s)\n", job.Status, elapsed.Round(time.Second))
				return
			}
		}
	}
}

func printFinalState(jobID uint) {
	fmt.Println("\n┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│                    FINAL DATABASE STATE                     │")
	fmt.Println("└─────────────────────────────────────────────────────────────┘")

	// Indexing Job
	var job IndexingJob
	if err := db.First(&job, jobID).Error; err == nil {
		fmt.Printf("\n📦 Indexing Job #%d:\n", job.ID)
		fmt.Printf("   Status: %s\n", job.Status)
		fmt.Printf("   Progress: %d/%d completed, %d failed\n", job.CompletedItems, job.TotalItems, job.FailedItems)
		if job.StartedAt != nil {
			fmt.Printf("   Started: %s\n", job.StartedAt.Format("15:04:05"))
		}
		if job.CompletedAt != nil {
			fmt.Printf("   Completed: %s\n", job.CompletedAt.Format("15:04:05"))
		}
		if job.ErrorMessage != "" {
			fmt.Printf("   Error: %s\n", job.ErrorMessage)
		}
	}

	// Job Items
	var items []IndexingJobItem
	db.Where("job_id = ?", jobID).Find(&items)
	if len(items) > 0 {
		fmt.Printf("\n📄 Job Items (%d):\n", len(items))
		for i, item := range items {
			fmt.Printf("   %d. [%s] %s\n", i+1, item.Status, truncate(item.SourceURL, 50))
			if item.DocumentID != nil {
				fmt.Printf("      → Document ID: %d\n", *item.DocumentID)
			}
			if item.PYQPaperID != nil {
				fmt.Printf("      → PYQ Paper ID: %d\n", *item.PYQPaperID)
			}
			if item.ErrorMessage != "" {
				fmt.Printf("      → Error: %s\n", item.ErrorMessage)
			}
		}
	}

	// Documents created
	var docs []Document
	db.Where("type = ?", "pyq").Order("created_at DESC").Find(&docs)
	if len(docs) > 0 {
		fmt.Printf("\n📑 PYQ Documents Created (%d):\n", len(docs))
		for i, doc := range docs {
			fmt.Printf("   %d. [%d] %s\n", i+1, doc.ID, doc.Filename)
			fmt.Printf("      Indexing Status: %s\n", doc.IndexingStatus)
			if doc.SpacesURL != "" {
				fmt.Printf("      Spaces URL: %s\n", truncate(doc.SpacesURL, 60))
			}
		}
	} else {
		fmt.Println("\n📑 PYQ Documents: None created")
	}

	// PYQ Papers created
	var papers []PYQPaper
	db.Order("created_at DESC").Find(&papers)
	if len(papers) > 0 {
		fmt.Printf("\n📝 PYQ Papers Created (%d):\n", len(papers))
		for i, paper := range papers {
			fmt.Printf("   %d. [%d] %s %d - Doc:%d\n", i+1, paper.ID, paper.Month, paper.Year, paper.DocumentID)
			fmt.Printf("      Extraction Status: %s\n", paper.ExtractionStatus)
			fmt.Printf("      Questions: %d, Marks: %d\n", paper.TotalQuestions, paper.TotalMarks)
		}
	} else {
		fmt.Println("\n📝 PYQ Papers: None created")
	}

	// Notifications
	var notifications []UserNotification
	db.Where("category = ?", "pyq_ingest").Order("updated_at DESC").Limit(5).Find(&notifications)
	if len(notifications) > 0 {
		fmt.Printf("\n🔔 Recent Notifications (%d):\n", len(notifications))
		for i, n := range notifications {
			fmt.Printf("   %d. [%s] %s\n", i+1, n.Type, n.Title)
			fmt.Printf("      Message: %s\n", n.Message)
			if n.IndexingJobID != nil {
				fmt.Printf("      Job ID: %d\n", *n.IndexingJobID)
			}
			fmt.Printf("      Updated: %s\n", n.UpdatedAt.Format("15:04:05"))
		}
	} else {
		fmt.Println("\n🔔 Notifications: None")
	}

	fmt.Println("\n════════════════════════════════════════════════════════════════")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
