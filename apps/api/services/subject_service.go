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

	// Get embedding model UUID from environment or use default
	embeddingModel := os.Getenv("DO_EMBEDDING_MODEL_UUID")

	createKBReq := digitalocean.CreateKnowledgeBaseRequest{
		Name:           kbName,
		Description:    kbDescription,
		EmbeddingModel: embeddingModel,
	}

	kb, err := s.doClient.CreateKnowledgeBase(ctx, createKBReq)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create knowledge base: %w", err)
	}

	result.KnowledgeBaseCreated = true
	subject.KnowledgeBaseUUID = kb.UUID

	// 4. Create AI Agent connected to Knowledge Base
	agentName := fmt.Sprintf("%s Agent", subject.Name)
	agentDescription := fmt.Sprintf("AI assistant for %s course", subject.Name)
	agentInstructions := fmt.Sprintf(
		"You are a helpful AI assistant for the %s course. "+
			"Use the knowledge base to answer questions accurately. "+
			"If you don't know the answer, say so honestly. "+
			"Always be clear, concise, and educational.",
		subject.Name,
	)

	// Default to Claude 3 Sonnet or use environment variable
	modelID := os.Getenv("DO_AGENT_MODEL_ID")
	if modelID == "" {
		modelID = "anthropic/claude-3-sonnet" // Default model
	}

	createAgentReq := digitalocean.CreateAgentRequest{
		Name:           agentName,
		Description:    agentDescription,
		ModelID:        modelID,
		Instructions:   agentInstructions,
		Temperature:    0.7,
		TopP:           0.9,
		KnowledgeBases: []string{kb.UUID},
	}

	agent, err := s.doClient.CreateAgent(ctx, createAgentReq)
	if err != nil {
		// Try to clean up knowledge base on agent creation failure
		s.doClient.DeleteKnowledgeBase(ctx, kb.UUID)
		tx.Rollback()
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	result.AgentCreated = true
	subject.AgentUUID = agent.UUID

	// 5. Update subject with UUIDs
	if err := tx.Save(&subject).Error; err != nil {
		// Clean up DigitalOcean resources
		s.doClient.DeleteAgent(ctx, agent.UUID)
		s.doClient.DeleteKnowledgeBase(ctx, kb.UUID)
		tx.Rollback()
		return nil, fmt.Errorf("failed to update subject with UUIDs: %w", err)
	}

	// 6. Commit transaction
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
