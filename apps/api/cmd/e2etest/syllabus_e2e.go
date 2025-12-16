package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"github.com/sahilchouksey/go-init-setup/utils/crypto"
	"gorm.io/gorm"
)

// RunSyllabusE2ETest runs the full E2E test:
// 1. Extract syllabus from PDF -> Creates subjects with KB/Agent
// 2. Upload PYQ documents to the subject's KB
// 3. Wait for ingestion and agent deployment
// 4. Query the agent with questions from the PYQ paper
func RunSyllabusE2ETest() {
	log.Println("══════════════════════════════════════════════════════════════════")
	log.Println("  E2E TEST: Syllabus Extraction → Document Ingest → Agent Query")
	log.Println("══════════════════════════════════════════════════════════════════")

	ctx := context.Background()

	// Step 1: Initialize
	log.Println("\n[STEP 1] Initializing...")
	db, doClient, err := initialize()
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	// Step 2: Find the syllabus PDF and PYQ PDF
	log.Println("\n[STEP 2] Finding test PDFs...")
	syllabusPDF, pyqPDF := findTestPDFs()
	log.Printf("  Syllabus PDF: %s", syllabusPDF)
	log.Printf("  PYQ PDF: %s", pyqPDF)

	if syllabusPDF == "" || pyqPDF == "" {
		log.Fatalf("Required PDFs not found. Please ensure frm_download_file.pdf and mca-301-data-mining-dec-2024.pdf exist")
	}

	// Step 3: Create or get test semester
	log.Println("\n[STEP 3] Setting up test semester...")
	semester, user, err := setupTestSemesterForSyllabus(ctx, db)
	if err != nil {
		log.Fatalf("Failed to setup test semester: %v", err)
	}
	log.Printf("  Semester: %s (ID: %d)", semester.Name, semester.ID)
	log.Printf("  User: %s (ID: %d)", user.Email, user.ID)

	// Step 4: Upload syllabus document and trigger extraction
	log.Println("\n[STEP 4] Uploading syllabus and triggering extraction...")
	syllabusDoc, err := uploadAndExtractSyllabus(ctx, db, doClient, syllabusPDF, semester, user)
	if err != nil {
		log.Fatalf("Syllabus extraction failed: %v", err)
	}
	log.Printf("  ✓ Syllabus document created: ID=%d", syllabusDoc.ID)

	// Step 5: Wait for AI setup to complete (async goroutine)
	log.Println("\n[STEP 5] Waiting for subjects to be created with AI resources...")
	log.Println("  (AI setup runs asynchronously, waiting 15 seconds...)")
	time.Sleep(15 * time.Second)

	// Find subjects created by extraction (MCA 301 Data Mining)
	subject, err := findDataMiningSubject(ctx, db, semester.ID)
	if err != nil {
		log.Printf("  ⚠️  Could not find Data Mining subject: %v", err)
		log.Println("  Looking for any subject in this semester...")
		var subjects []model.Subject
		db.Where("semester_id = ?", semester.ID).Find(&subjects)
		if len(subjects) == 0 {
			log.Fatalf("No subjects found after syllabus extraction")
		}
		subject = &subjects[0]
	}

	log.Printf("  Found subject: %s (Code: %s, ID: %d)", subject.Name, subject.Code, subject.ID)
	log.Printf("  KB UUID: %s", subject.KnowledgeBaseUUID)
	log.Printf("  Agent UUID: %s", subject.AgentUUID)

	// Step 6: Wait for AI resources if not ready
	if subject.KnowledgeBaseUUID == "" || subject.AgentUUID == "" {
		log.Println("\n[STEP 6] AI resources not ready yet, waiting...")
		subject, err = waitForSubjectAISetup(ctx, db, subject.ID, 5*time.Minute)
		if err != nil {
			log.Printf("  ⚠️  AI setup timeout: %v", err)
			log.Println("  Manually triggering AI setup...")

			subjectService := services.NewSubjectService(db)
			result, err := subjectService.SetupSubjectAI(ctx, subject.ID)
			if err != nil {
				log.Fatalf("Manual AI setup failed: %v", err)
			}
			subject = result.Subject
			log.Printf("  ✓ Manual AI setup complete (KB: %v, Agent: %v, APIKey: %v)",
				result.KnowledgeBaseCreated, result.AgentCreated, result.APIKeyCreated)
		} else {
			log.Printf("  ✓ AI resources ready")
		}
	} else {
		log.Println("\n[STEP 6] AI resources already configured!")
	}

	log.Printf("  KB UUID: %s", subject.KnowledgeBaseUUID)
	log.Printf("  Agent UUID: %s", subject.AgentUUID)
	if subject.AgentAPIKeyEncrypted != "" {
		log.Printf("  Encrypted API Key: %s...", truncateString(subject.AgentAPIKeyEncrypted, 20))
	}

	// Step 7: Upload PYQ documents via batch ingest
	log.Println("\n[STEP 7] Uploading PYQ documents via batch ingest...")
	job, err := uploadPYQDocuments(ctx, db, doClient, subject, user, pyqPDF)
	if err != nil {
		log.Fatalf("PYQ upload failed: %v", err)
	}
	log.Printf("  ✓ Batch ingest job started: ID=%d", job.JobID)

	// Step 8: Wait for batch ingest to complete
	log.Println("\n[STEP 8] Waiting for batch ingest job to complete...")
	notificationService := services.NewNotificationService(db)
	pyqService := services.NewPYQService(db)
	batchIngestService := services.NewBatchIngestService(db, notificationService, pyqService)

	completedJob, err := waitForJobCompletion(ctx, batchIngestService, job.JobID, user.ID, 5*time.Minute)
	if err != nil {
		log.Fatalf("Batch ingest failed: %v", err)
	}
	log.Printf("  ✓ Batch ingest completed: Status=%s, Completed=%d, Failed=%d",
		completedJob.Status, completedJob.CompletedItems, completedJob.FailedItems)

	// Step 9: Wait for DO indexing if triggered
	if completedJob.DOIndexingJobUUID != "" && doClient != nil {
		log.Println("\n[STEP 9] Waiting for DigitalOcean KB indexing...")
		log.Printf("  DO Indexing Job: %s", completedJob.DOIndexingJobUUID)
		err = waitForDOIndexingComplete(ctx, doClient, subject.KnowledgeBaseUUID, completedJob.DOIndexingJobUUID, 10*time.Minute)
		if err != nil {
			log.Printf("  ⚠️  DO indexing may still be in progress: %v", err)
		} else {
			log.Println("  ✓ DO KB indexing completed!")
		}
	} else {
		log.Println("\n[STEP 9] Skipped - No DO indexing job triggered")
	}

	// Step 10: Wait for agent deployment
	log.Println("\n[STEP 10] Checking agent deployment status...")
	if doClient != nil && subject.AgentUUID != "" {
		isDeployed, agent, err := doClient.IsAgentDeployed(ctx, subject.AgentUUID)
		if err != nil {
			log.Printf("  ⚠️  Failed to check agent status: %v", err)
		} else if !isDeployed {
			log.Println("  Agent not deployed, waiting for deployment...")
			agent, err = doClient.WaitForAgentDeployment(ctx, subject.AgentUUID, 5*time.Minute)
			if err != nil {
				log.Printf("  ⚠️  Agent deployment wait failed: %v", err)
			} else {
				isDeployed = true
				// Update deployment URL in subject
				if agent.Deployment != nil && agent.Deployment.URL != "" {
					subject.AgentDeploymentURL = agent.Deployment.URL
					db.Save(subject)
				}
			}
		}

		if isDeployed && agent != nil && agent.Deployment != nil {
			log.Printf("  ✓ Agent deployed at: %s", agent.Deployment.URL)
		}
	}

	// Step 11: Query the agent
	log.Println("\n[STEP 11] Querying the agent with PYQ-related questions...")
	err = queryAgentWithStoredKey(ctx, db, doClient, subject)
	if err != nil {
		log.Printf("  ⚠️  Agent query failed: %v", err)
	}

	// Summary
	log.Println("\n══════════════════════════════════════════════════════════════════")
	log.Println("  TEST SUMMARY")
	log.Println("══════════════════════════════════════════════════════════════════")
	log.Printf("  Subject: %s (Code: %s)", subject.Name, subject.Code)
	log.Printf("  Knowledge Base UUID: %s", subject.KnowledgeBaseUUID)
	log.Printf("  Agent UUID: %s", subject.AgentUUID)
	log.Printf("  Agent Deployment URL: %s", subject.AgentDeploymentURL)
	log.Printf("  API Key Stored: %v", subject.AgentAPIKeyEncrypted != "")
	log.Printf("  Batch Ingest: %s (Completed: %d, Failed: %d)",
		completedJob.Status, completedJob.CompletedItems, completedJob.FailedItems)

	allGood := subject.KnowledgeBaseUUID != "" &&
		subject.AgentUUID != "" &&
		subject.AgentAPIKeyEncrypted != "" &&
		completedJob.Status == model.IndexingJobStatusCompleted

	if allGood {
		log.Println("\n  ✅ E2E TEST PASSED - Full flow working!")
	} else {
		log.Println("\n  ⚠️  PARTIAL SUCCESS - Some components may need attention")
	}
	log.Println("══════════════════════════════════════════════════════════════════")
}

