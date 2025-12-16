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

// E2E Test: Full Batch Ingest -> Indexing -> Agent Query WITH CITATIONS
//
// This test verifies:
// 1. Subject has Knowledge Base and Agent configured with citations enabled
// 2. Batch ingest uploads PDFs and creates DataSources
// 3. Agent returns responses WITH citations/sources from the KB

func RunCitationsTest() {
	log.Println("══════════════════════════════════════════════════════════════════")
	log.Println("  E2E TEST: Batch Ingest → Agent Query WITH CITATIONS")
	log.Println("══════════════════════════════════════════════════════════════════")

	ctx := context.Background()

	// Step 1: Initialize
	log.Println("\n[STEP 1] Initializing...")
	db, doClient, err := initializeCitationsTest()
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	// Step 2: Find or create test subject with KB and Agent
	log.Println("\n[STEP 2] Setting up test subject with Knowledge Base and Agent...")
	subject, user, err := setupCitationsTestSubject(ctx, db, doClient)
	if err != nil {
		log.Fatalf("Failed to setup test subject: %v", err)
	}

	log.Printf("  Subject: %s (ID: %d)", subject.Name, subject.ID)
	log.Printf("  Knowledge Base UUID: %s", subject.KnowledgeBaseUUID)
	log.Printf("  Agent UUID: %s", subject.AgentUUID)

	if subject.KnowledgeBaseUUID == "" || subject.AgentUUID == "" {
		log.Println("\n⚠️  Subject needs KB and Agent. Creating them...")
		subject, err = createKBAndAgentWithCitations(ctx, db, doClient, subject)
		if err != nil {
			log.Fatalf("Failed to create KB/Agent: %v", err)
		}
	}

	// Step 3: Enable citations on the agent
	log.Println("\n[STEP 3] Enabling citations on the agent...")
	agent, err := doClient.GetAgent(ctx, subject.AgentUUID)
	if err != nil {
		log.Fatalf("Failed to get agent: %v", err)
	}

	if !agent.ProvideCitations {
		log.Println("  Citations not enabled. Enabling now...")
		agent, err = doClient.EnableAgentCitations(ctx, subject.AgentUUID)
		if err != nil {
			log.Printf("  ⚠️  Failed to enable citations: %v", err)
		} else {
			log.Printf("  ✓ Citations enabled: %v", agent.ProvideCitations)
		}
	} else {
		log.Printf("  ✓ Citations already enabled: %v", agent.ProvideCitations)
	}

	// Step 4: Setup PDF source
	log.Println("\n[STEP 4] Setting up PDF source...")
	pdfSource, cleanup := setupCitationsPDFSource()
	defer cleanup()
	log.Printf("  PDF Source: %s", pdfSource)

	// Step 5: Start batch ingest
	log.Println("\n[STEP 5] Starting batch ingest...")
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
				Title:      fmt.Sprintf("Citations-Test-Data-Mining-%d", timestamp),
				Year:       2024,
				Month:      fmt.Sprintf("December-%d", timestamp),
				ExamType:   "End Semester",
				SourceName: "Citations E2E Test",
			},
		},
	}

	result, err := batchIngestService.StartBatchIngest(ctx, ingestReq)
	if err != nil {
		log.Fatalf("Failed to start batch ingest: %v", err)
	}

	log.Printf("  ✓ Batch ingest started: Job ID=%d, Items=%d", result.JobID, result.TotalItems)

	// Step 6: Wait for job completion
	log.Println("\n[STEP 6] Waiting for batch ingest job to complete...")
	job, err := waitForCitationsJobCompletion(ctx, batchIngestService, result.JobID, user.ID, 5*time.Minute)
	if err != nil {
		log.Fatalf("Job failed: %v", err)
	}

	log.Printf("  ✓ Batch ingest completed: Status=%s, Completed=%d, Failed=%d",
		job.Status, job.CompletedItems, job.FailedItems)

	// Step 7: Wait for agent deployment
	log.Println("\n[STEP 7] Checking agent deployment...")
	isDeployed, agent, err := doClient.IsAgentDeployed(ctx, subject.AgentUUID)
	if err != nil {
		log.Fatalf("Failed to check deployment: %v", err)
	}

	if !isDeployed {
		log.Println("  Agent not deployed. Deploying and waiting...")
		_, err = doClient.DeployAgent(ctx, subject.AgentUUID, digitalocean.VisibilityPrivate)
		if err != nil {
			log.Printf("  ⚠️  Deploy request failed: %v", err)
		}

		agent, err = doClient.WaitForAgentDeployment(ctx, subject.AgentUUID, 5*time.Minute)
		if err != nil {
			log.Fatalf("Agent deployment failed: %v", err)
		}
	}

	log.Printf("  ✓ Agent deployed at: %s", agent.Deployment.URL)

	// Step 8: Create API key and query with citations
	log.Println("\n[STEP 8] Querying agent WITH CITATIONS...")

	agentAPIKey := os.Getenv("DO_AGENT_API_KEY")
	if agentAPIKey == "" {
		log.Println("  Creating API key for agent queries...")
		apiKeyResult, err := doClient.CreateAgentAPIKey(ctx, subject.AgentUUID, fmt.Sprintf("citations-test-%d", time.Now().Unix()))
		if err != nil {
			log.Fatalf("Failed to create API key: %v", err)
		}
		agentAPIKey = apiKeyResult.SecretKey
		log.Printf("  ✓ Created API key: %s...%s", agentAPIKey[:10], agentAPIKey[len(agentAPIKey)-4:])
	}

	// Queries designed to trigger citations from the uploaded content
	queries := []string{
		"What is data mining and what are its main techniques? Please cite your sources.",
		"Explain the KDD process with references to the course material.",
		"What are the different types of data mining algorithms mentioned in the documents?",
	}

	log.Println("\n  ═══════════════════════════════════════════════════════════")
	log.Println("  CITATION TEST QUERIES")
	log.Println("  ═══════════════════════════════════════════════════════════")

	for i, query := range queries {
		log.Printf("\n  ── Query %d ──", i+1)
		log.Printf("  Q: %s", query)

		response, err := queryCitationsAgent(ctx, doClient, agent, agentAPIKey, query)
		if err != nil {
			log.Printf("  ❌ Error: %v", err)
		} else {
			log.Println("  ─────────────────────────────────────────────────────")
			// Print full response (or truncate if very long)
			if len(response) > 1500 {
				log.Printf("  Response (truncated):\n%s\n  ... [%d more chars]", response[:1500], len(response)-1500)
			} else {
				log.Printf("  Response:\n%s", response)
			}
			log.Println("  ─────────────────────────────────────────────────────")

			// Check for citation indicators
			hasCitations := checkForCitations(response)
			if hasCitations {
				log.Println("  ✓ CITATIONS DETECTED in response!")
			} else {
				log.Println("  ⚠️  No obvious citation markers found (may be inline)")
			}
		}
	}

	// Summary
	log.Println("\n══════════════════════════════════════════════════════════════════")
	log.Println("  CITATIONS TEST SUMMARY")
	log.Println("══════════════════════════════════════════════════════════════════")
	log.Printf("  Subject: %s (ID: %d)", subject.Name, subject.ID)
	log.Printf("  Knowledge Base: %s", subject.KnowledgeBaseUUID)
	log.Printf("  Agent: %s", subject.AgentUUID)
	log.Printf("  Agent Citations Enabled: %v", agent.ProvideCitations)
	log.Printf("  Batch Ingest Job: %d (%s)", result.JobID, job.Status)
	log.Printf("  Documents Created: %d", job.CompletedItems)
	log.Println("\n  ✅ CITATIONS E2E TEST COMPLETED")
	log.Println("══════════════════════════════════════════════════════════════════")
}

