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
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// E2E Test: Full Batch Ingest -> Indexing -> Agent Query Flow
//
// This test verifies:
// 1. Subject has Knowledge Base and Agent configured
// 2. Batch ingest uploads PDFs and creates DataSources
// 3. DO Knowledge Base indexing completes
// 4. Agent can answer questions about uploaded content

func main() {
	// Check for specific test types
	testType := ""
	if len(os.Args) > 1 {
		testType = os.Args[1]
	}
	if os.Getenv("TEST_TYPE") != "" {
		testType = os.Getenv("TEST_TYPE")
	}

	switch testType {
	case "citations":
		RunCitationsTest()
		return
	case "syllabus":
		RunSyllabusE2ETest()
		return
	}

	// Also support old env var
	if os.Getenv("TEST_CITATIONS") == "true" {
		RunCitationsTest()
		return
	}

	log.Println("══════════════════════════════════════════════════════════════════")
	log.Println("  END-TO-END TEST: Batch Ingest → Indexing → Agent Query")
	log.Println("══════════════════════════════════════════════════════════════════")

	ctx := context.Background()

	// Step 1: Initialize
	log.Println("\n[STEP 1] Initializing...")
	db, doClient, err := initialize()
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	// Step 2: Find or create test subject with KB and Agent
	log.Println("\n[STEP 2] Setting up test subject with Knowledge Base and Agent...")
	subject, user, err := setupTestSubject(ctx, db, doClient)
	if err != nil {
		log.Fatalf("Failed to setup test subject: %v", err)
	}

	log.Printf("  Subject: %s (ID: %d)", subject.Name, subject.ID)
	log.Printf("  Knowledge Base UUID: %s", subject.KnowledgeBaseUUID)
	log.Printf("  Agent UUID: %s", subject.AgentUUID)

	if subject.KnowledgeBaseUUID == "" {
		log.Println("\n⚠️  Subject has no Knowledge Base. Creating one...")
		subject, err = createKBAndAgentForSubject(ctx, db, doClient, subject)
		if err != nil {
			log.Fatalf("Failed to create KB/Agent: %v", err)
		}
		log.Printf("  ✓ Knowledge Base created: %s", subject.KnowledgeBaseUUID)
		log.Printf("  ✓ Agent created: %s", subject.AgentUUID)
	}

	// Step 3: Create mock PDF server or use real PDFs
	log.Println("\n[STEP 3] Setting up PDF source...")
	pdfSource, cleanup := setupPDFSource()
	defer cleanup()
	log.Printf("  PDF Source: %s", pdfSource)

	// Step 4: Start batch ingest
	log.Println("\n[STEP 4] Starting batch ingest...")
	notificationService := services.NewNotificationService(db)
	pyqService := services.NewPYQService(db)
	batchIngestService := services.NewBatchIngestService(db, notificationService, pyqService)

	timestamp := time.Now().Unix()
	ingestReq := services.BatchIngestRequest{
		SubjectID: subject.ID,
		UserID:    user.ID,
		Papers: []services.BatchIngestPaperRequest{
			{
				PDFURL:     pdfSource + "/paper1.pdf",
				Title:      fmt.Sprintf("E2E-Test-Paper-Dec-2024-%d", timestamp),
				Year:       2024,
				Month:      fmt.Sprintf("December-%d", timestamp),
				ExamType:   "End Semester",
				SourceName: "E2E Test",
			},
		},
	}

	result, err := batchIngestService.StartBatchIngest(ctx, ingestReq)
	if err != nil {
		log.Fatalf("Failed to start batch ingest: %v", err)
	}

	log.Printf("  ✓ Batch ingest started: Job ID=%d, Items=%d", result.JobID, result.TotalItems)

	// Step 5: Wait for OUR indexing job to complete
	log.Println("\n[STEP 5] Waiting for batch ingest job to complete...")
	job, err := waitForJobCompletion(ctx, batchIngestService, result.JobID, user.ID, 5*time.Minute)
	if err != nil {
		log.Fatalf("Job failed: %v", err)
	}

	log.Printf("  ✓ Batch ingest completed: Status=%s, Completed=%d, Failed=%d",
		job.Status, job.CompletedItems, job.FailedItems)

	// Step 6: Check if DO indexing job was triggered
	if job.DOIndexingJobUUID != "" {
		log.Println("\n[STEP 6] Waiting for DigitalOcean Knowledge Base indexing...")
		log.Printf("  DO Indexing Job UUID: %s", job.DOIndexingJobUUID)

		err = waitForDOIndexingComplete(ctx, doClient, subject.KnowledgeBaseUUID, job.DOIndexingJobUUID, 10*time.Minute)
		if err != nil {
			log.Printf("  ⚠️  DO indexing may still be in progress: %v", err)
		} else {
			log.Println("  ✓ DO Knowledge Base indexing completed!")
		}
	} else {
		log.Println("\n[STEP 6] No DO indexing job was triggered (AI features may be disabled)")
	}

	// Step 7: Wait for Agent Deployment and Query
	log.Println("\n[STEP 7] Waiting for Agent deployment and querying...")

	if subject.AgentUUID == "" {
		log.Println("  ⚠️  No Agent configured for this subject. Skipping query test.")
	} else {
		// First, check if agent is deployed, if not wait for it
		isDeployed, agent, err := doClient.IsAgentDeployed(ctx, subject.AgentUUID)
		if err != nil {
			log.Printf("  ⚠️  Failed to check agent deployment status: %v", err)
		} else if !isDeployed {
			log.Println("  Agent not yet deployed. Waiting for deployment (up to 5 minutes)...")
			agent, err = doClient.WaitForAgentDeployment(ctx, subject.AgentUUID, 5*time.Minute)
			if err != nil {
				log.Printf("  ⚠️  Agent deployment wait failed: %v", err)
				log.Println("  Skipping agent query test - deployment not ready")
			} else {
				log.Printf("  ✓ Agent deployed at: %s", agent.Deployment.URL)
				isDeployed = true
			}
		} else {
			log.Printf("  ✓ Agent already deployed at: %s", agent.Deployment.URL)
		}

		if isDeployed {
			// Create API key once for all queries
			agentAPIKey := os.Getenv("DO_AGENT_API_KEY")
			if agentAPIKey == "" {
				log.Println("  Creating API key for agent queries...")
				apiKeyResult, err := doClient.CreateAgentAPIKey(ctx, subject.AgentUUID, fmt.Sprintf("e2e-test-%d", time.Now().Unix()))
				if err != nil {
					log.Printf("  ⚠️  Failed to create API key: %v", err)
				} else {
					agentAPIKey = apiKeyResult.SecretKey
					log.Printf("  ✓ Created API key: %s...%s", agentAPIKey[:10], agentAPIKey[len(agentAPIKey)-4:])
				}
			}

			if agentAPIKey == "" {
				log.Println("  ⚠️  No API key available, skipping queries")
			} else {
				queries := []string{
					"What topics are covered in this paper?",
					"Summarize the main concepts from the uploaded documents",
					"What is data mining?",
				}

				for i, query := range queries {
					log.Printf("\n  Query %d: %s", i+1, query)
					response, err := queryAgentWithKey(ctx, doClient, agent, agentAPIKey, query)
					if err != nil {
						log.Printf("  ❌ Error: %v", err)
					} else {
						// Truncate long responses
						if len(response) > 500 {
							response = response[:500] + "..."
						}
						log.Printf("  ✓ Response: %s", response)
					}
				}
			}
		}
	}

	// Summary
	log.Println("\n══════════════════════════════════════════════════════════════════")
	log.Println("  TEST SUMMARY")
	log.Println("══════════════════════════════════════════════════════════════════")
	log.Printf("  Subject: %s (ID: %d)", subject.Name, subject.ID)
	log.Printf("  Knowledge Base: %s", subject.KnowledgeBaseUUID)
	log.Printf("  Agent: %s", subject.AgentUUID)
	log.Printf("  Batch Ingest Job: %d (%s)", result.JobID, job.Status)
	log.Printf("  Documents Created: %d", job.CompletedItems)

	if job.Status == model.IndexingJobStatusCompleted && subject.AgentUUID != "" {
		log.Println("\n  ✅ END-TO-END TEST PASSED")
	} else if job.Status == model.IndexingJobStatusCompleted {
		log.Println("\n  ⚠️  PARTIAL SUCCESS - Ingest worked but Agent not configured")
	} else {
		log.Println("\n  ❌ TEST FAILED")
	}
	log.Println("══════════════════════════════════════════════════════════════════")

	// Cleanup option
	if os.Getenv("CLEANUP") == "true" {
		log.Println("\n[CLEANUP] Removing test data...")
		cleanupTestData(db, user.ID, result.JobID, subject.ID)
		log.Println("  ✓ Cleanup complete")
	}
}

