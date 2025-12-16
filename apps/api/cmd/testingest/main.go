package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	log.Println("==============================================")
	log.Println("  Batch Ingest Polling Integration Test")
	log.Println("==============================================")

	// 1. Connect to database
	log.Println("\n[Step 1] Connecting to database...")
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("✓ Database connected")

	// 2. Initialize services
	log.Println("\n[Step 2] Initializing services...")
	notificationService := services.NewNotificationService(db)
	pyqService := services.NewPYQService(db)
	batchIngestService := services.NewBatchIngestService(db, notificationService, pyqService)
	log.Println("✓ Services initialized")

	// 3. Find or create test user
	log.Println("\n[Step 3] Setting up test user...")
	user, err := getOrCreateTestUser(db)
	if err != nil {
		log.Fatalf("Failed to setup test user: %v", err)
	}
	log.Printf("✓ Using user: ID=%d, Email=%s", user.ID, user.Email)

	// 4. Find a subject
	log.Println("\n[Step 4] Finding subject...")
	subject, err := findSubject(db)
	if err != nil {
		log.Fatalf("Failed to find subject: %v", err)
	}
	log.Printf("✓ Using subject: ID=%d, Name=%s", subject.ID, subject.Name)

	// 5. Create mock PDF server
	log.Println("\n[Step 5] Creating mock PDF server...")
	pdfContent, err := loadTestPDF()
	if err != nil {
		log.Printf("Warning: Could not load test PDF: %v", err)
		log.Println("Using minimal PDF content for testing")
		pdfContent = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")
	}
	mockServer := createMockPDFServer(pdfContent)
	defer mockServer.Close()
	log.Printf("✓ Mock PDF server running at: %s", mockServer.URL)

	// 6. Start batch ingest
	log.Println("\n[Step 6] Starting batch ingest...")
	ctx := context.Background()

	timestamp := time.Now().Unix()
	req := services.BatchIngestRequest{
		SubjectID: subject.ID,
		UserID:    user.ID,
		Papers: []services.BatchIngestPaperRequest{
			{
				PDFURL:     mockServer.URL + "/paper1.pdf",
				Title:      fmt.Sprintf("Test-Paper-Dec-2024-%d", timestamp),
				Year:       2024,
				Month:      fmt.Sprintf("December-%d", timestamp), // Unique to avoid duplicates
				ExamType:   "End Semester",
				SourceName: "Integration Test",
			},
			{
				PDFURL:     mockServer.URL + "/paper2.pdf",
				Title:      fmt.Sprintf("Test-Paper-May-2024-%d", timestamp),
				Year:       2024,
				Month:      fmt.Sprintf("May-%d", timestamp),
				ExamType:   "End Semester",
				SourceName: "Integration Test",
			},
			{
				PDFURL:     mockServer.URL + "/paper3.pdf",
				Title:      fmt.Sprintf("Test-Paper-Nov-2023-%d", timestamp),
				Year:       2023,
				Month:      fmt.Sprintf("November-%d", timestamp),
				ExamType:   "End Semester",
				SourceName: "Integration Test",
			},
		},
	}

	result, err := batchIngestService.StartBatchIngest(ctx, req)
	if err != nil {
		log.Fatalf("Failed to start batch ingest: %v", err)
	}

	log.Printf("✓ Job started: ID=%d, Status=%s, TotalItems=%d", result.JobID, result.Status, result.TotalItems)

	// 7. Poll job status (simulating frontend polling)
	log.Println("\n[Step 7] Polling job status (simulating frontend)...")
	log.Println("─────────────────────────────────────────────────")

	pollInterval := 2 * time.Second
	maxPolls := 60
	pollCount := 0

	var finalJob *model.IndexingJob
	for pollCount < maxPolls {
		pollCount++
		time.Sleep(pollInterval)

		job, err := batchIngestService.GetJobStatus(ctx, result.JobID, user.ID)
		if err != nil {
			log.Printf("[Poll %d] Error getting status: %v", pollCount, err)
			continue
		}

		progress := job.GetProgress()
		log.Printf("[Poll %d] Status: %-12s | Progress: %3d%% | Completed: %d | Failed: %d | Total: %d",
			pollCount, job.Status, progress, job.CompletedItems, job.FailedItems, job.TotalItems)

		// Check if complete
		if job.IsComplete() {
			finalJob = job
			break
		}
	}

	log.Println("─────────────────────────────────────────────────")

	if pollCount >= maxPolls {
		log.Println("⚠ Polling timed out")
	}

	// 8. Verify final state
	log.Println("\n[Step 8] Verifying final state...")

	if finalJob == nil {
		finalJob, _ = batchIngestService.GetJobStatus(ctx, result.JobID, user.ID)
	}

	if finalJob != nil {
		log.Printf("  Job Status: %s", finalJob.Status)
		log.Printf("  Completed Items: %d", finalJob.CompletedItems)
		log.Printf("  Failed Items: %d", finalJob.FailedItems)
		log.Printf("  Total Items: %d", finalJob.TotalItems)
		log.Printf("  Progress: %d%%", finalJob.GetProgress())

		if finalJob.DOIndexingJobUUID != "" {
			log.Printf("  DO Indexing Job UUID: %s", finalJob.DOIndexingJobUUID)
		}
	}

	// 9. Check notifications
	log.Println("\n[Step 9] Checking notifications...")

	notifications, total, err := notificationService.GetNotificationsByUser(ctx, services.ListNotificationsOptions{
		UserID: user.ID,
		Limit:  5,
	})
	if err != nil {
		log.Printf("Warning: Failed to get notifications: %v", err)
	} else {
		log.Printf("  Total notifications: %d", total)
		for _, n := range notifications {
			if n.IndexingJobID != nil && *n.IndexingJobID == result.JobID {
				log.Printf("  ✓ Found job notification:")
				log.Printf("    - ID: %d", n.ID)
				log.Printf("    - Type: %s", n.Type)
				log.Printf("    - Title: %s", n.Title)
				log.Printf("    - Category: %s", n.Category)
			}
		}
	}

	// 10. Check created documents and PYQ papers
	log.Println("\n[Step 10] Checking created records...")

	var docCount int64
	db.Model(&model.Document{}).Where("subject_id = ?", subject.ID).Count(&docCount)
	log.Printf("  Documents for subject: %d", docCount)

	var pyqCount int64
	db.Model(&model.PYQPaper{}).Where("subject_id = ?", subject.ID).Count(&pyqCount)
	log.Printf("  PYQ Papers for subject: %d", pyqCount)

	// Summary
	log.Println("\n==============================================")
	log.Println("  TEST SUMMARY")
	log.Println("==============================================")
	log.Printf("  Job ID: %d", result.JobID)

	if finalJob != nil {
		log.Printf("  Final Status: %s", finalJob.Status)

		switch finalJob.Status {
		case model.IndexingJobStatusCompleted:
			log.Println("  Result: ✅ SUCCESS - All items ingested")
		case model.IndexingJobStatusPartial:
			log.Println("  Result: ⚠️  PARTIAL - Some items failed")
		case model.IndexingJobStatusFailed:
			log.Println("  Result: ❌ FAILED - Job failed")
		default:
			log.Printf("  Result: ⏳ %s", finalJob.Status)
		}
	}

	log.Printf("  Total Polls: %d", pollCount)
	log.Println("==============================================")

	// Cleanup option
	if os.Getenv("CLEANUP") == "true" {
		log.Println("\n[Cleanup] Removing test data...")
		cleanup(db, user.ID, result.JobID, subject.ID)
		log.Println("✓ Cleanup complete")
	} else {
		log.Println("\nTip: Set CLEANUP=true to remove test data after run")
	}
}

