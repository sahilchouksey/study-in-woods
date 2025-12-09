package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"gorm.io/gorm"
)

// SyllabusService handles syllabus extraction and management
type SyllabusService struct {
	db              *gorm.DB
	inferenceClient *digitalocean.InferenceClient
	spacesClient    *digitalocean.SpacesClient
	enableAI        bool
	enableSpaces    bool
}

// NewSyllabusService creates a new syllabus service
func NewSyllabusService(db *gorm.DB) *SyllabusService {
	service := &SyllabusService{
		db:           db,
		enableAI:     false,
		enableSpaces: false,
	}

	// Initialize inference client for AI extraction
	inferenceAPIKey := os.Getenv("DO_INFERENCE_API_KEY")
	if inferenceAPIKey != "" {
		service.inferenceClient = digitalocean.NewInferenceClient(digitalocean.InferenceConfig{
			APIKey: inferenceAPIKey,
		})
		service.enableAI = true
	} else {
		log.Println("Warning: DO_INFERENCE_API_KEY not set. Syllabus extraction will be disabled.")
	}

	// Initialize Spaces client using global config (supports auto-generation of keys)
	spacesClient, err := digitalocean.NewSpacesClientFromGlobalConfig()
	if err != nil {
		log.Printf("Warning: Failed to initialize Spaces client: %v", err)
	} else {
		service.spacesClient = spacesClient
		service.enableSpaces = true
	}

	return service
}

// SyllabusExtractionResult holds the result of syllabus extraction
type SyllabusExtractionResult struct {
	SubjectName  string                    `json:"subject_name"`
	SubjectCode  string                    `json:"subject_code"`
	TotalCredits int                       `json:"total_credits"`
	Units        []SyllabusUnitExtraction  `json:"units"`
	Books        []BookReferenceExtraction `json:"books"`
}

// SyllabusUnitExtraction represents extracted unit data
type SyllabusUnitExtraction struct {
	UnitNumber  int                       `json:"unit_number"`
	Title       string                    `json:"title"`
	Description string                    `json:"description,omitempty"`
	Hours       int                       `json:"hours,omitempty"`
	Topics      []SyllabusTopicExtraction `json:"topics"`
}

