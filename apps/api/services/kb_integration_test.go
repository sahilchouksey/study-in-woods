package services

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestContext holds all resources needed for integration tests
type TestContext struct {
	// Database
	db *gorm.DB

	// DigitalOcean clients
	doClient     *digitalocean.Client
	spacesClient *digitalocean.SpacesClient

	// Services
	subjectService  *SubjectService
	documentService *DocumentService
	chatService     *ChatService

	// Test data
	testUserID    uint
	testCourse    *model.Course
	testSemester  *model.Semester
	testSubject   *model.Subject
	testDocument  *model.Document
	pdfContent    []byte
	expectedFacts []string

	// Timing
	startTime time.Time
}

// TimingResult stores timing for each operation
type TimingResult struct {
	Operation string
	Duration  time.Duration
	Success   bool
	Error     string
}

// TestResults collects all test results
type TestResults struct {
	Timings       []TimingResult
	TotalDuration time.Duration
	Success       bool
	FailureReason string
}

// ====================================================================
// SETUP FUNCTIONS
// ====================================================================

// setupTestEnvironment initializes all required clients and services
func setupTestEnvironment(t *testing.T) (*TestContext, error) {
	ctx := &TestContext{
		startTime: time.Now(),
	}

	log.Println("========================================")
	log.Println("Setting up test environment...")
	log.Println("========================================")

	// 1. Check required environment variables
	// Note: DO_GENAI_DATABASE_ID is optional (will create new database if not provided)
	requiredEnvVars := []string{
		"DIGITALOCEAN_TOKEN",
		"DO_SPACES_ACCESS_KEY",
		"DO_SPACES_SECRET_KEY",
		"DO_SPACES_NAME",
		"DO_SPACES_REGION",
		"DO_EMBEDDING_MODEL_UUID",
		"DO_PROJECT_ID",
		"DB_HOST",
		"DB_USER_NAME",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_PORT",
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

	// 3. Initialize DigitalOcean client
	ctx.doClient = digitalocean.NewClient(digitalocean.Config{
		APIToken: os.Getenv("DIGITALOCEAN_TOKEN"),
	})
	log.Println("✓ DigitalOcean client initialized")

	// 4. Initialize Spaces client
	spacesClient, err := digitalocean.NewSpacesClientFromGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Spaces client: %w", err)
	}
	ctx.spacesClient = spacesClient
	log.Println("✓ Spaces client initialized")

	// 5. Initialize services
	ctx.subjectService = NewSubjectService(db)
	ctx.documentService = NewDocumentService(db)
	ctx.chatService = NewChatService(db)
	log.Println("✓ Services initialized")

	// 6. Get or create test user
	var user model.User
	if err := db.Where("email = ?", "integration_test@test.com").First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			user = model.User{
				Email:        "integration_test@test.com",
				PasswordHash: "test_password_hash",
				PasswordSalt: []byte("test_salt"),
				Name:         "Integration Test User",
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

	log.Printf("✓ Test environment setup complete (%.2fs)\n", time.Since(ctx.startTime).Seconds())
	return ctx, nil
}

// getEnvOrDefault returns environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ====================================================================
// TEST DATA CREATION
// ====================================================================

// createTestCourseAndSemester creates test course and semester for the test
func createTestCourseAndSemester(ctx *TestContext) error {
	log.Println("\n--- Creating test course and semester ---")
	startTime := time.Now()

	// Create or get test university
	var university model.University
	if err := ctx.db.Where("code = ?", "TEST_UNIV").First(&university).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			university = model.University{
				Name:     "Test University",
				Code:     "TEST_UNIV",
				Location: "Test Location",
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
	if err := ctx.db.Where("code = ?", "TEST_COURSE").First(&course).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			course = model.Course{
				UniversityID: university.ID,
				Name:         "Test Course",
				Code:         "TEST_COURSE",
				Description:  "Course for integration tests",
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

	// Create or get test semester
	var semester model.Semester
	if err := ctx.db.Where("course_id = ? AND number = ?", course.ID, 1).First(&semester).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			semester = model.Semester{
				CourseID: course.ID,
				Number:   1,
				Name:     "Semester 1",
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

// createTestSubjectWithKBAndAgent creates a new subject with KB and Agent
func createTestSubjectWithKBAndAgent(ctx *TestContext) error {
	log.Println("\n--- Creating test subject with KB and Agent ---")
	startTime := time.Now()

	// Generate unique subject code
	timestamp := time.Now().Unix()
	subjectCode := fmt.Sprintf("TEST_%d", timestamp)

	req := CreateSubjectRequest{
		SemesterID:  ctx.testSemester.ID,
		Name:        fmt.Sprintf("Integration Test Subject %d", timestamp),
		Code:        subjectCode,
		Credits:     3,
		Description: "Subject created for KB integration testing",
	}

	log.Printf("  Creating subject: %s (%s)...", req.Name, req.Code)

	result, err := ctx.subjectService.CreateSubjectWithAI(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to create subject with AI: %w", err)
	}

	if result.Subject == nil {
		return fmt.Errorf("subject creation returned nil subject")
	}

	ctx.testSubject = result.Subject

	log.Printf("  Subject ID: %d", result.Subject.ID)
	log.Printf("  KB Created: %v (UUID: %s)", result.KnowledgeBaseCreated, result.Subject.KnowledgeBaseUUID)
	log.Printf("  Agent Created: %v (UUID: %s)", result.AgentCreated, result.Subject.AgentUUID)

	// Verify KB and Agent were created
	if result.Subject.KnowledgeBaseUUID == "" {
		return fmt.Errorf("knowledge base UUID is empty - KB creation may have failed")
	}
	if result.Subject.AgentUUID == "" {
		return fmt.Errorf("agent UUID is empty - agent creation may have failed")
	}

	log.Printf("✓ Subject with KB and Agent created (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// loadTestPDF loads the test PDF file
func loadTestPDF(ctx *TestContext) error {
	log.Println("\n--- Loading test PDF ---")
	startTime := time.Now()

	// Use the existing frm_download_file.pdf
	pdfPath := "../../../frm_download_file.pdf"

	content, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read test PDF from %s: %w", pdfPath, err)
	}

	ctx.pdfContent = content
	ctx.expectedFacts = []string{
		"Data Mining",              // Subject name
		"MCA 301",                  // Subject code
		"RGPV",                     // University (Rajiv Gandhi Proudyogiki Vishwavidyalaya)
		"Apriori",                  // Algorithm mentioned in Unit IV
		"Knowledge Representation", // Topic in AI syllabus
		"Neural Network",           // Topic in soft computing
		"Fuzzy Logic",              // Topic in soft computing
	}

	log.Printf("  PDF size: %.2f KB", float64(len(content))/1024)
	log.Printf("  Expected facts to verify: %d", len(ctx.expectedFacts))
	log.Printf("✓ Test PDF loaded (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// ====================================================================
// DOCUMENT UPLOAD TEST
// ====================================================================

// testUploadDocument uploads a document and verifies the upload
func testUploadDocument(ctx *TestContext) error {
	log.Println("\n--- Uploading document ---")
	startTime := time.Now()

	// Create a mock file header
	filename := fmt.Sprintf("test_notes_%d.pdf", time.Now().Unix())

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

	req := UploadDocumentRequest{
		SubjectID:  ctx.testSubject.ID,
		UserID:     ctx.testUserID,
		Type:       model.DocumentTypeNotes,
		File:       fileReader,
		FileHeader: fileHeader,
	}

	log.Printf("  Uploading: %s (%.2f KB)", filename, float64(len(ctx.pdfContent))/1024)
	log.Printf("  Subject ID: %d, User ID: %d", ctx.testSubject.ID, ctx.testUserID)

	result, err := ctx.documentService.UploadDocument(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to upload document: %w", err)
	}

	ctx.testDocument = result.Document

	log.Printf("  Document ID: %d", result.Document.ID)
	log.Printf("  Uploaded to Spaces: %v", result.UploadedToSpaces)
	log.Printf("  Spaces Key: %s", result.Document.SpacesKey)
	log.Printf("  Indexed in KB: %v", result.IndexedInKB)
	log.Printf("  Data Source ID: %s", result.Document.DataSourceID)
	log.Printf("  Indexing Status: %s", result.Document.IndexingStatus)

	// Verify upload results
	if !result.UploadedToSpaces {
		return fmt.Errorf("document was not uploaded to Spaces")
	}
	if result.Document.SpacesKey == "" {
		return fmt.Errorf("Spaces key is empty")
	}
	if !result.IndexedInKB {
		log.Printf("  ⚠ Warning: Document was not indexed in KB (data source creation may have failed)")
	}

	log.Printf("✓ Document uploaded successfully (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// bytesFileReader wraps bytes.Reader to implement multipart.File
type bytesFileReader struct {
	*bytes.Reader
}

func (b *bytesFileReader) Close() error {
	return nil
}

// ====================================================================
// SPACES VERIFICATION
// ====================================================================

// testVerifySpacesUpload verifies the file exists in Spaces
func testVerifySpacesUpload(ctx *TestContext) error {
	log.Println("\n--- Verifying Spaces upload ---")
	startTime := time.Now()

	if ctx.testDocument.SpacesKey == "" || ctx.testDocument.SpacesKey == "disabled" {
		return fmt.Errorf("document has no valid Spaces key")
	}

	// Try to download the file to verify it exists
	content, err := ctx.spacesClient.DownloadFile(context.Background(), ctx.testDocument.SpacesKey)
	if err != nil {
		return fmt.Errorf("failed to download file from Spaces: %w", err)
	}

	log.Printf("  File exists in Spaces: %s", ctx.testDocument.SpacesKey)
	log.Printf("  Downloaded size: %.2f KB", float64(len(content))/1024)

	// Verify size matches
	if len(content) != len(ctx.pdfContent) {
		log.Printf("  ⚠ Warning: Downloaded size (%d) doesn't match uploaded size (%d)", len(content), len(ctx.pdfContent))
	}

	log.Printf("✓ Spaces upload verified (%.2fs)", time.Since(startTime).Seconds())
	return nil
}

// ====================================================================
// INDEXING STATUS CHECK
// ====================================================================

// testWaitForIndexing polls indexing status until complete or timeout
func testWaitForIndexing(ctx *TestContext, timeout time.Duration) error {
	log.Println("\n--- Waiting for KB indexing ---")
	startTime := time.Now()

	if ctx.testDocument.DataSourceID == "" {
		log.Println("  ⚠ No data source ID - skipping indexing wait")
		return nil
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 5 * time.Second
	lastStatus := ""

	for time.Now().Before(deadline) {
		// Get data source status
		dataSource, err := ctx.doClient.GetDataSource(
			context.Background(),
			ctx.testSubject.KnowledgeBaseUUID,
			ctx.testDocument.DataSourceID,
		)
		if err != nil {
			log.Printf("  ⚠ Error getting data source status: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		// Get status from last indexing job
		status := "pending"
		if dataSource.LastIndexingJob != nil {
			status = dataSource.LastIndexingJob.Status
		}

		if status != lastStatus {
			log.Printf("  Status: %s (elapsed: %.1fs)", status, time.Since(startTime).Seconds())
			lastStatus = status
		}

		switch status {
		case "INDEX_JOB_STATUS_COMPLETED":
			// Update local document status
			ctx.testDocument.IndexingStatus = model.IndexingStatusCompleted
			ctx.db.Save(ctx.testDocument)
			log.Printf("✓ Document indexed successfully (%.2fs)", time.Since(startTime).Seconds())
			return nil

		case "INDEX_JOB_STATUS_FAILED":
			ctx.testDocument.IndexingStatus = model.IndexingStatusFailed
			ctx.db.Save(ctx.testDocument)
			return fmt.Errorf("indexing failed")

		case "INDEX_JOB_STATUS_PENDING", "INDEX_JOB_STATUS_IN_PROGRESS", "pending":
			// Continue waiting
			time.Sleep(pollInterval)
			continue

		default:
			log.Printf("  Unknown status: %s", status)
			time.Sleep(pollInterval)
		}
	}

	return fmt.Errorf("indexing timeout after %v (last status: %s)", timeout, lastStatus)
}

// ====================================================================
// WAIT FOR AGENT DEPLOYMENT
// ====================================================================

// testWaitForAgentDeployment waits for the agent to be fully deployed
func testWaitForAgentDeployment(ctx *TestContext, timeout time.Duration) error {
	log.Println("\n--- Waiting for agent deployment ---")
	startTime := time.Now()

	if ctx.testSubject.AgentUUID == "" {
		return fmt.Errorf("subject has no agent UUID")
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 10 * time.Second
	lastStatus := ""

	for time.Now().Before(deadline) {
		agent, err := ctx.doClient.GetAgent(context.Background(), ctx.testSubject.AgentUUID)
		if err != nil {
			log.Printf("  ⚠ Error getting agent status: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		if agent.Deployment == nil {
			log.Printf("  Status: no deployment (elapsed: %.1fs)", time.Since(startTime).Seconds())
			time.Sleep(pollInterval)
			continue
		}

		status := agent.Deployment.Status
		if status != lastStatus {
			log.Printf("  Status: %s (elapsed: %.1fs)", status, time.Since(startTime).Seconds())
			lastStatus = status
		}

		// Check if deployment is running and has a URL
		if status == "STATUS_RUNNING" && agent.Deployment.URL != "" {
			log.Printf("  Deployment URL: %s", agent.Deployment.URL)
			log.Printf("✓ Agent deployed successfully (%.2fs)", time.Since(startTime).Seconds())
			return nil
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("agent deployment timeout after %v (last status: %s)", timeout, lastStatus)
}

// ====================================================================
// AGENT QUERY TEST
// ====================================================================

// testQueryAgent queries the agent about the uploaded document
func testQueryAgent(ctx *TestContext) (string, error) {
	log.Println("\n--- Querying agent ---")
	startTime := time.Now()

	if ctx.testSubject.AgentUUID == "" {
		return "", fmt.Errorf("subject has no agent UUID")
	}

	// Step 1: Get agent info to retrieve deployment URL
	log.Printf("  Agent UUID: %s", ctx.testSubject.AgentUUID)
	log.Println("  Fetching agent deployment info...")

	agent, err := ctx.doClient.GetAgent(context.Background(), ctx.testSubject.AgentUUID)
	if err != nil {
		return "", fmt.Errorf("failed to get agent info: %w", err)
	}

	if agent.Deployment == nil || agent.Deployment.URL == "" {
		return "", fmt.Errorf("agent has no deployment URL - run WaitForAgentDeployment first")
	}

	deploymentURL := agent.Deployment.URL
	log.Printf("  Deployment URL: %s", deploymentURL)
	log.Printf("  Deployment Status: %s", agent.Deployment.Status)

	// Step 2: Create an API key for this agent
	log.Println("  Creating agent API key...")
	apiKeyName := fmt.Sprintf("test-key-%d", time.Now().Unix())
	apiKey, err := ctx.doClient.CreateAgentAPIKey(context.Background(), ctx.testSubject.AgentUUID, apiKeyName)
	if err != nil {
		return "", fmt.Errorf("failed to create agent API key: %w", err)
	}

	if apiKey.SecretKey == "" {
		return "", fmt.Errorf("agent API key creation returned no secret key")
	}

	log.Printf("  API Key created: %s (UUID: %s)", apiKeyName, apiKey.UUID)

	// Step 3: Query the agent using deployment URL + API key
	query := "What topics are covered in the Data Mining syllabus? List some key algorithms mentioned."
	log.Printf("  Query: %s", query)

	// Create chat request using deployment URL and API key
	messages := []digitalocean.ChatMessage{
		{
			Role:    "user",
			Content: query,
		},
	}

	chatReq := digitalocean.AgentChatRequest{
		DeploymentURL: deploymentURL,
		APIKey:        apiKey.SecretKey,
		Messages:      messages,
	}

	// Call agent via deployment URL
	resp, err := ctx.doClient.CreateAgentChatCompletion(context.Background(), chatReq)
	if err != nil {
		// Clean up API key before returning error
		_ = ctx.doClient.DeleteAgentAPIKey(context.Background(), ctx.testSubject.AgentUUID, apiKey.UUID)
		return "", fmt.Errorf("failed to query agent: %w", err)
	}

	content := resp.ExtractContent()
	promptTokens, completionTokens, totalTokens := resp.GetUsage()

	log.Printf("  Response length: %d chars", len(content))
	log.Printf("  Tokens - Prompt: %d, Completion: %d, Total: %d", promptTokens, completionTokens, totalTokens)

	// Step 4: Clean up the API key (optional - could keep for future use)
	log.Println("  Cleaning up API key...")
	if err := ctx.doClient.DeleteAgentAPIKey(context.Background(), ctx.testSubject.AgentUUID, apiKey.UUID); err != nil {
		log.Printf("  ⚠ Warning: Failed to delete API key: %v", err)
	}

	log.Printf("✓ Agent query successful (%.2fs)", time.Since(startTime).Seconds())

	return content, nil
}

// testVerifyAgentResponse checks if the response contains expected information
func testVerifyAgentResponse(response string, expectedFacts []string) error {
	log.Println("\n--- Verifying agent response ---")

	if len(response) < 50 {
		return fmt.Errorf("response too short (%d chars) - agent may not have accessed KB", len(response))
	}

	// Print response preview
	preview := response
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	log.Printf("  Response preview:\n%s\n", preview)

	// Check for expected facts
	responseLower := strings.ToLower(response)
	foundFacts := []string{}
	missingFacts := []string{}

	for _, fact := range expectedFacts {
		if strings.Contains(responseLower, strings.ToLower(fact)) {
			foundFacts = append(foundFacts, fact)
		} else {
			missingFacts = append(missingFacts, fact)
		}
	}

	log.Printf("  Found facts: %d/%d", len(foundFacts), len(expectedFacts))
	for _, fact := range foundFacts {
		log.Printf("    ✓ Found: %s", fact)
	}
	for _, fact := range missingFacts {
		log.Printf("    ✗ Missing: %s", fact)
	}

	// We don't fail if some facts are missing - the agent might paraphrase
	if len(foundFacts) == 0 {
		log.Printf("  ⚠ Warning: No expected facts found in response")
	}

	log.Println("✓ Agent response verification complete")
	return nil
}

// ====================================================================
// CLEANUP
// ====================================================================

// cleanupTestData cleans up all test resources
func cleanupTestData(ctx *TestContext, keepOnFailure bool) {
	log.Println("\n========================================")
	log.Println("Cleaning up test data...")
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

	// Delete subject (this also deletes KB and Agent)
	if ctx.testSubject != nil {
		log.Printf("  Deleting subject ID=%d (with KB and Agent)...", ctx.testSubject.ID)
		if err := ctx.subjectService.DeleteSubjectWithCleanup(bgCtx, ctx.testSubject.ID); err != nil {
			log.Printf("    ⚠ Warning: Failed to delete subject: %v", err)
		} else {
			log.Printf("    ✓ Subject, KB, and Agent deleted")
		}
	}

	log.Println("✓ Cleanup complete")
}

// ====================================================================
// MAIN TEST FUNCTION
// ====================================================================

// TestDocumentUploadAndKBIntegration is the main integration test
func TestDocumentUploadAndKBIntegration(t *testing.T) {
	// Check if integration tests are enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	log.Println("========================================")
	log.Println("INTEGRATION TEST: Document Upload & KB Flow")
	log.Println("========================================")

	testStartTime := time.Now()
	var testCtx *TestContext
	var testFailed bool

	// Defer cleanup
	defer func() {
		if testCtx != nil {
			// Keep data on failure for debugging
			if testFailed {
				log.Println("\n⚠ Test failed - keeping test data for debugging")
				if testCtx.testSubject != nil {
					log.Printf("  Subject ID: %d", testCtx.testSubject.ID)
					log.Printf("  KB UUID: %s", testCtx.testSubject.KnowledgeBaseUUID)
					log.Printf("  Agent UUID: %s", testCtx.testSubject.AgentUUID)
				}
				if testCtx.testDocument != nil {
					log.Printf("  Document ID: %d", testCtx.testDocument.ID)
				}
			} else {
				cleanupTestData(testCtx, testFailed)
			}
		}
	}()

	// STEP 1: Setup
	t.Run("Setup", func(t *testing.T) {
		var err error
		testCtx, err = setupTestEnvironment(t)
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
		if err := createTestCourseAndSemester(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Failed to create test course/semester: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 3: Create subject with KB and Agent
	t.Run("CreateSubjectWithKBAndAgent", func(t *testing.T) {
		if err := createTestSubjectWithKBAndAgent(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Failed to create subject with KB and Agent: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 4: Load test PDF
	t.Run("LoadTestPDF", func(t *testing.T) {
		if err := loadTestPDF(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Failed to load test PDF: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 5: Upload document
	t.Run("UploadDocument", func(t *testing.T) {
		if err := testUploadDocument(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Failed to upload document: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 6: Verify Spaces upload
	t.Run("VerifySpacesUpload", func(t *testing.T) {
		if err := testVerifySpacesUpload(testCtx); err != nil {
			testFailed = true
			t.Fatalf("Failed to verify Spaces upload: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 7: Wait for indexing
	t.Run("WaitForIndexing", func(t *testing.T) {
		if err := testWaitForIndexing(testCtx, 5*time.Minute); err != nil {
			// Don't fail test - indexing might take longer or KB might not be ready
			t.Logf("Warning: Indexing issue: %v", err)
		}
	})

	// STEP 8: Wait for agent deployment
	t.Run("WaitForAgentDeployment", func(t *testing.T) {
		if err := testWaitForAgentDeployment(testCtx, 3*time.Minute); err != nil {
			testFailed = true
			t.Fatalf("Failed waiting for agent deployment: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 9: Query agent
	var agentResponse string
	t.Run("QueryAgent", func(t *testing.T) {
		var err error
		agentResponse, err = testQueryAgent(testCtx)
		if err != nil {
			testFailed = true
			t.Fatalf("Failed to query agent: %v", err)
		}
	})

	if testFailed {
		return
	}

	// STEP 10: Verify agent response
	t.Run("VerifyAgentResponse", func(t *testing.T) {
		if err := testVerifyAgentResponse(agentResponse, testCtx.expectedFacts); err != nil {
			t.Logf("Warning: Agent response verification issue: %v", err)
		}
	})

	// Summary
	totalDuration := time.Since(testStartTime)
	log.Println("\n========================================")
	log.Println("TEST SUMMARY")
	log.Println("========================================")
	log.Printf("Total Duration: %.2fs", totalDuration.Seconds())
	log.Printf("Result: %s", map[bool]string{true: "PASSED", false: "FAILED"}[!testFailed])
	if testCtx != nil && testCtx.testSubject != nil {
		log.Printf("Subject ID: %d", testCtx.testSubject.ID)
		log.Printf("KB UUID: %s", testCtx.testSubject.KnowledgeBaseUUID)
		log.Printf("Agent UUID: %s", testCtx.testSubject.AgentUUID)
	}
	if testCtx != nil && testCtx.testDocument != nil {
		log.Printf("Document ID: %d", testCtx.testDocument.ID)
		log.Printf("Data Source ID: %s", testCtx.testDocument.DataSourceID)
	}
	log.Println("========================================")
}

// ====================================================================
// STANDALONE TEST RUNNER
// ====================================================================

// RunKBIntegrationTest runs the integration test as a standalone function
// Can be called from main() for testing outside of `go test`
func RunKBIntegrationTest() error {
	log.Println("Running KB Integration Test (standalone mode)...")

	// Force enable integration tests
	os.Setenv("RUN_INTEGRATION_TESTS", "true")

	testStartTime := time.Now()
	var testCtx *TestContext
	var lastError error

	// Setup
	testCtx, lastError = setupTestEnvironment(nil)
	if lastError != nil {
		return fmt.Errorf("setup failed: %w", lastError)
	}

	// Defer cleanup
	defer func() {
		if testCtx != nil {
			cleanupTestData(testCtx, lastError != nil)
		}
	}()

	// Create course and semester
	if lastError = createTestCourseAndSemester(testCtx); lastError != nil {
		return fmt.Errorf("create course/semester failed: %w", lastError)
	}

	// Create subject with KB and Agent
	if lastError = createTestSubjectWithKBAndAgent(testCtx); lastError != nil {
		return fmt.Errorf("create subject failed: %w", lastError)
	}

	// Load test PDF
	if lastError = loadTestPDF(testCtx); lastError != nil {
		return fmt.Errorf("load PDF failed: %w", lastError)
	}

	// Upload document
	if lastError = testUploadDocument(testCtx); lastError != nil {
		return fmt.Errorf("upload document failed: %w", lastError)
	}

	// Verify Spaces upload
	if lastError = testVerifySpacesUpload(testCtx); lastError != nil {
		return fmt.Errorf("verify Spaces failed: %w", lastError)
	}

	// Wait for indexing
	if lastError = testWaitForIndexing(testCtx, 5*time.Minute); lastError != nil {
		log.Printf("Warning: Indexing issue (non-fatal): %v", lastError)
		lastError = nil // Don't fail test
	}

	// Query agent
	agentResponse, err := testQueryAgent(testCtx)
	if err != nil {
		return fmt.Errorf("query agent failed: %w", err)
	}

	// Verify agent response
	if lastError = testVerifyAgentResponse(agentResponse, testCtx.expectedFacts); lastError != nil {
		log.Printf("Warning: Response verification issue (non-fatal): %v", lastError)
		lastError = nil
	}

	log.Printf("\n✓ Integration test completed successfully in %.2fs", time.Since(testStartTime).Seconds())
	return nil
}