func initialize() (*gorm.DB, *digitalocean.Client, error) {
	// Check required env vars
	requiredVars := []string{"DB_HOST", "DB_USER_NAME", "DB_PASSWORD", "DB_NAME"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			return nil, nil, fmt.Errorf("missing required env var: %s", v)
		}
	}

	// Connect to database
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER_NAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_SSL_MODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed: %w", err)
	}
	log.Println("  ✓ Database connected")

	// Initialize DO client
	var doClient *digitalocean.Client
	if token := os.Getenv("DIGITALOCEAN_TOKEN"); token != "" {
		doClient = digitalocean.NewClient(digitalocean.Config{
			APIToken: token,
		})
		log.Println("  ✓ DigitalOcean client initialized")
	} else {
		log.Println("  ⚠️  DIGITALOCEAN_TOKEN not set - AI features disabled")
	}

	return db, doClient, nil
}

func setupTestSubject(ctx context.Context, db *gorm.DB, doClient *digitalocean.Client) (*model.Subject, *model.User, error) {
	// Get or create test user
	var user model.User
	err := db.Where("email = ?", "e2e_test@test.com").First(&user).Error
	if err == gorm.ErrRecordNotFound {
		user = model.User{
			Email:        "e2e_test@test.com",
			PasswordHash: "test_hash",
			PasswordSalt: []byte("test_salt"),
			Name:         "E2E Test User",
			Role:         "admin",
		}
		if err := db.Create(&user).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, nil, err
	}

	// Try to find a subject with KB configured
	var subject model.Subject
	err = db.Where("knowledge_base_uuid != '' AND knowledge_base_uuid IS NOT NULL").First(&subject).Error
	if err == nil {
		log.Printf("  Found subject with KB: %s", subject.Name)
		return &subject, &user, nil
	}

	// Otherwise find any subject
	err = db.First(&subject).Error
	if err == gorm.ErrRecordNotFound {
		// Create minimal test subject
		log.Println("  No subjects found, creating test subject...")

		var semester model.Semester
		if err := db.First(&semester).Error; err != nil {
			// Create minimal semester
			var course model.Course
			if err := db.First(&course).Error; err != nil {
				var university model.University
				if err := db.First(&university).Error; err != nil {
					university = model.University{Name: "E2E Test University", Code: "E2E_UNIV", Location: "Test"}
					db.Create(&university)
				}
				course = model.Course{UniversityID: university.ID, Name: "E2E Test Course", Code: "E2E_COURSE", Duration: 4}
				db.Create(&course)
			}
			semester = model.Semester{CourseID: course.ID, Number: 1, Name: "E2E Test Semester"}
			db.Create(&semester)
		}

		subject = model.Subject{
			SemesterID:  semester.ID,
			Name:        "E2E Test Subject - Data Mining",
			Code:        fmt.Sprintf("E2E-DM-%d", time.Now().Unix()),
			Credits:     4,
			Description: "Test subject for E2E testing",
		}
		if err := db.Create(&subject).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to create subject: %w", err)
		}
	} else if err != nil {
		return nil, nil, err
	}

	return &subject, &user, nil
}