func findTestPDFs() (syllabusPDF, pyqPDF string) {
	// Look for PDFs in various locations
	possibleRoots := []string{
		"../../..", // From cmd/e2etest
		"../..",    // From apps/api
		".",        // Current dir
		"/Users/sahilchouksey/Documents/fun/study-in-woods",
	}

	var baseDir string
	for _, root := range possibleRoots {
		if _, err := os.Stat(filepath.Join(root, "frm_download_file.pdf")); err == nil {
			baseDir = root
			break
		}
	}

	if baseDir == "" {
		log.Println("  ⚠️  Could not find project root with test PDFs")
		return "", ""
	}

	syllabusPDF = filepath.Join(baseDir, "frm_download_file.pdf")
	pyqPDF = filepath.Join(baseDir, "mca-301-data-mining-dec-2024.pdf")

	// Verify files exist
	if _, err := os.Stat(syllabusPDF); err != nil {
		log.Printf("  ⚠️  Syllabus PDF not found at: %s", syllabusPDF)
		syllabusPDF = ""
	}
	if _, err := os.Stat(pyqPDF); err != nil {
		log.Printf("  ⚠️  PYQ PDF not found at: %s", pyqPDF)
		pyqPDF = ""
	}

	return syllabusPDF, pyqPDF
}

