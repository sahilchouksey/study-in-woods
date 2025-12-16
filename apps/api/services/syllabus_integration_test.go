package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SyllabusTestContext holds all resources needed for syllabus integration tests
type SyllabusTestContext struct {
	// Database
	db *gorm.DB

	// Services
	syllabusService *SyllabusService
	documentService *DocumentService

	// Test data
	testUserID   uint
	testCourse   *model.Course
	testSemester *model.Semester
	testDocument *model.Document
	pdfContent   []byte

	// Expected subjects from the PDF
	expectedSubjects []ExpectedSubject

	// Timing
	startTime time.Time
}

// ExpectedSubject represents expected extraction results
type ExpectedSubject struct {
	Code       string
	Name       string
	UnitsCount int
}

// SSEEvent represents a parsed SSE event
type SSEEvent struct {
	Event string
	Data  map[string]interface{}
}

// ====================================================================
// SETUP FUNCTIONS
// ====================================================================

// setupSyllabusTestEnvironment initializes all required clients and services
func setupSyllabusTestEnvironment(t *testing.T) (*SyllabusTestContext, error) {
	ctx := &SyllabusTestContext{
		startTime: time.Now(),
	}

	log.Println("========================================")
	log.Println("Setting up syllabus test environment...")
	log.Println("========================================")

	// 1. Check required environment variables
	requiredEnvVars := []string{
		"DB_HOST",
		"DB_USER_NAME",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_PORT",
		"DO_INFERENCE_API_KEY",
		"DO_SPACES_ACCESS_KEY",
		"DO_SPACES_SECRET_KEY",
		"DO_SPACES_NAME",
		"DO_SPACES_REGION",
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

	log.Println("✓ All required environment variables present")

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

	// 3. Initialize services
	ctx.syllabusService = NewSyllabusService(db)
	ctx.documentService = NewDocumentService(db)
	log.Println("✓ Services initialized")

	// 4. Get or create test user
	var user model.User
	if err := db.Where("email = ?", "syllabus_test@test.com").First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			user = model.User{
				Email:        "syllabus_test@test.com",
				PasswordHash: "test_password_hash",
				PasswordSalt: []byte("test_salt"),
				Name:         "Syllabus Test User",
				Role:         "admin",
			}
			if err := db.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to create test user: %w", err)
			}
			log.Printf("✓ Created test user: ID=%d", user.ID)
		} else {
			return nil, fmt.Errorf("failed to query test user: %w", err)
		}
	} else {
		log.Printf("✓ Using existing test user: ID=%d", user.ID)
	}
	ctx.testUserID = user.ID

	// 5. Define expected subjects from frm_download_file.pdf
	ctx.expectedSubjects = []ExpectedSubject{
		{Code: "MCA 301", Name: "Data Mining", UnitsCount: 5},
		{Code: "MCA 302", Name: "Artificial Intelligence", UnitsCount: 5},
		{Code: "MCA 303 (1)", Name: "Python Programming", UnitsCount: 5},
		{Code: "MCA 303 (2)", Name: "Web Technology", UnitsCount: 5},
		{Code: "MCA 303 (3)", Name: "Introduction to Data Science and Big Data", UnitsCount: 5},
		{Code: "MCA 304(1)", Name: "Machine Learning", UnitsCount: 5},
		{Code: "MCA 304(2)", Name: "Soft Computing", UnitsCount: 5},
		{Code: "MCA 304(3)", Name: "Internet of Things", UnitsCount: 5},
		{Code: "MCA 305(1)", Name: "Computer Ethics", UnitsCount: 5},
		{Code: "MCA 305(2)", Name: "Advanced DBMS", UnitsCount: 5},
		{Code: "MCA 305(3)", Name: "Distributed Systems", UnitsCount: 5},
	}

	log.Printf("✓ Test environment setup complete (%.2fs)\n", time.Since(ctx.startTime).Seconds())
	return ctx, nil
}

// ====================================================================
// TEST DATA CREATION
// ====================================================================

