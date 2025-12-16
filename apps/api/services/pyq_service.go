package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"github.com/sahilchouksey/go-init-setup/utils"
	"gorm.io/gorm"
)

// PYQService handles PYQ extraction and management
type PYQService struct {
	db               *gorm.DB
	inferenceClient  *digitalocean.InferenceClient
	spacesClient     *digitalocean.SpacesClient
	pdfExtractor     *PDFExtractor
	chunkedExtractor *ChunkedPYQExtractor
	enableAI         bool
	enableSpaces     bool
}

// NewPYQService creates a new PYQ service
func NewPYQService(db *gorm.DB) *PYQService {
	service := &PYQService{
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
		log.Println("Warning: DO_INFERENCE_API_KEY not set. PYQ extraction will be disabled.")
	}

	// Initialize Spaces client using global config (supports auto-generation of keys)
	spacesClient, err := digitalocean.NewSpacesClientFromGlobalConfig()
	if err != nil {
		log.Printf("Warning: Failed to initialize Spaces client: %v", err)
	} else {
		service.spacesClient = spacesClient
		service.enableSpaces = true
	}

	// Initialize chunked extractor for parallel processing
	if service.enableAI && service.enableSpaces {
		service.chunkedExtractor = NewChunkedPYQExtractor(
			db,
			service.inferenceClient,
			service.spacesClient,
			service.pdfExtractor,
			DefaultChunkedPYQExtractorConfig(),
		)
	}

	return service
}

// PYQExtractionResult holds the result of PYQ extraction
type PYQExtractionResult struct {
	Year         int                     `json:"year"`
	Month        string                  `json:"month"`
	ExamType     string                  `json:"exam_type"`
	TotalMarks   int                     `json:"total_marks"`
	Duration     string                  `json:"duration"`
	Instructions string                  `json:"instructions"`
	Questions    []PYQQuestionExtraction `json:"questions"`
}

// PYQQuestionExtraction represents extracted question data
type PYQQuestionExtraction struct {
	QuestionNumber string                `json:"question_number"`
	SectionName    string                `json:"section_name,omitempty"`
	QuestionText   string                `json:"question_text"`
	Marks          int                   `json:"marks"`
	IsCompulsory   bool                  `json:"is_compulsory"`
	HasChoices     bool                  `json:"has_choices"`
	ChoiceGroup    string                `json:"choice_group,omitempty"`
	UnitNumber     int                   `json:"unit_number,omitempty"`
	TopicKeywords  string                `json:"topic_keywords,omitempty"`
	Choices        []PYQChoiceExtraction `json:"choices,omitempty"`
}

// PYQChoiceExtraction represents an extracted choice within a question
type PYQChoiceExtraction struct {
	ChoiceLabel string `json:"choice_label"`
	ChoiceText  string `json:"choice_text"`
	Marks       int    `json:"marks,omitempty"`
}