func setupTestSemesterForSyllabus(ctx context.Context, db *gorm.DB) (*model.Semester, *model.User, error) {
	// Get or create test user
	var user model.User
	err := db.Where("email = ?", "syllabus_e2e_test@test.com").First(&user).Error
	if err == gorm.ErrRecordNotFound {
		user = model.User{
			Email:        "syllabus_e2e_test@test.com",
			PasswordHash: "test_hash",
			PasswordSalt: []byte("test_salt"),
			Name:         "Syllabus E2E Test User",
			Role:         "admin",
		}
		if err := db.Create(&user).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, nil, err
	}

	// Try to find existing MCA Semester 3
	var semester model.Semester
	err = db.Preload("Course.University").
		Joins("JOIN courses ON courses.id = semesters.course_id").
		Where("courses.code LIKE ? AND semesters.number = ?", "%MCA%", 3).
		First(&semester).Error

	if err == nil {
		log.Printf("  Found existing semester: %s (Course: %s)", semester.Name, semester.Course.Name)
		return &semester, &user, nil
	}

	// Create test hierarchy
	log.Println("  Creating test university/course/semester structure...")

	// University
	var university model.University
	err = db.Where("code = ?", "RGPV_E2E").First(&university).Error
	if err == gorm.ErrRecordNotFound {
		university = model.University{
			Name:     "RGPV E2E Test University",
			Code:     "RGPV_E2E",
			Location: "Bhopal",
		}
		db.Create(&university)
	}

	// Course
	var course model.Course
	err = db.Where("code = ? AND university_id = ?", "MCA_E2E", university.ID).First(&course).Error
	if err == gorm.ErrRecordNotFound {
		course = model.Course{
			UniversityID: university.ID,
			Name:         "Master of Computer Applications (E2E Test)",
			Code:         "MCA_E2E",
			Duration:     4,
		}
		db.Create(&course)
	}

	// Semester
	err = db.Where("course_id = ? AND number = ?", course.ID, 3).First(&semester).Error
	if err == gorm.ErrRecordNotFound {
		semester = model.Semester{
			CourseID: course.ID,
			Number:   3,
			Name:     "Semester 3 (E2E Test)",
		}
		db.Create(&semester)
	}

	return &semester, &user, nil
}

