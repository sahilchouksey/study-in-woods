package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"github.com/sahilchouksey/go-init-setup/utils"
	"gorm.io/gorm"
)

// ChunkedPYQExtractor handles parallel chunked PDF PYQ extraction
type ChunkedPYQExtractor struct {
	db              *gorm.DB
	inferenceClient *digitalocean.InferenceClient
	spacesClient    *digitalocean.SpacesClient
	pdfExtractor    *PDFExtractor

	// Configuration
	maxConcurrent int           // Max parallel LLM calls (default: 5)
	maxRetries    int           // Max retries per chunk (default: 5)
	pagesPerChunk int           // Pages per chunk (default: 10)
	overlapPages  int           // Overlap between chunks (default: 1)
	chunkTimeout  time.Duration // Timeout per chunk (default: 4 min)
	mergeTimeout  time.Duration // Timeout for merge call (default: 3 min)
}

// ChunkedPYQExtractorConfig holds configuration for the chunked PYQ extractor
type ChunkedPYQExtractorConfig struct {
	MaxConcurrent int
	MaxRetries    int
	PagesPerChunk int
	OverlapPages  int
	ChunkTimeout  time.Duration
	MergeTimeout  time.Duration
}

// DefaultChunkedPYQExtractorConfig returns default configuration
func DefaultChunkedPYQExtractorConfig() ChunkedPYQExtractorConfig {
	return ChunkedPYQExtractorConfig{
		MaxConcurrent: 5, // Process 5 chunks in parallel (batch of 5)
		MaxRetries:    5,
		PagesPerChunk: 10, // 10 PDF pages per chunk
		OverlapPages:  1,  // 1 page overlap to avoid missing questions at boundaries
		ChunkTimeout:  4 * time.Minute,
		MergeTimeout:  3 * time.Minute,
	}
}

// PYQChunkResult holds the extraction result for a single chunk
type PYQChunkResult struct {
	ChunkIndex  int
	PageRange   PageRange
	Questions   []PYQQuestionExtraction
	PaperInfo   *PYQPaperInfoExtraction
	RawResponse string
	Error       error
	Retries     int
}

// PYQPaperInfoExtraction holds paper-level info extracted from chunks
type PYQPaperInfoExtraction struct {
	Year         int    `json:"year"`
	Month        string `json:"month"`
	ExamType     string `json:"exam_type"`
	TotalMarks   int    `json:"total_marks"`
	Duration     string `json:"duration"`
	Instructions string `json:"instructions"`
}

// NewChunkedPYQExtractor creates a new chunked PYQ extractor
func NewChunkedPYQExtractor(
	db *gorm.DB,
	inferenceClient *digitalocean.InferenceClient,
	spacesClient *digitalocean.SpacesClient,
	pdfExtractor *PDFExtractor,
	config ChunkedPYQExtractorConfig,
) *ChunkedPYQExtractor {
	// Apply defaults for zero values
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 5
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 5
	}
	if config.PagesPerChunk <= 0 {
		config.PagesPerChunk = 10
	}
	if config.OverlapPages < 0 {
		config.OverlapPages = 1
	}
	if config.ChunkTimeout <= 0 {
		config.ChunkTimeout = 4 * time.Minute
	}
	if config.MergeTimeout <= 0 {
		config.MergeTimeout = 3 * time.Minute
	}

	return &ChunkedPYQExtractor{
		db:              db,
		inferenceClient: inferenceClient,
		spacesClient:    spacesClient,
		pdfExtractor:    pdfExtractor,
		maxConcurrent:   config.MaxConcurrent,
		maxRetries:      config.MaxRetries,
		pagesPerChunk:   config.PagesPerChunk,
		overlapPages:    config.OverlapPages,
		chunkTimeout:    config.ChunkTimeout,
		mergeTimeout:    config.MergeTimeout,
	}
}