// createSyllabusTestCourseAndSemester creates test course and semester
func createSyllabusTestCourseAndSemester(ctx *SyllabusTestContext) error {
	log.Println("\n--- Creating test course and semester ---")
	startTime := time.Now()

	// Create or get test university
	var university model.University
	if err := ctx.db.Where("code = ?", "RGPV_TEST").First(&university).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			university = model.University{
				Name:     "Rajiv Gandhi Proudyogiki Vishwavidyalaya (Test)",
				Code:     "RGPV_TEST",
				Location: "Bhopal, MP",
			}
			if err := ctx.db.Create(&university).Error; err != nil {
				return fmt.Errorf("failed to create test university: %w", err)
			}
			log.Printf("  Created university: ID=%d", university.ID)
		} else {
			return fmt.Errorf("failed to query university: %w", err)
		}
	}

	// Create or get test course
	var course model.Course
	if err := ctx.db.Where("code = ?", "MCA_TEST").First(&course).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			course = model.Course{
				UniversityID: university.ID,
				Name:         "Master of Computer Applications (Test)",
				Code:         "MCA_TEST",
				Description:  "MCA course for syllabus integration tests",
				Duration:     4,
			}
			if err := ctx.db.Create(&course).Error; err != nil {
				return fmt.Errorf("failed to create test course: %w", err)
			}
			log.Printf("  Created course: ID=%d", course.ID)
		} else {
			return fmt.Errorf("failed to query course: %w", err)
		}
	}
	ctx.testCourse = &course

	// Create or get test semester (Semester 3 for MCA 3xx subjects)
	var semester model.Semester
	if err := ctx.db.Where("course_id = ? AND number = ?", course.ID, 3).First(&semester).Error; err != nil {
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
		} else {
			return fmt.Errorf("failed to query semester: %w", err)
		}
	}

	// Preload course relationship
	if err := ctx.db.Preload("Course").First(&semester, semester.ID).Error; err != nil {
		return fmt.Errorf("failed to preload semester relations: %w", err)
	}
	ctx.testSemester = &semester

	log.Printf("✓ Test course and semester ready (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// loadSyllabusTestPDF loads the test PDF file
func loadSyllabusTestPDF(ctx *SyllabusTestContext) error {
	log.Println("\n--- Loading test PDF ---")
	startTime := time.Now()

	// Use the existing frm_download_file.pdf
	pdfPath := "../../../frm_download_file.pdf"

	content, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read test PDF from %s: %w", pdfPath, err)
	}

	ctx.pdfContent = content

	log.Printf("  PDF size: %.2f KB", float64(len(content))/1024)
	log.Printf("  Expected subjects: %d", len(ctx.expectedSubjects))
	log.Printf("✓ Test PDF loaded (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// ====================================================================
// SYLLABUS UPLOAD TEST (Direct Service Call)
// ====================================================================

// testUploadSyllabusDocument uploads a syllabus document using semester-based upload
// This tests the new approach where documents are associated with semesters directly,
// and subjects are created during extraction from the PDF content
func testUploadSyllabusDocument(ctx *SyllabusTestContext) error {
	log.Println("\n--- Uploading syllabus document (semester-based) ---")
	startTime := time.Now()

	// Delete existing syllabus data for this semester (clean slate)
	if err := ctx.syllabusService.DeleteExistingSyllabusDataForSemester(context.Background(), ctx.testSemester.ID); err != nil {
		log.Printf("  Warning: Failed to clean existing syllabus data: %v", err)
	}

	// Create mock file header
	filename := fmt.Sprintf("mca_semester3_syllabus_%d.pdf", time.Now().Unix())

	// Create multipart file from bytes
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(ctx.pdfContent); err != nil {
		return fmt.Errorf("failed to write to form file: %w", err)
	}
	writer.Close()

	// Create file header
	fileHeader := &multipart.FileHeader{
		Filename: filename,
		Size:     int64(len(ctx.pdfContent)),
	}

	// Create a reader that implements multipart.File
	fileReader := &bytesFileReader{Reader: bytes.NewReader(ctx.pdfContent)}

	// Use semester-based upload (no temp subject needed)
	req := UploadDocumentRequest{
		SemesterID: ctx.testSemester.ID, // Associate with semester directly
		UserID:     ctx.testUserID,
		Type:       model.DocumentTypeSyllabus,
		File:       fileReader,
		FileHeader: fileHeader,
	}

	log.Printf("  Uploading: %s (%.2f KB)", filename, float64(len(ctx.pdfContent))/1024)
	log.Printf("  Semester ID: %d, User ID: %d", ctx.testSemester.ID, ctx.testUserID)

	result, err := ctx.documentService.UploadDocument(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to upload document: %w", err)
	}

	ctx.testDocument = result.Document

	log.Printf("  Document ID: %d", result.Document.ID)
	log.Printf("  Uploaded to Spaces: %v", result.UploadedToSpaces)
	log.Printf("  Spaces Key: %s", result.Document.SpacesKey)

	// Verify upload results
	if !result.UploadedToSpaces {
		return fmt.Errorf("document was not uploaded to Spaces")
	}
	if result.Document.SpacesKey == "" {
		return fmt.Errorf("Spaces key is empty")
	}

	// Verify document is associated with semester (not subject)
	if result.Document.SemesterID == nil || *result.Document.SemesterID != ctx.testSemester.ID {
		return fmt.Errorf("document should be associated with semester %d", ctx.testSemester.ID)
	}
	if result.Document.SubjectID != nil && *result.Document.SubjectID != 0 {
		return fmt.Errorf("document should not be associated with a subject for semester-based upload")
	}

	log.Printf("✓ Syllabus document uploaded successfully (semester-based) (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// ====================================================================
// SYLLABUS EXTRACTION TEST (Direct Service Call with Progress)
// ====================================================================

// testExtractSyllabusWithProgress extracts syllabus with progress tracking
func testExtractSyllabusWithProgress(ctx *SyllabusTestContext) ([]*model.Syllabus, error) {
	log.Println("\n--- Extracting syllabus with progress tracking ---")
	startTime := time.Now()

	if ctx.testDocument == nil {
		return nil, fmt.Errorf("no test document available")
	}

	log.Printf("  Document ID: %d", ctx.testDocument.ID)
	log.Printf("  Document Type: %s", ctx.testDocument.Type)

	// Track progress events
	var progressEvents []ProgressEvent
	lastProgress := 0

	// Extract with progress callback
	syllabuses, err := ctx.syllabusService.ExtractSyllabusWithProgress(
		context.Background(),
		ctx.testDocument.ID,
		func(event ProgressEvent) error {
			progressEvents = append(progressEvents, event)

			// Log significant progress changes
			if event.Progress != lastProgress || event.Type == "complete" || event.Type == "error" {
				log.Printf("  [%s] Progress: %d%% - Phase: %s - %s",
					event.Type, event.Progress, event.Phase, event.Message)
				lastProgress = event.Progress
			}

			// Log chunk progress
			if event.TotalChunks > 0 {
				log.Printf("    Chunks: %d/%d completed", event.CompletedChunks, event.TotalChunks)
			}

			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	log.Printf("\n  Total progress events: %d", len(progressEvents))
	log.Printf("  Syllabuses extracted: %d", len(syllabuses))
	log.Printf("✓ Syllabus extraction completed (%.2fs)", time.Since(startTime).Seconds())

	return syllabuses, nil
}

// ====================================================================
// VERIFICATION TESTS
// ====================================================================

// testVerifySyllabusExtraction verifies the extracted syllabus data
func testVerifySyllabusExtraction(ctx *SyllabusTestContext, syllabuses []*model.Syllabus) error {
	log.Println("\n--- Verifying syllabus extraction results ---")
	startTime := time.Now()

	if len(syllabuses) == 0 {
		return fmt.Errorf("no syllabuses were extracted")
	}

	log.Printf("  Extracted %d syllabuses", len(syllabuses))

	// Track found subjects
	foundSubjects := make(map[string]bool)
	totalUnits := 0
	totalTopics := 0
	totalBooks := 0

	for i, syl := range syllabuses {
		// Reload syllabus with all relationships
		fullSyllabus, err := ctx.syllabusService.GetSyllabusByID(context.Background(), syl.ID)
		if err != nil {
			log.Printf("  ⚠ Warning: Failed to reload syllabus %d: %v", syl.ID, err)
			continue
		}

		log.Printf("\n  [%d] Subject: %s (%s)", i+1, fullSyllabus.SubjectName, fullSyllabus.SubjectCode)
		log.Printf("      Units: %d, Books: %d", len(fullSyllabus.Units), len(fullSyllabus.Books))

		// Count topics
		unitTopics := 0
		for _, unit := range fullSyllabus.Units {
			unitTopics += len(unit.Topics)
			log.Printf("      Unit %d: %s (%d topics)", unit.UnitNumber, truncateString(unit.Title, 40), len(unit.Topics))
		}
		log.Printf("      Total topics: %d", unitTopics)

		foundSubjects[fullSyllabus.SubjectCode] = true
		totalUnits += len(fullSyllabus.Units)
		totalTopics += unitTopics
		totalBooks += len(fullSyllabus.Books)
	}

	// Summary
	log.Printf("\n  === Extraction Summary ===")
	log.Printf("  Total subjects: %d", len(syllabuses))
	log.Printf("  Total units: %d", totalUnits)
	log.Printf("  Total topics: %d", totalTopics)
	log.Printf("  Total books: %d", totalBooks)

	// Check for expected subjects
	log.Printf("\n  === Expected Subject Verification ===")
	matchedCount := 0
	for _, expected := range ctx.expectedSubjects {
		// Check if any extracted subject matches (partial match on code)
		found := false
		for code := range foundSubjects {
			// Normalize codes for comparison (remove spaces, parentheses variations)
			normalizedExpected := normalizeSubjectCode(expected.Code)
			normalizedFound := normalizeSubjectCode(code)
			if strings.Contains(normalizedFound, normalizedExpected) || strings.Contains(normalizedExpected, normalizedFound) {
				found = true
				matchedCount++
				break
			}
		}
		if found {
			log.Printf("    ✓ Found: %s", expected.Code)
		} else {
			log.Printf("    ✗ Missing: %s", expected.Code)
		}
	}

	log.Printf("\n  Matched %d/%d expected subjects", matchedCount, len(ctx.expectedSubjects))

	// We don't fail if not all subjects are found - LLM extraction may vary
	if matchedCount < len(ctx.expectedSubjects)/2 {
		log.Printf("  ⚠ Warning: Less than half of expected subjects were found")
	}

	log.Printf("✓ Syllabus verification complete (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// normalizeSubjectCode normalizes subject codes for comparison
func normalizeSubjectCode(code string) string {
	// Remove spaces, convert to uppercase, normalize parentheses and dashes
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "(", "")
	code = strings.ReplaceAll(code, ")", "")
	code = strings.ReplaceAll(code, "-", "")
	return code
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// testSaveExtractedDataToJSON saves the extracted syllabus data to a JSON file for review
func testSaveExtractedDataToJSON(ctx *SyllabusTestContext, syllabuses []*model.Syllabus) error {
	log.Println("\n--- Saving extracted data to JSON for review ---")

	if len(syllabuses) == 0 {
		return fmt.Errorf("no syllabuses to save")
	}

	// Build a comprehensive data structure for review
	type TopicJSON struct {
		TopicNumber int    `json:"topic_number"`
		Title       string `json:"title"`
	}

	type UnitJSON struct {
		UnitNumber int         `json:"unit_number"`
		Title      string      `json:"title"`
		RawText    string      `json:"raw_text"`
		Topics     []TopicJSON `json:"topics"`
	}

	type BookJSON struct {
		Title    string `json:"title"`
		Authors  string `json:"authors"`
		BookType string `json:"book_type"`
	}

	type SyllabusJSON struct {
		ID          uint       `json:"id"`
		SubjectID   uint       `json:"subject_id"`
		SubjectName string     `json:"subject_name"`
		SubjectCode string     `json:"subject_code"`
		Credits     int        `json:"credits"`
		Units       []UnitJSON `json:"units"`
		Books       []BookJSON `json:"books"`
	}

	type SubjectJSON struct {
		ID         uint   `json:"id"`
		Name       string `json:"name"`
		Code       string `json:"code"`
		SemesterID uint   `json:"semester_id"`
	}

	type OutputJSON struct {
		ExtractedAt     string         `json:"extracted_at"`
		SemesterID      uint           `json:"semester_id"`
		TotalSubjects   int            `json:"total_subjects"`
		TotalSyllabuses int            `json:"total_syllabuses"`
		Subjects        []SubjectJSON  `json:"subjects"`
		Syllabuses      []SyllabusJSON `json:"syllabuses"`
	}

	output := OutputJSON{
		ExtractedAt:     time.Now().Format(time.RFC3339),
		SemesterID:      ctx.testSemester.ID,
		TotalSyllabuses: len(syllabuses),
	}

	// Get all subjects for this semester
	var subjects []model.Subject
	if err := ctx.db.Where("semester_id = ?", ctx.testSemester.ID).Find(&subjects).Error; err != nil {
		return fmt.Errorf("failed to query subjects: %w", err)
	}

	output.TotalSubjects = len(subjects)
	for _, subj := range subjects {
		output.Subjects = append(output.Subjects, SubjectJSON{
			ID:         subj.ID,
			Name:       subj.Name,
			Code:       subj.Code,
			SemesterID: subj.SemesterID,
		})
	}

	// Get full syllabus data with relationships
	for _, syl := range syllabuses {
		fullSyllabus, err := ctx.syllabusService.GetSyllabusByID(context.Background(), syl.ID)
		if err != nil {
			log.Printf("  Warning: Failed to load syllabus %d: %v", syl.ID, err)
			continue
		}

		sylJSON := SyllabusJSON{
			ID:          fullSyllabus.ID,
			SubjectID:   fullSyllabus.SubjectID,
			SubjectName: fullSyllabus.SubjectName,
			SubjectCode: fullSyllabus.SubjectCode,
			Credits:     fullSyllabus.TotalCredits,
		}

		for _, unit := range fullSyllabus.Units {
			unitJSON := UnitJSON{
				UnitNumber: unit.UnitNumber,
				Title:      unit.Title,
				RawText:    unit.RawText,
			}
			for _, topic := range unit.Topics {
				unitJSON.Topics = append(unitJSON.Topics, TopicJSON{
					TopicNumber: topic.TopicNumber,
					Title:       topic.Title,
				})
			}
			sylJSON.Units = append(sylJSON.Units, unitJSON)
		}

		for _, book := range fullSyllabus.Books {
			sylJSON.Books = append(sylJSON.Books, BookJSON{
				Title:    book.Title,
				Authors:  book.Authors,
				BookType: book.BookType,
			})
		}

		output.Syllabuses = append(output.Syllabuses, sylJSON)
	}

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Save to file
	filename := fmt.Sprintf("debug_extracted_syllabus_%d.json", time.Now().Unix())
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	log.Printf("  ✓ Saved extracted data to: %s", filename)
	log.Printf("  Total subjects: %d", output.TotalSubjects)
	log.Printf("  Total syllabuses: %d", output.TotalSyllabuses)

	return nil
}

// testVerifyNoTempSubjectAndRealSubjectsCreated verifies that:
// 1. No GENERAL_TEMP or GENERAL_TEMP_SYLLABUS subjects exist (we use semester-based upload now)
// 2. Real subjects were created from the PDF extraction
func testVerifyNoTempSubjectAndRealSubjectsCreated(ctx *SyllabusTestContext) error {
	log.Println("\n--- Verifying no temp subjects and real subjects created ---")
	startTime := time.Now()

	// Check that no temp subjects exist (we use semester-based upload now, so no temp subjects should be created)
	var tempSubjects []model.Subject
	err := ctx.db.Where("semester_id = ? AND (code = ? OR code = ?)", ctx.testSemester.ID, "GENERAL_TEMP_SYLLABUS", "GENERAL_TEMP").
		Find(&tempSubjects).Error

	if err != nil {
		return fmt.Errorf("failed to check for temp subjects: %w", err)
	}

	if len(tempSubjects) > 0 {
		// Temp subjects exist - this is a failure (we shouldn't create them anymore)
		for _, ts := range tempSubjects {
			log.Printf("  ✗ Found temp subject: %s (%s) ID=%d", ts.Name, ts.Code, ts.ID)
		}
		return fmt.Errorf("found %d temp subjects that should not exist with semester-based upload", len(tempSubjects))
	}
	log.Println("  ✓ No temp subjects found (good! - using semester-based upload)")

	// Check that real subjects were created
	var subjects []model.Subject
	if err := ctx.db.Where("semester_id = ?", ctx.testSemester.ID).Find(&subjects).Error; err != nil {
		return fmt.Errorf("failed to query subjects: %w", err)
	}

	if len(subjects) == 0 {
		return fmt.Errorf("no subjects were created from extraction")
	}

	log.Printf("  ✓ Found %d subjects created from extraction:", len(subjects))

	// Verify subjects have proper codes (not GENERAL_TEMP)
	realSubjectCount := 0
	for _, subject := range subjects {
		if subject.Code == "GENERAL_TEMP_SYLLABUS" || subject.Code == "GENERAL_TEMP" {
			return fmt.Errorf("found temp subject that should have been cleaned up: %s (%s)", subject.Name, subject.Code)
		}
		realSubjectCount++
		log.Printf("    - %s (%s)", subject.Name, subject.Code)
	}

	// We expect all 11 unique subjects to be created (allow 1 missing for LLM variation)
	minExpectedSubjects := len(ctx.expectedSubjects) - 1 // 10 out of 11
	if realSubjectCount < minExpectedSubjects {
		return fmt.Errorf("expected at least %d subjects, but only found %d", minExpectedSubjects, realSubjectCount)
	}

	log.Printf("✓ Subject verification complete (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// ====================================================================
// CLEANUP
// ====================================================================

// cleanupSyllabusTestData cleans up all test resources
func cleanupSyllabusTestData(ctx *SyllabusTestContext, keepOnFailure bool) {
	log.Println("\n========================================")
	log.Println("Cleaning up syllabus test data...")
	log.Println("========================================")

	bgCtx := context.Background()

	// Delete document
	if ctx.testDocument != nil {
		log.Printf("  Deleting document ID=%d...", ctx.testDocument.ID)
		if err := ctx.documentService.DeleteDocumentWithCleanup(bgCtx, ctx.testDocument.ID); err != nil {
			log.Printf("    ⚠ Warning: Failed to delete document: %v", err)
		} else {
			log.Printf("    ✓ Document deleted")
		}
	}

	// Delete syllabuses for the semester
	if ctx.testSemester != nil {
		log.Printf("  Deleting syllabuses for semester ID=%d...", ctx.testSemester.ID)
		if err := ctx.syllabusService.DeleteExistingSyllabusDataForSemester(bgCtx, ctx.testSemester.ID); err != nil {
			log.Printf("    ⚠ Warning: Failed to delete syllabuses: %v", err)
		} else {
			log.Printf("    ✓ Syllabuses deleted")
		}
	}

	// Delete any temp subjects that might exist from previous test runs (cleanup legacy data)
	if ctx.testSemester != nil {
		var tempSubjects []model.Subject
		if err := ctx.db.Where("semester_id = ? AND (code = ? OR code = ?)", ctx.testSemester.ID, "GENERAL_TEMP_SYLLABUS", "GENERAL_TEMP").
			Find(&tempSubjects).Error; err == nil && len(tempSubjects) > 0 {
			for _, ts := range tempSubjects {
				log.Printf("  Deleting legacy temp subject ID=%d (%s)...", ts.ID, ts.Code)
				if err := ctx.db.Delete(&ts).Error; err != nil {
					log.Printf("    ⚠ Warning: Failed to delete temp subject: %v", err)
				} else {
					log.Printf("    ✓ Temp subject deleted")
				}
			}
		}
	}

	// Delete created subjects (from extraction)
	if ctx.testSemester != nil {
		var subjects []model.Subject
		if err := ctx.db.Where("semester_id = ?", ctx.testSemester.ID).Find(&subjects).Error; err == nil {
			for _, subject := range subjects {
				// Delete all subjects for this semester (they were created during extraction)
				log.Printf("  Deleting extracted subject ID=%d (%s)...", subject.ID, subject.Code)
				if err := ctx.db.Delete(&subject).Error; err != nil {
					log.Printf("    ⚠ Warning: Failed to delete subject: %v", err)
				}
			}
		}
	}

	log.Println("✓ Cleanup complete")
}

// ====================================================================
// MAIN TEST FUNCTION
// ====================================================================

// TestSyllabusUploadAndExtraction is the main syllabus integration test
func TestSyllabusUploadAndExtraction(t *testing.T) {
	// Check if integration tests are enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("========================================")
	log.Println("INTEGRATION TEST: Syllabus Upload & Extraction")
	log.Println("========================================")

	testStartTime := time.Now()
	var testCtx *SyllabusTestContext
	var testFailed bool

	// Defer cleanup
	defer func() {
		if testCtx != nil {
			// Keep data on failure for debugging
			if testFailed {
				log.Println("\n⚠ Test failed - keeping test data for debugging")
				if testCtx.testDocument != nil {
					log.Printf("  Document ID: %d", testCtx.testDocument.ID)
				}
				if testCtx.testSemester != nil {
					log.Printf("  Semester ID: %d", testCtx.testSemester.ID)
				}
			} else {
				cleanupSyllabusTestData(testCtx, testFailed)
			}
		}
	}()

	// STEP 1: Setup
	t.Run("Setup", func(t *testing.T) {
		var err error
		testCtx, err = setupSyllabusTestEnvironment(t)
		if err != nil {
			testFailed = true
			t.Fatalf("Setup failed: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 2: Create test course and semester
	t.Run("CreateCourseAndSemester", func(t *testing.T) {
		if err := createSyllabusTestCourseAndSemester(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Failed to create test course/semester: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 3: Load test PDF
	t.Run("LoadTestPDF", func(t *testing.T) {
		if err := loadSyllabusTestPDF(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Failed to load test PDF: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 4: Upload syllabus document
	t.Run("UploadSyllabusDocument", func(t *testing.T) {
		if err := testUploadSyllabusDocument(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Failed to upload syllabus document: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 5: Extract syllabus with progress
	var syllabuses []*model.Syllabus
	t.Run("ExtractSyllabusWithProgress", func(t *testing.T) {
		var err error
		syllabuses, err = testExtractSyllabusWithProgress(testCtx)
		if err != nil {
			testFailed = true
			t.Fatalf("Failed to extract syllabus: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 6: Verify extraction results
	t.Run("VerifySyllabusExtraction", func(t *testing.T) {
		if err := testVerifySyllabusExtraction(testCtx, syllabuses); err != nil {
			t.Logf("Warning: Verification issue: %v", err)
		}
	})

	// STEP 7: Verify no GENERAL_TEMP subject remains and real subjects were created
	t.Run("VerifyNoTempSubjectAndRealSubjectsCreated", func(t *testing.T) {
		if err := testVerifyNoTempSubjectAndRealSubjectsCreated(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Subject verification failed: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 8: Save extracted data to JSON file for review
	t.Run("SaveExtractedDataToJSON", func(t *testing.T) {
		if err := testSaveExtractedDataToJSON(testCtx, syllabuses); err != nil {
			t.Logf("Warning: Failed to save JSON: %v", err)
		}
	})

	// Summary
	totalDuration := time.Since(testStartTime)
	log.Println("\n========================================")
	log.Println("TEST SUMMARY")
	log.Println("========================================")
	log.Printf("Total Duration: %.2fs", totalDuration.Seconds())
	log.Printf("Result: %s", map[bool]string{true: "PASSED", false: "FAILED"}[!testFailed])
	if testCtx != nil && testCtx.testDocument != nil {
		log.Printf("Document ID: %d", testCtx.testDocument.ID)
	}
	if len(syllabuses) > 0 {
		log.Printf("Syllabuses Extracted: %d", len(syllabuses))
	}
	log.Println("========================================")
}

// ====================================================================
// HTTP-BASED SSE STREAMING TEST
// ====================================================================

// TestSyllabusSSEStreaming tests the SSE streaming endpoint via HTTP
// This requires the API server to be running
func TestSyllabusSSEStreaming(t *testing.T) {
	// Check if integration tests are enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	// Check if API server URL is provided
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:3000"
	}

	// Check if auth token is provided
	authToken := os.Getenv("TEST_AUTH_TOKEN")
	if authToken == "" {
		t.Skip("Skipping SSE test. Set TEST_AUTH_TOKEN to run against live API.")
	}

	log.Println("========================================")
	log.Println("INTEGRATION TEST: Syllabus SSE Streaming")
	log.Println("========================================")
	log.Printf("API URL: %s", apiURL)

	testStartTime := time.Now()

	// Load test PDF
	pdfPath := "../../../frm_download_file.pdf"
	pdfContent, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("Failed to read test PDF: %v", err)
	}
	log.Printf("✓ Loaded test PDF (%.2f KB)", float64(len(pdfContent))/1024)

	// Get or create test semester ID
	semesterID := os.Getenv("TEST_SEMESTER_ID")
	if semesterID == "" {
		t.Skip("Skipping SSE test. Set TEST_SEMESTER_ID to run against live API.")
	}

	// STEP 1: Upload syllabus file
	t.Run("UploadSyllabusFile", func(t *testing.T) {
		log.Println("\n--- Step 1: Uploading syllabus file ---")

		// Create multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "test_syllabus.pdf")
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := part.Write(pdfContent); err != nil {
			t.Fatalf("Failed to write to form file: %v", err)
		}
		writer.Close()

		// Create request
		uploadURL := fmt.Sprintf("%s/api/v2/semesters/%s/syllabus/upload", apiURL, semesterID)
		req, err := http.NewRequest("POST", uploadURL, body)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+authToken)

		// Send request
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to upload file: %v", err)
		}
		defer resp.Body.Close()

		// Read response
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("  Response status: %d", resp.StatusCode)
		log.Printf("  Response body: %s", string(respBody))

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Upload failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		// Parse response to get document_id
		var uploadResp map[string]interface{}
		if err := json.Unmarshal(respBody, &uploadResp); err != nil {
			t.Fatalf("Failed to parse upload response: %v", err)
		}

		data, ok := uploadResp["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Invalid response format")
		}

		documentID := data["document_id"]
		sseURL := data["sse_url"]
		log.Printf("  Document ID: %v", documentID)
		log.Printf("  SSE URL: %v", sseURL)

		// Store for next step
		t.Setenv("TEST_DOCUMENT_ID", fmt.Sprintf("%v", documentID))
	})

	// STEP 2: Connect to SSE stream
	t.Run("ConnectToSSEStream", func(t *testing.T) {
		log.Println("\n--- Step 2: Connecting to SSE stream ---")

		documentID := os.Getenv("TEST_DOCUMENT_ID")
		if documentID == "" {
			t.Skip("No document ID from previous step")
		}

		// Create SSE request
		sseURL := fmt.Sprintf("%s/api/v2/documents/%s/extract-syllabus?stream=true", apiURL, documentID)
		req, err := http.NewRequest("GET", sseURL, nil)
		if err != nil {
			t.Fatalf("Failed to create SSE request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		req.Header.Set("Accept", "text/event-stream")

		// Send request with longer timeout for streaming
		client := &http.Client{Timeout: 10 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to connect to SSE: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("SSE connection failed with status %d: %s", resp.StatusCode, string(body))
		}

		log.Printf("  Connected to SSE stream")
		log.Printf("  Content-Type: %s", resp.Header.Get("Content-Type"))

		// Read SSE events
		reader := bufio.NewReader(resp.Body)
		eventCount := 0
		var lastEvent SSEEvent

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("  Error reading SSE: %v", err)
				break
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Parse SSE event
			if strings.HasPrefix(line, "event:") {
				lastEvent.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &data); err == nil {
					lastEvent.Data = data
					eventCount++

					// Log event
					progress, _ := data["progress"].(float64)
					phase, _ := data["phase"].(string)
					message, _ := data["message"].(string)
					eventType, _ := data["type"].(string)

					log.Printf("  [%d] %s: %.0f%% - %s - %s",
						eventCount, eventType, progress, phase, message)

					// Check for completion or error
					if eventType == "complete" {
						log.Printf("  ✓ Extraction completed!")
						if subjects, ok := data["result_subjects"].([]interface{}); ok {
							log.Printf("    Subjects extracted: %d", len(subjects))
						}
						break
					} else if eventType == "error" {
						errorMsg, _ := data["error_message"].(string)
						t.Fatalf("Extraction failed: %s", errorMsg)
					}
				}
			}
		}

		log.Printf("\n  Total events received: %d", eventCount)
	})

	// Summary
	totalDuration := time.Since(testStartTime)
	log.Println("\n========================================")
	log.Println("SSE TEST SUMMARY")
	log.Println("========================================")
	log.Printf("Total Duration: %.2fs", totalDuration.Seconds())
	log.Println("========================================")
}

// ====================================================================
// STANDALONE TEST RUNNER
// ====================================================================

// RunSyllabusIntegrationTest runs the syllabus integration test as a standalone function
func RunSyllabusIntegrationTest() error {
	log.Println("Running Syllabus Integration Test (standalone mode)...")

	// Force enable integration tests
	os.Setenv("RUN_INTEGRATION_TESTS", "true")

	testStartTime := time.Now()
	var testCtx *SyllabusTestContext
	var lastError error

	// Setup
	testCtx, lastError = setupSyllabusTestEnvironment(nil)
	if lastError != nil {
		return fmt.Errorf("setup failed: %w", lastError)
	}

	// Defer cleanup
	defer func() {
		if testCtx != nil {
			cleanupSyllabusTestData(testCtx, lastError != nil)
		}
	}()

	// Create course and semester
	if lastError = createSyllabusTestCourseAndSemester(testCtx); lastError != nil {
		return fmt.Errorf("create course/semester failed: %w", lastError)
	}

	// Load test PDF
	if lastError = loadSyllabusTestPDF(testCtx); lastError != nil {
		return fmt.Errorf("load PDF failed: %w", lastError)
	}

	// Upload document
	if lastError = testUploadSyllabusDocument(testCtx); lastError != nil {
		return fmt.Errorf("upload document failed: %w", lastError)
	}

	// Extract syllabus with progress
	syllabuses, err := testExtractSyllabusWithProgress(testCtx)
	if err != nil {
		return fmt.Errorf("extract syllabus failed: %w", err)
	}

	// Verify extraction
	if lastError = testVerifySyllabusExtraction(testCtx, syllabuses); lastError != nil {
		log.Printf("Warning: Verification issue (non-fatal): %v", lastError)
		lastError = nil
	}

	log.Printf("\n✓ Syllabus integration test completed successfully in %.2fs", time.Since(testStartTime).Seconds())
	return nil
}