func uploadAndExtractSyllabus(ctx context.Context, db *gorm.DB, doClient *digitalocean.Client, syllabusPDF string, semester *model.Semester, user *model.User) (*model.Document, error) {
	if syllabusPDF == "" {
		return nil, fmt.Errorf("syllabus PDF path not provided")
	}

	// Read PDF content
	pdfContent, err := os.ReadFile(syllabusPDF)
	if err != nil {
		return nil, fmt.Errorf("failed to read syllabus PDF: %w", err)
	}
	log.Printf("  Read syllabus PDF: %.2f KB", float64(len(pdfContent))/1024)

	// Try to get Spaces client for upload
	spacesClient, err := digitalocean.NewSpacesClientFromGlobalConfig()
	var spacesURL, spacesKey string
	if err != nil {
		log.Printf("  ⚠️  Spaces client not available: %v", err)
		log.Println("  Using placeholder URLs...")
		spacesKey = fmt.Sprintf("syllabus/e2e-test-%d.pdf", time.Now().Unix())
		spacesURL = fmt.Sprintf("https://placeholder.spaces.digitalocean.com/%s", spacesKey)
	} else {
		// Upload to Spaces
		spacesKey = fmt.Sprintf("syllabus/e2e-test-%d.pdf", time.Now().Unix())
		spacesURL, err = spacesClient.UploadBytes(ctx, spacesKey, pdfContent, "application/pdf")
		if err != nil {
			log.Printf("  ⚠️  Failed to upload to Spaces: %v", err)
			spacesURL = fmt.Sprintf("https://placeholder.spaces.digitalocean.com/%s", spacesKey)
		} else {
			log.Printf("  ✓ Uploaded to Spaces: %s", spacesKey)
		}
	}

	// Create document record
	doc := model.Document{
		Type:             model.DocumentTypeSyllabus,
		Filename:         "rgpv-mca-sem3-syllabus-e2e.pdf",
		SpacesURL:        spacesURL,
		SpacesKey:        spacesKey,
		IndexingStatus:   model.IndexingStatusPending,
		SemesterID:       &semester.ID,
		UploadedByUserID: user.ID,
		FileSize:         int64(len(pdfContent)),
	}
	if err := db.Create(&doc).Error; err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	// Initialize syllabus service and extract
	syllabusService := services.NewSyllabusService(db)

	log.Println("  Starting syllabus extraction...")
	results, err := syllabusService.ExtractSyllabusFromDocument(ctx, doc.ID)
	if err != nil {
		return &doc, fmt.Errorf("syllabus extraction failed: %w", err)
	}

	log.Printf("  ✓ Extracted %d syllabi from document", len(results))
	for i, result := range results {
		log.Printf("    [%d] %s (Code: %s) - %d units",
			i+1, result.SubjectName, result.SubjectCode, len(result.Units))
	}

	// Update document status
	doc.IndexingStatus = model.IndexingStatusCompleted
	db.Save(&doc)

	return &doc, nil
}

