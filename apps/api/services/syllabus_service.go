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
	"github.com/sahilchouksey/go-init-setup/utils"
	"gorm.io/gorm"
)

// SyllabusService handles syllabus extraction and management
type SyllabusService struct {
	db               *gorm.DB
	inferenceClient  *digitalocean.InferenceClient
	spacesClient     *digitalocean.SpacesClient
	pdfExtractor     *PDFExtractor
	chunkedExtractor *ChunkedSyllabusExtractor
	subjectService   *SubjectService
	enableAI         bool
	enableSpaces     bool
}

// SmallPDFThreshold is the page count below which we use direct extraction
// PDFs with more pages than this will use chunked parallel extraction
const SmallPDFThreshold = 3

// NewSyllabusService creates a new syllabus service
func NewSyllabusService(db *gorm.DB) *SyllabusService {
	service := &SyllabusService{
		db:           db,
		pdfExtractor: NewPDFExtractor(),
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

	// Initialize chunked extractor if AI and Spaces are available
	if service.enableAI && service.enableSpaces {
		service.chunkedExtractor = NewChunkedSyllabusExtractor(
			db,
			service.inferenceClient,
			service.spacesClient,
			service.pdfExtractor,
			DefaultChunkedExtractorConfig(),
		)
		log.Println("ChunkedSyllabusExtractor initialized for parallel PDF processing")
	}

	// Initialize subject service for AI resource creation
	service.subjectService = NewSubjectService(db)

	return service
}

// SyllabusExtractionResult holds the result of syllabus extraction (multiple subjects)
type SyllabusExtractionResult struct {
	Subjects []SubjectExtractionResult `json:"subjects"`
}

// SubjectExtractionResult holds extraction data for a single subject
type SubjectExtractionResult struct {
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
	RawText     string                    `json:"raw_text"` // Exact verbatim text from syllabus
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
const syllabusExtractionPrompt = `You are an expert at extracting structured information from academic syllabuses and course outlines.
You MUST respond with ONLY a valid JSON object. Do NOT include any explanatory text, markdown formatting, or code blocks.

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
- Be thorough and accurate - this data will be used for student learning

CRITICAL: You MUST output ONLY raw JSON without any markdown, explanation or commentary. Start your response with { and end with }`

// deleteExistingSyllabusDataForSubject deletes all existing syllabus data for a subject
// This ensures a clean slate when uploading a new syllabus
func (s *SyllabusService) deleteExistingSyllabusDataForSubject(ctx context.Context, subjectID uint) error {
	if subjectID == 0 {
		return nil // No subject to clean up
	}

	// Get all syllabuses for this subject
	var syllabuses []model.Syllabus
	if err := s.db.Where("subject_id = ?", subjectID).Find(&syllabuses).Error; err != nil {
		return fmt.Errorf("failed to find existing syllabuses: %w", err)
	}

	if len(syllabuses) == 0 {
		log.Printf("SyllabusService: No existing syllabus data found for subject %d", subjectID)
		return nil
	}

	log.Printf("SyllabusService: Deleting %d existing syllabus(es) for subject %d", len(syllabuses), subjectID)

	// Delete all syllabuses (cascade will delete units, topics, and books)
	// Using Unscoped() to permanently delete, not soft delete
	if err := s.db.Unscoped().Where("subject_id = ?", subjectID).Delete(&model.Syllabus{}).Error; err != nil {
		return fmt.Errorf("failed to delete existing syllabuses: %w", err)
	}

	log.Printf("SyllabusService: Successfully deleted existing syllabus data for subject %d", subjectID)
	return nil
}

// DeleteExistingSyllabusDataForSemester deletes all existing syllabus data for all subjects in a semester
// This is used when uploading a new semester-level syllabus to ensure clean data
func (s *SyllabusService) DeleteExistingSyllabusDataForSemester(ctx context.Context, semesterID uint) error {
	if semesterID == 0 {
		return nil // No semester to clean up
	}

	// Get all subjects in this semester
	var subjects []model.Subject
	if err := s.db.Where("semester_id = ?", semesterID).Find(&subjects).Error; err != nil {
		return fmt.Errorf("failed to find subjects in semester: %w", err)
	}

	if len(subjects) == 0 {
		log.Printf("SyllabusService: No subjects found in semester %d", semesterID)
		return nil
	}

	// Collect all subject IDs
	subjectIDs := make([]uint, 0, len(subjects))
	for _, subject := range subjects {
		subjectIDs = append(subjectIDs, subject.ID)
	}

	// Count existing syllabuses
	var count int64
	if err := s.db.Model(&model.Syllabus{}).Where("subject_id IN ?", subjectIDs).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count existing syllabuses: %w", err)
	}

	if count == 0 {
		log.Printf("SyllabusService: No existing syllabus data found for semester %d", semesterID)
		return nil
	}

	log.Printf("SyllabusService: Deleting %d existing syllabus(es) for semester %d (%d subjects)", count, semesterID, len(subjects))

	// Delete all syllabuses for these subjects (cascade will delete units, topics, and books)
	// Using Unscoped() to permanently delete, not soft delete
	if err := s.db.Unscoped().Where("subject_id IN ?", subjectIDs).Delete(&model.Syllabus{}).Error; err != nil {
		return fmt.Errorf("failed to delete existing syllabuses: %w", err)
	}

	log.Printf("SyllabusService: Successfully deleted existing syllabus data for semester %d", semesterID)
	return nil
}

// ExtractSyllabusFromDocument extracts syllabus data from a document
// Returns multiple syllabuses if the document contains multiple subjects
// Uses chunked parallel extraction for large PDFs (>4 pages)
// IMPORTANT: Deletes all existing syllabus data for the subject before extraction
func (s *SyllabusService) ExtractSyllabusFromDocument(ctx context.Context, documentID uint) ([]*model.Syllabus, error) {
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

	// 3. Delete existing syllabus data for this subject (if subject-based document)
	// For semester-based documents, we skip this as subjects will be created during extraction
	if document.SubjectID != nil && *document.SubjectID > 0 {
		if err := s.deleteExistingSyllabusDataForSubject(ctx, *document.SubjectID); err != nil {
			log.Printf("Warning: Failed to delete existing syllabus data: %v", err)
			// Continue anyway - non-critical error
		}
	}

	// 4. Download PDF content from Spaces
	if !s.enableSpaces || document.SpacesKey == "" || document.SpacesKey == "disabled" {
		return nil, fmt.Errorf("document storage not available")
	}

	pdfContent, err := s.spacesClient.DownloadFile(ctx, document.SpacesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to download document: %w", err)
	}

	// 5. Get page count to determine extraction strategy
	pageCount, err := s.pdfExtractor.GetPageCount(pdfContent)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	log.Printf("SyllabusService: PDF has %d pages, choosing extraction strategy...", pageCount)

	// 6. Choose extraction strategy based on page count
	if pageCount <= SmallPDFThreshold {
		// Small PDF - use direct extraction (existing method)
		log.Printf("SyllabusService: Using direct extraction for small PDF (%d pages)", pageCount)
		return s.extractDirectly(ctx, &document, pdfContent)
	}

	// Large PDF - use chunked parallel extraction
	if s.chunkedExtractor == nil {
		return nil, fmt.Errorf("chunked extractor not initialized")
	}

	log.Printf("SyllabusService: Using chunked parallel extraction for large PDF (%d pages)", pageCount)
	return s.chunkedExtractor.ExtractSyllabusChunked(ctx, &document, pdfContent)
}

// extractDirectly extracts syllabus using the original direct method (for small PDFs)
func (s *SyllabusService) extractDirectly(ctx context.Context, document *model.Document, pdfContent []byte) ([]*model.Syllabus, error) {
	// Extract text from PDF
	documentText, err := s.pdfExtractor.ExtractText(pdfContent)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text from PDF: %w", err)
	}

	// Call LLM for extraction
	extractedData, rawResponse, err := s.extractWithLLM(ctx, documentText)
	if err != nil {
		return nil, fmt.Errorf("failed to extract syllabus data: %w", err)
	}

	// Save extracted data to database (creates subjects if needed)
	syllabuses, err := s.saveMultiSubjectSyllabusData(ctx, document, extractedData, rawResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to save syllabus data: %w", err)
	}

	return syllabuses, nil
}

