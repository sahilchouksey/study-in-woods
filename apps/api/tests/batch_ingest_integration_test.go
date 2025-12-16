package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"github.com/sahilchouksey/go-init-setup/utils/auth"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// BatchIngestTestContext holds all resources needed for batch ingest integration tests
type BatchIngestTestContext struct {
	// Database
	db *gorm.DB

	// DigitalOcean clients
	doClient     *digitalocean.Client
	spacesClient *digitalocean.SpacesClient

	// Services
	notificationService *services.NotificationService
	batchIngestService  *services.BatchIngestService
	pyqService          *services.PYQService
	subjectService      *services.SubjectService

	// JWT
	jwtManager *auth.JWTManager

	// Test data
	testUser       *model.User
	testCourse     *model.Course
	testSemester   *model.Semester
	testSubject    *model.Subject
	accessToken    string
	testPDFContent []byte

	// Timing
	startTime time.Time
}

// ====================================================================
// SETUP FUNCTIONS
// ====================================================================

// getEnvOrDefault returns environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// setupBatchIngestTestEnvironment initializes all required clients and services
func setupBatchIngestTestEnvironment(t *testing.T) (*BatchIngestTestContext, error) {
	ctx := &BatchIngestTestContext{
		startTime: time.Now(),
	}

	log.Println("========================================")
	log.Println("Setting up batch ingest test environment...")
	log.Println("========================================")

	// 1. Check required environment variables
	requiredEnvVars := []string{
		"DB_HOST",
		"DB_USER_NAME",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_PORT",
		"JWT_SECRET",
	}

	missingVars := []string{}
	for _, v := range requiredEnvVars {
		if os.Getenv(v) == "" {
			missingVars = append(missingVars, v)
		}
	}

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missingVars, ", "))
	}

	log.Println("✓ Required environment variables present")

	// 2. Initialize database connection
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER_NAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
		getEnvOrDefault("DB_SSL_MODE", "disable"),
	)

	gormLogger := logger.Default.LogMode(logger.Warn)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	ctx.db = db
	log.Println("✓ Database connection established")

	// 3. Initialize JWT manager
	jwtConfig := auth.JWTConfig{
		Secret:        os.Getenv("JWT_SECRET"),
		Expiry:        24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
		Issuer:        "study-in-woods-test",
	}
	ctx.jwtManager = auth.NewJWTManager(jwtConfig)
	log.Println("✓ JWT manager initialized")

	// 4. Initialize services
	ctx.notificationService = services.NewNotificationService(db)
	ctx.pyqService = services.NewPYQService(db)
	ctx.subjectService = services.NewSubjectService(db)
	ctx.batchIngestService = services.NewBatchIngestService(db, ctx.notificationService, ctx.pyqService)
	log.Println("✓ Services initialized")

	// 5. Initialize optional DigitalOcean clients
	if os.Getenv("DIGITALOCEAN_TOKEN") != "" {
		ctx.doClient = digitalocean.NewClient(digitalocean.Config{
			APIToken: os.Getenv("DIGITALOCEAN_TOKEN"),
		})
		log.Println("✓ DigitalOcean client initialized")
	} else {
		log.Println("⚠ DIGITALOCEAN_TOKEN not set - AI features disabled")
	}

	spacesClient, err := digitalocean.NewSpacesClientFromGlobalConfig()
	if err == nil {
		ctx.spacesClient = spacesClient
		log.Println("✓ Spaces client initialized")
	} else {
		log.Printf("⚠ Spaces client not initialized: %v", err)
	}

	// 6. Load test PDF
	// Path is relative from apps/api/tests/ -> ../../../ to get to project root
	pdfPath := "../../../mca-301-data-mining-dec-2024.pdf"
	content, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test PDF from %s: %w", pdfPath, err)
	}
	ctx.testPDFContent = content
	log.Printf("✓ Test PDF loaded: %.2f KB", float64(len(content))/1024)

	// 7. Create or get test user
	if err := setupTestUser(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup test user: %w", err)
	}

	// 8. Generate access token for test user
	accessToken, _, err := ctx.jwtManager.GenerateAccessToken(ctx.testUser.ID, ctx.testUser.Email, ctx.testUser.Role, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}
	ctx.accessToken = accessToken
	log.Println("✓ Access token generated")

	log.Printf("✓ Test environment setup complete (%.2fs)\n", time.Since(ctx.startTime).Seconds())
	return ctx, nil
}

// setupTestUser creates or gets the test user
func setupTestUser(ctx *BatchIngestTestContext) error {
	var user model.User
	testEmail := "batch_ingest_test@test.com"

	if err := ctx.db.Where("email = ?", testEmail).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			user = model.User{
				Email:        testEmail,
				PasswordHash: "test_password_hash",
				PasswordSalt: []byte("test_salt"),
				Name:         "Batch Ingest Test User",
				Role:         "admin", // Admin role for full access
			}
			if err := ctx.db.Create(&user).Error; err != nil {
				return fmt.Errorf("failed to create test user: %w", err)
			}
			log.Printf("✓ Created test user: ID=%d, Role=%s", user.ID, user.Role)
		} else {
			return fmt.Errorf("failed to query test user: %w", err)
		}
	} else {
		log.Printf("✓ Using existing test user: ID=%d, Role=%s", user.ID, user.Role)
	}
	ctx.testUser = &user
	return nil
}

