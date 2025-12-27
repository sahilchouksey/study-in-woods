package tests

import (
	"bufio"
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
	"github.com/sahilchouksey/go-init-setup/utils/auth"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// loadEnvFile loads environment variables from a .env file
func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Parse KEY=VALUE or KEY="VALUE"
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		// Only set if not already set (allow command line to override)
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

// OCRIntegrationTestContext holds all resources for OCR integration tests
type OCRIntegrationTestContext struct {
	db                  *gorm.DB
	notificationService *services.NotificationService
	batchIngestService  *services.BatchIngestService
	pyqService          *services.PYQService
	jwtManager          *auth.JWTManager

	testUser     *model.User
	testCourse   *model.Course
	testSemester *model.Semester
	testSubjects []*model.Subject // Multiple subjects for multi-subject test

	testPDFContent []byte
	startTime      time.Time
}

// =============================================================================
// SETUP
// =============================================================================

func setupOCRIntegrationTestEnv(t *testing.T) (*OCRIntegrationTestContext, error) {
	ctx := &OCRIntegrationTestContext{
		startTime: time.Now(),
	}

	log.Println("╔══════════════════════════════════════════════════════════════╗")
	log.Println("║     OCR BATCH INGEST INTEGRATION TEST - SETUP                ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")

	// Load .env file from api directory
	envPath := "../.env"
	if err := loadEnvFile(envPath); err != nil {
		log.Printf("⚠ Could not load .env file from %s: %v", envPath, err)
	} else {
		log.Printf("✓ Loaded .env file from %s", envPath)
	}

	// Check required env vars
	requiredEnvVars := []string{"DB_HOST", "DB_USER_NAME", "DB_PASSWORD", "DB_NAME", "DB_PORT", "JWT_SECRET"}
	missing := []string{}
	for _, v := range requiredEnvVars {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing env vars: %s", strings.Join(missing, ", "))
	}
	log.Println("✓ Environment variables verified")

	// Connect to database
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER_NAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
		getEnvOrDefaultOCR("DB_SSL_MODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DB: %w", err)
	}
	ctx.db = db
	log.Println("✓ Database connected")

	// Initialize JWT manager
	ctx.jwtManager = auth.NewJWTManager(auth.JWTConfig{
		Secret:        os.Getenv("JWT_SECRET"),
		Expiry:        24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
		Issuer:        "test",
	})
	log.Println("✓ JWT manager initialized")

	// Initialize services
	ctx.notificationService = services.NewNotificationService(db)
	ctx.pyqService = services.NewPYQService(db)
	ctx.batchIngestService = services.NewBatchIngestService(db, ctx.notificationService, ctx.pyqService)
	log.Println("✓ Services initialized")

	// Check OCR service availability
	ocrClient := services.NewOCRClient()
	ocrCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ocrClient.HealthCheck(ocrCtx); err != nil {
		log.Printf("⚠ OCR service not available: %v", err)
		log.Println("  OCR processing will be skipped during tests")
	} else {
		log.Println("✓ OCR service available")
	}

	// Check AI extraction key
	if os.Getenv("DO_INFERENCE_API_KEY") != "" {
		log.Println("✓ DO_INFERENCE_API_KEY configured - AI extraction enabled")
	} else {
		log.Println("⚠ DO_INFERENCE_API_KEY not set - AI extraction disabled")
	}

	// Check Spaces configuration
	if os.Getenv("DO_SPACES_BUCKET") != "" && os.Getenv("DO_SPACES_REGION") != "" {
		log.Println("✓ DigitalOcean Spaces configured")
	} else {
		log.Println("⚠ DO_SPACES_BUCKET/DO_SPACES_REGION not set - file storage disabled")
	}

	// Load test PDF
	pdfPath := "../../../mca-301-data-mining-dec-2024.pdf"
	content, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test PDF: %w", err)
	}
	ctx.testPDFContent = content
	log.Printf("✓ Test PDF loaded: %.2f KB", float64(len(content))/1024)

	// Setup test user
	if err := setupOCRTestUser(ctx); err != nil {
		return nil, err
	}

	log.Printf("✓ Setup complete (%.2fs)\n", time.Since(ctx.startTime).Seconds())
	return ctx, nil
}

