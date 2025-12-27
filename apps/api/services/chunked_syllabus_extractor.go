package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
	"github.com/sahilchouksey/go-init-setup/utils"
	"gorm.io/gorm"
)

// ChunkedSyllabusExtractor handles parallel chunked PDF syllabus extraction
type ChunkedSyllabusExtractor struct {
	db              *gorm.DB
	inferenceClient *digitalocean.InferenceClient
	spacesClient    *digitalocean.SpacesClient
	pdfExtractor    *PDFExtractor
	subjectService  *SubjectService

	// Configuration
	maxConcurrent int           // Max parallel LLM calls (default: 5)
	maxRetries    int           // Max retries per chunk (default: 5)
	pagesPerChunk int           // Pages per chunk (default: 4)
	overlapPages  int           // Overlap between chunks (default: 1)
	chunkTimeout  time.Duration // Timeout per chunk (default: 2 min)
	mergeTimeout  time.Duration // Timeout for merge call (default: 3 min)
}

// ChunkedExtractorConfig holds configuration for the chunked extractor
type ChunkedExtractorConfig struct {
	MaxConcurrent int
	MaxRetries    int
	PagesPerChunk int
	OverlapPages  int
	ChunkTimeout  time.Duration
	MergeTimeout  time.Duration
}

// DefaultChunkedExtractorConfig returns default configuration
func DefaultChunkedExtractorConfig() ChunkedExtractorConfig {
	return ChunkedExtractorConfig{
		MaxConcurrent: 100,              // Process ALL chunks in parallel - LLM API does heavy lifting
		MaxRetries:    2,                // 2 retries for faster failure
		PagesPerChunk: 1,                // 1 PDF page per chunk for maximum reliability
		OverlapPages:  0,                // No overlap to maximize speed
		ChunkTimeout:  90 * time.Second, // 90 second timeout per chunk (smaller chunks = faster)
		MergeTimeout:  60 * time.Second, // 60 second timeout for merge
	}
}

// ChunkResult holds the extraction result for a single chunk
type ChunkResult struct {
	ChunkIndex  int
	PageRange   PageRange
	Subjects    []SubjectExtractionResult
	RawResponse string
	Error       error
	Retries     int
}

// getSemesterIDFromDocument extracts the semester ID from a document
// For semester-based documents (syllabus PDFs), it uses document.SemesterID directly
// For subject-based documents, it gets the semester from the subject relationship
func (c *ChunkedSyllabusExtractor) getSemesterIDFromDocument(document *model.Document) (uint, error) {
	// First, check if document has a direct semester reference (new approach)
	if document.SemesterID != nil && *document.SemesterID > 0 {
		return *document.SemesterID, nil
	}

	// Fall back to getting semester from subject (legacy approach)
	if document.Subject != nil && document.Subject.SemesterID > 0 {
		return document.Subject.SemesterID, nil
	}

	// Try to load subject if not preloaded
	if document.SubjectID != nil && *document.SubjectID > 0 {
		var subject model.Subject
		if err := c.db.First(&subject, *document.SubjectID).Error; err != nil {
			return 0, fmt.Errorf("failed to get subject for document: %w", err)
		}
		return subject.SemesterID, nil
	}

	return 0, fmt.Errorf("document has no semester or subject reference")
}

// NewChunkedSyllabusExtractor creates a new chunked syllabus extractor
func NewChunkedSyllabusExtractor(
	db *gorm.DB,
	inferenceClient *digitalocean.InferenceClient,
	spacesClient *digitalocean.SpacesClient,
	pdfExtractor *PDFExtractor,
	config ChunkedExtractorConfig,
) *ChunkedSyllabusExtractor {
	// Apply defaults for zero values
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 100 // Process ALL chunks in parallel - LLM API does heavy lifting
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.PagesPerChunk <= 0 {
		config.PagesPerChunk = 3 // Default to 3 pages per chunk for speed
	}
	if config.OverlapPages < 0 {
		config.OverlapPages = 0
	}
	if config.ChunkTimeout <= 0 {
		config.ChunkTimeout = 90 * time.Second
	}
	if config.MergeTimeout <= 0 {
		config.MergeTimeout = 60 * time.Second
	}

	return &ChunkedSyllabusExtractor{
		db:              db,
		inferenceClient: inferenceClient,
		spacesClient:    spacesClient,
		pdfExtractor:    pdfExtractor,
		subjectService:  NewSubjectService(db),
		maxConcurrent:   config.MaxConcurrent,
		maxRetries:      config.MaxRetries,
		pagesPerChunk:   config.PagesPerChunk,
		overlapPages:    config.OverlapPages,
		chunkTimeout:    config.ChunkTimeout,
		mergeTimeout:    config.MergeTimeout,
	}
}

// ExtractSyllabusChunked performs parallel chunked extraction from a document
func (c *ChunkedSyllabusExtractor) ExtractSyllabusChunked(
	ctx context.Context,
	document *model.Document,
	pdfContent []byte,
) ([]*model.Syllabus, error) {
	// 1. Get page count
	pageCount, err := c.pdfExtractor.GetPageCount(pdfContent)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	log.Printf("ChunkedExtractor: PDF has %d pages, starting chunked extraction", pageCount)

	// 2. Calculate chunks
	chunkConfig := ChunkConfig{
		PagesPerChunk: c.pagesPerChunk,
		OverlapPages:  c.overlapPages,
	}
	chunks := c.pdfExtractor.CalculateChunks(pageCount, chunkConfig)

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks calculated for PDF with %d pages", pageCount)
	}

	log.Printf("ChunkedExtractor: Processing %d chunks in parallel (max %d concurrent)", len(chunks), c.maxConcurrent)

	// 3. Process chunks in parallel
	chunkResults := c.processChunksParallel(ctx, pdfContent, chunks, pageCount)

	// 4. Check failure rate
	failedChunks := 0
	for _, result := range chunkResults {
		if result.Error != nil {
			failedChunks++
			log.Printf("ChunkedExtractor: Chunk %d (pages %d-%d) failed: %v",
				result.ChunkIndex, result.PageRange.Start, result.PageRange.End, result.Error)
		}
	}

	failureRate := float64(failedChunks) / float64(len(chunks)) * 100
	log.Printf("ChunkedExtractor: %d/%d chunks failed (%.1f%%)", failedChunks, len(chunks), failureRate)

	if failedChunks == len(chunks) {
		return nil, fmt.Errorf("all chunks failed to extract. Please re-upload the file")
	}

	if failureRate > 50 {
		return nil, fmt.Errorf("extraction failed for too many chunks (%.1f%%). Please re-upload the file", failureRate)
	}

	// 5. Merge and deduplicate results
	mergedResult, err := c.mergeAndDeduplicate(ctx, chunkResults)
	if err != nil {
		return nil, fmt.Errorf("failed to merge chunk results: %w", err)
	}

	if len(mergedResult.Subjects) == 0 {
		return nil, fmt.Errorf("no subjects extracted from document")
	}

	log.Printf("ChunkedExtractor: Merged %d subjects from chunks", len(mergedResult.Subjects))

	// 6. Save to database
	syllabuses, err := c.saveMultiSubjectSyllabusData(ctx, document, mergedResult)
	if err != nil {
		return nil, fmt.Errorf("failed to save syllabus data: %w", err)
	}

	return syllabuses, nil
}