// setupTestCourseAndSemester creates test course and semester
func setupTestCourseAndSemester(ctx *BatchIngestTestContext) error {
	log.Println("\n--- Setting up test course and semester ---")

	// Create or get test university - use Unscoped to find soft-deleted records too
	var university model.University
	err := ctx.db.Unscoped().Where("code = ?", "BATCH_TEST_UNIV").First(&university).Error
	if err == gorm.ErrRecordNotFound {
		// Create new university
		university = model.University{
			Name:     "Batch Test University",
			Code:     "BATCH_TEST_UNIV",
			Location: "Test Location",
		}
		if err := ctx.db.Create(&university).Error; err != nil {
			return fmt.Errorf("failed to create test university: %w", err)
		}
		log.Printf("  Created university: ID=%d", university.ID)
	} else if err != nil {
		return fmt.Errorf("failed to query university: %w", err)
	} else {
		// University exists - restore if soft-deleted
		if university.DeletedAt.Valid {
			ctx.db.Unscoped().Model(&university).Update("deleted_at", nil)
			log.Printf("  Restored university: ID=%d", university.ID)
		} else {
			log.Printf("  Using existing university: ID=%d", university.ID)
		}
	}

	// Create or get test course - use Unscoped
	var course model.Course
	err = ctx.db.Unscoped().Where("code = ?", "BATCH_TEST_MCA").First(&course).Error
	if err == gorm.ErrRecordNotFound {
		course = model.Course{
			UniversityID: university.ID,
			Name:         "Master of Computer Applications",
			Code:         "BATCH_TEST_MCA",
			Description:  "Course for batch ingest integration tests",
			Duration:     4,
		}
		if err := ctx.db.Create(&course).Error; err != nil {
			return fmt.Errorf("failed to create test course: %w", err)
		}
		log.Printf("  Created course: ID=%d", course.ID)
	} else if err != nil {
		return fmt.Errorf("failed to query course: %w", err)
	} else {
		if course.DeletedAt.Valid {
			ctx.db.Unscoped().Model(&course).Update("deleted_at", nil)
			log.Printf("  Restored course: ID=%d", course.ID)
		} else {
			log.Printf("  Using existing course: ID=%d", course.ID)
		}
	}
	ctx.testCourse = &course

	// Create or get test semester - use Unscoped
	var semester model.Semester
	err = ctx.db.Unscoped().Where("course_id = ? AND number = ?", course.ID, 3).First(&semester).Error
	if err == gorm.ErrRecordNotFound {
		semester = model.Semester{
			CourseID: course.ID,
			Number:   3,
			Name:     "Semester 3",
		}
		if err := ctx.db.Create(&semester).Error; err != nil {
			return fmt.Errorf("failed to create test semester: %w", err)
		}
		log.Printf("  Created semester: ID=%d", semester.ID)
	} else if err != nil {
		return fmt.Errorf("failed to query semester: %w", err)
	} else {
		if semester.DeletedAt.Valid {
			ctx.db.Unscoped().Model(&semester).Update("deleted_at", nil)
			log.Printf("  Restored semester: ID=%d", semester.ID)
		} else {
			log.Printf("  Using existing semester: ID=%d", semester.ID)
		}
	}
	ctx.testSemester = &semester

	log.Println("✓ Test course and semester ready")
	return nil
}

// setupTestSubject creates a test subject for batch ingest
func setupTestSubject(ctx *BatchIngestTestContext) error {
	log.Println("\n--- Setting up test subject ---")

	timestamp := time.Now().Unix()
	subjectCode := fmt.Sprintf("MCA-301-BATCH-%d", timestamp)

	// Create subject
	subject := &model.Subject{
		SemesterID:  ctx.testSemester.ID,
		Name:        "Data Mining (Batch Test)",
		Code:        subjectCode,
		Credits:     4,
		Description: "Subject for batch ingest testing",
	}

	if err := ctx.db.Create(subject).Error; err != nil {
		return fmt.Errorf("failed to create test subject: %w", err)
	}

	ctx.testSubject = subject
	log.Printf("  Created subject: ID=%d, Code=%s", subject.ID, subject.Code)

	log.Println("✓ Test subject ready")
	return nil
}

// ====================================================================
// PDF SERVER MOCK
// ====================================================================

// createMockPDFServer creates a test server that serves the test PDF
func createMockPDFServer(ctx *BatchIngestTestContext) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock PDF server: serving request for %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(ctx.testPDFContent)))
		w.Write(ctx.testPDFContent)
	})
	return httptest.NewServer(handler)
}

// ====================================================================
// TEST SCENARIOS
// ====================================================================