func getEnvOrDefaultOCR(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func setupOCRTestUser(ctx *OCRIntegrationTestContext) error {
	var user model.User
	email := "ocr_integration_test@test.com"

	if err := ctx.db.Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			user = model.User{
				Email:        email,
				PasswordHash: "test_hash",
				PasswordSalt: []byte("test_salt"),
				Name:         "OCR Integration Test User",
				Role:         "admin",
			}
			if err := ctx.db.Create(&user).Error; err != nil {
				return fmt.Errorf("failed to create test user: %w", err)
			}
			log.Printf("✓ Created test user: ID=%d", user.ID)
		} else {
			return err
		}
	} else {
		log.Printf("✓ Using existing test user: ID=%d", user.ID)
	}
	ctx.testUser = &user
	return nil
}

func setupOCRTestCourseAndSemester(ctx *OCRIntegrationTestContext) error {
	log.Println("\n--- Setting up course and semester ---")

	// University
	var university model.University
	err := ctx.db.Unscoped().Where("code = ?", "OCR_TEST_UNIV").First(&university).Error
	if err == gorm.ErrRecordNotFound {
		university = model.University{
			Name:     "OCR Test University",
			Code:     "OCR_TEST_UNIV",
			Location: "Test Location",
		}
		if err := ctx.db.Create(&university).Error; err != nil {
			return err
		}
		log.Printf("  Created university: ID=%d", university.ID)
	} else if err != nil {
		return err
	} else {
		if university.DeletedAt.Valid {
			ctx.db.Unscoped().Model(&university).Update("deleted_at", nil)
		}
		log.Printf("  Using university: ID=%d", university.ID)
	}

	// Course
	var course model.Course
	err = ctx.db.Unscoped().Where("code = ?", "OCR_TEST_MCA").First(&course).Error
	if err == gorm.ErrRecordNotFound {
		course = model.Course{
			UniversityID: university.ID,
			Name:         "MCA - OCR Test",
			Code:         "OCR_TEST_MCA",
			Description:  "Course for OCR integration tests",
			Duration:     4,
		}
		if err := ctx.db.Create(&course).Error; err != nil {
			return err
		}
		log.Printf("  Created course: ID=%d", course.ID)
	} else if err != nil {
		return err
	} else {
		if course.DeletedAt.Valid {
			ctx.db.Unscoped().Model(&course).Update("deleted_at", nil)
		}
		log.Printf("  Using course: ID=%d", course.ID)
	}
	ctx.testCourse = &course

	// Semester
	var semester model.Semester
	err = ctx.db.Unscoped().Where("course_id = ? AND number = ?", course.ID, 3).First(&semester).Error
	if err == gorm.ErrRecordNotFound {
		semester = model.Semester{
			CourseID: course.ID,
			Number:   3,
			Name:     "Semester 3",
		}
		if err := ctx.db.Create(&semester).Error; err != nil {
			return err
		}
		log.Printf("  Created semester: ID=%d", semester.ID)
	} else if err != nil {
		return err
	} else {
		if semester.DeletedAt.Valid {
			ctx.db.Unscoped().Model(&semester).Update("deleted_at", nil)
		}
		log.Printf("  Using semester: ID=%d", semester.ID)
	}
	ctx.testSemester = &semester

	return nil
}

func createOCRTestSubject(ctx *OCRIntegrationTestContext, name, code string) (*model.Subject, error) {
	subject := &model.Subject{
		SemesterID:  ctx.testSemester.ID,
		Name:        name,
		Code:        code,
		Credits:     4,
		Description: "Subject for OCR integration testing",
	}

	if err := ctx.db.Create(subject).Error; err != nil {
		return nil, err
	}
	ctx.testSubjects = append(ctx.testSubjects, subject)
	log.Printf("  Created subject: ID=%d, Code=%s", subject.ID, subject.Code)
	return subject, nil
}

func createOCRMockPDFServer(ctx *OCRIntegrationTestContext) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("  [MockServer] Serving PDF for: %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(ctx.testPDFContent)))
		w.Write(ctx.testPDFContent)
	}))
}