// ExtractSyllabusChunkedWithProgress performs parallel chunked extraction with progress callbacks
func (c *ChunkedSyllabusExtractor) ExtractSyllabusChunkedWithProgress(
	ctx context.Context,
	document *model.Document,
	pdfContent []byte,
	progressCallback ProgressCallback,
) ([]*model.Syllabus, error) {
	startTime := time.Now()

	// 1. Get page count
	pageCount, err := c.pdfExtractor.GetPageCount(pdfContent)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	log.Printf("ChunkedExtractor: PDF has %d pages, starting chunked extraction with progress", pageCount)

	// 2. Calculate chunks
	chunkConfig := ChunkConfig{
		PagesPerChunk: c.pagesPerChunk,
		OverlapPages:  c.overlapPages,
	}
	chunks := c.pdfExtractor.CalculateChunks(pageCount, chunkConfig)

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks calculated for PDF with %d pages", pageCount)
	}

	// Emit extraction started event
	if err := progressCallback(ProgressEvent{
		Type:        "progress",
		Progress:    25,
		Phase:       "extraction",
		Message:     fmt.Sprintf("Starting chunked extraction (%d chunks)", len(chunks)),
		TotalChunks: len(chunks),
		Timestamp:   time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	log.Printf("ChunkedExtractor: Processing %d chunks in parallel (max %d concurrent)", len(chunks), c.maxConcurrent)

	// 3. Process chunks in parallel with progress updates
	chunkResults := c.processChunksParallelWithProgress(ctx, pdfContent, chunks, pageCount, progressCallback)

	// 4. Check failure rate
	failedChunks := 0
	successfulSubjects := 0
	for _, result := range chunkResults {
		if result.Error != nil {
			failedChunks++
		} else {
			successfulSubjects += len(result.Subjects)
		}
	}

	failureRate := float64(failedChunks) / float64(len(chunks)) * 100
	log.Printf("ChunkedExtractor: %d/%d chunks failed (%.1f%%)", failedChunks, len(chunks), failureRate)

	if failedChunks == len(chunks) {
		return nil, fmt.Errorf("all chunks failed to extract. Please re-upload the file")
	}

	if failureRate > 50 {
		return nil, fmt.Errorf("extraction failed for too many chunks (%.1f%%). Please re-upload the file", failureRate)
	}

	// Emit merge phase started
	if err := progressCallback(ProgressEvent{
		Type:            "progress",
		Progress:        75,
		Phase:           "merging",
		Message:         fmt.Sprintf("Merging %d subjects from %d chunks", successfulSubjects, len(chunks)-failedChunks),
		CompletedChunks: len(chunks) - failedChunks,
		TotalChunks:     len(chunks),
		Timestamp:       time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	// 5. Merge and deduplicate results
	mergedResult, err := c.mergeAndDeduplicate(ctx, chunkResults)
	if err != nil {
		return nil, fmt.Errorf("failed to merge chunk results: %w", err)
	}

	if len(mergedResult.Subjects) == 0 {
		return nil, fmt.Errorf("no subjects extracted from document")
	}

	log.Printf("ChunkedExtractor: Merged %d subjects from chunks", len(mergedResult.Subjects))

	// Emit save phase started
	if err := progressCallback(ProgressEvent{
		Type:          "progress",
		Progress:      85,
		Phase:         "saving",
		Message:       fmt.Sprintf("Saving %d subjects to database", len(mergedResult.Subjects)),
		SubjectsFound: len(mergedResult.Subjects),
		Timestamp:     time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("client disconnected: %w", err)
	}

	// 6. Save to database
	syllabuses, err := c.saveMultiSubjectSyllabusData(ctx, document, mergedResult)
	if err != nil {
		return nil, fmt.Errorf("failed to save syllabus data: %w", err)
	}

	// Emit completion within chunked extractor
	elapsed := time.Since(startTime).Milliseconds()
	if err := progressCallback(ProgressEvent{
		Type:          "progress",
		Progress:      95,
		Phase:         "finalizing",
		Message:       fmt.Sprintf("Extracted %d subjects successfully", len(syllabuses)),
		SubjectsFound: len(syllabuses),
		ElapsedMs:     elapsed,
		Timestamp:     time.Now(),
	}); err != nil {
		log.Printf("Warning: Failed to emit progress event: %v", err)
	}

	return syllabuses, nil
}

// processChunksParallelWithProgress processes chunks with progress callbacks
func (c *ChunkedSyllabusExtractor) processChunksParallelWithProgress(
	ctx context.Context,
	pdfContent []byte,
	chunks []PageRange,
	totalPages int,
	progressCallback ProgressCallback,
) []ChunkResult {
	results := make([]ChunkResult, len(chunks))
	var wg sync.WaitGroup
	var completedMu sync.Mutex
	completedCount := 0

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
				results[idx] = ChunkResult{
					ChunkIndex: idx,
					PageRange:  pageRange,
					Error:      ctx.Err(),
				}
				return
			}

			log.Printf("ChunkedExtractor: Processing chunk %d/%d (pages %d-%d)",
				idx+1, len(chunks), pageRange.Start, pageRange.End)

			// Emit chunk start event (ignore errors for info events)
			_ = progressCallback(ProgressEvent{
				Type:         "info",
				Phase:        "extraction",
				Message:      fmt.Sprintf("Processing pages %d-%d", pageRange.Start, pageRange.End),
				CurrentChunk: idx + 1,
				TotalChunks:  len(chunks),
				PageRange:    fmt.Sprintf("pages %d-%d", pageRange.Start, pageRange.End),
				Timestamp:    time.Now(),
			})

			// Extract with retry
			results[idx] = c.extractChunkWithRetry(ctx, pdfContent, pageRange, totalPages, idx)

			// Update completed count safely
			completedMu.Lock()
			completedCount++
			completed := completedCount
			completedMu.Unlock()

			progress := 25 + (completed * 50 / len(chunks)) // Progress from 25% to 75%

			if results[idx].Error == nil {
				_ = progressCallback(ProgressEvent{
					Type:            "progress",
					Progress:        progress,
					Phase:           "extraction",
					Message:         fmt.Sprintf("Completed chunk %d/%d", completed, len(chunks)),
					CompletedChunks: completed,
					TotalChunks:     len(chunks),
					CurrentChunk:    idx + 1,
					SubjectsFound:   len(results[idx].Subjects),
					Timestamp:       time.Now(),
				})
				log.Printf("ChunkedExtractor: Chunk %d completed successfully", idx+1)
			} else {
				_ = progressCallback(ProgressEvent{
					Type:            "warning",
					Progress:        progress,
					Phase:           "extraction",
					Message:         fmt.Sprintf("Chunk %d failed: %v", idx+1, results[idx].Error),
					CompletedChunks: completed,
					TotalChunks:     len(chunks),
					CurrentChunk:    idx + 1,
					ErrorMessage:    results[idx].Error.Error(),
					Recoverable:     true,
					Timestamp:       time.Now(),
				})
			}
		}(i, chunk)
	}

	wg.Wait()
	return results
}

// extractTopicsFromRawText intelligently extracts individual topics from raw_text
// Splits on common separators: comma, dash (–), semicolon, newline
func extractTopicsFromRawText(rawText string) []string {
	if rawText == "" {
		return []string{}
	}

	// Replace common separators with a delimiter
	text := rawText
	text = strings.ReplaceAll(text, " – ", "|") // em-dash
	text = strings.ReplaceAll(text, " - ", "|") // regular dash
	text = strings.ReplaceAll(text, "; ", "|")  // semicolon
	text = strings.ReplaceAll(text, ", ", "|")  // comma
	text = strings.ReplaceAll(text, "\n", "|")  // newline

	// Split by delimiter
	parts := strings.Split(text, "|")

	// Clean and filter topics
	var topics []string
	seenTopics := make(map[string]bool)

	for _, part := range parts {
		// Trim whitespace
		topic := strings.TrimSpace(part)

		// Skip empty, very short, or already seen topics
		if topic == "" || len(topic) < 3 {
			continue
		}

		// Normalize for deduplication (lowercase)
		normalized := strings.ToLower(topic)
		if seenTopics[normalized] {
			continue
		}

		seenTopics[normalized] = true
		topics = append(topics, topic)
	}

	// If no topics found, return the raw text as a single topic
	if len(topics) == 0 {
		return []string{rawText}
	}

	return topics
}

// processChunksParallel processes PDF chunks in parallel with concurrency control
func (c *ChunkedSyllabusExtractor) processChunksParallel(
	ctx context.Context,
	pdfContent []byte,
	chunks []PageRange,
	totalPages int,
) []ChunkResult {
	results := make([]ChunkResult, len(chunks))
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
				results[idx] = ChunkResult{
					ChunkIndex: idx,
					PageRange:  pageRange,
					Error:      ctx.Err(),
				}
				return
			}

			log.Printf("ChunkedExtractor: Processing chunk %d/%d (pages %d-%d)",
				idx+1, len(chunks), pageRange.Start, pageRange.End)

			// Extract with retry
			results[idx] = c.extractChunkWithRetry(ctx, pdfContent, pageRange, totalPages, idx)

			if results[idx].Error == nil {
				log.Printf("ChunkedExtractor: Chunk %d completed successfully", idx+1)
			}
		}(i, chunk)
	}

	wg.Wait()
	return results
}