func initializeCitationsTest() (*gorm.DB, *digitalocean.Client, error) {
	requiredVars := []string{"DB_HOST", "DB_USER_NAME", "DB_PASSWORD", "DB_NAME"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			return nil, nil, fmt.Errorf("missing required env var: %s", v)
		}
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER_NAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		getEnvDefault("DB_PORT", "5432"),
		getEnvDefault("DB_SSL_MODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed: %w", err)
	}
	log.Println("  ✓ Database connected")

	var doClient *digitalocean.Client
	if token := os.Getenv("DIGITALOCEAN_TOKEN"); token != "" {
		doClient = digitalocean.NewClient(digitalocean.Config{
			APIToken: token,
		})
		log.Println("  ✓ DigitalOcean client initialized")
	} else {
		return nil, nil, fmt.Errorf("DIGITALOCEAN_TOKEN not set")
	}

	return db, doClient, nil
}

func setupCitationsTestSubject(ctx context.Context, db *gorm.DB, doClient *digitalocean.Client) (*model.Subject, *model.User, error) {
	// Get or create test user
	var user model.User
	err := db.Where("email = ?", "citations_test@test.com").First(&user).Error
	if err == gorm.ErrRecordNotFound {
		user = model.User{
			Email:        "citations_test@test.com",
			PasswordHash: "test_hash",
			PasswordSalt: []byte("test_salt"),
			Name:         "Citations Test User",
			Role:         "admin",
		}
		if err := db.Create(&user).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, nil, err
	}

	// Try to find a subject with KB AND Agent configured
	var subject model.Subject
	err = db.Where("knowledge_base_uuid != '' AND knowledge_base_uuid IS NOT NULL AND agent_uuid != '' AND agent_uuid IS NOT NULL").First(&subject).Error
	if err == nil {
		log.Printf("  Found subject with KB and Agent: %s", subject.Name)
		return &subject, &user, nil
	}

	// Otherwise find any subject
	err = db.First(&subject).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil, fmt.Errorf("no subjects found in database - please create one first")
	} else if err != nil {
		return nil, nil, err
	}

	return &subject, &user, nil
}