// pyqExtractionSchema is the JSON schema for structured PYQ extraction
var pyqExtractionSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"year": map[string]any{
			"type":        "integer",
			"description": "The year of the examination (e.g., 2023, 2024)",
		},
		"month": map[string]any{
			"type":        "string",
			"description": "Month of examination if available (e.g., December, May, June)",
		},
		"exam_type": map[string]any{
			"type":        "string",
			"description": "Type of exam (e.g., End Semester, Mid Semester, Supplementary, Regular)",
		},
		"total_marks": map[string]any{
			"type":        "integer",
			"description": "Total marks for the paper",
		},
		"duration": map[string]any{
			"type":        "string",
			"description": "Duration of the exam (e.g., 3 hours, 2 hours 30 minutes)",
		},
		"instructions": map[string]any{
			"type":        "string",
			"description": "General instructions for the paper",
		},
		"questions": map[string]any{
			"type":        "array",
			"description": "List of questions in the paper",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question_number": map[string]any{
						"type":        "string",
						"description": "Question number (e.g., 1, 2, 3a, 3b)",
					},
					"section_name": map[string]any{
						"type":        "string",
						"description": "Section name if applicable (e.g., Section A, Part I)",
					},
					"question_text": map[string]any{
						"type":        "string",
						"description": "The full question text",
					},
					"marks": map[string]any{
						"type":        "integer",
						"description": "Marks allocated for this question",
					},
					"is_compulsory": map[string]any{
						"type":        "boolean",
						"description": "Whether the question is compulsory",
					},
					"has_choices": map[string]any{
						"type":        "boolean",
						"description": "Whether the question has OR choices (student picks one)",
					},
					"choice_group": map[string]any{
						"type":        "string",
						"description": "Group identifier for questions that are alternatives to each other",
					},
					"unit_number": map[string]any{
						"type":        "integer",
						"description": "Unit number this question belongs to (if identifiable)",
					},
					"topic_keywords": map[string]any{
						"type":        "string",
						"description": "Comma-separated keywords/topics covered by this question",
					},
					"choices": map[string]any{
						"type":        "array",
						"description": "Alternative choices within this question (for OR type questions)",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"choice_label": map[string]any{
									"type":        "string",
									"description": "Choice label (e.g., a, b, OR, i, ii)",
								},
								"choice_text": map[string]any{
									"type":        "string",
									"description": "The choice/alternative question text",
								},
								"marks": map[string]any{
									"type":        "integer",
									"description": "Marks for this specific choice (if different from parent)",
								},
							},
							"required": []string{"choice_label", "choice_text"},
						},
					},
				},
				"required": []string{"question_number", "question_text", "marks"},
			},
		},
	},
	"required": []string{"year", "total_marks", "questions"},
}

// pyqExtractionPrompt is the system prompt for LLM extraction
const pyqExtractionPrompt = `You are an expert at extracting structured information from academic examination question papers (Previous Year Questions / PYQs).

CRITICAL: You MUST respond with ONLY valid JSON. No markdown, no explanations, no code blocks, no text before or after the JSON. Start your response with { and end with }.

IMPORTANT - UNDERSTAND THE EXAM PAPER STRUCTURE:
Most university exam papers have this structure:
- Questions are numbered 1, 2, 3, 4, etc.
- Each question typically has SUB-PARTS labeled (a), (b), (c) - these are SEPARATE questions that students must answer
- Sometimes a question has "OR" - meaning student chooses ONE option from alternatives

EXTRACTION RULES:
1. SUB-PARTS (a), (b) = Extract as SEPARATE questions with question_number "1a", "1b", "2a", "2b", etc.
   - has_choices = false
   - Each sub-part is its own question entry
   
2. OR CHOICES = When you see "OR" between options, extract as ONE question with choices array
   - has_choices = true
   - Put the alternatives in the choices array
   - Example: "Write short notes on any three: (a) Topic1 OR (b) Topic2 OR (c) Topic3"

Your task is to analyze the provided question paper and extract information into this exact JSON structure:

{
  "year": 2024,
  "month": "December",
  "exam_type": "End Semester Examination",
  "total_marks": 70,
  "duration": "3 Hours",
  "instructions": "Attempt any five questions. All questions carry equal marks.",
  "questions": [
    {
      "question_number": "1a",
      "section_name": "Section A",
      "question_text": "Explain the KDD process with example.",
      "marks": 7,
      "is_compulsory": false,
      "has_choices": false,
      "unit_number": 1,
      "topic": "KDD Process",
      "keywords": ["KDD", "knowledge discovery"],
      "choices": []
    },
    {
      "question_number": "1b",
      "section_name": "Section A",
      "question_text": "What are the applications of data mining?",
      "marks": 7,
      "is_compulsory": false,
      "has_choices": false,
      "unit_number": 1,
      "topic": "Data Mining Applications",
      "keywords": ["applications", "data mining"],
      "choices": []
    },
    {
      "question_number": "8",
      "section_name": "",
      "question_text": "Write short notes on any three of the following:",
      "marks": 14,
      "is_compulsory": false,
      "has_choices": true,
      "unit_number": 0,
      "topic": "Short Notes",
      "keywords": ["short notes"],
      "choices": [
        {"choice_label": "a", "choice_text": "Constraint Based Association Mining"},
        {"choice_label": "b", "choice_text": "Outlier Analysis"},
        {"choice_label": "c", "choice_text": "Gini Index"},
        {"choice_label": "d", "choice_text": "Data Integration"},
        {"choice_label": "e", "choice_text": "OLAP, MOLAP, HOLAP"}
      ]
    }
  ]
}

Guidelines:
1. Extract year, month from the paper header (e.g., "December 2024" -> year: 2024, month: "December")
2. ALWAYS split sub-parts (a), (b) into separate question entries with numbers like "1a", "1b", "2a", "2b"
3. Only use has_choices=true and choices array when there's "OR" or "any X of the following"
4. Include section_name if paper has sections (Section A, Part I, etc.)
5. Try to identify unit_number and topic for each question
6. Add relevant keywords for searchability
7. Extract ALL questions - don't miss any sub-parts!

REMEMBER: Output ONLY the JSON object. No markdown formatting. No explanatory text. Start with { end with }.`