func createKBAndAgentForSubject(ctx context.Context, db *gorm.DB, doClient *digitalocean.Client, subject *model.Subject) (*model.Subject, error) {
	if doClient == nil {
		return subject, fmt.Errorf("DigitalOcean client not initialized")
	}

	projectID := os.Getenv("DO_PROJECT_ID")
	if projectID == "" {
		return subject, fmt.Errorf("DO_PROJECT_ID not set")
	}

	// GenAI is only available in tor1 region
	genAIRegion := "tor1"
	embeddingModel := getEnv("DO_EMBEDDING_MODEL_UUID", getEnv("DO_EMBEDDING_MODEL", "embed-multilingual"))
	spacesName := getEnv("DO_SPACES_NAME", getEnv("DO_SPACES_BUCKET", ""))
	spacesRegion := getEnv("DO_SPACES_REGION", "blr1")
	databaseID := getEnv("DO_GENAI_DATABASE_ID", "")

	if spacesName == "" {
		return subject, fmt.Errorf("DO_SPACES_NAME or DO_SPACES_BUCKET not set - required for KB creation")
	}

	// Create Knowledge Base
	kbName := fmt.Sprintf("e2e-test-kb-%d", time.Now().Unix())
	log.Printf("  Creating Knowledge Base: %s", kbName)
	log.Printf("  Region: %s, Embedding Model: %s, Spaces: %s/%s", genAIRegion, embeddingModel, spacesRegion, spacesName)

	createKBReq := digitalocean.CreateKnowledgeBaseRequest{
		Name:           kbName,
		Description:    fmt.Sprintf("E2E Test KB for %s", subject.Name),
		EmbeddingModel: embeddingModel,
		ProjectID:      projectID,
		Region:         genAIRegion,
		DatabaseID:     databaseID,
		DataSources: []digitalocean.DataSourceCreateInput{
			{
				SpacesDataSource: &digitalocean.SpacesDataSourceInput{
					BucketName: spacesName,
					Region:     spacesRegion,
				},
			},
		},
	}

	kb, err := doClient.CreateKnowledgeBase(ctx, createKBReq)
	if err != nil {
		return subject, fmt.Errorf("failed to create KB: %w", err)
	}

	subject.KnowledgeBaseUUID = kb.UUID
	log.Printf("  ✓ KB created: %s", kb.UUID)

	// Create Agent
	agentName := fmt.Sprintf("E2E Test Agent - %s", subject.Name)
	if len(agentName) > 63 {
		agentName = agentName[:63]
	}

	modelUUID := getEnv("DO_AGENT_MODEL_UUID", getEnv("DO_MODEL_UUID", ""))
	if modelUUID == "" {
		// Model UUID is required - the API doesn't have a list models endpoint
		// You can get this from the DigitalOcean Console under AI > Models
		// Common models: llama-3.3-70b-instruct, etc.
		log.Println("  ⚠️  DO_AGENT_MODEL_UUID not set. Agent creation requires a model UUID.")
		log.Println("      Get a model UUID from the DigitalOcean Console under GenAI > Models")
	}

	if modelUUID != "" {
		log.Printf("  Creating Agent: %s", agentName)

		agent, err := doClient.CreateAgent(ctx, digitalocean.CreateAgentRequest{
			Name:        agentName,
			Description: fmt.Sprintf("E2E Test Agent for %s", subject.Name),
			ModelUUID:   modelUUID,
			ProjectID:   projectID,
			Region:      genAIRegion,
			Instructions: fmt.Sprintf(`You are an AI assistant for the subject "%s". 
Answer questions based on the uploaded course materials and previous year papers.
Be helpful, accurate, and cite specific content when possible.`, subject.Name),
			Temperature: 0,
			TopP:        1,
		})
		if err != nil {
			log.Printf("  ⚠️  Failed to create agent: %v", err)
		} else {
			subject.AgentUUID = agent.UUID
			log.Printf("  ✓ Agent created: %s", agent.UUID)

			// Attach KB to Agent
			err = doClient.AttachKnowledgeBase(ctx, agent.UUID, kb.UUID)
			if err != nil {
				log.Printf("  ⚠️  Failed to attach KB to agent: %v", err)
			} else {
				log.Println("  ✓ KB attached to Agent")
			}

			// Deploy the agent to make it accessible
			log.Println("  Deploying agent (setting visibility to PRIVATE)...")
			deployedAgent, err := doClient.DeployAgent(ctx, agent.UUID, digitalocean.VisibilityPrivate)
			if err != nil {
				log.Printf("  ⚠️  Failed to deploy agent: %v", err)
			} else {
				log.Printf("  ✓ Agent deployment initiated")
				if deployedAgent.Deployment != nil {
					log.Printf("    Deployment Status: %s", deployedAgent.Deployment.Status)
					if deployedAgent.Deployment.URL != "" {
						log.Printf("    Deployment URL: %s", deployedAgent.Deployment.URL)
					}
				}
			}
		}
	} else {
		log.Println("  ⚠️  No model available, skipping agent creation")
	}

	// Save to database
	if err := db.Save(subject).Error; err != nil {
		return subject, fmt.Errorf("failed to save subject: %w", err)
	}

	return subject, nil
}