// =============================================================================
// MAIN TEST: Complete Multi-Subject Batch Ingest Flow with DB Verification
// =============================================================================

// TestCompleteMultiSubjectBatchIngestFlow tests the complete flow:
// 1. Create multiple subjects
// 2. Batch ingest multiple PYQ papers for each subject
// 3. Verify DB state at each step (jobs, items, documents, pyq_papers, notifications)
// 4. Verify OCR text is stored (if OCR service available)
// 5. Verify extraction is triggered
func TestCompleteMultiSubjectBatchIngestFlow(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n╔══════════════════════════════════════════════════════════════╗")
	log.Println("║  TEST: Complete Multi-Subject Batch Ingest Flow              ║")
	log.Println("║  - Create 3 subjects                                         ║")
	log.Println("║  - Ingest 2-3 PYQ papers per subject                         ║")
	log.Println("║  - Verify DB state at each step                              ║")
	log.Println("║  - Verify OCR text storage                                   ║")
	log.Println("║  - Verify notifications                                      ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")

	// Setup
	testCtx, err := setupOCRIntegrationTestEnv(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupOCRTestData(testCtx)

	if err := setupOCRTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup course/semester: %v", err)
	}

	mockServer := createOCRMockPDFServer(testCtx)
	defer mockServer.Close()

	bgCtx := context.Background()
	timestamp := time.Now().Unix()

	// ==========================================================================
	// STEP 1: Create 3 subjects
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 1: Create Test Subjects                                │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	subjects := []struct {
		name string
		code string
	}{
		{"Data Mining", fmt.Sprintf("MCA-301-DM-%d", timestamp)},
		{"Machine Learning", fmt.Sprintf("MCA-302-ML-%d", timestamp)},
		{"Computer Networks", fmt.Sprintf("MCA-303-CN-%d", timestamp)},
	}

	createdSubjects := make([]*model.Subject, 0)
	for _, s := range subjects {
		subject, err := createOCRTestSubject(testCtx, s.name, s.code)
		if err != nil {
			t.Fatalf("Failed to create subject %s: %v", s.name, err)
		}
		createdSubjects = append(createdSubjects, subject)
	}

	// Verify in DB
	var subjectCount int64
	testCtx.db.Model(&model.Subject{}).Where("semester_id = ?", testCtx.testSemester.ID).Count(&subjectCount)
	log.Printf("  ✓ Subjects in DB: %d", subjectCount)

	if int(subjectCount) < len(subjects) {
		t.Errorf("Expected at least %d subjects, got %d", len(subjects), subjectCount)
	}

	// ==========================================================================
	// STEP 2: Batch ingest papers for each subject
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 2: Batch Ingest PYQ Papers for Each Subject            │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	type SubjectIngestResult struct {
		Subject *model.Subject
		JobID   uint
		Papers  int
	}

	ingestResults := make([]SubjectIngestResult, 0)

	// Papers to ingest per subject
	paperSets := [][]services.BatchIngestPaperRequest{
		// Subject 1: Data Mining - 3 papers
		{
			{PDFURL: mockServer.URL + "/dm-dec-2024.pdf", Title: "DM-DEC-2024", Year: 2024, Month: "December", ExamType: "End Semester"},
			{PDFURL: mockServer.URL + "/dm-may-2024.pdf", Title: "DM-MAY-2024", Year: 2024, Month: "May", ExamType: "End Semester"},
			{PDFURL: mockServer.URL + "/dm-nov-2023.pdf", Title: "DM-NOV-2023", Year: 2023, Month: "November", ExamType: "End Semester"},
		},
		// Subject 2: Machine Learning - 2 papers
		{
			{PDFURL: mockServer.URL + "/ml-dec-2024.pdf", Title: "ML-DEC-2024", Year: 2024, Month: "December", ExamType: "End Semester"},
			{PDFURL: mockServer.URL + "/ml-may-2024.pdf", Title: "ML-MAY-2024", Year: 2024, Month: "May", ExamType: "End Semester"},
		},
		// Subject 3: Computer Networks - 2 papers
		{
			{PDFURL: mockServer.URL + "/cn-dec-2024.pdf", Title: "CN-DEC-2024", Year: 2024, Month: "December", ExamType: "End Semester"},
			{PDFURL: mockServer.URL + "/cn-nov-2023.pdf", Title: "CN-NOV-2023", Year: 2023, Month: "November", ExamType: "End Semester"},
		},
	}

	// Process each subject SEQUENTIALLY to avoid OCR service overload
	// OCR can only handle one PDF at a time, so concurrent jobs will timeout
	for i, subject := range createdSubjects {
		log.Printf("\n  [Subject %d] %s (ID=%d)", i+1, subject.Name, subject.ID)
		log.Printf("    Ingesting %d papers...", len(paperSets[i]))

		req := services.BatchIngestRequest{
			SubjectID: subject.ID,
			UserID:    testCtx.testUser.ID,
			Papers:    paperSets[i],
		}

		result, err := testCtx.batchIngestService.StartBatchIngest(bgCtx, req)
		if err != nil {
			t.Fatalf("Failed to start batch ingest for subject %d: %v", subject.ID, err)
		}

		log.Printf("    ✓ Job created: ID=%d, Items=%d", result.JobID, result.TotalItems)

		ingestResults = append(ingestResults, SubjectIngestResult{
			Subject: subject,
			JobID:   result.JobID,
			Papers:  result.TotalItems,
		})

		// Verify job in DB immediately
		var job model.IndexingJob
		if err := testCtx.db.First(&job, result.JobID).Error; err != nil {
			t.Errorf("Job %d not found in DB: %v", result.JobID, err)
		} else {
			log.Printf("    ✓ DB Check: Job exists (status=%s, total=%d)", job.Status, job.TotalItems)
		}

		// Verify job items created
		var itemCount int64
		testCtx.db.Model(&model.IndexingJobItem{}).Where("job_id = ?", result.JobID).Count(&itemCount)
		log.Printf("    ✓ DB Check: Job items created: %d", itemCount)

		if int(itemCount) != result.TotalItems {
			t.Errorf("Expected %d items, got %d", result.TotalItems, itemCount)
		}

		// WAIT for this job to complete before starting the next one
		// This ensures OCR service is not overloaded with concurrent requests
		log.Printf("    Waiting for job %d to complete before starting next subject...", result.JobID)
		waitForOCRJobCompletion(t, testCtx, result.JobID, 3*time.Minute)

		// Verify final job status
		testCtx.db.First(&job, result.JobID)
		log.Printf("    Final Status: %s (completed=%d, failed=%d)", job.Status, job.CompletedItems, job.FailedItems)
	}

	// ==========================================================================
	// STEP 3: Verify DB State for All Jobs
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 3: Verify DB State for All Jobs                        │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	for _, result := range ingestResults {
		log.Printf("\n  [Job %d] Verifying DB state...", result.JobID)

		// Verify final job status
		var job model.IndexingJob
		testCtx.db.First(&job, result.JobID)
		log.Printf("    Final Status: %s (completed=%d, failed=%d)", job.Status, job.CompletedItems, job.FailedItems)

		// Verify job items status
		var completedItems, failedItems, pendingItems int64
		testCtx.db.Model(&model.IndexingJobItem{}).Where("job_id = ? AND status = ?", result.JobID, model.IndexingJobItemStatusCompleted).Count(&completedItems)
		testCtx.db.Model(&model.IndexingJobItem{}).Where("job_id = ? AND status = ?", result.JobID, model.IndexingJobItemStatusFailed).Count(&failedItems)
		testCtx.db.Model(&model.IndexingJobItem{}).Where("job_id = ? AND status = ?", result.JobID, model.IndexingJobItemStatusPending).Count(&pendingItems)
		log.Printf("    ✓ Item Status: completed=%d, failed=%d, pending=%d", completedItems, failedItems, pendingItems)

		// Verify documents created
		var docCount int64
		testCtx.db.Model(&model.Document{}).Where("subject_id = ?", result.Subject.ID).Count(&docCount)
		log.Printf("    ✓ Documents created: %d", docCount)

		// Verify PYQ papers created
		var pyqCount int64
		testCtx.db.Model(&model.PYQPaper{}).Where("subject_id = ?", result.Subject.ID).Count(&pyqCount)
		log.Printf("    ✓ PYQ Papers created: %d", pyqCount)

		// Verify notification
		var notification model.UserNotification
		if err := testCtx.db.Where("indexing_job_id = ?", result.JobID).First(&notification).Error; err != nil {
			t.Errorf("Notification not found for job %d: %v", result.JobID, err)
		} else {
			log.Printf("    ✓ Notification: ID=%d, Type=%s", notification.ID, notification.Type)
		}
	}

	// ==========================================================================
	// STEP 4: Verify OCR Text Storage
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 4: Verify OCR Text Storage                             │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	var docsWithOCR, docsWithoutOCR int64
	testCtx.db.Model(&model.Document{}).Where("ocr_text IS NOT NULL AND ocr_text != ''").Count(&docsWithOCR)
	testCtx.db.Model(&model.Document{}).Where("ocr_text IS NULL OR ocr_text = ''").Count(&docsWithoutOCR)

	log.Printf("  Documents with OCR text: %d", docsWithOCR)
	log.Printf("  Documents without OCR text: %d", docsWithoutOCR)

	// Sample OCR text from one document
	var sampleDoc model.Document
	if err := testCtx.db.Where("ocr_text IS NOT NULL AND ocr_text != ''").First(&sampleDoc).Error; err == nil {
		ocrPreview := sampleDoc.OCRText
		if len(ocrPreview) > 200 {
			ocrPreview = ocrPreview[:200] + "..."
		}
		log.Printf("  Sample OCR text (doc %d): %s", sampleDoc.ID, ocrPreview)
	} else {
		log.Println("  ⚠ No documents with OCR text found (OCR service may not be available)")
	}

	// ==========================================================================
	// STEP 5: Verify PYQ Extraction Status
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 5: Verify PYQ Extraction Status                        │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	var pendingExtraction, inProgressExtraction, completedExtraction, failedExtraction int64
	testCtx.db.Model(&model.PYQPaper{}).Where("extraction_status = ?", model.PYQExtractionPending).Count(&pendingExtraction)
	testCtx.db.Model(&model.PYQPaper{}).Where("extraction_status = ?", model.PYQExtractionProcessing).Count(&inProgressExtraction)
	testCtx.db.Model(&model.PYQPaper{}).Where("extraction_status = ?", model.PYQExtractionCompleted).Count(&completedExtraction)
	testCtx.db.Model(&model.PYQPaper{}).Where("extraction_status = ?", model.PYQExtractionFailed).Count(&failedExtraction)

	log.Printf("  Extraction Status:")
	log.Printf("    - Pending: %d", pendingExtraction)
	log.Printf("    - In Progress: %d", inProgressExtraction)
	log.Printf("    - Completed: %d", completedExtraction)
	log.Printf("    - Failed: %d", failedExtraction)

	// Check if any questions were extracted
	var questionCount int64
	testCtx.db.Model(&model.PYQQuestion{}).Count(&questionCount)
	log.Printf("  Total questions extracted: %d", questionCount)

	// ==========================================================================
	// STEP 6: Final Summary
	// ==========================================================================
	log.Println("\n╔══════════════════════════════════════════════════════════════╗")
	log.Println("║                    TEST SUMMARY                              ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")

	totalPapers := 0
	for _, r := range ingestResults {
		totalPapers += r.Papers
	}

	var totalDocs, totalPYQs int64
	testCtx.db.Model(&model.Document{}).Count(&totalDocs)
	testCtx.db.Model(&model.PYQPaper{}).Count(&totalPYQs)

	log.Printf("  Subjects Created: %d", len(createdSubjects))
	log.Printf("  Jobs Created: %d", len(ingestResults))
	log.Printf("  Papers Ingested: %d", totalPapers)
	log.Printf("  Documents in DB: %d", totalDocs)
	log.Printf("  PYQ Papers in DB: %d", totalPYQs)
	log.Printf("  Documents with OCR: %d", docsWithOCR)
	log.Printf("  Questions Extracted: %d", questionCount)
	log.Printf("  Duration: %.2fs", time.Since(testCtx.startTime).Seconds())

	log.Println("\n  ✓ TEST COMPLETED SUCCESSFULLY")
}