// ExtractPYQFromDocument extracts PYQ data from a document
// Uses OCR text if available (preferred), otherwise falls back to PDF text extraction
// Uses chunked parallel extraction for PDFs with more than 10 pages (when no OCR text)
func (s *PYQService) ExtractPYQFromDocument(ctx context.Context, documentID uint) (*model.PYQPaper, error) {
	if !s.enableAI {
		return nil, fmt.Errorf("AI extraction is not enabled - DO_INFERENCE_API_KEY not configured")
	}

	// 1. Get the document
	var document model.Document
	if err := s.db.Preload("Subject").First(&document, documentID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch document: %w", err)
	}

	// 2. Check if document is a PYQ type
	if document.Type != model.DocumentTypePYQ {
		return nil, fmt.Errorf("document is not a PYQ type")
	}

	// 3. Verify document has a subject (PYQs must be associated with a subject)
	if document.SubjectID == nil || *document.SubjectID == 0 {
		return nil, fmt.Errorf("PYQ document must be associated with a subject")
	}

	// 4. Check if there's an existing PYQ paper record (created by batch ingest)
	var paper *model.PYQPaper
	var existingPaper model.PYQPaper
	if err := s.db.Where("document_id = ?", documentID).First(&existingPaper).Error; err == nil {
		// Use existing paper record
		paper = &existingPaper
		paper.ExtractionStatus = model.PYQExtractionProcessing
		s.db.Save(paper)
		log.Printf("PYQService: Using existing PYQ paper record %d for document %d", paper.ID, documentID)
	} else {
		// Create new PYQ paper record with processing status
		paper = &model.PYQPaper{
			SubjectID:        *document.SubjectID,
			DocumentID:       documentID,
			ExtractionStatus: model.PYQExtractionProcessing,
		}
		if err := s.db.Create(paper).Error; err != nil {
			return nil, fmt.Errorf("failed to create PYQ paper record: %w", err)
		}
	}

	// 5. Check if we have OCR text available (preferred - works for scanned PDFs)
	var documentText string
	if document.OCRText != "" {
		log.Printf("PYQService: Using OCR text for document %d (%d chars)", documentID, len(document.OCRText))
		documentText = document.OCRText
	} else {
		// 6. No OCR text - fall back to PDF text extraction
		log.Printf("PYQService: No OCR text available, falling back to PDF extraction for document %d", documentID)

		if !s.enableSpaces || document.SpacesKey == "" || document.SpacesKey == "disabled" {
			s.updatePYQError(paper, "document storage not available")
			return paper, fmt.Errorf("document storage not available")
		}

		pdfContent, err := s.spacesClient.DownloadFile(ctx, document.SpacesKey)
		if err != nil {
			s.updatePYQError(paper, err.Error())
			return paper, fmt.Errorf("failed to download document: %w", err)
		}

		// Check page count to decide extraction method
		pageCount, err := s.pdfExtractor.GetPageCount(pdfContent)
		if err != nil {
			log.Printf("Warning: Could not get page count, using chunked extraction: %v", err)
			pageCount = 100 // Assume large PDF, use chunked
		}

		// Use chunked extraction for PDFs with more than 10 pages
		if pageCount > 10 && s.chunkedExtractor != nil {
			log.Printf("PYQService: Using chunked extraction for %d page PDF", pageCount)
			if err := s.chunkedExtractor.ExtractPYQChunked(ctx, paper, pdfContent); err != nil {
				s.updatePYQError(paper, err.Error())
				return paper, fmt.Errorf("chunked extraction failed: %w", err)
			}
			return paper, nil
		}

		// For smaller PDFs, use single-pass extraction
		log.Printf("PYQService: Using single-pass PDF extraction for %d page PDF", pageCount)
		documentText, err = s.pdfExtractor.ExtractText(pdfContent)
		if err != nil {
			s.updatePYQError(paper, err.Error())
			return paper, fmt.Errorf("failed to extract text: %w", err)
		}
	}

	// 7. Call LLM for structured question extraction
	log.Printf("PYQService: Calling LLM to extract questions from %d chars of text", len(documentText))
	extractedData, rawResponse, err := s.extractWithLLM(ctx, documentText)
	if err != nil {
		s.updatePYQError(paper, err.Error())
		return paper, fmt.Errorf("failed to extract PYQ data: %w", err)
	}

	// 8. Save extracted data to database
	if err := s.savePYQData(paper, extractedData, rawResponse); err != nil {
		s.updatePYQError(paper, err.Error())
		return paper, fmt.Errorf("failed to save PYQ data: %w", err)
	}

	return paper, nil
}