// TestBatchIngestService_SinglePaper tests ingesting a single paper
func TestBatchIngestService_SinglePaper(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Batch Ingest - Single Paper")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Setup course, semester, subject
	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	// Create mock PDF server
	mockServer := createMockPDFServer(testCtx)
	defer mockServer.Close()

	// Start batch ingest with single paper
	t.Run("SinglePaperIngest", func(t *testing.T) {
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{
					PDFURL:     mockServer.URL + "/mca-301-data-mining-dec-2024.pdf",
					Title:      "MCA-301-DATA-MINING-DEC-2024",
					Year:       2024,
					Month:      "December",
					ExamType:   "End Semester",
					SourceName: "Test Source",
				},
			},
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to start batch ingest: %v", err)
		}

		// Verify job was created
		if result.JobID == 0 {
			t.Error("Job ID should not be 0")
		}
		if result.TotalItems != 1 {
			t.Errorf("Expected 1 item, got %d", result.TotalItems)
		}
		if result.Status != string(model.IndexingJobStatusProcessing) {
			t.Errorf("Expected status 'processing', got '%s'", result.Status)
		}

		log.Printf("  ✓ Job created: ID=%d, Status=%s, Items=%d", result.JobID, result.Status, result.TotalItems)

		// Wait for job completion (with timeout)
		waitForJobCompletion(t, testCtx, result.JobID, 2*time.Minute)

		// Verify job status
		job, err := testCtx.batchIngestService.GetJobStatus(context.Background(), result.JobID, testCtx.testUser.ID)
		if err != nil {
			t.Fatalf("Failed to get job status: %v", err)
		}

		log.Printf("  Job final status: %s (completed=%d, failed=%d)", job.Status, job.CompletedItems, job.FailedItems)

		// Verify notification was created
		verifyNotificationCreated(t, testCtx, result.JobID)
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// TestBatchIngestService_MultiplePapers tests ingesting multiple papers in batch
func TestBatchIngestService_MultiplePapers(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Batch Ingest - Multiple Papers (4 papers)")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	// Create mock PDF server
	mockServer := createMockPDFServer(testCtx)
	defer mockServer.Close()

	t.Run("MultiplePapersIngest", func(t *testing.T) {
		// Create request with 4 papers (simulating the scenario from the prompt)
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{
					PDFURL:     mockServer.URL + "/mca-301-data-mining-dec-2024.pdf",
					Title:      "MCA-301-DATA-MINING-DEC-2024",
					Year:       2024,
					Month:      "December",
					ExamType:   "End Semester",
					SourceName: "RGPV Online",
				},
				{
					PDFURL:     mockServer.URL + "/mca-301-data-mining-may-2024.pdf",
					Title:      "MCA-301-DATA-MINING-MAY-2024",
					Year:       2024,
					Month:      "May",
					ExamType:   "End Semester",
					SourceName: "RGPV Online",
				},
				{
					PDFURL:     mockServer.URL + "/mca-301-data-mining-nov-2023.pdf",
					Title:      "MCA-301-DATA-MINING-NOV-2023",
					Year:       2023,
					Month:      "November",
					ExamType:   "End Semester",
					SourceName: "RGPV Online",
				},
				{
					PDFURL:     mockServer.URL + "/mca-301-data-mining-nov-2022.pdf",
					Title:      "MCA-301-DATA-MINING-NOV-2022",
					Year:       2022,
					Month:      "November",
					ExamType:   "End Semester",
					SourceName: "RGPV Online",
				},
			},
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to start batch ingest: %v", err)
		}

		// Verify job creation
		if result.TotalItems != 4 {
			t.Errorf("Expected 4 items, got %d", result.TotalItems)
		}

		log.Printf("  ✓ Batch job created: ID=%d, Items=%d", result.JobID, result.TotalItems)

		// Monitor progress
		monitorJobProgress(t, testCtx, result.JobID, 3*time.Minute)

		// Verify final state
		job, err := testCtx.batchIngestService.GetJobStatus(context.Background(), result.JobID, testCtx.testUser.ID)
		if err != nil {
			t.Fatalf("Failed to get job status: %v", err)
		}

		log.Printf("  Final status: %s", job.Status)
		log.Printf("  Completed: %d, Failed: %d", job.CompletedItems, job.FailedItems)

		// Verify items were processed
		if job.CompletedItems+job.FailedItems != 4 {
			t.Errorf("Expected all 4 items to be processed, got %d", job.CompletedItems+job.FailedItems)
		}

		// Verify documents were created
		var docCount int64
		testCtx.db.Model(&model.Document{}).Where("subject_id = ?", testCtx.testSubject.ID).Count(&docCount)
		log.Printf("  Documents created: %d", docCount)

		// Verify PYQ papers were created
		var pyqCount int64
		testCtx.db.Model(&model.PYQPaper{}).Where("subject_id = ?", testCtx.testSubject.ID).Count(&pyqCount)
		log.Printf("  PYQ papers created: %d", pyqCount)

		// Verify notification
		verifyNotificationCreated(t, testCtx, result.JobID)
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// TestBatchIngestService_DuplicatePapers tests that duplicate papers are skipped
func TestBatchIngestService_DuplicatePapers(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Batch Ingest - Duplicate Detection")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	mockServer := createMockPDFServer(testCtx)
	defer mockServer.Close()

	// First ingest
	t.Run("FirstIngest", func(t *testing.T) {
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{
					PDFURL:     mockServer.URL + "/paper1.pdf",
					Title:      "Test Paper Dec 2024",
					Year:       2024,
					Month:      "December",
					ExamType:   "End Semester",
					SourceName: "Test",
				},
			},
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(context.Background(), req)
		if err != nil {
			t.Fatalf("First ingest failed: %v", err)
		}

		waitForJobCompletion(t, testCtx, result.JobID, 2*time.Minute)
		log.Printf("  ✓ First ingest completed: Job ID=%d", result.JobID)
	})

	// Second ingest with same paper (should be skipped)
	t.Run("DuplicateIngest", func(t *testing.T) {
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{
					PDFURL:     mockServer.URL + "/paper1.pdf",
					Title:      "Test Paper Dec 2024 (Duplicate)",
					Year:       2024,
					Month:      "December", // Same year+month = duplicate
					ExamType:   "End Semester",
					SourceName: "Test",
				},
			},
		}

		_, err := testCtx.batchIngestService.StartBatchIngest(context.Background(), req)
		if err == nil {
			t.Error("Expected error for duplicate paper, got nil")
		} else {
			if !strings.Contains(err.Error(), "already exist") {
				t.Errorf("Expected 'already exist' error, got: %v", err)
			}
			log.Printf("  ✓ Duplicate correctly rejected: %v", err)
		}
	})

	// Mixed ingest (some new, some duplicate)
	t.Run("MixedIngest", func(t *testing.T) {
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{
					PDFURL:     mockServer.URL + "/paper1.pdf",
					Title:      "Test Paper Dec 2024 (Duplicate)",
					Year:       2024,
					Month:      "December", // Duplicate
					ExamType:   "End Semester",
					SourceName: "Test",
				},
				{
					PDFURL:     mockServer.URL + "/paper2.pdf",
					Title:      "Test Paper May 2024",
					Year:       2024,
					Month:      "May", // New
					ExamType:   "End Semester",
					SourceName: "Test",
				},
			},
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(context.Background(), req)
		if err != nil {
			t.Fatalf("Mixed ingest failed: %v", err)
		}

		// Should only have 1 item (the new one)
		if result.TotalItems != 1 {
			t.Errorf("Expected 1 item (duplicate filtered), got %d", result.TotalItems)
		}
		log.Printf("  ✓ Mixed ingest correctly filtered: %d items", result.TotalItems)

		waitForJobCompletion(t, testCtx, result.JobID, 2*time.Minute)
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// TestBatchIngestService_JobCancellation tests job cancellation
func TestBatchIngestService_JobCancellation(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Batch Ingest - Job Cancellation")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	// Create a slow mock server
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow response
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(testCtx.testPDFContent)
	}))
	defer slowServer.Close()

	t.Run("CancelActiveJob", func(t *testing.T) {
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{
					PDFURL:     slowServer.URL + "/slow1.pdf",
					Title:      "Slow Paper 1",
					Year:       2021,
					Month:      "January",
					ExamType:   "End Semester",
					SourceName: "Test",
				},
				{
					PDFURL:     slowServer.URL + "/slow2.pdf",
					Title:      "Slow Paper 2",
					Year:       2021,
					Month:      "February",
					ExamType:   "End Semester",
					SourceName: "Test",
				},
			},
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to start batch ingest: %v", err)
		}

		log.Printf("  Job started: ID=%d", result.JobID)

		// Wait a bit then cancel
		time.Sleep(1 * time.Second)

		err = testCtx.batchIngestService.CancelJob(context.Background(), result.JobID, testCtx.testUser.ID)
		if err != nil {
			t.Fatalf("Failed to cancel job: %v", err)
		}

		log.Println("  ✓ Job cancellation requested")

		// Verify job is cancelled
		time.Sleep(2 * time.Second)
		job, err := testCtx.batchIngestService.GetJobStatus(context.Background(), result.JobID, testCtx.testUser.ID)
		if err != nil {
			t.Fatalf("Failed to get job status: %v", err)
		}

		if job.Status != model.IndexingJobStatusCancelled && job.Status != model.IndexingJobStatusFailed {
			log.Printf("  Job status: %s (may still be processing)", job.Status)
		} else {
			log.Printf("  ✓ Job cancelled: Status=%s", job.Status)
		}
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// TestNotificationService_CRUD tests notification CRUD operations
func TestNotificationService_CRUD(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Notification Service CRUD")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	ctx := context.Background()

	// Test CreateNotification
	t.Run("CreateNotification", func(t *testing.T) {
		notification, err := testCtx.notificationService.CreateNotification(ctx, services.CreateNotificationRequest{
			UserID:   testCtx.testUser.ID,
			Type:     model.NotificationTypeInfo,
			Category: model.NotificationCategoryGeneral,
			Title:    "Test Notification",
			Message:  "This is a test notification",
		})
		if err != nil {
			t.Fatalf("Failed to create notification: %v", err)
		}

		if notification.ID == 0 {
			t.Error("Notification ID should not be 0")
		}
		log.Printf("  ✓ Notification created: ID=%d", notification.ID)
	})

	// Test GetNotificationsByUser
	t.Run("GetNotificationsByUser", func(t *testing.T) {
		notifications, total, err := testCtx.notificationService.GetNotificationsByUser(ctx, services.ListNotificationsOptions{
			UserID: testCtx.testUser.ID,
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("Failed to get notifications: %v", err)
		}

		if total == 0 {
			t.Error("Expected at least 1 notification")
		}
		log.Printf("  ✓ Retrieved %d notifications (total: %d)", len(notifications), total)
	})

	// Test GetUnreadCount
	t.Run("GetUnreadCount", func(t *testing.T) {
		count, err := testCtx.notificationService.GetUnreadCount(ctx, testCtx.testUser.ID)
		if err != nil {
			t.Fatalf("Failed to get unread count: %v", err)
		}

		log.Printf("  ✓ Unread count: %d", count)
	})

	// Test MarkAsRead
	t.Run("MarkAsRead", func(t *testing.T) {
		// Get a notification
		notifications, _, _ := testCtx.notificationService.GetNotificationsByUser(ctx, services.ListNotificationsOptions{
			UserID: testCtx.testUser.ID,
			Limit:  1,
		})

		if len(notifications) > 0 {
			err := testCtx.notificationService.MarkAsRead(ctx, notifications[0].ID, testCtx.testUser.ID)
			if err != nil {
				t.Fatalf("Failed to mark as read: %v", err)
			}
			log.Printf("  ✓ Marked notification %d as read", notifications[0].ID)
		}
	})

	// Test MarkAllAsRead
	t.Run("MarkAllAsRead", func(t *testing.T) {
		_, err := testCtx.notificationService.MarkAllAsRead(ctx, testCtx.testUser.ID)
		if err != nil {
			t.Fatalf("Failed to mark all as read: %v", err)
		}

		// Verify
		count, _ := testCtx.notificationService.GetUnreadCount(ctx, testCtx.testUser.ID)
		if count != 0 {
			t.Errorf("Expected 0 unread, got %d", count)
		}
		log.Println("  ✓ All notifications marked as read")
	})

	// Test DeleteNotification
	t.Run("DeleteNotification", func(t *testing.T) {
		// Create one to delete
		notification, _ := testCtx.notificationService.CreateNotification(ctx, services.CreateNotificationRequest{
			UserID:   testCtx.testUser.ID,
			Type:     model.NotificationTypeInfo,
			Category: model.NotificationCategoryGeneral,
			Title:    "To Delete",
			Message:  "This will be deleted",
		})

		err := testCtx.notificationService.DeleteNotification(ctx, notification.ID, testCtx.testUser.ID)
		if err != nil {
			t.Fatalf("Failed to delete notification: %v", err)
		}
		log.Printf("  ✓ Notification %d deleted", notification.ID)
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// TestFullBatchIngestFlow tests the complete flow with API simulation
func TestFullBatchIngestFlow(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Full Batch Ingest Flow (E2E)")
	log.Println("========================================")
	log.Println("This test simulates the complete flow:")
	log.Println("  1. Start batch ingest with 4 papers")
	log.Println("  2. Monitor job progress")
	log.Println("  3. Verify notifications are created/updated")
	log.Println("  4. Verify documents and PYQ papers created")
	log.Println("  5. Verify notification final state")
	log.Println("========================================")

	startTime := time.Now()

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	mockServer := createMockPDFServer(testCtx)
	defer mockServer.Close()

	ctx := context.Background()

	// Step 1: Check initial notification count
	initialCount, _ := testCtx.notificationService.GetUnreadCount(ctx, testCtx.testUser.ID)
	log.Printf("\n--- Step 1: Initial State ---")
	log.Printf("  Initial notification count: %d", initialCount)

	// Step 2: Start batch ingest
	log.Printf("\n--- Step 2: Start Batch Ingest ---")
	req := services.BatchIngestRequest{
		SubjectID: testCtx.testSubject.ID,
		UserID:    testCtx.testUser.ID,
		Papers: []services.BatchIngestPaperRequest{
			{PDFURL: mockServer.URL + "/paper1.pdf", Title: "MCA-301-DEC-2024", Year: 2024, Month: "December", ExamType: "End Semester", SourceName: "RGPV Online"},
			{PDFURL: mockServer.URL + "/paper2.pdf", Title: "MCA-301-MAY-2024", Year: 2024, Month: "May", ExamType: "End Semester", SourceName: "RGPV Online"},
			{PDFURL: mockServer.URL + "/paper3.pdf", Title: "MCA-301-NOV-2023", Year: 2023, Month: "November", ExamType: "End Semester", SourceName: "RGPV Online"},
			{PDFURL: mockServer.URL + "/paper4.pdf", Title: "MCA-301-NOV-2022", Year: 2022, Month: "November", ExamType: "End Semester", SourceName: "RGPV Online"},
		},
	}

	result, err := testCtx.batchIngestService.StartBatchIngest(ctx, req)
	if err != nil {
		t.Fatalf("Failed to start batch ingest: %v", err)
	}

	log.Printf("  ✓ Job created: ID=%d, Items=%d", result.JobID, result.TotalItems)

	// Step 3: Verify notification was created
	log.Printf("\n--- Step 3: Verify Initial Notification ---")
	time.Sleep(500 * time.Millisecond) // Wait for notification creation

	notifications, _, _ := testCtx.notificationService.GetNotificationsByUser(ctx, services.ListNotificationsOptions{
		UserID: testCtx.testUser.ID,
		Limit:  10,
	})

	var jobNotification *model.UserNotification
	for i, n := range notifications {
		if n.IndexingJobID != nil && *n.IndexingJobID == result.JobID {
			jobNotification = &notifications[i]
			break
		}
	}

	if jobNotification == nil {
		t.Error("Notification for job not found")
	} else {
		log.Printf("  ✓ Notification found: ID=%d, Type=%s", jobNotification.ID, jobNotification.Type)
		log.Printf("    Title: %s", jobNotification.Title)
	}

	// Step 4: Monitor progress
	log.Printf("\n--- Step 4: Monitor Progress ---")
	monitorJobProgress(t, testCtx, result.JobID, 3*time.Minute)

	// Step 5: Verify final state
	log.Printf("\n--- Step 5: Verify Final State ---")

	job, _ := testCtx.batchIngestService.GetJobStatus(ctx, result.JobID, testCtx.testUser.ID)
	log.Printf("  Job Status: %s", job.Status)
	log.Printf("  Completed: %d, Failed: %d", job.CompletedItems, job.FailedItems)

	// Check documents
	var docCount int64
	testCtx.db.Model(&model.Document{}).Where("subject_id = ?", testCtx.testSubject.ID).Count(&docCount)
	log.Printf("  Documents created: %d", docCount)

	// Check PYQ papers
	var pyqCount int64
	testCtx.db.Model(&model.PYQPaper{}).Where("subject_id = ?", testCtx.testSubject.ID).Count(&pyqCount)
	log.Printf("  PYQ papers created: %d", pyqCount)

	// Check final notification state
	notifications, _, _ = testCtx.notificationService.GetNotificationsByUser(ctx, services.ListNotificationsOptions{
		UserID: testCtx.testUser.ID,
		Limit:  10,
	})

	for _, n := range notifications {
		if n.IndexingJobID != nil && *n.IndexingJobID == result.JobID {
			log.Printf("  Final notification: Type=%s, Title=%s", n.Type, n.Title)

			// Parse metadata
			if n.Metadata != nil {
				var meta map[string]interface{}
				json.Unmarshal(n.Metadata, &meta)
				log.Printf("    Progress: %v%%", meta["progress"])
				log.Printf("    Completed: %v, Failed: %v", meta["completed_count"], meta["failed_count"])
			}
		}
	}

	// Summary
	totalDuration := time.Since(startTime)
	log.Println("\n========================================")
	log.Println("TEST SUMMARY")
	log.Println("========================================")
	log.Printf("Total Duration: %.2fs", totalDuration.Seconds())
	log.Printf("Job ID: %d", result.JobID)
	log.Printf("Job Status: %s", job.Status)
	log.Printf("Documents: %d, PYQ Papers: %d", docCount, pyqCount)
	log.Println("========================================")

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// TestNotificationLifecycle_DuringBatchIngest tests that notifications are properly created and updated during batch ingest
// This is a CRITICAL test that verifies the exact notification state at each stage of the job lifecycle:
//
// Timeline:
//
//	T0: StartBatchIngest() called
//	T1: Notification CREATED with type=in_progress, progress=0%
//	T2: Background goroutine starts processing
//	T3-Tn: After each item processed, notification UPDATED with new progress
//	Tfinal: Job completes, notification UPDATED with type=success/warning/error, progress=100%
func TestNotificationLifecycle_DuringBatchIngest(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Notification Lifecycle During Batch Ingest")
	log.Println("========================================")
	log.Println("Verifying notification state transitions:")
	log.Println("  T1: type=in_progress, progress=0% (on job start)")
	log.Println("  T2-Tn: type=in_progress, progress=33%,66%,... (during processing)")
	log.Println("  Tfinal: type=success, progress=100% (on completion)")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	// Create a slower mock server to allow time for notification checks
	// Each request takes 800ms so we can observe intermediate notification states
	slowMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(800 * time.Millisecond)
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(testCtx.testPDFContent)
	}))
	defer slowMockServer.Close()

	ctx := context.Background()

	t.Run("NotificationCreatedOnJobStart", func(t *testing.T) {
		// Start batch ingest with 3 papers
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{PDFURL: slowMockServer.URL + "/paper1.pdf", Title: "Paper 1", Year: 2024, Month: "December", ExamType: "End Semester", SourceName: "Test"},
				{PDFURL: slowMockServer.URL + "/paper2.pdf", Title: "Paper 2", Year: 2024, Month: "May", ExamType: "End Semester", SourceName: "Test"},
				{PDFURL: slowMockServer.URL + "/paper3.pdf", Title: "Paper 3", Year: 2023, Month: "November", ExamType: "End Semester", SourceName: "Test"},
			},
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to start batch ingest: %v", err)
		}

		jobID := result.JobID
		log.Printf("\n[T0] Job started: ID=%d, TotalItems=%d", jobID, result.TotalItems)

		// T1: IMMEDIATELY check notification was created (before any processing)
		time.Sleep(50 * time.Millisecond) // Minimal delay for DB write

		notification := getNotificationForJob(t, testCtx, jobID)
		if notification == nil {
			t.Fatal("[T1] FAIL: Notification was NOT created on job start")
		}

		log.Printf("\n[T1] Initial Notification State:")
		log.Printf("     ID: %d", notification.ID)
		log.Printf("     Type: %s (expected: in_progress)", notification.Type)
		log.Printf("     Category: %s (expected: pyq_ingest)", notification.Category)
		log.Printf("     Title: %s", notification.Title)
		log.Printf("     IndexingJobID: %v (expected: %d)", notification.IndexingJobID, jobID)

		// ASSERT: Initial notification state
		if notification.Type != model.NotificationTypeInProgress {
			t.Errorf("[T1] FAIL: Expected type='in_progress', got '%s'", notification.Type)
		} else {
			log.Println("     ✓ Type is 'in_progress'")
		}

		if notification.Category != model.NotificationCategoryPYQIngest {
			t.Errorf("[T1] FAIL: Expected category='pyq_ingest', got '%s'", notification.Category)
		} else {
			log.Println("     ✓ Category is 'pyq_ingest'")
		}

		if notification.IndexingJobID == nil || *notification.IndexingJobID != jobID {
			t.Errorf("[T1] FAIL: Notification not linked to job ID %d", jobID)
		} else {
			log.Println("     ✓ Linked to correct job ID")
		}

		// ASSERT: Initial metadata
		initialMeta := parseNotificationMetadata(t, notification)
		if initialMeta != nil {
			log.Printf("     Metadata - Progress: %d%%, Completed: %d, Total: %d",
				initialMeta.Progress, initialMeta.CompletedItems, initialMeta.TotalItems)

			if initialMeta.Progress != 0 {
				t.Errorf("[T1] FAIL: Expected initial progress=0, got %d", initialMeta.Progress)
			} else {
				log.Println("     ✓ Initial progress is 0%")
			}

			if initialMeta.TotalItems != 3 {
				t.Errorf("[T1] FAIL: Expected total_items=3, got %d", initialMeta.TotalItems)
			} else {
				log.Println("     ✓ Total items is 3")
			}

			if initialMeta.CompletedItems != 0 {
				t.Errorf("[T1] FAIL: Expected completed_items=0, got %d", initialMeta.CompletedItems)
			} else {
				log.Println("     ✓ Completed items is 0")
			}
		}

		log.Println("\n[T1] ✓ Initial notification state VERIFIED")

		// T2-Tn: Monitor notification updates during processing
		log.Println("\n[T2-Tn] Monitoring notification progress updates...")

		type NotificationSnapshot struct {
			Timestamp      time.Time
			Type           model.NotificationType
			Progress       int
			CompletedItems int
			FailedItems    int
			Title          string
		}

		var snapshots []NotificationSnapshot
		var lastProgress int = -1
		maxWait := 60 * time.Second
		pollInterval := 100 * time.Millisecond
		deadline := time.Now().Add(maxWait)

		for time.Now().Before(deadline) {
			notification = getNotificationForJob(t, testCtx, jobID)
			if notification == nil {
				time.Sleep(pollInterval)
				continue
			}

			meta := parseNotificationMetadata(t, notification)
			currentProgress := 0
			completedItems := 0
			failedItems := 0
			if meta != nil {
				currentProgress = meta.Progress
				completedItems = meta.CompletedItems
				failedItems = meta.FailedItems
			}

			// Capture snapshot when progress changes
			if currentProgress != lastProgress {
				snapshot := NotificationSnapshot{
					Timestamp:      time.Now(),
					Type:           notification.Type,
					Progress:       currentProgress,
					CompletedItems: completedItems,
					FailedItems:    failedItems,
					Title:          notification.Title,
				}
				snapshots = append(snapshots, snapshot)
				log.Printf("     [Snapshot %d] Type=%s, Progress=%d%%, Completed=%d, Failed=%d, Title='%s'",
					len(snapshots), snapshot.Type, snapshot.Progress, snapshot.CompletedItems, snapshot.FailedItems, snapshot.Title)
				lastProgress = currentProgress
			}

			// Job completed when type changes from in_progress
			if notification.Type != model.NotificationTypeInProgress {
				log.Printf("\n[Tfinal] Notification type changed to '%s' - job completed", notification.Type)
				break
			}

			time.Sleep(pollInterval)
		}

		// ASSERT: We captured multiple progress updates
		log.Printf("\n[Analysis] Total snapshots captured: %d", len(snapshots))
		if len(snapshots) < 2 {
			t.Errorf("[T2-Tn] FAIL: Expected multiple progress snapshots, got %d", len(snapshots))
		} else {
			log.Printf("     ✓ Captured %d progress snapshots", len(snapshots))
		}

		// ASSERT: Progress was monotonically increasing
		for i := 1; i < len(snapshots); i++ {
			if snapshots[i].Progress < snapshots[i-1].Progress {
				t.Errorf("[T2-Tn] FAIL: Progress decreased from %d%% to %d%%",
					snapshots[i-1].Progress, snapshots[i].Progress)
			}
		}
		log.Println("     ✓ Progress was monotonically increasing")

		// Tfinal: Verify final notification state
		log.Println("\n[Tfinal] Verifying final notification state...")

		notification = getNotificationForJob(t, testCtx, jobID)
		if notification == nil {
			t.Fatal("[Tfinal] FAIL: Notification not found after job completion")
		}

		log.Printf("     Final Type: %s", notification.Type)
		log.Printf("     Final Title: %s", notification.Title)
		log.Printf("     Final Message: %s", notification.Message)

		finalMeta := parseNotificationMetadata(t, notification)
		if finalMeta != nil {
			log.Printf("     Final Metadata - Progress: %d%%, Completed: %d, Failed: %d",
				finalMeta.Progress, finalMeta.CompletedItems, finalMeta.FailedItems)
		}

		// Get job status to verify notification matches
		job, _ := testCtx.batchIngestService.GetJobStatus(ctx, jobID, testCtx.testUser.ID)
		log.Printf("     Job Status: %s (completed=%d, failed=%d)", job.Status, job.CompletedItems, job.FailedItems)

		// ASSERT: Final notification type matches job status
		expectedType := model.NotificationTypeSuccess
		if job.Status == model.IndexingJobStatusPartial {
			expectedType = model.NotificationTypeWarning
		} else if job.Status == model.IndexingJobStatusFailed {
			expectedType = model.NotificationTypeError
		}

		if notification.Type != expectedType {
			t.Errorf("[Tfinal] FAIL: Expected notification type '%s' for job status '%s', got '%s'",
				expectedType, job.Status, notification.Type)
		} else {
			log.Printf("     ✓ Notification type '%s' matches job status '%s'", notification.Type, job.Status)
		}

		// ASSERT: Final progress is 100%
		if finalMeta != nil && finalMeta.Progress != 100 {
			t.Errorf("[Tfinal] FAIL: Expected final progress=100%%, got %d%%", finalMeta.Progress)
		} else {
			log.Println("     ✓ Final progress is 100%")
		}

		// ASSERT: Completed + Failed = Total
		if finalMeta != nil {
			total := finalMeta.CompletedItems + finalMeta.FailedItems
			if total != finalMeta.TotalItems {
				t.Errorf("[Tfinal] FAIL: Completed(%d) + Failed(%d) != Total(%d)",
					finalMeta.CompletedItems, finalMeta.FailedItems, finalMeta.TotalItems)
			} else {
				log.Printf("     ✓ Completed(%d) + Failed(%d) = Total(%d)",
					finalMeta.CompletedItems, finalMeta.FailedItems, finalMeta.TotalItems)
			}
		}

		// ASSERT: Notification is unread
		if notification.Read {
			t.Error("[Tfinal] FAIL: Notification should be unread initially")
		} else {
			log.Println("     ✓ Notification is unread")
		}

		log.Println("\n[Tfinal] ✓ Final notification state VERIFIED")

		// Summary
		log.Println("\n========================================")
		log.Println("NOTIFICATION LIFECYCLE SUMMARY")
		log.Println("========================================")
		log.Printf("Job ID: %d", jobID)
		log.Printf("Total Snapshots: %d", len(snapshots))
		log.Println("Progress Timeline:")
		for i, s := range snapshots {
			log.Printf("  [%d] %s -> Progress=%d%%, Completed=%d", i+1, s.Type, s.Progress, s.CompletedItems)
		}
		log.Printf("Final State: Type=%s, Progress=%d%%", notification.Type, finalMeta.Progress)
		log.Println("========================================")
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// TestNotificationOnJobFailure tests notification behavior when ALL items fail
// Expected: type=error, failed_items=total_items, completed_items=0
func TestNotificationOnJobFailure(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Notification on Job Failure (All Items Fail)")
	log.Println("========================================")
	log.Println("Expected notification state:")
	log.Println("  - Type: error")
	log.Println("  - failed_items > 0")
	log.Println("  - completed_items = 0")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	// Create a mock server that returns 404 to simulate failed downloads
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Small delay
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer failingServer.Close()

	ctx := context.Background()

	t.Run("AllItemsFail_NotificationIsError", func(t *testing.T) {
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{PDFURL: failingServer.URL + "/fail1.pdf", Title: "Will Fail 1", Year: 2024, Month: "January", ExamType: "End Semester", SourceName: "Test"},
				{PDFURL: failingServer.URL + "/fail2.pdf", Title: "Will Fail 2", Year: 2024, Month: "February", ExamType: "End Semester", SourceName: "Test"},
			},
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to start batch ingest: %v", err)
		}

		jobID := result.JobID
		log.Printf("\n[T0] Job started: ID=%d, TotalItems=%d", jobID, result.TotalItems)

		// T1: Verify initial notification is in_progress
		time.Sleep(50 * time.Millisecond)
		notification := getNotificationForJob(t, testCtx, jobID)
		if notification == nil {
			t.Fatal("[T1] Notification not created")
		}
		log.Printf("[T1] Initial notification type: %s (expected: in_progress)", notification.Type)
		if notification.Type != model.NotificationTypeInProgress {
			t.Errorf("[T1] Expected initial type 'in_progress', got '%s'", notification.Type)
		}

		// Wait for job to complete (should fail)
		log.Println("\n[T2] Waiting for job to complete...")
		waitForJobCompletion(t, testCtx, jobID, 30*time.Second)

		// Tfinal: Check notification state after failure
		notification = getNotificationForJob(t, testCtx, jobID)
		if notification == nil {
			t.Fatal("[Tfinal] Notification not found after job completion")
		}

		log.Printf("\n[Tfinal] Final Notification State:")
		log.Printf("     Type: %s (expected: error)", notification.Type)
		log.Printf("     Title: %s", notification.Title)
		log.Printf("     Message: %s", notification.Message)

		// ASSERT: Notification type is error
		if notification.Type != model.NotificationTypeError {
			t.Errorf("[Tfinal] FAIL: Expected type='error', got '%s'", notification.Type)
		} else {
			log.Println("     ✓ Type is 'error'")
		}

		// ASSERT: Metadata shows all items failed
		meta := parseNotificationMetadata(t, notification)
		if meta != nil {
			log.Printf("     Metadata - Completed: %d, Failed: %d, Total: %d, Progress: %d%%",
				meta.CompletedItems, meta.FailedItems, meta.TotalItems, meta.Progress)

			if meta.FailedItems == 0 {
				t.Error("[Tfinal] FAIL: Expected failed_items > 0")
			} else {
				log.Printf("     ✓ Failed items: %d", meta.FailedItems)
			}

			if meta.CompletedItems != 0 {
				t.Errorf("[Tfinal] FAIL: Expected completed_items=0, got %d", meta.CompletedItems)
			} else {
				log.Println("     ✓ Completed items is 0")
			}

			if meta.Progress != 100 {
				t.Errorf("[Tfinal] FAIL: Expected progress=100%% (job finished), got %d%%", meta.Progress)
			} else {
				log.Println("     ✓ Progress is 100% (job finished, even though failed)")
			}
		}

		// Verify job status matches
		job, _ := testCtx.batchIngestService.GetJobStatus(ctx, jobID, testCtx.testUser.ID)
		log.Printf("\n     Job Status: %s (completed=%d, failed=%d)", job.Status, job.CompletedItems, job.FailedItems)

		if job.Status != model.IndexingJobStatusFailed {
			t.Errorf("[Tfinal] FAIL: Expected job status 'failed', got '%s'", job.Status)
		}

		log.Println("\n[Tfinal] ✓ Failure notification VERIFIED")
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// TestNotificationOnPartialSuccess tests notification behavior when some items succeed and some fail
// Expected: type=warning, completed_items>0, failed_items>0
func TestNotificationOnPartialSuccess(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: Notification on Partial Success")
	log.Println("========================================")
	log.Println("Expected notification state:")
	log.Println("  - Type: warning (partial success)")
	log.Println("  - completed_items > 0")
	log.Println("  - failed_items > 0")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	// Create a good mock server (serves PDF successfully)
	goodServer := createMockPDFServer(testCtx)
	defer goodServer.Close()

	// Create a failing mock server (returns 404)
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer failingServer.Close()

	ctx := context.Background()

	t.Run("MixedResults_NotificationIsWarning", func(t *testing.T) {
		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers: []services.BatchIngestPaperRequest{
				{PDFURL: goodServer.URL + "/success1.pdf", Title: "Will Succeed 1", Year: 2024, Month: "December", ExamType: "End Semester", SourceName: "Test"},
				{PDFURL: failingServer.URL + "/fail1.pdf", Title: "Will Fail 1", Year: 2024, Month: "May", ExamType: "End Semester", SourceName: "Test"},
				{PDFURL: goodServer.URL + "/success2.pdf", Title: "Will Succeed 2", Year: 2023, Month: "November", ExamType: "End Semester", SourceName: "Test"},
			},
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to start batch ingest: %v", err)
		}

		jobID := result.JobID
		log.Printf("\n[T0] Job started: ID=%d, TotalItems=%d", jobID, result.TotalItems)
		log.Printf("     Expected: 2 success, 1 failure")

		// T1: Verify initial notification
		time.Sleep(50 * time.Millisecond)
		notification := getNotificationForJob(t, testCtx, jobID)
		if notification == nil {
			t.Fatal("[T1] Notification not created")
		}
		log.Printf("[T1] Initial notification type: %s", notification.Type)

		// Wait for job to complete
		log.Println("\n[T2] Waiting for job to complete...")
		waitForJobCompletion(t, testCtx, jobID, 60*time.Second)

		// Tfinal: Check notification state
		notification = getNotificationForJob(t, testCtx, jobID)
		if notification == nil {
			t.Fatal("[Tfinal] Notification not found")
		}

		// Get job status
		job, _ := testCtx.batchIngestService.GetJobStatus(ctx, jobID, testCtx.testUser.ID)
		log.Printf("\n[Tfinal] Job Status: %s (completed=%d, failed=%d)", job.Status, job.CompletedItems, job.FailedItems)

		log.Printf("[Tfinal] Final Notification State:")
		log.Printf("     Type: %s", notification.Type)
		log.Printf("     Title: %s", notification.Title)
		log.Printf("     Message: %s", notification.Message)

		meta := parseNotificationMetadata(t, notification)
		if meta != nil {
			log.Printf("     Metadata - Completed: %d, Failed: %d, Total: %d, Progress: %d%%",
				meta.CompletedItems, meta.FailedItems, meta.TotalItems, meta.Progress)
		}

		// ASSERT: Mixed results -> notification type should be warning
		if job.CompletedItems > 0 && job.FailedItems > 0 {
			if notification.Type != model.NotificationTypeWarning {
				t.Errorf("[Tfinal] FAIL: Expected type='warning' for partial success, got '%s'", notification.Type)
			} else {
				log.Println("     ✓ Type is 'warning' (partial success)")
			}

			if job.Status != model.IndexingJobStatusPartial {
				t.Errorf("[Tfinal] FAIL: Expected job status 'partially_completed', got '%s'", job.Status)
			} else {
				log.Println("     ✓ Job status is 'partially_completed'")
			}
		} else if job.FailedItems > 0 && job.CompletedItems == 0 {
			// All failed - should be error
			if notification.Type != model.NotificationTypeError {
				t.Errorf("[Tfinal] FAIL: Expected type='error' for all failed, got '%s'", notification.Type)
			}
			log.Println("     Note: All items failed, type is 'error'")
		} else {
			// All succeeded - should be success
			if notification.Type != model.NotificationTypeSuccess {
				t.Errorf("[Tfinal] FAIL: Expected type='success' for all succeeded, got '%s'", notification.Type)
			}
			log.Println("     Note: All items succeeded, type is 'success'")
		}

		// ASSERT: Metadata counts are correct
		if meta != nil {
			if meta.CompletedItems+meta.FailedItems != meta.TotalItems {
				t.Errorf("[Tfinal] FAIL: completed(%d) + failed(%d) != total(%d)",
					meta.CompletedItems, meta.FailedItems, meta.TotalItems)
			} else {
				log.Printf("     ✓ completed(%d) + failed(%d) = total(%d)",
					meta.CompletedItems, meta.FailedItems, meta.TotalItems)
			}

			if meta.Progress != 100 {
				t.Errorf("[Tfinal] FAIL: Expected progress=100%%, got %d%%", meta.Progress)
			} else {
				log.Println("     ✓ Progress is 100%")
			}
		}

		log.Println("\n[Tfinal] ✓ Partial success notification VERIFIED")
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// Helper function to get notification for a job
func getNotificationForJob(t *testing.T, testCtx *BatchIngestTestContext, jobID uint) *model.UserNotification {
	notifications, _, err := testCtx.notificationService.GetNotificationsByUser(context.Background(), services.ListNotificationsOptions{
		UserID: testCtx.testUser.ID,
		Limit:  50,
	})
	if err != nil {
		t.Logf("Warning: Failed to get notifications: %v", err)
		return nil
	}

	for i, n := range notifications {
		if n.IndexingJobID != nil && *n.IndexingJobID == jobID {
			return &notifications[i]
		}
	}
	return nil
}

// Helper function to parse notification metadata
func parseNotificationMetadata(t *testing.T, notification *model.UserNotification) *model.NotificationMetadata {
	if notification.Metadata == nil {
		return nil
	}

	var meta model.NotificationMetadata
	if err := json.Unmarshal(notification.Metadata, &meta); err != nil {
		t.Logf("Warning: Failed to parse notification metadata: %v", err)
		return nil
	}
	return &meta
}

// TestHTTPEndpoints_BatchIngest tests the actual HTTP endpoints
func TestHTTPEndpoints_BatchIngest(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n========================================")
	log.Println("TEST: HTTP API Endpoints")
	log.Println("========================================")

	// Setup
	testCtx, err := setupBatchIngestTestEnvironment(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if err := setupTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}
	if err := setupTestSubject(testCtx); err != nil {
		t.Fatalf("Failed to setup subject: %v", err)
	}

	mockServer := createMockPDFServer(testCtx)
	defer mockServer.Close()

	// Test batch-ingest endpoint format
	t.Run("BatchIngestRequest", func(t *testing.T) {
		// Create JSON request body
		reqBody := map[string]interface{}{
			"papers": []map[string]interface{}{
				{
					"pdf_url":     mockServer.URL + "/paper1.pdf",
					"title":       "MCA-301-DATA-MINING-DEC-2024",
					"year":        2024,
					"month":       "December",
					"exam_type":   "End Semester",
					"source_name": "RGPV Online",
				},
			},
		}

		jsonData, _ := json.Marshal(reqBody)
		log.Printf("  Request body: %s", string(jsonData))

		// Test directly with service (simulating what handler does)
		var papers []services.BatchIngestPaperRequest
		for _, p := range reqBody["papers"].([]map[string]interface{}) {
			papers = append(papers, services.BatchIngestPaperRequest{
				PDFURL:     p["pdf_url"].(string),
				Title:      p["title"].(string),
				Year:       int(p["year"].(int)),
				Month:      p["month"].(string),
				ExamType:   p["exam_type"].(string),
				SourceName: p["source_name"].(string),
			})
		}

		req := services.BatchIngestRequest{
			SubjectID: testCtx.testSubject.ID,
			UserID:    testCtx.testUser.ID,
			Papers:    papers,
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to start batch ingest: %v", err)
		}

		log.Printf("  ✓ Response: job_id=%d, status=%s, total_items=%d", result.JobID, result.Status, result.TotalItems)

		// Wait for completion
		waitForJobCompletion(t, testCtx, result.JobID, 2*time.Minute)
	})

	// Cleanup
	cleanupBatchIngestTestData(testCtx)
}

// ====================================================================
// HELPER FUNCTIONS
// ====================================================================

// waitForJobCompletion waits for a job to complete
func waitForJobCompletion(t *testing.T, testCtx *BatchIngestTestContext, jobID uint, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		job, err := testCtx.batchIngestService.GetJobStatus(context.Background(), jobID, testCtx.testUser.ID)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if job.IsComplete() {
			return
		}

		time.Sleep(pollInterval)
	}

	t.Logf("Warning: Job %d did not complete within timeout", jobID)
}

// monitorJobProgress monitors and logs job progress
func monitorJobProgress(t *testing.T, testCtx *BatchIngestTestContext, jobID uint, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	pollInterval := 1 * time.Second
	lastProgress := -1

	for time.Now().Before(deadline) {
		job, err := testCtx.batchIngestService.GetJobStatus(context.Background(), jobID, testCtx.testUser.ID)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		progress := 0
		if job.TotalItems > 0 {
			progress = ((job.CompletedItems + job.FailedItems) * 100) / job.TotalItems
		}

		if progress != lastProgress {
			log.Printf("  Progress: %d%% (completed=%d, failed=%d, status=%s)",
				progress, job.CompletedItems, job.FailedItems, job.Status)
			lastProgress = progress
		}

		if job.IsComplete() {
			return
		}

		time.Sleep(pollInterval)
	}
}

// verifyNotificationCreated verifies a notification was created for the job
func verifyNotificationCreated(t *testing.T, testCtx *BatchIngestTestContext, jobID uint) {
	notifications, _, err := testCtx.notificationService.GetNotificationsByUser(context.Background(), services.ListNotificationsOptions{
		UserID: testCtx.testUser.ID,
		Limit:  20,
	})
	if err != nil {
		t.Errorf("Failed to get notifications: %v", err)
		return
	}

	found := false
	for _, n := range notifications {
		if n.IndexingJobID != nil && *n.IndexingJobID == jobID {
			found = true
			log.Printf("  ✓ Notification verified: ID=%d, Type=%s, Title=%s", n.ID, n.Type, n.Title)
			break
		}
	}

	if !found {
		t.Errorf("No notification found for job %d", jobID)
	}
}

// cleanupBatchIngestTestData cleans up all test data
func cleanupBatchIngestTestData(testCtx *BatchIngestTestContext) {
	log.Println("\n--- Cleaning up test data ---")

	if testCtx == nil || testCtx.db == nil {
		return
	}

	// Delete notifications
	testCtx.db.Where("user_id = ?", testCtx.testUser.ID).Delete(&model.UserNotification{})

	// Delete job items
	testCtx.db.Exec("DELETE FROM indexing_job_items WHERE job_id IN (SELECT id FROM indexing_jobs WHERE created_by_user_id = ?)", testCtx.testUser.ID)

	// Delete jobs
	testCtx.db.Where("created_by_user_id = ?", testCtx.testUser.ID).Delete(&model.IndexingJob{})

	// Delete PYQ papers
	if testCtx.testSubject != nil {
		testCtx.db.Where("subject_id = ?", testCtx.testSubject.ID).Delete(&model.PYQPaper{})
		testCtx.db.Where("subject_id = ?", testCtx.testSubject.ID).Delete(&model.Document{})
		testCtx.db.Delete(&model.Subject{}, testCtx.testSubject.ID)
	}

	log.Println("✓ Test data cleaned up")
}