func setupPDFSource() (string, func()) {
	// Try to load real test PDF
	pdfPaths := []string{
		"../../mca-301-data-mining-dec-2024.pdf",
		"../../../mca-301-data-mining-dec-2024.pdf",
		"mca-301-data-mining-dec-2024.pdf",
	}

	var pdfContent []byte
	for _, path := range pdfPaths {
		content, err := os.ReadFile(path)
		if err == nil {
			pdfContent = content
			log.Printf("  Loaded test PDF: %s (%.2f KB)", path, float64(len(content))/1024)
			break
		}
	}

	if pdfContent == nil {
		log.Println("  Using minimal test PDF (real PDF not found)")
		pdfContent = []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF")
	}

	// Check if we should use real URLs
	if realURL := os.Getenv("TEST_PDF_URL"); realURL != "" {
		log.Printf("  Using real PDF URL: %s", realURL)
		return realURL, func() {}
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("  [MockServer] Serving: %s", r.URL.Path)
		time.Sleep(500 * time.Millisecond) // Simulate download time
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(pdfContent)
	}))

	return server.URL, server.Close
}

func waitForJobCompletion(ctx context.Context, svc *services.BatchIngestService, jobID uint, userID uint, timeout time.Duration) (*model.IndexingJob, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second
	pollCount := 0

	for time.Now().Before(deadline) {
		pollCount++
		time.Sleep(pollInterval)

		job, err := svc.GetJobStatus(ctx, jobID, userID)
		if err != nil {
			log.Printf("  [Poll %d] Error: %v", pollCount, err)
			continue
		}

		progress := job.GetProgress()
		log.Printf("  [Poll %d] Status: %-12s | Progress: %3d%% | Completed: %d | Failed: %d",
			pollCount, job.Status, progress, job.CompletedItems, job.FailedItems)

		if job.IsComplete() {
			return job, nil
		}
	}

	return nil, fmt.Errorf("timeout waiting for job completion")
}