// getDocumentText retrieves the text content from a document
func (s *PYQService) getDocumentText(ctx context.Context, document *model.Document) (string, error) {
	if !s.enableSpaces || document.SpacesKey == "" || document.SpacesKey == "disabled" {
		return "", fmt.Errorf("document storage not available")
	}

	// Download the file
	fileContent, err := s.spacesClient.DownloadFile(ctx, document.SpacesKey)
	if err != nil {
		return "", fmt.Errorf("failed to download document: %w", err)
	}

	// Check file type
	if strings.HasSuffix(strings.ToLower(document.Filename), ".pdf") {
		return s.extractTextFromPDF(fileContent, document.Filename)
	}

	return string(fileContent), nil
}

// extractTextFromPDF extracts text from PDF content using ledongthuc/pdf
func (s *PYQService) extractTextFromPDF(content []byte, filename string) (string, error) {
	text, err := s.pdfExtractor.ExtractText(content)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from '%s': %w", filename, err)
	}
	return text, nil
}

// extractWithLLM calls the LLM to extract PYQ data using structured outputs
func (s *PYQService) extractWithLLM(ctx context.Context, documentText string) (*PYQExtractionResult, string, error) {
	// Truncate if too long
	maxChars := 50000
	if len(documentText) > maxChars {
		documentText = documentText[:maxChars] + "\n\n[Document truncated due to length]"
	}

	userPrompt := fmt.Sprintf("Extract the question paper information from the following document:\n\n%s", documentText)

	// Call LLM with structured output
	response, err := s.inferenceClient.StructuredCompletion(
		ctx,
		pyqExtractionPrompt,
		userPrompt,
		"pyq_extraction",
		"Structured extraction of previous year question paper with questions and choices",
		pyqExtractionSchema,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0), // Deterministic output
	)
	if err != nil {
		log.Printf("Structured output failed, falling back to JSONCompletion: %v", err)
		return s.extractWithLLMFallback(ctx, documentText)
	}

	var result PYQExtractionResult
	if err := utils.ExtractJSONTo(response, &result); err != nil {
		log.Printf("Failed to extract JSON from response (length=%d): %v", len(response), err)
		if len(response) > 500 {
			log.Printf("Response preview: %s...", response[:500])
		}
		return nil, response, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	return &result, response, nil
}