// SyllabusTopicExtraction represents extracted topic data
type SyllabusTopicExtraction struct {
	TopicNumber int    `json:"topic_number"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Keywords    string `json:"keywords,omitempty"`
}

// BookReferenceExtraction represents extracted book reference data
type BookReferenceExtraction struct {
	Title      string `json:"title"`
	Authors    string `json:"authors"`
	Publisher  string `json:"publisher,omitempty"`
	Edition    string `json:"edition,omitempty"`
	Year       int    `json:"year,omitempty"`
	ISBN       string `json:"isbn,omitempty"`
	IsTextbook bool   `json:"is_textbook"`
	BookType   string `json:"book_type"` // textbook, reference, recommended
}

// syllabusExtractionSchema is the JSON schema for structured syllabus extraction
var syllabusExtractionSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"subject_name": map[string]interface{}{
			"type":        "string",
			"description": "The name of the subject/course",
		},
		"subject_code": map[string]interface{}{
			"type":        "string",
			"description": "The subject/course code if available",
		},
		"total_credits": map[string]interface{}{
			"type":        "integer",
			"description": "Total credits for the course",
		},
		"units": map[string]interface{}{
			"type":        "array",
			"description": "List of units/modules in the syllabus",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"unit_number": map[string]interface{}{
						"type":        "integer",
						"description": "Unit number (1, 2, 3, etc.)",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Title of the unit",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Description of what the unit covers",
					},
					"hours": map[string]interface{}{
						"type":        "integer",
						"description": "Teaching hours for this unit",
					},
					"topics": map[string]interface{}{
						"type":        "array",
						"description": "Topics covered in this unit",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"topic_number": map[string]interface{}{
									"type":        "integer",
									"description": "Topic number within the unit",
								},
								"title": map[string]interface{}{
									"type":        "string",
									"description": "Title of the topic",
								},
								"description": map[string]interface{}{
									"type":        "string",
									"description": "Description of the topic",
								},
								"keywords": map[string]interface{}{
									"type":        "string",
									"description": "Comma-separated keywords for the topic",
								},
							},
							"required": []string{"topic_number", "title"},
						},
					},
				},
				"required": []string{"unit_number", "title", "topics"},
			},
		},
		"books": map[string]interface{}{
			"type":        "array",
			"description": "List of textbooks and reference materials",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Book title",
					},
					"authors": map[string]interface{}{
						"type":        "string",
						"description": "Book authors",
					},
					"publisher": map[string]interface{}{
						"type":        "string",
						"description": "Publisher name",
					},
					"edition": map[string]interface{}{
						"type":        "string",
						"description": "Book edition",
					},
					"year": map[string]interface{}{
						"type":        "integer",
						"description": "Publication year",
					},
					"isbn": map[string]interface{}{
						"type":        "string",
						"description": "ISBN number",
					},
					"is_textbook": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether this is a primary textbook",
					},
					"book_type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"textbook", "reference", "recommended"},
						"description": "Type of book reference",
					},
				},
				"required": []string{"title", "authors", "book_type"},
			},
		},
	},
	"required": []string{"subject_name", "units", "books"},
}

// syllabusExtractionPrompt is the system prompt for LLM extraction
const syllabusExtractionPrompt = `You are an expert at extracting structured information from academic syllabi and course outlines.

Your task is to analyze the provided syllabus document and extract the following information:

1. **Subject Information**: Subject name, subject code (if available), and total credits
2. **Units/Modules**: Each unit with its number, title, description, and teaching hours (if mentioned)
3. **Topics**: For each unit, list the topics covered with their order number, title, description, and keywords
4. **Books/References**: All textbooks and reference materials mentioned, including title, authors, publisher, edition, year, ISBN, and type (textbook/reference/recommended)

Important guidelines:
- Extract ALL units and topics, even if there are many
- Preserve the original numbering (Unit 1, Unit 2, etc.)
- For topics, assign topic_number starting from 1 within each unit
- Keywords should be comma-separated relevant terms for each topic
- If information is not available, use empty string or 0 for numbers
- Be thorough and accurate - this data will be used for student learning`

// ExtractSyllabusFromDocument extracts syllabus data from a document
func (s *SyllabusService) ExtractSyllabusFromDocument(ctx context.Context, documentID uint) (*model.Syllabus, error) {
	if !s.enableAI {
		return nil, fmt.Errorf("AI extraction is not enabled - DO_INFERENCE_API_KEY not configured")
	}

	// 1. Get the document
	var document model.Document
	if err := s.db.Preload("Subject").First(&document, documentID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch document: %w", err)
	}

	// 2. Check if document is a syllabus type
	if document.Type != model.DocumentTypeSyllabus {
		return nil, fmt.Errorf("document is not a syllabus type")
	}

	// 3. Check if syllabus already exists for this subject
	var existingSyllabus model.Syllabus
	err := s.db.Where("subject_id = ?", document.SubjectID).First(&existingSyllabus).Error
	if err == nil {
		// Update existing syllabus
		return s.updateSyllabusExtraction(ctx, &existingSyllabus, &document)
	}

	// 4. Create new syllabus record with pending status
	syllabus := &model.Syllabus{
		SubjectID:        document.SubjectID,
		DocumentID:       documentID,
		ExtractionStatus: model.SyllabusExtractionProcessing,
	}
	if err := s.db.Create(syllabus).Error; err != nil {
		return nil, fmt.Errorf("failed to create syllabus record: %w", err)
	}

	// 5. Extract text from document (currently supports getting document content)
	documentText, err := s.getDocumentText(ctx, &document)
	if err != nil {
		s.updateSyllabusError(syllabus, err.Error())
		return syllabus, fmt.Errorf("failed to get document text: %w", err)
	}

	// 6. Call LLM for extraction
	extractedData, rawResponse, err := s.extractWithLLM(ctx, documentText)
	if err != nil {
		s.updateSyllabusError(syllabus, err.Error())
		return syllabus, fmt.Errorf("failed to extract syllabus data: %w", err)
	}

	// 7. Save extracted data to database
	if err := s.saveSyllabusData(syllabus, extractedData, rawResponse); err != nil {
		s.updateSyllabusError(syllabus, err.Error())
		return syllabus, fmt.Errorf("failed to save syllabus data: %w", err)
	}

	return syllabus, nil
}

// updateSyllabusExtraction updates an existing syllabus with new extraction
func (s *SyllabusService) updateSyllabusExtraction(ctx context.Context, syllabus *model.Syllabus, document *model.Document) (*model.Syllabus, error) {
	// Update status to processing
	syllabus.ExtractionStatus = model.SyllabusExtractionProcessing
	syllabus.DocumentID = document.ID
	s.db.Save(syllabus)

	// Delete existing units and books
	s.db.Where("syllabus_id = ?", syllabus.ID).Delete(&model.SyllabusUnit{})
	s.db.Where("syllabus_id = ?", syllabus.ID).Delete(&model.BookReference{})

	// Get document text
	documentText, err := s.getDocumentText(ctx, document)
	if err != nil {
		s.updateSyllabusError(syllabus, err.Error())
		return syllabus, err
	}

	// Extract with LLM
	extractedData, rawResponse, err := s.extractWithLLM(ctx, documentText)
	if err != nil {
		s.updateSyllabusError(syllabus, err.Error())
		return syllabus, err
	}

	// Save extracted data
	if err := s.saveSyllabusData(syllabus, extractedData, rawResponse); err != nil {
		s.updateSyllabusError(syllabus, err.Error())
		return syllabus, err
	}

	return syllabus, nil
}

// getDocumentText retrieves the text content from a document
func (s *SyllabusService) getDocumentText(ctx context.Context, document *model.Document) (string, error) {
	// For now, we need to download from Spaces and extract text
	// In a production system, you'd want to use a proper PDF text extraction library

	if !s.enableSpaces || document.SpacesKey == "" || document.SpacesKey == "disabled" {
		return "", fmt.Errorf("document storage not available")
	}

	// Download the file
	fileContent, err := s.spacesClient.DownloadFile(ctx, document.SpacesKey)
	if err != nil {
		return "", fmt.Errorf("failed to download document: %w", err)
	}

	// For PDF files, we'll send the raw content to the LLM
	// The LLM can handle base64-encoded content or we can use a PDF extraction service
	// For now, we'll just send a message indicating we have a PDF and its metadata

	// Check file type
	if strings.HasSuffix(strings.ToLower(document.Filename), ".pdf") {
		// For PDFs, we'll need to extract text first
		// This is a placeholder - in production, use a PDF library like pdfcpu or unipdf
		// Or use an external service for PDF to text conversion
		return s.extractTextFromPDF(fileContent, document.Filename)
	}

	// For text-based formats, return content directly
	return string(fileContent), nil
}

// extractTextFromPDF extracts text from PDF content
// This is a simplified implementation - in production, use a proper PDF library
func (s *SyllabusService) extractTextFromPDF(content []byte, filename string) (string, error) {
	// For now, we'll send a description and let the LLM work with what we have
	// In a real implementation, you would:
	// 1. Use a Go PDF library (pdfcpu, unipdf, etc.)
	// 2. Or use an external service (Google Cloud Vision, AWS Textract, etc.)
	// 3. Or pre-process PDFs on upload

	// Basic text extraction attempt - this won't work well for all PDFs
	// but handles simple text-based PDFs
	text := string(content)

	// Try to find readable text content
	if len(text) > 0 {
		// Filter out non-printable characters but keep structure
		var cleanText strings.Builder
		for _, r := range text {
			if r >= 32 && r < 127 || r == '\n' || r == '\t' {
				cleanText.WriteRune(r)
			}
		}
		extracted := cleanText.String()
		if len(extracted) > 100 { // Minimum viable text
			return extracted, nil
		}
	}

	// If we can't extract text, return error with suggestion
	return "", fmt.Errorf("unable to extract text from PDF '%s'. Consider uploading a text-based document or ensuring the PDF contains selectable text", filename)
}

// extractWithLLM calls the LLM to extract syllabus data using structured outputs
func (s *SyllabusService) extractWithLLM(ctx context.Context, documentText string) (*SyllabusExtractionResult, string, error) {
	// Truncate if too long (LLM context limit)
	maxChars := 50000 // Adjust based on model context window
	if len(documentText) > maxChars {
		documentText = documentText[:maxChars] + "\n\n[Document truncated due to length]"
	}

	userPrompt := fmt.Sprintf("Extract the syllabus information from the following document:\n\n%s", documentText)

	// Call LLM with structured output
	response, err := s.inferenceClient.StructuredCompletion(
		ctx,
		syllabusExtractionPrompt,
		userPrompt,
		"syllabus_extraction",
		"Structured syllabus extraction result with units, topics, and book references",
		syllabusExtractionSchema,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0.1), // Low temperature for structured extraction
	)
	if err != nil {
		// Fall back to JSONCompletion if structured output fails
		log.Printf("Structured output failed, falling back to JSONCompletion: %v", err)
		return s.extractWithLLMFallback(ctx, documentText)
	}

	// Parse JSON response - no need to clean up markdown since structured output guarantees valid JSON
	var result SyllabusExtractionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, response, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	return &result, response, nil
}

// extractWithLLMFallback uses the traditional JSONCompletion method as a fallback
func (s *SyllabusService) extractWithLLMFallback(ctx context.Context, documentText string) (*SyllabusExtractionResult, string, error) {
	// Truncate if too long (LLM context limit)
	maxChars := 50000
	if len(documentText) > maxChars {
		documentText = documentText[:maxChars] + "\n\n[Document truncated due to length]"
	}

	// Build a prompt that explicitly requests JSON format (fallback behavior)
	fallbackPrompt := syllabusExtractionPrompt + `

Respond with valid JSON only matching this structure:
{
  "subject_name": "string",
  "subject_code": "string",
  "total_credits": number,
  "units": [...],
  "books": [...]
}`

	userPrompt := fmt.Sprintf("Extract the syllabus information from the following document:\n\n%s", documentText)

	// Call LLM
	response, err := s.inferenceClient.JSONCompletion(
		ctx,
		fallbackPrompt,
		userPrompt,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0.1),
	)
	if err != nil {
		return nil, "", fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse JSON response
	var result SyllabusExtractionResult
	// Clean up response - remove markdown code blocks if present
	cleanResponse := strings.TrimSpace(response)
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)

	if err := json.Unmarshal([]byte(cleanResponse), &result); err != nil {
		return nil, response, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	return &result, response, nil
}

// saveSyllabusData saves the extracted syllabus data to the database
func (s *SyllabusService) saveSyllabusData(syllabus *model.Syllabus, data *SyllabusExtractionResult, rawResponse string) error {
	// Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update syllabus basic info
	syllabus.SubjectName = data.SubjectName
	syllabus.SubjectCode = data.SubjectCode
	syllabus.TotalCredits = data.TotalCredits
	syllabus.RawExtraction = rawResponse
	syllabus.ExtractionStatus = model.SyllabusExtractionCompleted
	syllabus.ExtractionError = ""

	if err := tx.Save(syllabus).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update syllabus: %w", err)
	}

	// Create units and topics
	for _, unitData := range data.Units {
		unit := model.SyllabusUnit{
			SyllabusID:  syllabus.ID,
			UnitNumber:  unitData.UnitNumber,
			Title:       unitData.Title,
			Description: unitData.Description,
			Hours:       unitData.Hours,
		}

		if err := tx.Create(&unit).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create unit: %w", err)
		}

		// Create topics for this unit
		for _, topicData := range unitData.Topics {
			topic := model.SyllabusTopic{
				UnitID:      unit.ID,
				TopicNumber: topicData.TopicNumber,
				Title:       topicData.Title,
				Description: topicData.Description,
				Keywords:    topicData.Keywords,
			}

			if err := tx.Create(&topic).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create topic: %w", err)
			}
		}
	}

	// Create book references
	for _, bookData := range data.Books {
		book := model.BookReference{
			SyllabusID: syllabus.ID,
			Title:      bookData.Title,
			Authors:    bookData.Authors,
			Publisher:  bookData.Publisher,
			Edition:    bookData.Edition,
			Year:       bookData.Year,
			ISBN:       bookData.ISBN,
			IsTextbook: bookData.IsTextbook,
			BookType:   bookData.BookType,
		}

		if err := tx.Create(&book).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create book reference: %w", err)
		}
	}

	return tx.Commit().Error
}

// updateSyllabusError updates syllabus with error status
func (s *SyllabusService) updateSyllabusError(syllabus *model.Syllabus, errMsg string) {
	syllabus.ExtractionStatus = model.SyllabusExtractionFailed
	syllabus.ExtractionError = errMsg
	s.db.Save(syllabus)
}

// GetSyllabusBySubject retrieves syllabus for a subject
func (s *SyllabusService) GetSyllabusBySubject(ctx context.Context, subjectID uint) (*model.Syllabus, error) {
	var syllabus model.Syllabus
	err := s.db.Preload("Units", func(db *gorm.DB) *gorm.DB {
		return db.Order("unit_number ASC")
	}).Preload("Units.Topics", func(db *gorm.DB) *gorm.DB {
		return db.Order("topic_number ASC")
	}).Preload("Books").Where("subject_id = ?", subjectID).First(&syllabus).Error

	if err != nil {
		return nil, err
	}

	return &syllabus, nil
}

// GetSyllabusByID retrieves syllabus by ID
func (s *SyllabusService) GetSyllabusByID(ctx context.Context, syllabusID uint) (*model.Syllabus, error) {
	var syllabus model.Syllabus
	err := s.db.Preload("Units", func(db *gorm.DB) *gorm.DB {
		return db.Order("unit_number ASC")
	}).Preload("Units.Topics", func(db *gorm.DB) *gorm.DB {
		return db.Order("topic_number ASC")
	}).Preload("Books").First(&syllabus, syllabusID).Error

	if err != nil {
		return nil, err
	}

	return &syllabus, nil
}

// TriggerExtractionAsync triggers syllabus extraction in a goroutine
// Returns immediately, extraction happens in background
func (s *SyllabusService) TriggerExtractionAsync(documentID uint) {
	go func() {
		ctx := context.Background()
		_, err := s.ExtractSyllabusFromDocument(ctx, documentID)
		if err != nil {
			log.Printf("Background syllabus extraction failed for document %d: %v", documentID, err)
		} else {
			log.Printf("Background syllabus extraction completed for document %d", documentID)
		}
	}()
}

// RetryExtraction retries failed extraction for a syllabus
func (s *SyllabusService) RetryExtraction(ctx context.Context, syllabusID uint) (*model.Syllabus, error) {
	var syllabus model.Syllabus
	if err := s.db.First(&syllabus, syllabusID).Error; err != nil {
		return nil, fmt.Errorf("syllabus not found: %w", err)
	}

	if syllabus.ExtractionStatus != model.SyllabusExtractionFailed {
		return nil, fmt.Errorf("can only retry failed extractions")
	}

	return s.ExtractSyllabusFromDocument(ctx, syllabus.DocumentID)
}

// DeleteSyllabus deletes a syllabus and all related data
func (s *SyllabusService) DeleteSyllabus(ctx context.Context, syllabusID uint) error {
	// Cascading delete is handled by GORM constraints
	return s.db.Delete(&model.Syllabus{}, syllabusID).Error
}

// GetExtractionStatus returns the current extraction status
func (s *SyllabusService) GetExtractionStatus(ctx context.Context, syllabusID uint) (model.SyllabusExtractionStatus, string, error) {
	var syllabus model.Syllabus
	if err := s.db.Select("extraction_status", "extraction_error").First(&syllabus, syllabusID).Error; err != nil {
		return "", "", err
	}
	return syllabus.ExtractionStatus, syllabus.ExtractionError, nil
}