func findDataMiningSubject(ctx context.Context, db *gorm.DB, semesterID uint) (*model.Subject, error) {
	var subject model.Subject

	// Try to find MCA 301 Data Mining
	err := db.Where("semester_id = ? AND (code LIKE ? OR name LIKE ?)",
		semesterID, "%301%", "%Data Mining%").First(&subject).Error

	if err != nil {
		return nil, err
	}
	return &subject, nil
}

func waitForSubjectAISetup(ctx context.Context, db *gorm.DB, subjectID uint, timeout time.Duration) (*model.Subject, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 10 * time.Second

	for time.Now().Before(deadline) {
		var subject model.Subject
		if err := db.First(&subject, subjectID).Error; err != nil {
			return nil, err
		}

		if subject.KnowledgeBaseUUID != "" && subject.AgentUUID != "" && subject.AgentAPIKeyEncrypted != "" {
			return &subject, nil
		}

		log.Printf("  Waiting for AI setup... (KB: %v, Agent: %v, APIKey: %v)",
			subject.KnowledgeBaseUUID != "",
			subject.AgentUUID != "",
			subject.AgentAPIKeyEncrypted != "")

		time.Sleep(pollInterval)
	}

	// Return current state even if not complete
	var subject model.Subject
	db.First(&subject, subjectID)
	return &subject, fmt.Errorf("timeout waiting for AI setup")
}

func uploadPYQDocuments(ctx context.Context, db *gorm.DB, doClient *digitalocean.Client, subject *model.Subject, user *model.User, pyqPDF string) (*services.BatchIngestResult, error) {
	if pyqPDF == "" {
		return nil, fmt.Errorf("PYQ PDF path not provided")
	}

	// Read PDF content
	pdfContent, err := os.ReadFile(pyqPDF)
	if err != nil {
		return nil, fmt.Errorf("failed to read PYQ PDF: %w", err)
	}
	log.Printf("  Read PYQ PDF: %.2f KB", float64(len(pdfContent))/1024)

	// Upload to Spaces first
	spacesClient, err := digitalocean.NewSpacesClientFromGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("Spaces client required for batch ingest: %w", err)
	}

	timestamp := time.Now().Unix()

	// Upload multiple copies to simulate multiple PYQs
	pdfURLs := make([]string, 3)
	for i := 0; i < 3; i++ {
		spacesKey := fmt.Sprintf("pyq/e2e-test-%d-paper%d.pdf", timestamp, i+1)
		url, err := spacesClient.UploadBytes(ctx, spacesKey, pdfContent, "application/pdf")
		if err != nil {
			return nil, fmt.Errorf("failed to upload PDF %d: %w", i+1, err)
		}
		pdfURLs[i] = url
		log.Printf("  ✓ Uploaded PYQ %d: %s", i+1, spacesKey)
	}

	// Create batch ingest request
	notificationService := services.NewNotificationService(db)
	pyqService := services.NewPYQService(db)
	batchIngestService := services.NewBatchIngestService(db, notificationService, pyqService)

	ingestReq := services.BatchIngestRequest{
		SubjectID: subject.ID,
		UserID:    user.ID,
		Papers: []services.BatchIngestPaperRequest{
			{
				PDFURL:     pdfURLs[0],
				Title:      fmt.Sprintf("MCA 301 Data Mining - Dec 2024 (E2E Test %d-1)", timestamp),
				Year:       2024,
				Month:      "December",
				ExamType:   "End Semester",
				SourceName: "E2E Test",
			},
			{
				PDFURL:     pdfURLs[1],
				Title:      fmt.Sprintf("MCA 301 Data Mining - May 2024 (E2E Test %d-2)", timestamp),
				Year:       2024,
				Month:      "May",
				ExamType:   "End Semester",
				SourceName: "E2E Test",
			},
			{
				PDFURL:     pdfURLs[2],
				Title:      fmt.Sprintf("MCA 301 Data Mining - Dec 2023 (E2E Test %d-3)", timestamp),
				Year:       2023,
				Month:      "December",
				ExamType:   "End Semester",
				SourceName: "E2E Test",
			},
		},
	}

	result, err := batchIngestService.StartBatchIngest(ctx, ingestReq)
	if err != nil {
		return nil, fmt.Errorf("batch ingest start failed: %w", err)
	}

	return result, nil
}