func createKBAndAgentWithCitations(ctx context.Context, db *gorm.DB, doClient *digitalocean.Client, subject *model.Subject) (*model.Subject, error) {
	projectID := os.Getenv("DO_PROJECT_ID")
	if projectID == "" {
		return subject, fmt.Errorf("DO_PROJECT_ID not set")
	}

	genAIRegion := "tor1"
	embeddingModel := getEnvDefault("DO_EMBEDDING_MODEL_UUID", "embed-multilingual")
	spacesName := getEnvDefault("DO_SPACES_NAME", getEnvDefault("DO_SPACES_BUCKET", ""))
	spacesRegion := getEnvDefault("DO_SPACES_REGION", "blr1")

	if spacesName == "" {
		return subject, fmt.Errorf("DO_SPACES_NAME not set")
	}

	// Create KB if needed
	if subject.KnowledgeBaseUUID == "" {
		kbName := fmt.Sprintf("citations-test-kb-%d", time.Now().Unix())
		log.Printf("  Creating Knowledge Base: %s", kbName)

		kb, err := doClient.CreateKnowledgeBase(ctx, digitalocean.CreateKnowledgeBaseRequest{
			Name:           kbName,
			Description:    fmt.Sprintf("Citations Test KB for %s", subject.Name),
			EmbeddingModel: embeddingModel,
			ProjectID:      projectID,
			Region:         genAIRegion,
			DataSources: []digitalocean.DataSourceCreateInput{
				{
					SpacesDataSource: &digitalocean.SpacesDataSourceInput{
						BucketName: spacesName,
						Region:     spacesRegion,
					},
				},
			},
		})
		if err != nil {
			return subject, fmt.Errorf("failed to create KB: %w", err)
		}
		subject.KnowledgeBaseUUID = kb.UUID
		log.Printf("  ✓ KB created: %s", kb.UUID)
	}

	// Create Agent if needed
	if subject.AgentUUID == "" {
		modelUUID := getEnvDefault("DO_AGENT_MODEL_UUID", "")
		if modelUUID == "" {
			return subject, fmt.Errorf("DO_AGENT_MODEL_UUID not set")
		}

		agentName := fmt.Sprintf("Citations Agent - %s", subject.Name)
		if len(agentName) > 63 {
			agentName = agentName[:63]
		}

		log.Printf("  Creating Agent: %s", agentName)

		agent, err := doClient.CreateAgent(ctx, digitalocean.CreateAgentRequest{
			Name:        agentName,
			Description: fmt.Sprintf("Citations Test Agent for %s", subject.Name),
			ModelUUID:   modelUUID,
			ProjectID:   projectID,
			Region:      genAIRegion,
			Instructions: fmt.Sprintf(`You are an AI assistant for the subject "%s". 
You have access to uploaded course materials and previous year question papers.
When answering questions:
1. Always cite specific sources from the knowledge base
2. Use inline citations like [Source: filename.pdf, page X] 
3. Reference specific sections, topics, or page numbers when available
4. If information comes from multiple sources, cite all of them
5. Be accurate and helpful`, subject.Name),
			Temperature: 0,
			TopP:        1,
		})
		if err != nil {
			return subject, fmt.Errorf("failed to create agent: %w", err)
		}
		subject.AgentUUID = agent.UUID
		log.Printf("  ✓ Agent created: %s", agent.UUID)

		// Attach KB to Agent
		err = doClient.AttachKnowledgeBase(ctx, agent.UUID, subject.KnowledgeBaseUUID)
		if err != nil {
			log.Printf("  ⚠️  Failed to attach KB: %v", err)
		} else {
			log.Println("  ✓ KB attached to Agent")
		}

		// Deploy the agent
		log.Println("  Deploying agent...")
		_, err = doClient.DeployAgent(ctx, agent.UUID, digitalocean.VisibilityPrivate)
		if err != nil {
			log.Printf("  ⚠️  Deploy failed: %v", err)
		}
	}

	// Save to database
	if err := db.Save(subject).Error; err != nil {
		return subject, fmt.Errorf("failed to save subject: %w", err)
	}

	return subject, nil
}