// extractWithLLMFallback uses traditional JSONCompletion as fallback
func (s *PYQService) extractWithLLMFallback(ctx context.Context, documentText string) (*PYQExtractionResult, string, error) {
	maxChars := 50000
	if len(documentText) > maxChars {
		documentText = documentText[:maxChars] + "\n\n[Document truncated due to length]"
	}

	fallbackPrompt := `You are a JSON extraction assistant. You MUST output ONLY valid JSON with no other text.

IMPORTANT STRUCTURE:
- Sub-parts (a), (b) = Extract as SEPARATE questions with numbers "1a", "1b", "2a", "2b"
- OR choices = ONE question with has_choices=true and choices array

Extract question paper information into this EXACT JSON structure:
{
  "year": 2024,
  "month": "December",
  "exam_type": "End Semester",
  "total_marks": 70,
  "duration": "3 Hours",
  "instructions": "General instructions here",
  "questions": [
    {
      "question_number": "1a",
      "section_name": "",
      "question_text": "First sub-part question text",
      "marks": 7,
      "is_compulsory": false,
      "has_choices": false,
      "unit_number": 1,
      "topic": "Topic name",
      "keywords": ["keyword1"],
      "choices": []
    },
    {
      "question_number": "1b",
      "section_name": "",
      "question_text": "Second sub-part question text",
      "marks": 7,
      "is_compulsory": false,
      "has_choices": false,
      "unit_number": 1,
      "topic": "Topic name",
      "keywords": ["keyword1"],
      "choices": []
    }
  ]
}

RULES:
1. Output ONLY JSON - no markdown, no code blocks, no explanations
2. Start response with { and end with }
3. Extract ALL questions - split (a), (b) sub-parts into separate entries like "1a", "1b"
4. Only use has_choices=true when there's "OR" between options, fill choices array with:
   {"choice_label": "a", "choice_text": "Choice text here"}`

	userPrompt := fmt.Sprintf("Extract as JSON:\n\n%s", documentText)

	response, err := s.inferenceClient.JSONCompletion(
		ctx,
		fallbackPrompt,
		userPrompt,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0), // Deterministic output
	)
	if err != nil {
		return nil, "", fmt.Errorf("LLM extraction failed: %w", err)
	}

	var result PYQExtractionResult
	if err := utils.ExtractJSONTo(response, &result); err != nil {
		log.Printf("Fallback: Failed to extract JSON from response (length=%d): %v", len(response), err)
		return nil, response, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	return &result, response, nil
}

// savePYQData saves the extracted PYQ data to the database
func (s *PYQService) savePYQData(paper *model.PYQPaper, data *PYQExtractionResult, rawResponse string) error {
	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update paper basic info
	paper.Year = data.Year
	paper.Month = data.Month
	paper.ExamType = data.ExamType
	paper.TotalMarks = data.TotalMarks
	paper.Duration = data.Duration
	paper.Instructions = data.Instructions
	paper.TotalQuestions = len(data.Questions)
	paper.RawExtraction = rawResponse
	paper.ExtractionStatus = model.PYQExtractionCompleted
	paper.ExtractionError = ""

	if err := tx.Save(paper).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update PYQ paper: %w", err)
	}

	// Create questions
	for _, qData := range data.Questions {
		question := model.PYQQuestion{
			PaperID:        paper.ID,
			QuestionNumber: qData.QuestionNumber,
			SectionName:    qData.SectionName,
			QuestionText:   qData.QuestionText,
			Marks:          qData.Marks,
			IsCompulsory:   qData.IsCompulsory,
			HasChoices:     qData.HasChoices,
			ChoiceGroup:    qData.ChoiceGroup,
			UnitNumber:     qData.UnitNumber,
			TopicKeywords:  qData.TopicKeywords,
		}

		if err := tx.Create(&question).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create question: %w", err)
		}

		// Create choices for this question
		for _, choiceData := range qData.Choices {
			choice := model.PYQQuestionChoice{
				QuestionID:  question.ID,
				ChoiceLabel: choiceData.ChoiceLabel,
				ChoiceText:  choiceData.ChoiceText,
				Marks:       choiceData.Marks,
			}

			if err := tx.Create(&choice).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create question choice: %w", err)
			}
		}
	}

	return tx.Commit().Error
}