func queryAgentWithStoredKey(ctx context.Context, db *gorm.DB, doClient *digitalocean.Client, subject *model.Subject) error {
	if doClient == nil {
		return fmt.Errorf("DO client not initialized")
	}

	if subject.AgentUUID == "" {
		return fmt.Errorf("no agent configured")
	}

	// Get agent to check deployment
	agent, err := doClient.GetAgent(ctx, subject.AgentUUID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	if agent.Deployment == nil || agent.Deployment.URL == "" {
		return fmt.Errorf("agent not deployed")
	}

	// Decrypt stored API key or create new one
	var apiKey string
	if subject.AgentAPIKeyEncrypted != "" {
		apiKey, err = crypto.DecryptAPIKeyFromStorage(subject.AgentAPIKeyEncrypted)
		if err != nil {
			log.Printf("  ⚠️  Failed to decrypt stored API key: %v", err)
			log.Println("  Creating new API key...")
			apiKeyResult, err := doClient.CreateAgentAPIKey(ctx, subject.AgentUUID, fmt.Sprintf("e2e-query-%d", time.Now().Unix()))
			if err != nil {
				return fmt.Errorf("failed to create API key: %w", err)
			}
			apiKey = apiKeyResult.SecretKey
		} else {
			log.Println("  ✓ Using stored encrypted API key")
		}
	} else {
		log.Println("  No stored API key, creating new one...")
		apiKeyResult, err := doClient.CreateAgentAPIKey(ctx, subject.AgentUUID, fmt.Sprintf("e2e-query-%d", time.Now().Unix()))
		if err != nil {
			return fmt.Errorf("failed to create API key: %w", err)
		}
		apiKey = apiKeyResult.SecretKey
	}

	// Questions based on the PYQ content (MCA-301 Data Mining Dec 2024)
	questions := []string{
		"What is the Apriori algorithm and how is it used to find frequent itemsets?",
		"Explain the difference between operational databases and data warehouses.",
		"What are OLAP, MOLAP, and HOLAP? Explain the different types of OLAP servers.",
		"How does the K-means algorithm work for clustering?",
		"What is a snowflake schema in data warehousing?",
	}

	log.Println("\n  Querying agent with PYQ-related questions:")
	for i, question := range questions {
		log.Printf("\n  Q%d: %s", i+1, question)

		chatReq := digitalocean.AgentChatRequest{
			DeploymentURL: agent.Deployment.URL,
			APIKey:        apiKey,
			Messages: []digitalocean.ChatMessage{
				{Role: "user", Content: question},
			},
			MaxTokens:   500,
			Temperature: 0.3,
		}

		response, err := doClient.CreateAgentChatCompletion(ctx, chatReq)
		if err != nil {
			log.Printf("  ❌ Error: %v", err)
			continue
		}

		content := response.ExtractContent()
		// Check for citations (DO returns inline citations like [[C1]], [[C2]], etc.)
		hasCitations := checkForInlineCitations(content)
		citationInfo := ""
		if hasCitations {
			citationInfo = " [has citations]"
		}

		// Truncate response for display
		if len(content) > 400 {
			content = content[:400] + "..."
		}
		log.Printf("  ✓ Response%s: %s", citationInfo, content)
	}

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// checkForInlineCitations checks if response contains DO citation markers
// DigitalOcean returns inline citations like [[C1]], [[C2]], 【C1】, etc.
func checkForInlineCitations(response string) bool {
	citationPatterns := []string{
		"[[C",  // DO citation format [[C1]], [[C2]], etc.
		"【C",   // Unicode bracket format 【C1】, 【C2】
		"[C1]", // Alternative formats
		"[C2]",
		"[C3]",
		"[Source:", // Explicit source markers
		"[Ref:",
		"[Citation:",
		"(Source:",
	}

	responseLower := strings.ToLower(response)
	for _, pattern := range citationPatterns {
		if strings.Contains(responseLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
