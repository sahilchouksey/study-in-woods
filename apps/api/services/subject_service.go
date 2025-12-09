package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
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
	Error                error
}

// generateKnowledgeBaseName generates a unique name for the knowledge base
func (s *SubjectService) generateKnowledgeBaseName(course *model.Course, semester *model.Semester, subject *model.Subject) string {
	// Format: coursecode-sem1-subjectcode (e.g., "mca-sem1-dsa")
	courseName := strings.ToLower(strings.ReplaceAll(course.Code, " ", "-"))
	semesterName := fmt.Sprintf("sem%d", semester.Number)
	subjectName := strings.ToLower(strings.ReplaceAll(subject.Code, " ", "-"))

	return fmt.Sprintf("%s-%s-%s", courseName, semesterName, subjectName)
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

	// Add datasource - Spaces bucket is required
	if spacesName != "" {
		createKBReq.DataSources = []digitalocean.DataSourceCreateInput{
			{
				SpacesDataSource: &digitalocean.SpacesDataSourceInput{
					BucketName: spacesName,
					Region:     spacesRegion,
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
		Temperature:  0.7,
		TopP:         0.9,
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

	// 7. Update subject with UUIDs
	if err := tx.Save(&subject).Error; err != nil {
		// Clean up DigitalOcean resources
		s.doClient.DeleteAgent(ctx, agent.UUID)
		s.doClient.DeleteKnowledgeBase(ctx, kb.UUID)
		tx.Rollback()
		return nil, fmt.Errorf("failed to update subject with UUIDs: %w", err)
	}

	// 8. Commit transaction
	if err := tx.Commit().Error; err != nil {
		// Clean up DigitalOcean resources if commit fails
		s.doClient.DeleteAgent(ctx, agent.UUID)
		s.doClient.DeleteKnowledgeBase(ctx, kb.UUID)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

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