func connectDB() (*gorm.DB, error) {
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
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getOrCreateTestUser(db *gorm.DB) (*model.User, error) {
	var user model.User
	testEmail := "batch_ingest_test@test.com"

	err := db.Where("email = ?", testEmail).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		user = model.User{
			Email:        testEmail,
			PasswordHash: "test_hash",
			PasswordSalt: []byte("test_salt"),
			Name:         "Batch Ingest Test User",
			Role:         "admin",
		}
		if err := db.Create(&user).Error; err != nil {
			return nil, err
		}
		log.Printf("  Created new test user: ID=%d", user.ID)
	} else if err != nil {
		return nil, err
	}

	return &user, nil
}

func findSubject(db *gorm.DB) (*model.Subject, error) {
	var subject model.Subject

	// Try to find any subject
	err := db.First(&subject).Error
	if err == gorm.ErrRecordNotFound {
		// Create a minimal subject for testing
		log.Println("  No subjects found, creating test subject...")

		// First need a semester
		var semester model.Semester
		err = db.First(&semester).Error
		if err == gorm.ErrRecordNotFound {
			// Need a course first
			var course model.Course
			err = db.First(&course).Error
			if err == gorm.ErrRecordNotFound {
				// Need a university first
				var university model.University
				err = db.First(&university).Error
				if err == gorm.ErrRecordNotFound {
					university = model.University{
						Name:     "Test University",
						Code:     "TEST_UNIV",
						Location: "Test Location",
					}
					db.Create(&university)
				}

				course = model.Course{
					UniversityID: university.ID,
					Name:         "Test Course",
					Code:         "TEST_COURSE",
					Duration:     4,
				}
				db.Create(&course)
			}

			semester = model.Semester{
				CourseID: course.ID,
				Number:   1,
				Name:     "Test Semester",
			}
			db.Create(&semester)
		}

		subject = model.Subject{
			SemesterID:  semester.ID,
			Name:        "Test Subject for Batch Ingest",
			Code:        fmt.Sprintf("TEST-%d", time.Now().Unix()),
			Credits:     4,
			Description: "Auto-created for batch ingest testing",
		}
		if err := db.Create(&subject).Error; err != nil {
			return nil, fmt.Errorf("failed to create test subject: %w", err)
		}
	} else if err != nil {
		return nil, err
	}

	return &subject, nil
}

func loadTestPDF() ([]byte, error) {
	// Try different paths
	paths := []string{
		"../../../mca-301-data-mining-dec-2024.pdf",
		"../../mca-301-data-mining-dec-2024.pdf",
		"mca-301-data-mining-dec-2024.pdf",
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err == nil {
			log.Printf("  Loaded test PDF from: %s (%.2f KB)", path, float64(len(content))/1024)
			return content, nil
		}
	}

	return nil, fmt.Errorf("test PDF not found in any expected location")
}

func createMockPDFServer(content []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("  [MockServer] Serving: %s (with 1s delay to simulate real download)", r.URL.Path)
		// Add delay to simulate real PDF download and allow polling to capture progress
		time.Sleep(1 * time.Second)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Write(content)
	}))
}

func cleanup(db *gorm.DB, userID uint, jobID uint, subjectID uint) {
	// Delete notifications for user
	db.Where("user_id = ?", userID).Delete(&model.UserNotification{})

	// Delete job items
	db.Where("job_id = ?", jobID).Delete(&model.IndexingJobItem{})

	// Delete job
	db.Delete(&model.IndexingJob{}, jobID)

	// Delete PYQ papers and documents for subject
	db.Where("subject_id = ?", subjectID).Delete(&model.PYQPaper{})
	db.Where("subject_id = ?", subjectID).Delete(&model.Document{})
}