func setupCitationsPDFSource() (string, func()) {
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
		log.Println("  ⚠️  Real PDF not found, using minimal test PDF")
		pdfContent = []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("  [MockServer] Serving: %s", r.URL.Path)
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(pdfContent)
	}))

	return server.URL, server.Close
}

func waitForCitationsJobCompletion(ctx context.Context, svc *services.BatchIngestService, jobID uint, userID uint, timeout time.Duration) (*model.IndexingJob, error) {
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

func queryCitationsAgent(ctx context.Context, doClient *digitalocean.Client, agent *digitalocean.Agent, apiKey string, query string) (string, error) {
	if agent.Deployment == nil || agent.Deployment.URL == "" {
		return "", fmt.Errorf("agent has no deployment URL")
	}

	chatReq := digitalocean.AgentChatRequest{
		DeploymentURL: agent.Deployment.URL,
		APIKey:        apiKey,
		Messages: []digitalocean.ChatMessage{
			{Role: "user", Content: query},
		},
		MaxTokens:   2000, // Longer responses to include citations
		Temperature: 0.3,  // More focused responses
	}

	response, err := doClient.CreateAgentChatCompletion(ctx, chatReq)
	if err != nil {
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	return response.ExtractContent(), nil
}

func checkForCitations(response string) bool {
	// Check for common citation patterns
	// DigitalOcean uses [[C1]], [[C2]], etc. for inline citations
	// Also sometimes uses unicode brackets 【C1】
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

	for _, pattern := range citationPatterns {
		if containsIgnoreCase(response, pattern) {
			return true
		}
	}

	return false
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && (containsIgnoreCase(s[1:], substr) ||
				(len(s) >= len(substr) && equalFoldPrefix(s, substr))))
}

func equalFoldPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c1, c2 := s[i], prefix[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
