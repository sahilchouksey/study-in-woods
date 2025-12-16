package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"github.com/sahilchouksey/go-init-setup/utils/crypto"
	"gorm.io/gorm"
)

// SubjectService handles subject creation with DigitalOcean AI integration
type SubjectService struct {
	db       *gorm.DB
	doClient *digitalocean.Client
}

// NewSubjectService creates a new subject service
func NewSubjectService(db *gorm.DB) *SubjectService {
	// Initialize DigitalOcean client
	apiToken := os.Getenv("DIGITALOCEAN_TOKEN")
	if apiToken == "" {
		log.Println("Warning: DIGITALOCEAN_TOKEN not set. AI features will be disabled.")
		return &SubjectService{
			db:       db,
			doClient: nil,
		}
	}

	doClient := digitalocean.NewClient(digitalocean.Config{
		APIToken: apiToken,
	})

	return &SubjectService{
		db:       db,
		doClient: doClient,
	}
}

// CreateSubjectRequest represents the request to create a subject with AI
type CreateSubjectRequest struct {
	SemesterID  uint   `json:"semester_id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Credits     int    `json:"credits"`
	Description string `json:"description"`
}

// CreateSubjectResult represents the result of subject creation
type CreateSubjectResult struct {
	Subject              *model.Subject
	KnowledgeBaseCreated bool
	AgentCreated         bool
	AgentDeployed        bool
	CitationsEnabled     bool
	APIKeyCreated        bool
	Error                error
}

// generateKnowledgeBaseName generates a unique name for the knowledge base
func (s *SubjectService) generateKnowledgeBaseName(course *model.Course, semester *model.Semester, subject *model.Subject) string {
	// Format: coursecode-sem1-subjectcode (e.g., "mca-sem1-mca301")
	// DigitalOcean KB names must be alphanumeric with hyphens only (no spaces, parentheses, etc.)
	courseName := sanitizeKBName(course.Code)
	semesterName := fmt.Sprintf("sem%d", semester.Number)
	subjectName := sanitizeKBName(subject.Code)

	name := fmt.Sprintf("%s-%s-%s", courseName, semesterName, subjectName)
	log.Printf("generateKnowledgeBaseName: course=%q semester=%d subject=%q -> %q",
		course.Code, semester.Number, subject.Code, name)
	return name
}

// sanitizeKBName sanitizes a string to be valid for DigitalOcean KB names
// KB names must be lowercase alphanumeric with hyphens, no consecutive hyphens
func sanitizeKBName(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")

	// Remove parentheses and their content like "(2)" -> "2"
	// Keep the number but remove the parens
	s = strings.ReplaceAll(s, "(", "-")
	s = strings.ReplaceAll(s, ")", "")

	// Remove any other non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	s = result.String()

	// Remove consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Remove leading/trailing hyphens
	s = strings.Trim(s, "-")

	// Ensure name is not empty
	if s == "" {
		s = "kb"
	}

	return s
}

// CreateSubjectWithAI creates a subject with automatic AI integration
func (s *SubjectService) CreateSubjectWithAI(ctx context.Context, req CreateSubjectRequest) (*CreateSubjectResult, error) {
	result := &CreateSubjectResult{}

	// Start database transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Rollback function
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			result.Error = fmt.Errorf("panic during subject creation: %v", r)
		}
	}()

	// 1. Validate semester exists and preload course
	var semester model.Semester
	if err := tx.Preload("Course").First(&semester, req.SemesterID).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("semester not found")
		}
		return nil, fmt.Errorf("failed to fetch semester: %w", err)
	}

	// 2. Create subject in database
	subject := model.Subject{
		SemesterID:  req.SemesterID,
		Name:        req.Name,
		Code:        req.Code,
		Credits:     req.Credits,
		Description: req.Description,
	}

	if err := tx.Create(&subject).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create subject: %w", err)
	}

	result.Subject = &subject

	// If DigitalOcean client is not available, just create the subject without AI
	if s.doClient == nil {
		if err := tx.Commit().Error; err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
		return result, nil
	}

	// 3. Create Knowledge Base in DigitalOcean
	kbName := s.generateKnowledgeBaseName(&semester.Course, &semester, &subject)
	kbDescription := fmt.Sprintf("Knowledge base for %s - %s (%s)", semester.Course.Name, subject.Name, subject.Code)

	// Get configuration from environment
	embeddingModel := os.Getenv("DO_EMBEDDING_MODEL_UUID")
	projectID := os.Getenv("DO_PROJECT_ID")
	spacesName := os.Getenv("DO_SPACES_NAME")
	spacesRegion := os.Getenv("DO_SPACES_REGION")
	// DatabaseID allows reusing a single OpenSearch database across multiple knowledge bases
	// instead of creating a new database for each one (which is expensive and wasteful)
	databaseID := os.Getenv("DO_GENAI_DATABASE_ID")

	// GenAI is only available in tor1 region - use tor1 for both KB and Agent
	// to ensure compatibility
	genAIRegion := "tor1"

	// Spaces region defaults to blr1 if not set
	if spacesRegion == "" {
		spacesRegion = "blr1"
	}

	createKBReq := digitalocean.CreateKnowledgeBaseRequest{
		Name:           kbName,
		Description:    kbDescription,
		EmbeddingModel: embeddingModel,
		ProjectID:      projectID,
		Region:         genAIRegion, // Use tor1 for KB to match agent region
		DatabaseID:     databaseID,  // Reuse existing database if provided
	}

	// Add subject-scoped data source - only indexes this subject's folder
	// Documents are uploaded to subjects/{id}/pyqs/ so this KB only sees its own documents
	if spacesName != "" {
		subjectFolderPath := fmt.Sprintf("subjects/%d/", subject.ID)
		createKBReq.DataSources = []digitalocean.DataSourceCreateInput{
			{
				SpacesDataSource: &digitalocean.SpacesDataSourceInput{
					BucketName: spacesName,
					Region:     spacesRegion,
					ItemPath:   subjectFolderPath,
				},
			},
		}
	}

	kb, err := s.doClient.CreateKnowledgeBase(ctx, createKBReq)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create knowledge base: %w", err)
	}

	result.KnowledgeBaseCreated = true
	subject.KnowledgeBaseUUID = kb.UUID

	// 4. Start indexing job for the knowledge base
	// This is required before the KB can be attached to an agent
	dataSources, err := s.doClient.ListKnowledgeBaseDataSources(ctx, kb.UUID)
	if err != nil {
		log.Printf("Warning: Failed to list data sources for KB %s: %v", kb.UUID, err)
	} else if len(dataSources) > 0 {
		// Collect data source UUIDs
		dsUUIDs := make([]string, len(dataSources))
		for i, ds := range dataSources {
			dsUUIDs[i] = ds.UUID
		}

		// Start indexing job
		indexReq := digitalocean.StartIndexingJobRequest{
			KnowledgeBaseUUID: kb.UUID,
			DataSourceUUIDs:   dsUUIDs,
		}
		if _, err := s.doClient.StartIndexingJob(ctx, indexReq); err != nil {
			log.Printf("Warning: Failed to start indexing job for KB %s: %v. KB attachment may fail.", kb.UUID, err)
		} else {
			log.Printf("Started indexing job for KB %s with %d data sources", kb.UUID, len(dsUUIDs))
		}
	}

	// 5. Create AI Agent connected to Knowledge Base
	agentName := fmt.Sprintf("%s Agent", subject.Name)
	agentDescription := fmt.Sprintf("AI assistant for %s course", subject.Name)
	agentInstructions := fmt.Sprintf(
		"You are a helpful AI assistant for the %s course. "+
			"Use the knowledge base to answer questions accurately. "+
			"If you don't know the answer, say so honestly. "+
			"Always be clear, concise, and educational.",
		subject.Name,
	)

	// Get model UUID from environment or use default (OpenAI GPT-oss-120b - works without extra API keys)
	modelUUID := os.Getenv("DO_AGENT_MODEL_UUID")
	if modelUUID == "" {
		modelUUID = "18bc9b8f-73c5-11f0-b074-4e013e2ddde4" // OpenAI GPT-oss-120b
	}

	// Agent region - only tor1 supports GenAI agents (same as KB region)

	// Create agent WITHOUT knowledge base first (KB may not be ready immediately)
	createAgentReq := digitalocean.CreateAgentRequest{
		Name:         agentName,
		Description:  agentDescription,
		ModelUUID:    modelUUID,
		ProjectID:    projectID,
		Region:       genAIRegion,
		Instructions: agentInstructions,
		Temperature:  0,
		TopP:         1,
		// Note: Don't attach KB during creation - it may not be ready yet
	}

	agent, err := s.doClient.CreateAgent(ctx, createAgentReq)
	if err != nil {
		// Try to clean up knowledge base on agent creation failure
		s.doClient.DeleteKnowledgeBase(ctx, kb.UUID)
		tx.Rollback()
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// 6. Attach Knowledge Base to Agent (separate API call)
	// This works around the KB not being ready at agent creation time
	if err := s.doClient.AttachKnowledgeBase(ctx, agent.UUID, kb.UUID); err != nil {
		log.Printf("Warning: Failed to attach KB %s to agent %s: %v. Will retry later.", kb.UUID, agent.UUID, err)
		// Don't fail the whole operation - agent and KB are created, just not linked yet
		// They can be linked manually or in a background job
	}

	result.AgentCreated = true
	subject.AgentUUID = agent.UUID

	// 7. Deploy the Agent (private visibility - accessible only via API key)
	deployedAgent, err := s.doClient.DeployAgent(ctx, agent.UUID, digitalocean.VisibilityPrivate)
	if err != nil {
		log.Printf("Warning: Failed to deploy agent %s: %v. Agent will need manual deployment.", agent.UUID, err)
	} else {
		result.AgentDeployed = true
		// Wait for deployment URL to be available (max 60 seconds)
		if deployedAgent.Deployment == nil || deployedAgent.Deployment.URL == "" {
			log.Printf("Waiting for agent deployment URL...")
			deployedAgent, err = s.doClient.WaitForAgentDeployment(ctx, agent.UUID, 60*time.Second)
			if err != nil {
				log.Printf("Warning: Deployment URL not ready: %v", err)
			}
		}
		if deployedAgent != nil && deployedAgent.Deployment != nil {
			subject.AgentDeploymentURL = deployedAgent.Deployment.URL
		}
	}

	// 8. Enable Citations for the Agent
	if _, err := s.doClient.EnableAgentCitations(ctx, agent.UUID); err != nil {
		log.Printf("Warning: Failed to enable citations for agent %s: %v", agent.UUID, err)
	} else {
		result.CitationsEnabled = true
		log.Printf("Enabled citations for agent %s", agent.UUID)
	}

	// 9. Create API Key for the Agent
	apiKeyName := fmt.Sprintf("%s-api-key", strings.ToLower(strings.ReplaceAll(subject.Code, " ", "-")))
	apiKeyResult, err := s.doClient.CreateAgentAPIKey(ctx, agent.UUID, apiKeyName)
	if err != nil {
		log.Printf("Warning: Failed to create API key for agent %s: %v", agent.UUID, err)
	} else if apiKeyResult.SecretKey != "" {
		// 10. Encrypt and store the API key
		encryptedKey, err := crypto.EncryptAPIKeyForStorage(apiKeyResult.SecretKey)
		if err != nil {
			log.Printf("Warning: Failed to encrypt API key: %v. Key will not be stored.", err)
		} else {
			subject.AgentAPIKeyEncrypted = encryptedKey
			result.APIKeyCreated = true
			log.Printf("Created and stored encrypted API key for agent %s", agent.UUID)
		}
	}

	// 11. Update subject with UUIDs and encrypted API key
	if err := tx.Save(&subject).Error; err != nil {
		// Clean up DigitalOcean resources
		s.doClient.DeleteAgent(ctx, agent.UUID)
		s.doClient.DeleteKnowledgeBase(ctx, kb.UUID)
		tx.Rollback()
		return nil, fmt.Errorf("failed to update subject with UUIDs: %w", err)
	}

	// 12. Commit transaction
	if err := tx.Commit().Error; err != nil {
		// Clean up DigitalOcean resources if commit fails
		s.doClient.DeleteAgent(ctx, agent.UUID)
		s.doClient.DeleteKnowledgeBase(ctx, kb.UUID)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// GetSubjectAgentAPIKey retrieves and decrypts the stored API key for a subject's agent
// Returns empty string if no API key is stored
func (s *SubjectService) GetSubjectAgentAPIKey(subject *model.Subject) (string, error) {
	if subject.AgentAPIKeyEncrypted == "" {
		return "", nil
	}
	return crypto.DecryptAPIKeyFromStorage(subject.AgentAPIKeyEncrypted)
}

// GetSubjectByID retrieves a subject by ID
func (s *SubjectService) GetSubjectByID(ctx context.Context, subjectID uint) (*model.Subject, error) {
	var subject model.Subject
	if err := s.db.First(&subject, subjectID).Error; err != nil {
		return nil, err
	}
	return &subject, nil
}

// GetDOClient returns the DigitalOcean client (for services that need to use it directly)
func (s *SubjectService) GetDOClient() *digitalocean.Client {
	return s.doClient
}

// SetupSubjectAI sets up AI resources (KB, Agent, API key) for an existing subject
// This is used when subjects are created outside of CreateSubjectWithAI (e.g., syllabus extraction)
// It's safe to call this even if the subject already has AI resources - it will skip setup
func (s *SubjectService) SetupSubjectAI(ctx context.Context, subjectID uint) (*CreateSubjectResult, error) {
	result := &CreateSubjectResult{}

	// Skip if DigitalOcean client is not available
	if s.doClient == nil {
		log.Printf("SetupSubjectAI: DO client not available, skipping AI setup for subject %d", subjectID)
		return result, nil
	}

	// Get subject with semester and course preloaded
	var subject model.Subject
	if err := s.db.Preload("Semester.Course").First(&subject, subjectID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch subject: %w", err)
	}

	result.Subject = &subject

	// Skip if already has AI resources
	if subject.KnowledgeBaseUUID != "" && subject.AgentUUID != "" && subject.AgentAPIKeyEncrypted != "" {
		log.Printf("SetupSubjectAI: Subject %d already has AI resources, skipping", subjectID)
		result.KnowledgeBaseCreated = true
		result.AgentCreated = true
		result.APIKeyCreated = true
		return result, nil
	}

	// Get configuration from environment
	embeddingModel := os.Getenv("DO_EMBEDDING_MODEL_UUID")
	projectID := os.Getenv("DO_PROJECT_ID")
	spacesName := os.Getenv("DO_SPACES_NAME")
	spacesRegion := os.Getenv("DO_SPACES_REGION")
	databaseID := os.Getenv("DO_GENAI_DATABASE_ID")
	genAIRegion := "tor1"

	if spacesRegion == "" {
		spacesRegion = "blr1"
	}

	// 1. Create Knowledge Base if not exists
	if subject.KnowledgeBaseUUID == "" {
		kbName := s.generateKnowledgeBaseName(&subject.Semester.Course, &subject.Semester, &subject)
		kbDescription := fmt.Sprintf("Knowledge base for %s - %s (%s)", subject.Semester.Course.Name, subject.Name, subject.Code)

		createKBReq := digitalocean.CreateKnowledgeBaseRequest{
			Name:           kbName,
			Description:    kbDescription,
			EmbeddingModel: embeddingModel,
			ProjectID:      projectID,
			Region:         genAIRegion,
			DatabaseID:     databaseID,
		}

		// Add subject-scoped data source - only indexes this subject's folder
		if spacesName != "" {
			subjectFolderPath := fmt.Sprintf("subjects/%d/", subject.ID)
			createKBReq.DataSources = []digitalocean.DataSourceCreateInput{
				{
					SpacesDataSource: &digitalocean.SpacesDataSourceInput{
						BucketName: spacesName,
						Region:     spacesRegion,
						ItemPath:   subjectFolderPath,
					},
				},
			}
		}

		kb, err := s.doClient.CreateKnowledgeBase(ctx, createKBReq)
		if err != nil {
			return nil, fmt.Errorf("failed to create knowledge base: %w", err)
		}

		subject.KnowledgeBaseUUID = kb.UUID
		result.KnowledgeBaseCreated = true
		log.Printf("SetupSubjectAI: Created KB %s for subject %d", kb.UUID, subjectID)

		// Start indexing job
		dataSources, err := s.doClient.ListKnowledgeBaseDataSources(ctx, kb.UUID)
		if err == nil && len(dataSources) > 0 {
			dsUUIDs := make([]string, len(dataSources))
			for i, ds := range dataSources {
				dsUUIDs[i] = ds.UUID
			}
			indexReq := digitalocean.StartIndexingJobRequest{
				KnowledgeBaseUUID: kb.UUID,
				DataSourceUUIDs:   dsUUIDs,
			}
			if _, err := s.doClient.StartIndexingJob(ctx, indexReq); err != nil {
				log.Printf("Warning: Failed to start indexing job for KB %s: %v", kb.UUID, err)
			}
		}
	} else {
		result.KnowledgeBaseCreated = true
	}

	// 2. Create Agent if not exists
	if subject.AgentUUID == "" {
		modelUUID := os.Getenv("DO_AGENT_MODEL_UUID")
		if modelUUID == "" {
			modelUUID = "18bc9b8f-73c5-11f0-b074-4e013e2ddde4" // OpenAI GPT-oss-120b
		}

		agentName := fmt.Sprintf("%s Agent", subject.Name)
		agentDescription := fmt.Sprintf("AI assistant for %s course", subject.Name)
		agentInstructions := fmt.Sprintf(
			"You are a helpful AI assistant for the %s course. "+
				"Use the knowledge base to answer questions accurately. "+
				"If you don't know the answer, say so honestly. "+
				"Always be clear, concise, and educational.",
			subject.Name,
		)

		createAgentReq := digitalocean.CreateAgentRequest{
			Name:         agentName,
			Description:  agentDescription,
			ModelUUID:    modelUUID,
			ProjectID:    projectID,
			Region:       genAIRegion,
			Instructions: agentInstructions,
			Temperature:  0,
			TopP:         1,
		}

		agent, err := s.doClient.CreateAgent(ctx, createAgentReq)
		if err != nil {
			return nil, fmt.Errorf("failed to create agent: %w", err)
		}

		subject.AgentUUID = agent.UUID
		result.AgentCreated = true
		log.Printf("SetupSubjectAI: Created Agent %s for subject %d", agent.UUID, subjectID)

		// Attach KB to Agent
		if subject.KnowledgeBaseUUID != "" {
			if err := s.doClient.AttachKnowledgeBase(ctx, agent.UUID, subject.KnowledgeBaseUUID); err != nil {
				log.Printf("Warning: Failed to attach KB to agent: %v", err)
			}
		}

		// Deploy Agent
		if deployedAgent, err := s.doClient.DeployAgent(ctx, agent.UUID, digitalocean.VisibilityPrivate); err != nil {
			log.Printf("Warning: Failed to deploy agent: %v", err)
		} else {
			result.AgentDeployed = true
			if deployedAgent.Deployment != nil {
				subject.AgentDeploymentURL = deployedAgent.Deployment.URL
			}
		}

		// Enable citations
		if _, err := s.doClient.EnableAgentCitations(ctx, agent.UUID); err != nil {
			log.Printf("Warning: Failed to enable citations: %v", err)
		} else {
			result.CitationsEnabled = true
		}
	} else {
		result.AgentCreated = true
	}

	// 3. Create API Key if not exists
	if subject.AgentAPIKeyEncrypted == "" && subject.AgentUUID != "" {
		apiKeyName := fmt.Sprintf("%s-api-key", strings.ToLower(strings.ReplaceAll(subject.Code, " ", "-")))
		apiKeyResult, err := s.doClient.CreateAgentAPIKey(ctx, subject.AgentUUID, apiKeyName)
		if err != nil {
			log.Printf("Warning: Failed to create API key: %v", err)
		} else if apiKeyResult.SecretKey != "" {
			encryptedKey, err := crypto.EncryptAPIKeyForStorage(apiKeyResult.SecretKey)
			if err != nil {
				log.Printf("Warning: Failed to encrypt API key: %v", err)
			} else {
				subject.AgentAPIKeyEncrypted = encryptedKey
				result.APIKeyCreated = true
				log.Printf("SetupSubjectAI: Created API key for subject %d", subjectID)
			}
		}
	} else if subject.AgentAPIKeyEncrypted != "" {
		result.APIKeyCreated = true
	}

	// Save updated subject
	if err := s.db.Save(&subject).Error; err != nil {
		return nil, fmt.Errorf("failed to save subject with AI resources: %w", err)
	}

	result.Subject = &subject
	return result, nil
}

// DeleteSubjectWithCleanup deletes a subject and cleans up DigitalOcean resources
func (s *SubjectService) DeleteSubjectWithCleanup(ctx context.Context, subjectID uint) error {
	// Get subject with UUIDs
	var subject model.Subject
	if err := s.db.First(&subject, subjectID).Error; err != nil {
		return fmt.Errorf("failed to fetch subject: %w", err)
	}

	// Clean up DigitalOcean resources if they exist
	if s.doClient != nil {
		if subject.AgentUUID != "" {
			if err := s.doClient.DeleteAgent(ctx, subject.AgentUUID); err != nil {
				log.Printf("Warning: Failed to delete agent %s: %v", subject.AgentUUID, err)
			}
		}

		if subject.KnowledgeBaseUUID != "" {
			if err := s.doClient.DeleteKnowledgeBase(ctx, subject.KnowledgeBaseUUID); err != nil {
				log.Printf("Warning: Failed to delete knowledge base %s: %v", subject.KnowledgeBaseUUID, err)
			}
		}
	}

	// Delete subject from database (soft delete)
	if err := s.db.Delete(&subject).Error; err != nil {
		return fmt.Errorf("failed to delete subject: %w", err)
	}

	return nil
}