// ExtractPYQChunked performs parallel chunked extraction from a PYQ document
func (c *ChunkedPYQExtractor) ExtractPYQChunked(
	ctx context.Context,
	paper *model.PYQPaper,
	pdfContent []byte,
) error {
	startTime := time.Now()

	// 1. Get page count
	pageCount, err := c.pdfExtractor.GetPageCount(pdfContent)
	if err != nil {
		return fmt.Errorf("failed to get page count: %w", err)
	}

	log.Printf("ChunkedPYQExtractor: PDF has %d pages, starting chunked extraction", pageCount)

	// 2. Calculate chunks
	chunkConfig := ChunkConfig{
		PagesPerChunk: c.pagesPerChunk,
		OverlapPages:  c.overlapPages,
	}
	chunks := c.pdfExtractor.CalculateChunks(pageCount, chunkConfig)

	if len(chunks) == 0 {
		return fmt.Errorf("no chunks calculated for PDF with %d pages", pageCount)
	}

	log.Printf("ChunkedPYQExtractor: Processing %d chunks in parallel (max %d concurrent)", len(chunks), c.maxConcurrent)

	// 3. Process chunks in parallel
	chunkResults := c.processChunksParallel(ctx, pdfContent, chunks, pageCount)

	// 4. Check failure rate
	failedChunks := 0
	for _, result := range chunkResults {
		if result.Error != nil {
			failedChunks++
			log.Printf("ChunkedPYQExtractor: Chunk %d (pages %d-%d) failed: %v",
				result.ChunkIndex, result.PageRange.Start, result.PageRange.End, result.Error)
		}
	}

	failureRate := float64(failedChunks) / float64(len(chunks)) * 100
	log.Printf("ChunkedPYQExtractor: %d/%d chunks failed (%.1f%%)", failedChunks, len(chunks), failureRate)

	if failedChunks == len(chunks) {
		return fmt.Errorf("all chunks failed to extract. Please re-upload the file")
	}

	if failureRate > 50 {
		return fmt.Errorf("extraction failed for too many chunks (%.1f%%). Please re-upload the file", failureRate)
	}

	// 5. Merge and deduplicate results
	mergedResult, err := c.mergeAndDeduplicate(ctx, chunkResults)
	if err != nil {
		return fmt.Errorf("failed to merge chunk results: %w", err)
	}

	if len(mergedResult.Questions) == 0 {
		return fmt.Errorf("no questions extracted from document")
	}

	log.Printf("ChunkedPYQExtractor: Merged %d questions from chunks", len(mergedResult.Questions))

	// 6. Save to database
	if err := c.savePYQData(paper, mergedResult); err != nil {
		return fmt.Errorf("failed to save PYQ data: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Printf("ChunkedPYQExtractor: Extraction completed in %.2fs - %d questions extracted", elapsed.Seconds(), len(mergedResult.Questions))

	return nil
}

// processChunksParallel processes PDF chunks in parallel with concurrency control
func (c *ChunkedPYQExtractor) processChunksParallel(
	ctx context.Context,
	pdfContent []byte,
	chunks []PageRange,
	totalPages int,
) []PYQChunkResult {
	results := make([]PYQChunkResult, len(chunks))
	var wg sync.WaitGroup

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, c.maxConcurrent)

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, pageRange PageRange) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Check if context is cancelled
			if ctx.Err() != nil {
				results[idx] = PYQChunkResult{
					ChunkIndex: idx,
					PageRange:  pageRange,
					Error:      ctx.Err(),
				}
				return
			}

			log.Printf("ChunkedPYQExtractor: Processing chunk %d/%d (pages %d-%d)",
				idx+1, len(chunks), pageRange.Start, pageRange.End)

			// Extract with retry
			results[idx] = c.extractChunkWithRetry(ctx, pdfContent, pageRange, totalPages, idx)

			if results[idx].Error == nil {
				log.Printf("ChunkedPYQExtractor: Chunk %d completed - %d questions found",
					idx+1, len(results[idx].Questions))
			}
		}(i, chunk)
	}

	wg.Wait()
	return results
}

// extractChunkWithRetry extracts a chunk with retry logic
func (c *ChunkedPYQExtractor) extractChunkWithRetry(
	ctx context.Context,
	pdfContent []byte,
	pageRange PageRange,
	totalPages int,
	chunkIndex int,
) PYQChunkResult {
	var result PYQChunkResult
	result.ChunkIndex = chunkIndex
	result.PageRange = pageRange

	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		result.Retries = attempt

		// Create timeout context for this chunk
		chunkCtx, cancel := context.WithTimeout(ctx, c.chunkTimeout)

		// Try extraction
		questions, paperInfo, rawResponse, err := c.extractChunk(chunkCtx, pdfContent, pageRange, totalPages)
		cancel()

		if err == nil {
			result.Questions = questions
			result.PaperInfo = paperInfo
			result.RawResponse = rawResponse
			result.Error = nil
			return result
		}

		result.Error = err

		// Check if parent context is done
		if ctx.Err() != nil {
			result.Error = ctx.Err()
			return result
		}

		// Exponential backoff: 1s, 2s, 4s, 8s, 16s
		if attempt < c.maxRetries {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			log.Printf("ChunkedPYQExtractor: Chunk %d attempt %d failed, retrying in %v: %v",
				chunkIndex+1, attempt, backoff, err)

			select {
			case <-ctx.Done():
				result.Error = ctx.Err()
				return result
			case <-time.After(backoff):
				// Continue to next retry
			}
		}
	}

	return result
}