// updateSyllabusExtraction updates an existing syllabus with new extraction
// Note: This now uses the first subject from multi-subject extraction for backward compatibility
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

	// Use first subject for backward compatibility
	if len(extractedData.Subjects) == 0 {
		s.updateSyllabusError(syllabus, "no subjects found in extraction result")
		return syllabus, fmt.Errorf("no subjects found in extraction result")
	}

	// Save extracted data using first subject
	if err := s.saveSyllabusData(syllabus, &extractedData.Subjects[0], rawResponse); err != nil {
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

// extractTextFromPDF extracts text from PDF content using ledongthuc/pdf
func (s *SyllabusService) extractTextFromPDF(content []byte, filename string) (string, error) {
	text, err := s.pdfExtractor.ExtractText(content)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from '%s': %w", filename, err)
	}
	return text, nil
}

// extractWithLLM calls the LLM to extract syllabus data
// Note: response_format: json_schema is not supported by DigitalOcean's serverless inference,
// so we use explicit JSON instructions in the prompt instead
func (s *SyllabusService) extractWithLLM(ctx context.Context, documentText string) (*SyllabusExtractionResult, string, error) {
	// Truncate if too long (LLM context limit)
	maxChars := 50000 // Adjust based on model context window
	if len(documentText) > maxChars {
		documentText = documentText[:maxChars] + "\n\n[Document truncated due to length]"
	}

	// Build few-shot prompt with clear examples for proper unit title extraction
	jsonPrompt := `You are a syllabus extraction expert. Extract structured data from academic syllabuses.

CRITICAL FIELD RULES:
- "title": SHORT summary (3-6 words, max 60 chars) - concise theme ONLY
- "raw_text": FULL verbatim text from syllabus - copy word-for-word
- "topics": Detailed breakdown - extract ALL individual topics

EXAMPLES:

Example 1 - CORRECT:
{
  "unit_number": 1,
  "title": "Neural Network Fundamentals",
  "raw_text": "Artificial Neural Networks: Introduction, Biological vs Artificial neurons, ANN architecture, Activation functions, McCulloch & Pitts model, Perceptron, ADALINE, MADALINE",
  "topics": [
    {"topic_number": 1, "title": "Introduction to ANN"},
    {"topic_number": 2, "title": "Biological neurons"},
    {"topic_number": 3, "title": "ANN architecture"},
    {"topic_number": 4, "title": "Activation functions"}
  ]
}

Example 2 - CORRECT:
{
  "unit_number": 2,
  "title": "Supervised Learning",
  "raw_text": "Supervised Learning: Perceptron, Backpropagation networks, multilayer perceptron, Hopfield network, BAM, RBF Neural Network",
  "topics": [
    {"topic_number": 1, "title": "Perceptron"},
    {"topic_number": 2, "title": "Backpropagation networks"},
    {"topic_number": 3, "title": "Hopfield network"}
  ]
}

Example 3 - WRONG (Do NOT do this):
{
  "unit_number": 4,
  "title": "Fuzzy Logic Crisp & fuzzy sets fuzzy relations fuzzy conditional statements fuzzy rules",
  "raw_text": "Fuzzy Logic Crisp & fuzzy sets fuzzy relations fuzzy conditional statements fuzzy rules",
  "topics": []
}
^^ PROBLEMS: Title too long (duplicates raw_text), no topics extracted

Example 3 - CORRECT VERSION:
{
  "unit_number": 4,
  "title": "Fuzzy Logic Fundamentals",
  "raw_text": "Fuzzy Logic Crisp & fuzzy sets fuzzy relations fuzzy conditional statements fuzzy rules fuzzy algorithm. Fuzzy logic controller",
  "topics": [
    {"topic_number": 1, "title": "Crisp sets"},
    {"topic_number": 2, "title": "Fuzzy sets"},
    {"topic_number": 3, "title": "Fuzzy relations"},
    {"topic_number": 4, "title": "Fuzzy rules"}
  ]
}

OUTPUT FORMAT:
{
  "subjects": [
    {
      "subject_name": "Course Name",
      "subject_code": "MCA 301",
      "total_credits": 3,
      "units": [
        {
          "unit_number": 1,
          "title": "SHORT_THEME_3-6_WORDS",
          "description": "Brief description",
          "raw_text": "EXACT_VERBATIM_TEXT_FROM_SYLLABUS",
          "hours": 10,
          "topics": [
            {"topic_number": 1, "title": "Topic name", "description": "", "keywords": ""}
          ]
        }
      ],
      "books": [
        {"title": "Book Title", "authors": "Authors", "book_type": "textbook"}
      ]
    }
  ]
}

KEY RULES:
1. title = SHORT (3-6 words max, NOT the full content list)
2. raw_text = FULL verbatim text (copy exactly from syllabus)
3. topics = DETAILED list (extract ALL individual topics)
4. title ≠ raw_text (they must be DIFFERENT)
5. Extract ALL subjects if document contains multiple
6. Output ONLY valid JSON, no markdown or explanations`

	userPrompt := fmt.Sprintf("Extract ALL subjects and their syllabus information from this document. Return ONLY a JSON object:\n\n%s", documentText)

	// Call LLM with simple completion (no response_format parameter)
	response, err := s.inferenceClient.SimpleCompletion(
		ctx,
		jsonPrompt,
		userPrompt,
		digitalocean.WithInferenceMaxTokens(16384), // Increased for multi-subject extraction
		digitalocean.WithInferenceTemperature(0.1), // Low temperature for structured extraction
	)
	if err != nil {
		return nil, "", fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse JSON response using robust extractor to handle any garbage characters
	var result SyllabusExtractionResult
	if err := utils.ExtractJSONTo(response, &result); err != nil {
		// Log the problematic response for debugging
		log.Printf("Failed to extract JSON from response (length=%d): %v", len(response), err)
		if len(response) > 500 {
			log.Printf("Response preview: %s...", response[:500])
		}
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

	// Parse JSON response using robust extractor
	var result SyllabusExtractionResult
	if err := utils.ExtractJSONTo(response, &result); err != nil {
		log.Printf("Fallback: Failed to extract JSON from response (length=%d): %v", len(response), err)
		return nil, response, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	return &result, response, nil
}

// saveSyllabusData saves the extracted syllabus data to the database (for single subject)
func (s *SyllabusService) saveSyllabusData(syllabus *model.Syllabus, data *SubjectExtractionResult, rawResponse string) error {
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
		// Validate and fix unit title if needed
		fixedTitle := ValidateAndFixUnitTitle(unitData.Title, unitData.RawText)

		unit := model.SyllabusUnit{
			SyllabusID:  syllabus.ID,
			UnitNumber:  unitData.UnitNumber,
			Title:       fixedTitle,
			Description: unitData.Description,
			RawText:     unitData.RawText,
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

// saveMultiSubjectSyllabusData saves extracted data for multiple subjects from a single document
// Finds existing subjects based on subject codes and creates syllabuses for each
// NOTE: This function NEVER creates new subjects - it only creates syllabuses for existing subjects
func (s *SyllabusService) saveMultiSubjectSyllabusData(ctx context.Context, document *model.Document, data *SyllabusExtractionResult, rawResponse string) ([]*model.Syllabus, error) {
	if len(data.Subjects) == 0 {
		return nil, fmt.Errorf("no subjects found in extraction result")
	}

	var syllabuses []*model.Syllabus
	var subjectsNeedingAISetup []uint // Track subjects that need AI setup after transaction commits

	// Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, subjectData := range data.Subjects {
		// Skip continuation pages or invalid subjects
		if subjectData.SubjectCode == "CONTINUATION" || subjectData.SubjectCode == "" {
			log.Printf("SyllabusService: Skipping invalid subject: %s (%s)",
				subjectData.SubjectName, subjectData.SubjectCode)
			continue
		}

		// Find or create subject based on subject code
		var subject model.Subject
		var subjectID uint

		// Try to find existing subject by code within the same semester
		var semesterID uint
		// First check if document has direct semester reference (new approach)
		if document.SemesterID != nil && *document.SemesterID > 0 {
			semesterID = *document.SemesterID
		} else if document.Subject != nil && document.Subject.SemesterID != 0 {
			// Get semester from preloaded subject
			semesterID = document.Subject.SemesterID
		} else if document.SubjectID != nil && *document.SubjectID > 0 {
			// Get semester from document's subject (fallback)
			var docSubject model.Subject
			if err := tx.First(&docSubject, *document.SubjectID).Error; err == nil {
				semesterID = docSubject.SemesterID
			}
		}

		err := tx.Where("code = ? AND semester_id = ?", subjectData.SubjectCode, semesterID).First(&subject).Error
		needsAISetup := false
		if err == gorm.ErrRecordNotFound {
			// Create new subject for this semester
			subject = model.Subject{
				Name:       subjectData.SubjectName,
				Code:       subjectData.SubjectCode,
				Credits:    subjectData.TotalCredits,
				SemesterID: semesterID,
			}
			if err := tx.Create(&subject).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create subject %s: %w", subjectData.SubjectCode, err)
			}
			log.Printf("SyllabusService: Created new subject: %s (%s) for semester %d",
				subject.Name, subject.Code, semesterID)
			needsAISetup = true // New subject needs AI setup
		} else if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to find subject: %w", err)
		} else {
			log.Printf("SyllabusService: Found existing subject ID=%d for %s (%s)",
				subject.ID, subjectData.SubjectName, subjectData.SubjectCode)
			// Check if existing subject needs AI setup (KB, Agent, or API Key missing)
			if subject.KnowledgeBaseUUID == "" || subject.AgentUUID == "" || subject.AgentAPIKeyEncrypted == "" {
				needsAISetup = true
			}
		}
		subjectID = subject.ID

		// Track subjects that need AI setup (will be processed after transaction commits)
		if needsAISetup && s.subjectService != nil {
			subjectsNeedingAISetup = append(subjectsNeedingAISetup, subject.ID)
		}

		// Create syllabus for this subject
		syllabus := &model.Syllabus{
			SubjectID:        subjectID,
			DocumentID:       document.ID,
			SubjectName:      subjectData.SubjectName,
			SubjectCode:      subjectData.SubjectCode,
			TotalCredits:     subjectData.TotalCredits,
			ExtractionStatus: model.SyllabusExtractionCompleted,
			RawExtraction:    rawResponse,
		}

		if err := tx.Create(syllabus).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create syllabus for %s: %w", subjectData.SubjectCode, err)
		}

		// Create units and topics
		for _, unitData := range subjectData.Units {
			// Validate and fix unit title if needed
			fixedTitle := ValidateAndFixUnitTitle(unitData.Title, unitData.RawText)

			unit := model.SyllabusUnit{
				SyllabusID:  syllabus.ID,
				UnitNumber:  unitData.UnitNumber,
				Title:       fixedTitle,
				Description: unitData.Description,
				RawText:     unitData.RawText,
				Hours:       unitData.Hours,
			}

			if err := tx.Create(&unit).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create unit: %w", err)
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
					return nil, fmt.Errorf("failed to create topic: %w", err)
				}
			}
		}

		// Create book references
		for _, bookData := range subjectData.Books {
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
				return nil, fmt.Errorf("failed to create book reference: %w", err)
			}
		}

		// Assign subject to syllabus for later use (e.g., in completion events)
		syllabus.Subject = subject
		syllabuses = append(syllabuses, syllabus)
		log.Printf("Created syllabus for subject %s (%s) with %d units and %d books",
			subjectData.SubjectName, subjectData.SubjectCode,
			len(subjectData.Units), len(subjectData.Books))
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Setup AI resources for subjects AFTER transaction commits (so subjects are visible)
	// Process sequentially with rate limiting and retry to avoid DigitalOcean API 429 errors
	if s.subjectService != nil && len(subjectsNeedingAISetup) > 0 {
		log.Printf("SyllabusService: Starting AI setup for %d subjects after transaction commit (sequential with rate limiting)", len(subjectsNeedingAISetup))

		// Single goroutine processes all subjects sequentially to avoid rate limits
		go func() {
			for i, subjectID := range subjectsNeedingAISetup {
				var lastErr error
				maxRetries := 5

				// Retry loop with exponential backoff for rate limit errors
				for attempt := 0; attempt < maxRetries; attempt++ {
					if attempt > 0 {
						backoff := time.Duration(1<<attempt) * time.Second // 2s, 4s, 8s, 16s
						log.Printf("SyllabusService: Retrying AI setup for subject %d (attempt %d/%d) after %v backoff",
							subjectID, attempt+1, maxRetries, backoff)
						time.Sleep(backoff)
					}

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					result, err := s.subjectService.SetupSubjectAI(ctx, subjectID)
					cancel()

					if err == nil {
						log.Printf("SyllabusService: AI setup complete for subject %d (KB: %v, Agent: %v, APIKey: %v)",
							subjectID, result.KnowledgeBaseCreated, result.AgentCreated, result.APIKeyCreated)
						lastErr = nil
						break
					}

					lastErr = err
					if !isSyllabusRateLimitError(err) {
						// Non-retriable error, log and move on
						log.Printf("Warning: SyllabusService failed to setup AI for subject %d: %v", subjectID, err)
						break
					}
					// Rate limit error - will retry
					log.Printf("SyllabusService: Rate limit hit for subject %d, will retry...", subjectID)
				}

				if lastErr != nil && isSyllabusRateLimitError(lastErr) {
					log.Printf("Warning: SyllabusService exhausted retries for subject %d due to rate limiting: %v", subjectID, lastErr)
				}

				// Rate limit delay between subjects (except after last one)
				if i < len(subjectsNeedingAISetup)-1 {
					time.Sleep(2 * time.Second)
				}
			}
			log.Printf("SyllabusService: Completed AI setup for all %d subjects", len(subjectsNeedingAISetup))
		}()
	}

	return syllabuses, nil
}