// updatePYQError updates PYQ paper with error status
func (s *PYQService) updatePYQError(paper *model.PYQPaper, errMsg string) {
	paper.ExtractionStatus = model.PYQExtractionFailed
	paper.ExtractionError = errMsg
	s.db.Save(paper)
}

// GetPYQsBySubject retrieves all PYQ papers for a subject
func (s *PYQService) GetPYQsBySubject(ctx context.Context, subjectID uint) ([]model.PYQPaper, error) {
	var papers []model.PYQPaper
	err := s.db.Where("subject_id = ?", subjectID).
		Order("year DESC, month DESC").
		Find(&papers).Error
	return papers, err
}

// GetPYQByID retrieves a PYQ paper by ID with all questions and choices
func (s *PYQService) GetPYQByID(ctx context.Context, paperID uint) (*model.PYQPaper, error) {
	var paper model.PYQPaper
	err := s.db.Preload("Questions", func(db *gorm.DB) *gorm.DB {
		return db.Order("question_number ASC")
	}).Preload("Questions.Choices").First(&paper, paperID).Error

	if err != nil {
		return nil, err
	}

	return &paper, nil
}

// TriggerExtractionAsync triggers PYQ extraction in a goroutine
func (s *PYQService) TriggerExtractionAsync(documentID uint) {
	go func() {
		ctx := context.Background()
		_, err := s.ExtractPYQFromDocument(ctx, documentID)
		if err != nil {
			log.Printf("Background PYQ extraction failed for document %d: %v", documentID, err)
		} else {
			log.Printf("Background PYQ extraction completed for document %d", documentID)
		}
	}()
}

// RetryExtraction retries failed extraction for a PYQ paper
func (s *PYQService) RetryExtraction(ctx context.Context, paperID uint) (*model.PYQPaper, error) {
	var paper model.PYQPaper
	if err := s.db.First(&paper, paperID).Error; err != nil {
		return nil, fmt.Errorf("PYQ paper not found: %w", err)
	}

	if paper.ExtractionStatus != model.PYQExtractionFailed {
		return nil, fmt.Errorf("can only retry failed extractions")
	}

	// Delete existing questions and choices
	s.db.Where("paper_id = ?", paper.ID).Delete(&model.PYQQuestion{})

	return s.ExtractPYQFromDocument(ctx, paper.DocumentID)
}

// DeletePYQ deletes a PYQ paper and all related data
func (s *PYQService) DeletePYQ(ctx context.Context, paperID uint) error {
	return s.db.Delete(&model.PYQPaper{}, paperID).Error
}

// GetExtractionStatus returns the current extraction status
func (s *PYQService) GetExtractionStatus(ctx context.Context, paperID uint) (model.PYQExtractionStatus, string, error) {
	var paper model.PYQPaper
	if err := s.db.Select("extraction_status", "extraction_error").First(&paper, paperID).Error; err != nil {
		return "", "", err
	}
	return paper.ExtractionStatus, paper.ExtractionError, nil
}

// SearchQuestions searches questions by keywords
func (s *PYQService) SearchQuestions(ctx context.Context, subjectID uint, query string) ([]model.PYQQuestion, error) {
	var questions []model.PYQQuestion

	err := s.db.Joins("JOIN pyq_papers ON pyq_papers.id = pyq_questions.paper_id").
		Where("pyq_papers.subject_id = ?", subjectID).
		Where("pyq_questions.question_text ILIKE ? OR pyq_questions.topic_keywords ILIKE ?",
			"%"+query+"%", "%"+query+"%").
		Order("pyq_papers.year DESC").
		Limit(50).
		Find(&questions).Error

	return questions, err
}