// extractChunkWithRetry extracts a chunk with retry logic
func (c *ChunkedSyllabusExtractor) extractChunkWithRetry(
	ctx context.Context,
	pdfContent []byte,
	pageRange PageRange,
	totalPages int,
	chunkIndex int,
) ChunkResult {
	var result ChunkResult
	result.ChunkIndex = chunkIndex
	result.PageRange = pageRange

	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		result.Retries = attempt

		// Create timeout context for this chunk
		chunkCtx, cancel := context.WithTimeout(ctx, c.chunkTimeout)

		// Try extraction
		subjects, rawResponse, err := c.extractChunk(chunkCtx, pdfContent, pageRange, totalPages)
		cancel()

		if err == nil {
			result.Subjects = subjects
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
			log.Printf("ChunkedExtractor: Chunk %d attempt %d failed, retrying in %v: %v",
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

// extractChunk extracts syllabus data from a single chunk
func (c *ChunkedSyllabusExtractor) extractChunk(
	ctx context.Context,
	pdfContent []byte,
	pageRange PageRange,
	totalPages int,
) ([]SubjectExtractionResult, string, error) {
	// Extract text for this page range
	text, err := c.pdfExtractor.ExtractPageRange(pdfContent, pageRange.Start, pageRange.End)
	if err != nil {
		return nil, "", fmt.Errorf("failed to extract text from pages %d-%d: %w", pageRange.Start, pageRange.End, err)
	}

	if len(text) < 50 {
		return nil, "", fmt.Errorf("insufficient text extracted from pages %d-%d", pageRange.Start, pageRange.End)
	}

	// Build MINIMAL prompt for chunk extraction - optimized for token efficiency
	systemPrompt := `Extract syllabus to JSON. Output ONLY valid JSON:
{"subjects":[{"subject_name":"Full Name","subject_code":"MCA 301","total_credits":4,"units":[{"unit_number":1,"title":"Unit 1: Topic Name","raw_text":"exact text","topics":[{"topic_number":1,"title":"Topic"}]}],"books":[{"title":"Book","authors":"Author","book_type":"textbook"}]}]}

Rules:
1. subject_code: Extract FULL code including elective number in parentheses
   - "Elective –I MCA 303 (1) PYTHON" → subject_code: "MCA 303 (1)", subject_name: "PYTHON PROGRAMMING"
   - "Elective –II MCA 304(1) Machine Learning" → subject_code: "MCA 304(1)", subject_name: "Machine Learning"
   - "MCA 301 Data Mining" → subject_code: "MCA 301", subject_name: "Data Mining"
2. subject_name: Extract the actual subject name WITHOUT "Elective –I/II/III" prefix
3. Unit title format: "Unit X: [Name]" 
4. Extract ALL units (usually 5 per subject)
5. raw_text: Copy exact text from PDF for each unit
6. topics: Real topic names only, skip "UNIT", roman numerals, numbers alone
7. IMPORTANT: If page has NO subject header but starts with "UNIT" (continuation page), use subject_name: "CONTINUATION_PAGE" and subject_code: "CONTINUATION"`

	userPrompt := fmt.Sprintf(`Pages %d-%d of %d. Parse this syllabus:

%s`, pageRange.Start, pageRange.End, totalPages, text)

	// Call LLM with 8192 max_tokens to prevent response truncation
	response, err := c.inferenceClient.SimpleCompletion(
		ctx,
		systemPrompt,
		userPrompt,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0), // Deterministic output
	)
	if err != nil {
		return nil, "", fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse response
	var result SyllabusExtractionResult
	if err := utils.ExtractJSONTo(response, &result); err != nil {
		log.Printf("ChunkedExtractor: Failed to parse chunk response (length=%d): %v", len(response), err)
		return nil, response, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return result.Subjects, response, nil
}

// mergeAndDeduplicate merges chunk results and removes duplicates
// Groups subjects by name/code and merges each group separately in parallel
func (c *ChunkedSyllabusExtractor) mergeAndDeduplicate(
	ctx context.Context,
	chunkResults []ChunkResult,
) (*SyllabusExtractionResult, error) {
	// DEBUG: Log each chunk's extracted subjects BEFORE sequential merge
	log.Printf("\n========== CHUNK EXTRACTION DETAILS (BEFORE SEQUENTIAL MERGE) ==========")
	for i, result := range chunkResults {
		if result.Error == nil && len(result.Subjects) > 0 {
			for _, subj := range result.Subjects {
				unitNums := make([]int, len(subj.Units))
				for j, u := range subj.Units {
					unitNums[j] = u.UnitNumber
				}
				log.Printf("  Chunk %d (pages %d-%d): %s (%s) - Units: %v",
					i+1, result.PageRange.Start, result.PageRange.End,
					subj.SubjectName, subj.SubjectCode, unitNums)
			}
		} else if result.Error != nil {
			log.Printf("  Chunk %d (pages %d-%d): ERROR - %v",
				i+1, result.PageRange.Start, result.PageRange.End, result.Error)
		}
	}
	log.Printf("========================================================================\n")

	// STEP 1: Sequential page merge - merge continuation pages into their parent subjects
	log.Printf("ChunkedExtractor: Starting sequential page merge...")
	mergedChunkResults := c.mergeSequentialPages(chunkResults)

	// DEBUG: Log after sequential merge
	log.Printf("\n========== CHUNK EXTRACTION DETAILS (AFTER SEQUENTIAL MERGE) ==========")
	for i, result := range mergedChunkResults {
		if result.Error == nil && len(result.Subjects) > 0 {
			for _, subj := range result.Subjects {
				unitNums := make([]int, len(subj.Units))
				for j, u := range subj.Units {
					unitNums[j] = u.UnitNumber
				}
				log.Printf("  Chunk %d (pages %d-%d): %s (%s) - Units: %v",
					i+1, result.PageRange.Start, result.PageRange.End,
					subj.SubjectName, subj.SubjectCode, unitNums)
			}
		} else if result.Error != nil {
			log.Printf("  Chunk %d (pages %d-%d): ERROR - %v",
				i+1, result.PageRange.Start, result.PageRange.End, result.Error)
		}
	}
	log.Printf("=======================================================================\n")

	// STEP 2: Collect all subjects from merged chunks
	var allPartialSubjects []SubjectExtractionResult
	for _, result := range mergedChunkResults {
		if result.Error == nil && len(result.Subjects) > 0 {
			allPartialSubjects = append(allPartialSubjects, result.Subjects...)
		}
	}

	if len(allPartialSubjects) == 0 {
		return nil, fmt.Errorf("no subjects extracted from any chunk")
	}

	log.Printf("ChunkedExtractor: Merging %d partial subjects from chunks", len(allPartialSubjects))

	// STEP 3: Group subjects by subject_code + name (for any remaining duplicates)
	subjectGroups := c.groupSubjectsByCode(allPartialSubjects)
	log.Printf("ChunkedExtractor: Grouped into %d unique subjects", len(subjectGroups))

	// DEBUG: Log grouping details
	log.Printf("\n========== GROUPING DETAILS ==========")
	for key, group := range subjectGroups {
		log.Printf("  Key: %s", key)
		for _, subj := range group {
			unitNums := make([]int, len(subj.Units))
			for j, u := range subj.Units {
				unitNums[j] = u.UnitNumber
			}
			log.Printf("    -> %s (%s) - Units: %v", subj.SubjectName, subj.SubjectCode, unitNums)
		}
	}
	log.Printf("=======================================\n")

	// STEP 4: Merge each subject group in parallel (fast programmatic merge, no LLM calls)
	mergedSubjects := c.mergeSubjectGroupsParallel(subjectGroups)

	log.Printf("ChunkedExtractor: Parallel merge successful, %d unique subjects", len(mergedSubjects))
	return &SyllabusExtractionResult{Subjects: mergedSubjects}, nil
}

// mergeSequentialPages processes chunks in page order and merges continuation pages into their parent subjects
// A continuation page is one that has orphan units (no proper subject header) - it belongs to the previous page's subject
func (c *ChunkedSyllabusExtractor) mergeSequentialPages(chunkResults []ChunkResult) []ChunkResult {
	if len(chunkResults) <= 1 {
		return chunkResults
	}

	// Sort chunks by page number (they should already be sorted, but let's be safe)
	sortedChunks := make([]ChunkResult, len(chunkResults))
	copy(sortedChunks, chunkResults)
	// Sort by PageRange.Start
	for i := 0; i < len(sortedChunks)-1; i++ {
		for j := i + 1; j < len(sortedChunks); j++ {
			if sortedChunks[i].PageRange.Start > sortedChunks[j].PageRange.Start {
				sortedChunks[i], sortedChunks[j] = sortedChunks[j], sortedChunks[i]
			}
		}
	}

	// Process sequentially: compare page N with page N+1
	// If page N+1 is a continuation, merge it into page N's subject
	mergedChunks := make([]ChunkResult, 0, len(sortedChunks))

	i := 0
	for i < len(sortedChunks) {
		currentChunk := sortedChunks[i]

		// Skip failed chunks
		if currentChunk.Error != nil || len(currentChunk.Subjects) == 0 {
			mergedChunks = append(mergedChunks, currentChunk)
			i++
			continue
		}

		// Look ahead to see if next chunk(s) are continuations
		j := i + 1
		for j < len(sortedChunks) {
			nextChunk := sortedChunks[j]

			// Skip failed chunks
			if nextChunk.Error != nil || len(nextChunk.Subjects) == 0 {
				break
			}

			// Check if next chunk is a continuation page
			if !c.isContinuationPage(nextChunk) {
				break
			}

			// Merge continuation into current chunk's subject
			log.Printf("ChunkedExtractor: Merging continuation page %d into page %d",
				nextChunk.PageRange.Start, currentChunk.PageRange.Start)

			// Merge all subjects from continuation into current chunk's last subject
			if len(currentChunk.Subjects) > 0 {
				lastSubjectIdx := len(currentChunk.Subjects) - 1
				for _, contSubject := range nextChunk.Subjects {
					// Merge units from continuation
					currentChunk.Subjects[lastSubjectIdx].Units = c.mergeUnits(
						currentChunk.Subjects[lastSubjectIdx].Units,
						contSubject.Units,
					)
					// Merge books from continuation
					currentChunk.Subjects[lastSubjectIdx].Books = c.mergeBooks(
						currentChunk.Subjects[lastSubjectIdx].Books,
						contSubject.Books,
					)
					// Take higher credits
					if contSubject.TotalCredits > currentChunk.Subjects[lastSubjectIdx].TotalCredits {
						currentChunk.Subjects[lastSubjectIdx].TotalCredits = contSubject.TotalCredits
					}
				}

				// Update page range to include continuation
				currentChunk.PageRange.End = nextChunk.PageRange.End
			}

			j++
		}

		mergedChunks = append(mergedChunks, currentChunk)
		i = j // Skip to after the last merged continuation
	}

	log.Printf("ChunkedExtractor: Sequential merge reduced %d chunks to %d chunks",
		len(chunkResults), len(mergedChunks))

	return mergedChunks
}

// isContinuationPage checks if a chunk's extraction looks like a continuation page
// (orphan units without a proper subject header)
func (c *ChunkedSyllabusExtractor) isContinuationPage(chunk ChunkResult) bool {
	if chunk.Error != nil || len(chunk.Subjects) == 0 {
		return false
	}

	for _, subject := range chunk.Subjects {
		// Check 1: LLM explicitly marked it as continuation
		if strings.ToUpper(subject.SubjectCode) == "CONTINUATION" ||
			strings.ToUpper(subject.SubjectName) == "CONTINUATION_PAGE" {
			log.Printf("ChunkedExtractor: Page %d detected as continuation (explicit marker)",
				chunk.PageRange.Start)
			return true
		}

		// Check 2: Generic/placeholder subject names that indicate LLM couldn't find a real header
		genericNames := []string{
			"full name", "subject", "untitled", "unknown", "n/a", "na",
			"course", "module", "chapter", "section", "part",
		}
		lowerName := strings.ToLower(strings.TrimSpace(subject.SubjectName))
		for _, generic := range genericNames {
			if lowerName == generic || strings.HasPrefix(lowerName, generic+" ") {
				log.Printf("ChunkedExtractor: Page %d detected as continuation (generic name: %s)",
					chunk.PageRange.Start, subject.SubjectName)
				return true
			}
		}

		// Check 3: Subject has only high unit numbers (e.g., only Unit 4, 5)
		// This suggests it's a continuation of a previous subject
		if len(subject.Units) > 0 {
			minUnit := subject.Units[0].UnitNumber
			for _, unit := range subject.Units {
				if unit.UnitNumber < minUnit {
					minUnit = unit.UnitNumber
				}
			}
			// If the minimum unit number is >= 4, it's likely a continuation
			if minUnit >= 4 {
				log.Printf("ChunkedExtractor: Page %d detected as continuation (starts at Unit %d)",
					chunk.PageRange.Start, minUnit)
				return true
			}
		}

		// Check 4: Subject code doesn't match expected patterns (MCA, CS, IT, etc.)
		// and looks like a placeholder
		codePatterns := []string{"MCA", "CS", "IT", "BCA", "BSC", "MSC", "BE", "BTECH", "MTECH"}
		upperCode := strings.ToUpper(subject.SubjectCode)
		hasValidCode := false
		for _, pattern := range codePatterns {
			if strings.Contains(upperCode, pattern) {
				hasValidCode = true
				break
			}
		}
		// If no valid code pattern and code is very short or generic
		if !hasValidCode && (len(subject.SubjectCode) <= 3 || subject.SubjectCode == subject.SubjectName) {
			log.Printf("ChunkedExtractor: Page %d detected as continuation (invalid code: %s)",
				chunk.PageRange.Start, subject.SubjectCode)
			return true
		}
	}

	return false
}

// groupSubjectsByCode groups subjects by their subject_code + subject_name combination
// This ensures subjects with the same code but different names (e.g., MCA 303 Python vs MCA 303 Web Tech)
// are treated as separate subjects
func (c *ChunkedSyllabusExtractor) groupSubjectsByCode(subjects []SubjectExtractionResult) map[string][]SubjectExtractionResult {
	groups := make(map[string][]SubjectExtractionResult)

	for _, subject := range subjects {
		// Use both code AND name to create unique key
		// This handles cases like MCA 303 having multiple subjects (Python, Web Tech, Data Science)
		key := subject.SubjectCode + "|" + subject.SubjectName
		if subject.SubjectCode == "" {
			key = subject.SubjectName
		}

		groups[key] = append(groups[key], subject)
	}

	return groups
}

// mergeSubjectGroupsParallel merges subject groups in parallel
func (c *ChunkedSyllabusExtractor) mergeSubjectGroupsParallel(subjectGroups map[string][]SubjectExtractionResult) []SubjectExtractionResult {
	results := make([]SubjectExtractionResult, len(subjectGroups))
	var wg sync.WaitGroup

	i := 0
	for _, group := range subjectGroups {
		wg.Add(1)
		go func(idx int, subjects []SubjectExtractionResult) {
			defer wg.Done()
			results[idx] = c.mergeSubjectGroup(subjects)
		}(i, group)
		i++
	}

	wg.Wait()
	return results
}

// mergeSubjectGroup merges multiple partial extractions of the same subject
func (c *ChunkedSyllabusExtractor) mergeSubjectGroup(subjects []SubjectExtractionResult) SubjectExtractionResult {
	if len(subjects) == 1 {
		return subjects[0]
	}

	// Start with first subject as base
	merged := subjects[0]

	// Merge all other subjects into it
	for i := 1; i < len(subjects); i++ {
		subject := subjects[i]

		// Merge units
		merged.Units = c.mergeUnits(merged.Units, subject.Units)

		// Merge books
		merged.Books = c.mergeBooks(merged.Books, subject.Books)

		// Take higher credits
		if subject.TotalCredits > merged.TotalCredits {
			merged.TotalCredits = subject.TotalCredits
		}

		// Prefer longer/more detailed names
		if len(subject.SubjectName) > len(merged.SubjectName) {
			merged.SubjectName = subject.SubjectName
		}
	}

	return merged
}

// programmaticMerge performs a simple merge by subject_code + subject_name
func (c *ChunkedSyllabusExtractor) programmaticMerge(subjects []SubjectExtractionResult) []SubjectExtractionResult {
	subjectMap := make(map[string]*SubjectExtractionResult)

	for _, subject := range subjects {
		// Use both code AND name to create unique key
		key := subject.SubjectCode + "|" + subject.SubjectName
		if subject.SubjectCode == "" {
			key = subject.SubjectName
		}

		if existing, ok := subjectMap[key]; ok {
			// Merge units
			existing.Units = c.mergeUnits(existing.Units, subject.Units)
			// Merge books
			existing.Books = c.mergeBooks(existing.Books, subject.Books)
			// Take higher credits
			if subject.TotalCredits > existing.TotalCredits {
				existing.TotalCredits = subject.TotalCredits
			}
		} else {
			// Copy to avoid modifying original
			copy := subject
			subjectMap[key] = &copy
		}
	}

	// Convert map to slice
	result := make([]SubjectExtractionResult, 0, len(subjectMap))
	for _, subject := range subjectMap {
		result = append(result, *subject)
	}

	return result
}

// mergeUnits merges unit lists, keeping the most complete version of each unit
func (c *ChunkedSyllabusExtractor) mergeUnits(existing, new []SyllabusUnitExtraction) []SyllabusUnitExtraction {
	unitMap := make(map[int]SyllabusUnitExtraction)

	// Add existing units
	for _, unit := range existing {
		unitMap[unit.UnitNumber] = unit
	}

	// Merge new units
	for _, unit := range new {
		if existingUnit, ok := unitMap[unit.UnitNumber]; ok {
			// Keep the one with more content
			if len(unit.RawText) > len(existingUnit.RawText) {
				unitMap[unit.UnitNumber] = unit
			} else if len(unit.Topics) > len(existingUnit.Topics) {
				unitMap[unit.UnitNumber] = unit
			}
		} else {
			unitMap[unit.UnitNumber] = unit
		}
	}

	// Convert to sorted slice
	result := make([]SyllabusUnitExtraction, 0, len(unitMap))
	for i := 1; i <= len(unitMap)+10; i++ { // +10 to handle gaps
		if unit, ok := unitMap[i]; ok {
			result = append(result, unit)
		}
	}

	return result
}

// mergeBooks merges book lists, removing duplicates by title
func (c *ChunkedSyllabusExtractor) mergeBooks(existing, new []BookReferenceExtraction) []BookReferenceExtraction {
	bookMap := make(map[string]BookReferenceExtraction)

	for _, book := range existing {
		bookMap[book.Title] = book
	}

	for _, book := range new {
		if _, ok := bookMap[book.Title]; !ok {
			bookMap[book.Title] = book
		}
	}

	result := make([]BookReferenceExtraction, 0, len(bookMap))
	for _, book := range bookMap {
		result = append(result, book)
	}

	return result
}

// romanToInt converts a Roman numeral string to an integer
// Supports I, II, III, IV, V (1-5) which covers typical elective numbering
func romanToInt(roman string) int {
	roman = strings.ToUpper(strings.TrimSpace(roman))
	switch roman {
	case "I":
		return 1
	case "II":
		return 2
	case "III":
		return 3
	case "IV":
		return 4
	case "V":
		return 5
	default:
		return 0
	}
}

// normalizeSubjectData cleans up subject name and code for elective subjects
// - Removes "Elective –I/II/III" prefix from name
// - Extracts the elective number from the code pattern like "MCA 303 (1)" or "MCA 304(2)"
// - Returns cleaned name and code with elective suffix like "MCA 304-(1)"
//
// Examples:
//
//	Input:  name="Elective –II Machine Learning", code="MCA 304"
//	Output: cleanName="Machine Learning", cleanCode="MCA 304-(2)"
//
//	Input:  name="PYTHON PROGRAMMING", code="MCA 303 (1)"
//	Output: cleanName="PYTHON PROGRAMMING", cleanCode="MCA 303-(1)"
func normalizeSubjectData(name, code string) (cleanName, cleanCode string) {
	cleanName = name
	cleanCode = code

	// Pattern to match elective prefix in name: "Elective –I", "Elective –II", "Elective-III", etc.
	// Handles various dash types: –, -, —
	electiveNamePattern := regexp.MustCompile(`(?i)^Elective\s*[–\-—]\s*(I{1,3}|IV|V)\s*`)

	// Check if name has elective prefix and extract the roman numeral
	var electiveNumFromName int
	if matches := electiveNamePattern.FindStringSubmatch(name); len(matches) > 1 {
		electiveNumFromName = romanToInt(matches[1])
		// Remove the elective prefix from name
		cleanName = strings.TrimSpace(electiveNamePattern.ReplaceAllString(name, ""))
	}

	// Pattern to extract elective number from code: "MCA 303 (1)", "MCA 304(2)", "MCA 305 (3)"
	codeWithNumPattern := regexp.MustCompile(`^([A-Z]+\s*\d+)\s*\((\d+)\)$`)

	if matches := codeWithNumPattern.FindStringSubmatch(code); len(matches) == 3 {
		// Code already has number like "MCA 303 (1)" -> normalize to "MCA 303-(1)"
		baseCode := strings.ReplaceAll(matches[1], " ", " ") // Normalize spaces
		electiveNum := matches[2]
		cleanCode = fmt.Sprintf("%s-(%s)", strings.TrimSpace(baseCode), electiveNum)
	} else if electiveNumFromName > 0 {
		// Code doesn't have number but name had elective prefix -> add number from name
		// e.g., code="MCA 304", name="Elective –II Machine Learning" -> "MCA 304-(2)"
		cleanCode = fmt.Sprintf("%s-(%d)", strings.TrimSpace(code), electiveNumFromName)
	}

	// Final cleanup: ensure no double spaces
	cleanName = strings.Join(strings.Fields(cleanName), " ")
	cleanCode = strings.Join(strings.Fields(cleanCode), " ")

	return cleanName, cleanCode
}

// cleanUnitTitle removes unit number prefixes from title
// Strips patterns like: "Unit 1:", "Unit I:", "Unit – I", "UNIT-1", "Unit 1", "UNIT I", etc.
//
// Examples:
//
//	Input:  "Unit 1: Introduction to machine learning"
//	Output: "Introduction to machine learning"
//
//	Input:  "UNIT I  INTRODUCTION TO PYTHON"
//	Output: "INTRODUCTION TO PYTHON"
//
//	Input:  "Unit – I"
//	Output: ""
//
//	Input:  "Unit IV: Natural Language processing"
//	Output: "Natural Language processing"
func cleanUnitTitle(title string) string {
	if title == "" {
		return ""
	}

	// Pattern to match unit number prefixes:
	// - "Unit 1:", "Unit 1", "UNIT 1:", "UNIT-1", "Unit-1:"
	// - "Unit I:", "Unit I", "UNIT I:", "UNIT-I", "Unit-I:"
	// - "Unit – I", "Unit – 1", "Unit — I" (with em-dash or en-dash)
	// - "Unit IV:", "Unit V:" (Roman numerals 4 and 5)
	// The pattern captures optional colon/dash after the number
	// IMPORTANT: Order matters in alternation - match longer patterns first (IV, III before I)
	unitPattern := regexp.MustCompile(`(?i)^UNIT\s*[–\-—]?\s*(\d+|IV|V|III|II|I)\s*[:\-–—]?\s*`)

	cleanTitle := unitPattern.ReplaceAllString(title, "")
	cleanTitle = strings.TrimSpace(cleanTitle)

	return cleanTitle
}

// needsLLMMerge determines if we need LLM for complex merge
func (c *ChunkedSyllabusExtractor) needsLLMMerge(original, merged []SubjectExtractionResult) bool {
	// If significant reduction in subjects, might have missed something
	if len(merged) < len(original)/2 && len(original) > 4 {
		return true
	}

	// Check for subjects with same code but different names (potential merge issues)
	codeNames := make(map[string][]string)
	for _, s := range original {
		codeNames[s.SubjectCode] = append(codeNames[s.SubjectCode], s.SubjectName)
	}

	for _, names := range codeNames {
		if len(names) > 1 {
			// Multiple different names for same code - needs LLM judgment
			uniqueNames := make(map[string]bool)
			for _, n := range names {
				uniqueNames[n] = true
			}
			if len(uniqueNames) > 1 {
				return true
			}
		}
	}

	return false
}

// llmMerge uses LLM to intelligently merge and deduplicate subjects
func (c *ChunkedSyllabusExtractor) llmMerge(
	ctx context.Context,
	partialSubjects []SubjectExtractionResult,
) (*SyllabusExtractionResult, error) {
	// Create timeout context for merge
	mergeCtx, cancel := context.WithTimeout(ctx, c.mergeTimeout)
	defer cancel()

	// Serialize partial subjects to JSON
	partialJSON, err := json.MarshalIndent(partialSubjects, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize partial subjects: %w", err)
	}

	systemPrompt := `You are merging multiple partial extraction results from different pages of the same syllabus PDF into a single consolidated result.

Tasks:
1. Identify unique subjects by subject_code (e.g., MCA 301, MCA 302)
2. Merge units from the same subject that appeared in different chunks
3. Remove duplicate units (same unit_number for same subject)
4. Keep the most complete version of each unit (prefer longer raw_text)
5. Consolidate books lists (remove duplicates by title)
6. Ensure each subject has complete, ordered units (unit 1, 2, 3...)

Output ONLY valid JSON with the merged result:
{
  "subjects": [
    {
      "subject_name": "...",
      "subject_code": "...",
      "total_credits": N,
      "units": [...all units in order...],
      "books": [...deduplicated books...]
    }
  ]
}

Rules:
- Same subject_code = same subject (merge their units)
- Keep units ordered by unit_number
- Prefer longer/more complete raw_text when merging duplicate units
- Output ONLY JSON, no explanation`

	userPrompt := fmt.Sprintf("Merge these partial syllabus extractions into a single consolidated result:\n\n%s", string(partialJSON))

	response, err := c.inferenceClient.SimpleCompletion(
		mergeCtx,
		systemPrompt,
		userPrompt,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0), // Deterministic output
	)
	if err != nil {
		log.Printf("ChunkedExtractor: LLM merge failed, falling back to programmatic merge: %v", err)
		// Fall back to programmatic merge
		merged := c.programmaticMerge(partialSubjects)
		return &SyllabusExtractionResult{Subjects: merged}, nil
	}

	var result SyllabusExtractionResult
	if err := utils.ExtractJSONTo(response, &result); err != nil {
		log.Printf("ChunkedExtractor: Failed to parse LLM merge response, falling back: %v", err)
		merged := c.programmaticMerge(partialSubjects)
		return &SyllabusExtractionResult{Subjects: merged}, nil
	}

	return &result, nil
}

// saveMultiSubjectSyllabusData saves multiple subjects to the database
func (c *ChunkedSyllabusExtractor) saveMultiSubjectSyllabusData(
	ctx context.Context,
	document *model.Document,
	extractedData *SyllabusExtractionResult,
) ([]*model.Syllabus, error) {
	var syllabuses []*model.Syllabus
	var subjectsNeedingAISetup []uint // Track subjects that need AI setup after transaction commits

	// Get semester ID from document (supports both semester-based and subject-based documents)
	semesterID, err := c.getSemesterIDFromDocument(document)
	if err != nil {
		return nil, fmt.Errorf("failed to get semester ID: %w", err)
	}

	tx := c.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, subjectData := range extractedData.Subjects {
		// Skip continuation pages or invalid subjects
		if subjectData.SubjectCode == "CONTINUATION" || subjectData.SubjectCode == "" {
			log.Printf("ChunkedExtractor: Skipping invalid subject: %s (%s)",
				subjectData.SubjectName, subjectData.SubjectCode)
			continue
		}

		// Normalize subject name and code:
		// - Remove "Elective –I/II/III" prefix from name
		// - Add elective number suffix to code like "MCA 304-(2)"
		cleanName, cleanCode := normalizeSubjectData(subjectData.SubjectName, subjectData.SubjectCode)
		if cleanName != subjectData.SubjectName || cleanCode != subjectData.SubjectCode {
			log.Printf("ChunkedExtractor: Normalized subject: '%s' (%s) -> '%s' (%s)",
				subjectData.SubjectName, subjectData.SubjectCode, cleanName, cleanCode)
		}
		subjectData.SubjectName = cleanName
		subjectData.SubjectCode = cleanCode

		// Find or create subject by code AND name within the same semester
		// This is important because electives can have the same code but different names
		// e.g., MCA 303 (1) Python Programming, MCA 303 (2) Web Technology, MCA 303 (3) Data Science
		var subject model.Subject
		subjectQuery := tx.Where("code = ? AND name = ? AND semester_id = ?", subjectData.SubjectCode, subjectData.SubjectName, semesterID)

		needsAISetup := false
		if err := subjectQuery.First(&subject).Error; err != nil {
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
					return nil, fmt.Errorf("failed to create subject %s (%s): %w", subjectData.SubjectName, subjectData.SubjectCode, err)
				}
				log.Printf("ChunkedExtractor: Created new subject: %s (%s) for semester %d",
					subject.Name, subject.Code, semesterID)
				needsAISetup = true // New subject needs AI setup
			} else {
				tx.Rollback()
				return nil, fmt.Errorf("failed to query subject: %w", err)
			}
		} else {
			log.Printf("ChunkedExtractor: Found existing subject ID=%d for %s (%s)",
				subject.ID, subjectData.SubjectName, subjectData.SubjectCode)
			// Check if existing subject needs AI setup (KB, Agent, or API Key missing)
			if subject.KnowledgeBaseUUID == "" || subject.AgentUUID == "" || subject.AgentAPIKeyEncrypted == "" {
				needsAISetup = true
			}
		}

		// Track subjects that need AI setup (will be processed after transaction commits)
		if needsAISetup && c.subjectService != nil {
			subjectsNeedingAISetup = append(subjectsNeedingAISetup, subject.ID)
		}

		// Check if syllabus already exists for this subject
		var existingSyllabus model.Syllabus
		syllabusErr := tx.Where("subject_id = ?", subject.ID).First(&existingSyllabus).Error

		var syllabus *model.Syllabus
		if syllabusErr == nil {
			// Syllabus exists - delete old units/topics and update
			log.Printf("ChunkedExtractor: Updating existing syllabus for subject %s (%s)", subject.Name, subject.Code)

			// Delete existing topics first (foreign key constraint)
			var existingUnits []model.SyllabusUnit
			tx.Where("syllabus_id = ?", existingSyllabus.ID).Find(&existingUnits)
			for _, unit := range existingUnits {
				tx.Where("unit_id = ?", unit.ID).Delete(&model.SyllabusTopic{})
			}
			// Delete existing units
			tx.Where("syllabus_id = ?", existingSyllabus.ID).Delete(&model.SyllabusUnit{})
			// Delete existing books
			tx.Where("syllabus_id = ?", existingSyllabus.ID).Delete(&model.BookReference{})

			// Update syllabus
			existingSyllabus.DocumentID = document.ID
			existingSyllabus.SubjectName = subjectData.SubjectName
			existingSyllabus.SubjectCode = subjectData.SubjectCode
			existingSyllabus.TotalCredits = subjectData.TotalCredits
			existingSyllabus.ExtractionStatus = model.SyllabusExtractionCompleted
			if err := tx.Save(&existingSyllabus).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to update syllabus: %w", err)
			}
			syllabus = &existingSyllabus
		} else if syllabusErr == gorm.ErrRecordNotFound {
			// Create new syllabus
			syllabus = &model.Syllabus{
				SubjectID:        subject.ID,
				DocumentID:       document.ID,
				SubjectName:      subjectData.SubjectName,
				SubjectCode:      subjectData.SubjectCode,
				TotalCredits:     subjectData.TotalCredits,
				ExtractionStatus: model.SyllabusExtractionCompleted,
			}

			if err := tx.Create(syllabus).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create syllabus: %w", err)
			}
		} else {
			tx.Rollback()
			return nil, fmt.Errorf("failed to query syllabus: %w", syllabusErr)
		}

		// Create units
		for _, unitData := range subjectData.Units {
			// Clean unit title by removing "Unit X:" prefix
			cleanTitle := cleanUnitTitle(unitData.Title)

			unit := model.SyllabusUnit{
				SyllabusID:  syllabus.ID,
				UnitNumber:  unitData.UnitNumber,
				Title:       cleanTitle,
				Description: unitData.Description,
				RawText:     unitData.RawText,
				Hours:       unitData.Hours,
			}

			if err := tx.Create(&unit).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create unit: %w", err)
			}

			// Create topics - intelligently extract from raw_text if needed
			var topicsToCreate []string
			if len(unitData.Topics) <= 1 && unitData.RawText != "" {
				// LLM gave generic/single topic, extract from raw_text instead
				topicsToCreate = extractTopicsFromRawText(unitData.RawText)
			} else {
				// Use LLM-provided topics
				for _, t := range unitData.Topics {
					if t.Title != "" {
						topicsToCreate = append(topicsToCreate, t.Title)
					}
				}
			}

			// Create topic records
			for i, topicTitle := range topicsToCreate {
				topic := model.SyllabusTopic{
					UnitID:      unit.ID,
					TopicNumber: i + 1,
					Title:       topicTitle,
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

		// Populate the Subject relationship for the completion event
		syllabus.Subject = subject

		syllabuses = append(syllabuses, syllabus)
		log.Printf("ChunkedExtractor: Saved syllabus for %s (%s) with %d units",
			subjectData.SubjectName, subjectData.SubjectCode, len(subjectData.Units))
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Setup AI resources for subjects AFTER transaction commits (so subjects are visible)
	// Process sequentially with rate limiting and retry to avoid DigitalOcean API 429 errors
	if c.subjectService != nil && len(subjectsNeedingAISetup) > 0 {
		log.Printf("ChunkedExtractor: Starting AI setup for %d subjects after transaction commit (sequential with rate limiting)", len(subjectsNeedingAISetup))

		// Single goroutine processes all subjects sequentially to avoid rate limits
		go func() {
			for i, subjectID := range subjectsNeedingAISetup {
				var lastErr error
				maxRetries := 5

				// Retry loop with exponential backoff for rate limit errors
				for attempt := 0; attempt < maxRetries; attempt++ {
					if attempt > 0 {
						backoff := time.Duration(1<<attempt) * time.Second // 2s, 4s, 8s, 16s
						log.Printf("ChunkedExtractor: Retrying AI setup for subject %d (attempt %d/%d) after %v backoff",
							subjectID, attempt+1, maxRetries, backoff)
						time.Sleep(backoff)
					}

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					result, err := c.subjectService.SetupSubjectAI(ctx, subjectID)
					cancel()

					if err == nil {
						log.Printf("ChunkedExtractor: AI setup complete for subject %d (KB: %v, Agent: %v, APIKey: %v)",
							subjectID, result.KnowledgeBaseCreated, result.AgentCreated, result.APIKeyCreated)
						lastErr = nil
						break
					}

					lastErr = err
					if !isRateLimitError(err) {
						// Non-retriable error, log and move on
						log.Printf("Warning: ChunkedExtractor failed to setup AI for subject %d: %v", subjectID, err)
						break
					}
					// Rate limit error - will retry
					log.Printf("ChunkedExtractor: Rate limit hit for subject %d, will retry...", subjectID)
				}

				if lastErr != nil && isRateLimitError(lastErr) {
					log.Printf("Warning: ChunkedExtractor exhausted retries for subject %d due to rate limiting: %v", subjectID, lastErr)
				}

				// Rate limit delay between subjects (except after last one)
				if i < len(subjectsNeedingAISetup)-1 {
					time.Sleep(2 * time.Second)
				}
			}
			log.Printf("ChunkedExtractor: Completed AI setup for all %d subjects", len(subjectsNeedingAISetup))
		}()
	}

	return syllabuses, nil
}

// isRateLimitError checks if an error is a DigitalOcean rate limit error (HTTP 429)
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too_many_requests") ||
		strings.Contains(errStr, "rate limit")
}