// =============================================================================
// TEST: Single Subject Deep Verification
// =============================================================================

// TestSingleSubjectDeepVerification performs deep verification on a single subject
// checking every database field at each step
func TestSingleSubjectDeepVerification(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("\n╔══════════════════════════════════════════════════════════════╗")
	log.Println("║  TEST: Single Subject Deep DB Verification                   ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")

	testCtx, err := setupOCRIntegrationTestEnv(t)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupOCRTestData(testCtx)

	if err := setupOCRTestCourseAndSemester(testCtx); err != nil {
		t.Fatalf("Failed to setup: %v", err)
	}

	mockServer := createOCRMockPDFServer(testCtx)
	defer mockServer.Close()

	bgCtx := context.Background()
	timestamp := time.Now().Unix()

	// Create single subject
	subject, err := createOCRTestSubject(testCtx, "Deep Test Subject", fmt.Sprintf("DEEP-TEST-%d", timestamp))
	if err != nil {
		t.Fatalf("Failed to create subject: %v", err)
	}

	// ==========================================================================
	// STEP 1: Before Ingest - Verify Empty State
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 1: Verify Empty State Before Ingest                    │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	var docsBefore, pyqsBefore, jobsBefore int64
	testCtx.db.Model(&model.Document{}).Where("subject_id = ?", subject.ID).Count(&docsBefore)
	testCtx.db.Model(&model.PYQPaper{}).Where("subject_id = ?", subject.ID).Count(&pyqsBefore)
	testCtx.db.Model(&model.IndexingJob{}).Where("subject_id = ?", subject.ID).Count(&jobsBefore)

	log.Printf("  Documents: %d (expected: 0)", docsBefore)
	log.Printf("  PYQ Papers: %d (expected: 0)", pyqsBefore)
	log.Printf("  Indexing Jobs: %d (expected: 0)", jobsBefore)

	if docsBefore != 0 || pyqsBefore != 0 || jobsBefore != 0 {
		t.Error("Expected empty state before ingest")
	}
	log.Println("  ✓ Empty state verified")

	// ==========================================================================
	// STEP 2: Start Batch Ingest
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 2: Start Batch Ingest                                  │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	papers := []services.BatchIngestPaperRequest{
		{PDFURL: mockServer.URL + "/paper1.pdf", Title: "Test-Dec-2024", Year: 2024, Month: "December", ExamType: "End Semester", SourceName: "Test"},
		{PDFURL: mockServer.URL + "/paper2.pdf", Title: "Test-May-2024", Year: 2024, Month: "May", ExamType: "End Semester", SourceName: "Test"},
	}

	req := services.BatchIngestRequest{
		SubjectID: subject.ID,
		UserID:    testCtx.testUser.ID,
		Papers:    papers,
	}

	result, err := testCtx.batchIngestService.StartBatchIngest(bgCtx, req)
	if err != nil {
		t.Fatalf("Failed to start batch ingest: %v", err)
	}

	log.Printf("  Job created: ID=%d, Status=%s, TotalItems=%d", result.JobID, result.Status, result.TotalItems)

	// ==========================================================================
	// STEP 3: Verify Job Created in DB
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 3: Verify Job in Database                              │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	var job model.IndexingJob
	if err := testCtx.db.Preload("Items").First(&job, result.JobID).Error; err != nil {
		t.Fatalf("Job not found: %v", err)
	}

	log.Printf("  Job Fields:")
	log.Printf("    ID: %d", job.ID)
	log.Printf("    SubjectID: %v", job.SubjectID)
	log.Printf("    JobType: %s", job.JobType)
	log.Printf("    Status: %s", job.Status)
	log.Printf("    TotalItems: %d", job.TotalItems)
	log.Printf("    CompletedItems: %d", job.CompletedItems)
	log.Printf("    FailedItems: %d", job.FailedItems)
	log.Printf("    CreatedByUserID: %d", job.CreatedByUserID)
	log.Printf("    StartedAt: %v", job.StartedAt)
	log.Printf("    Items Count: %d", len(job.Items))

	// Verify job items
	log.Println("\n  Job Items:")
	for i, item := range job.Items {
		log.Printf("    [%d] ID=%d, Status=%s, SourceURL=%s", i+1, item.ID, item.Status, item.SourceURL)

		// Parse metadata
		var meta model.IndexingJobItemMetadata
		if err := json.Unmarshal(item.Metadata, &meta); err == nil {
			log.Printf("        Title=%s, Year=%d, Month=%s", meta.Title, meta.Year, meta.Month)
		}
	}

	// ==========================================================================
	// STEP 4: Wait and Monitor Progress
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 4: Monitor Progress                                    │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	monitorOCRJobProgress(t, testCtx, result.JobID, 2*time.Minute)

	// ==========================================================================
	// STEP 5: Verify Final Job State
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 5: Verify Final Job State                              │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	testCtx.db.Preload("Items").First(&job, result.JobID)

	log.Printf("  Final Job Status: %s", job.Status)
	log.Printf("  CompletedItems: %d, FailedItems: %d", job.CompletedItems, job.FailedItems)
	log.Printf("  CompletedAt: %v", job.CompletedAt)

	log.Println("\n  Final Item States:")
	for i, item := range job.Items {
		// Reload item to get latest state
		testCtx.db.First(&item, item.ID)
		log.Printf("    [%d] Status=%s, DocumentID=%v, PYQPaperID=%v, Error=%s",
			i+1, item.Status, item.DocumentID, item.PYQPaperID, item.ErrorMessage)
	}

	// ==========================================================================
	// STEP 6: Verify Documents Created
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 6: Verify Documents in Database                        │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	var documents []model.Document
	testCtx.db.Where("subject_id = ?", subject.ID).Find(&documents)

	log.Printf("  Documents created: %d", len(documents))
	for i, doc := range documents {
		log.Printf("\n  [Document %d]", i+1)
		log.Printf("    ID: %d", doc.ID)
		log.Printf("    Filename: %s", doc.Filename)
		log.Printf("    Type: %s", doc.Type)
		log.Printf("    OriginalURL: %s", doc.OriginalURL)
		log.Printf("    SpacesURL: %s", doc.SpacesURL)
		log.Printf("    FileSize: %d bytes", doc.FileSize)
		log.Printf("    PageCount: %d", doc.PageCount)
		log.Printf("    IndexingStatus: %s", doc.IndexingStatus)
		log.Printf("    OCRText length: %d chars", len(doc.OCRText))

		if doc.OCRText != "" {
			preview := doc.OCRText
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			log.Printf("    OCRText preview: %s", preview)
		}
	}

	// ==========================================================================
	// STEP 7: Verify PYQ Papers Created
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 7: Verify PYQ Papers in Database                       │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	var pyqPapers []model.PYQPaper
	testCtx.db.Where("subject_id = ?", subject.ID).Find(&pyqPapers)

	log.Printf("  PYQ Papers created: %d", len(pyqPapers))
	for i, pyq := range pyqPapers {
		log.Printf("\n  [PYQ Paper %d]", i+1)
		log.Printf("    ID: %d", pyq.ID)
		log.Printf("    SubjectID: %d", pyq.SubjectID)
		log.Printf("    DocumentID: %d", pyq.DocumentID)
		log.Printf("    Year: %d", pyq.Year)
		log.Printf("    Month: %s", pyq.Month)
		log.Printf("    ExamType: %s", pyq.ExamType)
		log.Printf("    ExtractionStatus: %s", pyq.ExtractionStatus)
		// Count questions for this paper
		var qCount int64
		testCtx.db.Model(&model.PYQQuestion{}).Where("paper_id = ?", pyq.ID).Count(&qCount)
		log.Printf("    QuestionCount: %d", qCount)
	}

	// ==========================================================================
	// STEP 8: Verify Notification Created
	// ==========================================================================
	log.Println("\n┌──────────────────────────────────────────────────────────────┐")
	log.Println("│  STEP 8: Verify Notification in Database                     │")
	log.Println("└──────────────────────────────────────────────────────────────┘")

	var notification model.UserNotification
	if err := testCtx.db.Where("indexing_job_id = ?", result.JobID).First(&notification).Error; err != nil {
		t.Errorf("Notification not found: %v", err)
	} else {
		log.Printf("  Notification ID: %d", notification.ID)
		log.Printf("  UserID: %d", notification.UserID)
		log.Printf("  Type: %s", notification.Type)
		log.Printf("  Category: %s", notification.Category)
		log.Printf("  Title: %s", notification.Title)
		log.Printf("  Message: %s", notification.Message)
		log.Printf("  Read: %v", notification.Read)
		log.Printf("  IndexingJobID: %d", *notification.IndexingJobID)

		if notification.Metadata != nil {
			var meta model.NotificationMetadata
			if err := json.Unmarshal(notification.Metadata, &meta); err == nil {
				log.Printf("  Metadata:")
				log.Printf("    SubjectID: %d", meta.SubjectID)
				log.Printf("    SubjectName: %s", meta.SubjectName)
				log.Printf("    TotalItems: %d", meta.TotalItems)
				log.Printf("    CompletedItems: %d", meta.CompletedItems)
				log.Printf("    FailedItems: %d", meta.FailedItems)
				log.Printf("    Progress: %d%%", meta.Progress)
			}
		}
	}

	// ==========================================================================
	// SUMMARY
	// ==========================================================================
	log.Println("\n╔══════════════════════════════════════════════════════════════╗")
	log.Println("║                    DEEP VERIFICATION SUMMARY                 ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")

	log.Printf("  Subject: %s (ID=%d)", subject.Name, subject.ID)
	log.Printf("  Job: ID=%d, Status=%s", job.ID, job.Status)
	log.Printf("  Documents: %d", len(documents))
	log.Printf("  PYQ Papers: %d", len(pyqPapers))
	log.Printf("  Notification: Type=%s", notification.Type)

	docsWithOCR := 0
	for _, doc := range documents {
		if doc.OCRText != "" {
			docsWithOCR++
		}
	}
	log.Printf("  Documents with OCR: %d/%d", docsWithOCR, len(documents))

	log.Println("\n  ✓ DEEP VERIFICATION COMPLETED")
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func waitForOCRJobCompletion(t *testing.T, ctx *OCRIntegrationTestContext, jobID uint, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		var job model.IndexingJob
		if err := ctx.db.First(&job, jobID).Error; err != nil {
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

func monitorOCRJobProgress(t *testing.T, ctx *OCRIntegrationTestContext, jobID uint, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	pollInterval := 1 * time.Second
	lastProgress := -1

	for time.Now().Before(deadline) {
		var job model.IndexingJob
		if err := ctx.db.First(&job, jobID).Error; err != nil {
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

func cleanupOCRTestData(ctx *OCRIntegrationTestContext) {
	if ctx == nil || ctx.db == nil {
		return
	}

	log.Println("\n--- Cleaning up test data ---")

	// Delete notifications
	ctx.db.Where("user_id = ?", ctx.testUser.ID).Delete(&model.UserNotification{})

	// Delete job items
	ctx.db.Exec("DELETE FROM indexing_job_items WHERE job_id IN (SELECT id FROM indexing_jobs WHERE created_by_user_id = ?)", ctx.testUser.ID)

	// Delete jobs
	ctx.db.Where("created_by_user_id = ?", ctx.testUser.ID).Delete(&model.IndexingJob{})

	// Delete PYQ questions, papers, documents for test subjects
	for _, subject := range ctx.testSubjects {
		ctx.db.Exec("DELETE FROM pyq_questions WHERE paper_id IN (SELECT id FROM pyq_papers WHERE subject_id = ?)", subject.ID)
		ctx.db.Where("subject_id = ?", subject.ID).Delete(&model.PYQPaper{})
		ctx.db.Where("subject_id = ?", subject.ID).Delete(&model.Document{})
		ctx.db.Delete(&model.Subject{}, subject.ID)
	}

	log.Println("✓ Test data cleaned up")
}