// isSyllabusRateLimitError checks if an error is a DigitalOcean rate limit error (HTTP 429)
func isSyllabusRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too_many_requests") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "failed to check limits")
}

// GetSyllabusBySubject retrieves the first/primary syllabus for a subject
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

// GetAllSyllabusesBySubject retrieves all syllabuses for a subject (multiple may exist from different documents)
func (s *SyllabusService) GetAllSyllabusesBySubject(ctx context.Context, subjectID uint) ([]*model.Syllabus, error) {
	var syllabuses []model.Syllabus
	err := s.db.Preload("Units", func(db *gorm.DB) *gorm.DB {
		return db.Order("unit_number ASC")
	}).Preload("Units.Topics", func(db *gorm.DB) *gorm.DB {
		return db.Order("topic_number ASC")
	}).Preload("Books").Where("subject_id = ?", subjectID).Find(&syllabuses).Error

	if err != nil {
		return nil, err
	}

	// Convert to pointer slice
	result := make([]*model.Syllabus, len(syllabuses))
	for i := range syllabuses {
		result[i] = &syllabuses[i]
	}

	return result, nil
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
// Returns all syllabuses extracted (may be multiple subjects from one document)
func (s *SyllabusService) RetryExtraction(ctx context.Context, syllabusID uint) ([]*model.Syllabus, error) {
	var syllabus model.Syllabus
	if err := s.db.First(&syllabus, syllabusID).Error; err != nil {
		return nil, fmt.Errorf("syllabus not found: %w", err)
	}

	if syllabus.ExtractionStatus != model.SyllabusExtractionFailed {
		return nil, fmt.Errorf("can only retry failed extractions")
	}

	// Delete the failed syllabus before re-extraction
	s.db.Delete(&syllabus)

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

// ExtractSyllabusWithProgress extracts syllabus from a document with real-time progress updates via SSE
// The progressCallback is called at each key checkpoint during extraction
func (s *SyllabusService) ExtractSyllabusWithProgress(
	ctx context.Context,
	documentID uint,
	progressCallback ProgressCallback,
) ([]*model.Syllabus, error) {
	startTime := time.Now()

	// Emit started event
	if err := progressCallback(ProgressEvent{
		Type:      "started",
		Progress:  0,
		Phase:     "initializing",
		Message:   "Starting syllabus extraction...",
		Timestamp: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("failed to emit started event: %w", err)
	}

	// Check AI enabled
	if !s.enableAI {
		return nil, fmt.Errorf("AI extraction is not enabled - DO_INFERENCE_API_KEY not configured")
	}

	// Get the document
	var document model.Document
	if err := s.db.Preload("Subject").First(&document, documentID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch document: %w", err)
	}

	// Verify document is a syllabus type
	if document.Type != model.DocumentTypeSyllabus {
		return nil, fmt.Errorf("document is not a syllabus type")
	}

	// Delete existing syllabus data for this subject (if subject-based document)
	// For semester-based documents, we skip this as subjects will be created during extraction
	if document.SubjectID != nil && *document.SubjectID > 0 {
		if err := s.deleteExistingSyllabusDataForSubject(ctx, *document.SubjectID); err != nil {
			log.Printf("Warning: Failed to delete existing syllabus data: %v", err)
			// Continue anyway - non-critical error
		}
	}

	var syllabuses []*model.Syllabus
	var extractErr error

	// Progress: Downloading PDF
	if err := progressCallback(ProgressEvent{
		Type:      "progress",
		Progress:  2,
		Phase:     "download",
		Message:   "Downloading PDF from storage...",
		Timestamp: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	// Download PDF content from Spaces
	if !s.enableSpaces || document.SpacesKey == "" || document.SpacesKey == "disabled" {
		return nil, fmt.Errorf("document storage not available")
	}

	pdfContent, err := s.spacesClient.DownloadFile(ctx, document.SpacesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to download document: %w", err)
	}

	if err := progressCallback(ProgressEvent{
		Type:      "progress",
		Progress:  5,
		Phase:     "download",
		Message:   "PDF downloaded successfully",
		Timestamp: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	// Get page count to determine extraction strategy
	pageCount, err := s.pdfExtractor.GetPageCount(pdfContent)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	if err := progressCallback(ProgressEvent{
		Type:      "progress",
		Progress:  10,
		Phase:     "chunking",
		Message:   fmt.Sprintf("Analyzing document (%d pages)...", pageCount),
		Timestamp: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	log.Printf("SyllabusService: PDF has %d pages, choosing extraction strategy...", pageCount)

	// Choose extraction strategy based on page count
	if pageCount <= SmallPDFThreshold {
		// Small PDF - use direct extraction with progress
		log.Printf("SyllabusService: Using direct extraction for small PDF (%d pages)", pageCount)
		syllabuses, extractErr = s.extractDirectlyWithProgress(ctx, &document, pdfContent, progressCallback)
	} else {
		// Large PDF - use chunked parallel extraction with progress
		if s.chunkedExtractor == nil {
			return nil, fmt.Errorf("chunked extractor not initialized")
		}
		log.Printf("SyllabusService: Using chunked parallel extraction for large PDF (%d pages)", pageCount)
		syllabuses, extractErr = s.chunkedExtractor.ExtractSyllabusChunkedWithProgress(ctx, &document, pdfContent, progressCallback)
	}

	if extractErr != nil {
		return nil, extractErr
	}

	// Emit completion event
	elapsed := time.Since(startTime).Milliseconds()
	syllabusIDs := make([]uint, len(syllabuses))
	subjects := make([]SubjectSummary, len(syllabuses))
	for i, syl := range syllabuses {
		syllabusIDs[i] = syl.ID
		// Populate subject summary for UI display
		subjects[i] = SubjectSummary{
			ID:      syl.SubjectID,
			Name:    syl.Subject.Name,
			Code:    syl.Subject.Code,
			Credits: syl.Subject.Credits,
		}
	}

	if err := progressCallback(ProgressEvent{
		Type:              "complete",
		Progress:          100,
		Phase:             "complete",
		Message:           fmt.Sprintf("Extraction completed successfully (%d subjects)", len(syllabuses)),
		ResultSyllabusIDs: syllabusIDs,
		ResultSubjects:    subjects,
		ElapsedMs:         elapsed,
		Timestamp:         time.Now(),
	}); err != nil {
		// Don't fail here - extraction is complete, just log the error
		log.Printf("Warning: Failed to emit completion event: %v", err)
	}

	return syllabuses, nil
}

// extractDirectlyWithProgress extracts syllabus using the direct method with progress callbacks
func (s *SyllabusService) extractDirectlyWithProgress(
	ctx context.Context,
	document *model.Document,
	pdfContent []byte,
	progressCallback ProgressCallback,
) ([]*model.Syllabus, error) {
	// Extract text from PDF
	if err := progressCallback(ProgressEvent{
		Type:      "progress",
		Progress:  15,
		Phase:     "extraction",
		Message:   "Extracting text from PDF...",
		Timestamp: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	documentText, err := s.pdfExtractor.ExtractText(pdfContent)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text from PDF: %w", err)
	}

	// Call LLM for extraction
	if err := progressCallback(ProgressEvent{
		Type:        "progress",
		Progress:    30,
		Phase:       "extraction",
		Message:     "Processing with AI...",
		TotalChunks: 1,
		Timestamp:   time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	extractedData, rawResponse, err := s.extractWithLLM(ctx, documentText)
	if err != nil {
		return nil, fmt.Errorf("failed to extract syllabus data: %w", err)
	}

	if err := progressCallback(ProgressEvent{
		Type:            "progress",
		Progress:        70,
		Phase:           "extraction",
		Message:         "AI processing complete",
		TotalChunks:     1,
		CompletedChunks: 1,
		Timestamp:       time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	// Save to database
	if err := progressCallback(ProgressEvent{
		Type:      "progress",
		Progress:  75,
		Phase:     "save",
		Message:   "Saving to database...",
		Timestamp: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	syllabuses, err := s.saveMultiSubjectSyllabusData(ctx, document, extractedData, rawResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to save syllabus data: %w", err)
	}

	if err := progressCallback(ProgressEvent{
		Type:      "progress",
		Progress:  95,
		Phase:     "save",
		Message:   fmt.Sprintf("Saved %d subjects to database", len(syllabuses)),
		Timestamp: time.Now(),
	}); err != nil {
		// Don't fail - data is saved
		log.Printf("Warning: Failed to emit save complete event: %v", err)
	}

	return syllabuses, nil
}