// extractChunk extracts PYQ data from a single chunk
func (c *ChunkedPYQExtractor) extractChunk(
	ctx context.Context,
	pdfContent []byte,
	pageRange PageRange,
	totalPages int,
) ([]PYQQuestionExtraction, *PYQPaperInfoExtraction, string, error) {
	// Extract text for this page range
	text, err := c.pdfExtractor.ExtractPageRange(pdfContent, pageRange.Start, pageRange.End)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to extract text from pages %d-%d: %w", pageRange.Start, pageRange.End, err)
	}

	if len(text) < 50 {
		return nil, nil, "", fmt.Errorf("insufficient text extracted from pages %d-%d", pageRange.Start, pageRange.End)
	}

	// Build prompt for PYQ extraction
	systemPrompt := `You are an expert at extracting structured information from academic examination question papers.

Extract ALL questions from the provided text. For each question, identify:
1. Question number (preserve original numbering: 1, 2, 3a, 3b, etc.)
2. Section name if applicable (Section A, Part I, etc.)
3. Full question text
4. Marks allocated
5. Whether it's compulsory
6. If it has OR choices (alternatives where student picks one)
7. Unit number if identifiable
8. Topic keywords for searchability

IMPORTANT RULES:
- Extract EVERY question including sub-parts (a, b, c, i, ii, etc.)
- For OR questions, set has_choices=true and include alternatives in choices array
- Preserve exact question text - don't summarize
- If marks aren't specified, estimate based on context

Also extract paper info if visible (year, month, exam type, total marks, duration, instructions).

Output ONLY valid JSON:
{
  "paper_info": {
    "year": 2024,
    "month": "December",
    "exam_type": "End Semester",
    "total_marks": 70,
    "duration": "3 hours",
    "instructions": "..."
  },
  "questions": [
    {
      "question_number": "1",
      "section_name": "Section A",
      "question_text": "...",
      "marks": 10,
      "is_compulsory": true,
      "has_choices": false,
      "unit_number": 1,
      "topic_keywords": "keyword1, keyword2",
      "choices": []
    }
  ]
}`

	userPrompt := fmt.Sprintf(`Pages %d-%d of %d. Extract ALL questions from this examination paper text.

Text to parse:
%s`, pageRange.Start, pageRange.End, totalPages, text)

	// Call LLM with 8192 max_tokens for larger chunk processing
	response, err := c.inferenceClient.SimpleCompletion(
		ctx,
		systemPrompt,
		userPrompt,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0), // Deterministic output
	)
	if err != nil {
		return nil, nil, "", fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse response
	var result struct {
		PaperInfo *PYQPaperInfoExtraction `json:"paper_info"`
		Questions []PYQQuestionExtraction `json:"questions"`
	}
	if err := utils.ExtractJSONTo(response, &result); err != nil {
		log.Printf("ChunkedPYQExtractor: Failed to parse chunk response (length=%d): %v", len(response), err)
		return nil, nil, response, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return result.Questions, result.PaperInfo, response, nil
}

// mergeAndDeduplicate merges chunk results and removes duplicate questions
func (c *ChunkedPYQExtractor) mergeAndDeduplicate(
	ctx context.Context,
	chunkResults []PYQChunkResult,
) (*PYQExtractionResult, error) {
	// Collect all questions and paper info from successful chunks
	var allQuestions []PYQQuestionExtraction
	var paperInfo *PYQPaperInfoExtraction

	for _, result := range chunkResults {
		if result.Error == nil {
			allQuestions = append(allQuestions, result.Questions...)
			// Take paper info from first chunk that has it
			if paperInfo == nil && result.PaperInfo != nil && result.PaperInfo.Year > 0 {
				paperInfo = result.PaperInfo
			}
		}
	}

	if len(allQuestions) == 0 {
		return nil, fmt.Errorf("no questions extracted from any chunk")
	}

	log.Printf("ChunkedPYQExtractor: Merging %d questions from chunks", len(allQuestions))

	// Deduplicate questions by question_number
	deduped := c.deduplicateQuestions(allQuestions)

	log.Printf("ChunkedPYQExtractor: After deduplication: %d unique questions", len(deduped))

	// Build final result
	result := &PYQExtractionResult{
		Questions: deduped,
	}

	if paperInfo != nil {
		result.Year = paperInfo.Year
		result.Month = paperInfo.Month
		result.ExamType = paperInfo.ExamType
		result.TotalMarks = paperInfo.TotalMarks
		result.Duration = paperInfo.Duration
		result.Instructions = paperInfo.Instructions
	}

	return result, nil
}

// deduplicateQuestions removes duplicate questions by question_number
func (c *ChunkedPYQExtractor) deduplicateQuestions(questions []PYQQuestionExtraction) []PYQQuestionExtraction {
	seen := make(map[string]PYQQuestionExtraction)

	for _, q := range questions {
		key := q.QuestionNumber
		if q.SectionName != "" {
			key = q.SectionName + "_" + key
		}

		// Keep the one with more content (longer question text)
		if existing, ok := seen[key]; ok {
			if len(q.QuestionText) > len(existing.QuestionText) {
				seen[key] = q
			}
		} else {
			seen[key] = q
		}
	}

	// Convert map to slice and sort by question number
	result := make([]PYQQuestionExtraction, 0, len(seen))
	for _, q := range seen {
		result = append(result, q)
	}

	return result
}

// savePYQData saves the extracted PYQ data to the database
func (c *ChunkedPYQExtractor) savePYQData(paper *model.PYQPaper, data *PYQExtractionResult) error {
	tx := c.db.Begin()
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