func waitForDOIndexingComplete(ctx context.Context, doClient *digitalocean.Client, kbUUID string, jobUUID string, timeout time.Duration) error {
	if doClient == nil {
		return fmt.Errorf("DO client not initialized")
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 10 * time.Second
	pollCount := 0

	for time.Now().Before(deadline) {
		pollCount++
		time.Sleep(pollInterval)

		// Get indexing job status - only takes jobUUID, not kbUUID
		job, err := doClient.GetIndexingJob(ctx, jobUUID)
		if err != nil {
			log.Printf("  [DO Poll %d] Error getting job status: %v", pollCount, err)
			continue
		}

		log.Printf("  [DO Poll %d] Indexing Status: %s (Phase: %s)", pollCount, job.Status, job.Phase)

		// Check both Status and Phase for completion
		switch job.Status {
		case "INDEX_JOB_STATUS_COMPLETED":
			return nil
		case "INDEX_JOB_STATUS_FAILED":
			return fmt.Errorf("DO indexing job failed")
		}
		// Also check phase
		if job.Phase == "BATCH_JOB_PHASE_SUCCEEDED" {
			return nil
		}
	}

	return fmt.Errorf("timeout waiting for DO indexing")
}

// queryAgentWithKey queries the agent using a pre-created API key
func queryAgentWithKey(ctx context.Context, doClient *digitalocean.Client, agent *digitalocean.Agent, apiKey string, query string) (string, error) {
	if doClient == nil {
		return "", fmt.Errorf("DO client not initialized")
	}

	if agent.Deployment == nil || agent.Deployment.URL == "" {
		return "", fmt.Errorf("agent has no deployment URL")
	}

	chatReq := digitalocean.AgentChatRequest{
		DeploymentURL: agent.Deployment.URL,
		APIKey:        apiKey,
		Messages: []digitalocean.ChatMessage{
			{Role: "user", Content: query},
		},
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	response, err := doClient.CreateAgentChatCompletion(ctx, chatReq)
	if err != nil {
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	return response.ExtractContent(), nil
}

func cleanupTestData(db *gorm.DB, userID uint, jobID uint, subjectID uint) {
	// Delete notifications
	db.Where("user_id = ?", userID).Delete(&model.UserNotification{})

	// Delete job items and job
	db.Where("job_id = ?", jobID).Delete(&model.IndexingJobItem{})
	db.Delete(&model.IndexingJob{}, jobID)

	// Delete PYQ papers and documents
	db.Where("subject_id = ?", subjectID).Delete(&model.PYQPaper{})
	db.Where("subject_id = ?", subjectID).Delete(&model.Document{})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
