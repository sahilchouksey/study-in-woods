package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/sahilchouksey/go-init-setup/services/pyq_crawler"
)

// Minimum year for PYQ papers (filter out very old papers)
const MinPYQYear = 2019

// PYQCrawlerService handles PYQ crawling operations (no DB required)
type PYQCrawlerService struct {
	factory *pyq_crawler.CrawlerFactory
}

// NewPYQCrawlerService creates a new PYQ crawler service
func NewPYQCrawlerService() *PYQCrawlerService {
	return &PYQCrawlerService{
		factory: pyq_crawler.GetCrawlerFactory(),
	}
}

// SearchPapersRequest holds search parameters
type SearchPapersRequest struct {
	SubjectCode string `json:"subject_code"`
	SubjectName string `json:"subject_name"`
	Course      string `json:"course"`
	Semester    string `json:"semester"`
	Year        int    `json:"year,omitempty"`
	Month       string `json:"month,omitempty"`
	SourceName  string `json:"source_name,omitempty"` // Specific crawler to use (default: all)
	Limit       int    `json:"limit"`
}

// PYQCrawlerPaperResult represents a search result
type PYQCrawlerPaperResult struct {
	Title           string `json:"title"`
	SourceURL       string `json:"source_url"`
	PDFURL          string `json:"pdf_url"`
	FileType        string `json:"file_type"`
	SubjectCode     string `json:"subject_code"`
	SubjectName     string `json:"subject_name"`
	Year            int    `json:"year"`
	Month           string `json:"month,omitempty"`
	ExamType        string `json:"exam_type,omitempty"`
	SourceName      string `json:"source_name"`      // Which crawler found this
	CodeMatched     bool   `json:"code_matched"`     // Whether subject code matches current subject
	MatchConfidence string `json:"match_confidence"` // "exact", "partial", "none"
}

// SearchPapersResult contains categorized search results
type SearchPapersResult struct {
	MatchedPapers   []PYQCrawlerPaperResult `json:"matched_papers"`   // Papers matching current subject code
	UnmatchedPapers []PYQCrawlerPaperResult `json:"unmatched_papers"` // Papers with different codes (older syllabus)
	TotalMatched    int                     `json:"total_matched"`
	TotalUnmatched  int                     `json:"total_unmatched"`
}

// SearchPapers searches for PYQ papers across all or specific crawlers
// Returns papers categorized by code match status
// Only shows papers for the specific subject (matched by base code like MCA301)
// Unmatched papers are only shown if they match the subject NAME (potential renamed subjects)
func (s *PYQCrawlerService) SearchPapers(ctx context.Context, req SearchPapersRequest) (*SearchPapersResult, error) {
	result := &SearchPapersResult{
		MatchedPapers:   []PYQCrawlerPaperResult{},
		UnmatchedPapers: []PYQCrawlerPaperResult{},
	}

	// Set default limit
	if req.Limit <= 0 {
		req.Limit = 50
	}

	// Determine which crawlers to use
	var crawlers []pyq_crawler.PYQCrawlerInterface
	if req.SourceName != "" {
		crawler, err := s.factory.GetCrawler(req.SourceName)
		if err != nil {
			return nil, fmt.Errorf("crawler not found: %w", err)
		}
		crawlers = []pyq_crawler.PYQCrawlerInterface{crawler}
	} else {
		crawlers = s.factory.GetAllCrawlers()
	}

	// Extract codes for matching
	// fullCode preserves variant: "MCA 303 (1)" -> "MCA3031" for exact matching
	// baseCode strips variant: "MCA 303 (1)" -> "MCA303" for broader matching
	fullCode := normalizeFullCode(req.SubjectCode)
	baseCode := extractBaseCode(req.SubjectCode)
	targetCourse := extractCourseFromCode(req.SubjectCode)

	// Normalize subject name for matching (lowercase, remove extra spaces)
	normalizedSubjectName := strings.ToLower(strings.TrimSpace(req.SubjectName))
	// Extract key words from subject name for fuzzy matching
	subjectKeywords := extractKeywords(normalizedSubjectName)

	// Use course from request if provided, otherwise extract from subject code
	if req.Course == "" && targetCourse != "" {
		req.Course = targetCourse
	}

	// Search using each crawler
	for _, crawler := range crawlers {
		// Pass empty subject code to get all papers for the course, we'll filter ourselves
		papers, err := crawler.SearchPapers(ctx, "", req.SubjectName, req.Course, req.Semester, req.Limit*3)
		if err != nil {
			// Log error but continue with other crawlers
			continue
		}

		// Convert crawler results to response format
		for _, paper := range papers {
			// Filter out papers before MinPYQYear
			if paper.Year < MinPYQYear {
				continue
			}

			// Apply additional filters
			if req.Year > 0 && paper.Year != req.Year {
				continue
			}
			if req.Month != "" && !strings.EqualFold(paper.Month, req.Month) {
				continue
			}

			// PRIMARY FILTER: Keyword-based matching on subject name
			// This is the main filter - paper must match subject keywords
			paperNameLower := strings.ToLower(paper.SubjectName)
			paperTitleLower := strings.ToLower(paper.Title)

			// Check keyword match - this is now the PRIMARY filter
			keywordMatched := matchesSubjectKeywords(paperNameLower, paperTitleLower, subjectKeywords)

			// If keywords don't match, skip this paper entirely
			// This ensures "Python Programming" won't show "Web Technology" papers
			if !keywordMatched {
				continue
			}

			// SECONDARY: Check code match for categorization (matched vs unmatched)
			// Code matching is now only used for categorization, not filtering
			paperFullCode := normalizeFullCode(paper.SubjectCode)
			paperBaseCode := extractBaseCode(paper.SubjectCode)
			codeMatched := fullCode != "" && paperFullCode == fullCode

			// Determine match confidence based on both keyword and code matching
			matchConfidence := "keyword" // Default: matched by keywords
			if codeMatched {
				matchConfidence = "exact" // Both code and keywords match
			} else if baseCode != "" && len(baseCode) >= 3 && strings.HasPrefix(paperBaseCode, baseCode[:3]) {
				matchConfidence = "partial" // Same course prefix (e.g., MCA3xx)
			}

			paperResult := PYQCrawlerPaperResult{
				Title:           paper.Title,
				SourceURL:       paper.SourceURL,
				PDFURL:          paper.PDFURL,
				FileType:        paper.FileType,
				SubjectCode:     paper.SubjectCode,
				SubjectName:     paper.SubjectName,
				Year:            paper.Year,
				Month:           paper.Month,
				ExamType:        paper.ExamType,
				SourceName:      crawler.GetDisplayName(),
				CodeMatched:     codeMatched,
				MatchConfidence: matchConfidence,
			}

			// Categorize: exact code match goes to matched, others to unmatched
			// But ALL papers here have passed keyword matching, so they're relevant
			if codeMatched {
				if len(result.MatchedPapers) < req.Limit {
					result.MatchedPapers = append(result.MatchedPapers, paperResult)
				}
			} else {
				if len(result.UnmatchedPapers) < req.Limit {
					result.UnmatchedPapers = append(result.UnmatchedPapers, paperResult)
				}
			}
		}
	}

	result.TotalMatched = len(result.MatchedPapers)
	result.TotalUnmatched = len(result.UnmatchedPapers)

	return result, nil
}

// normalizeFullCode normalizes subject code while PRESERVING variant numbers
// "MCA 303 (1)" -> "MCA3031", "MCA-305-2" -> "MCA3052", "MCA 305(3)" -> "MCA3053"
// This is used for exact matching including variant
func normalizeFullCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))

	// Remove common separators
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, "_", "")
	code = strings.ReplaceAll(code, "(", "")
	code = strings.ReplaceAll(code, ")", "")

	return code
}

// extractBaseCode extracts the base code without variant numbers
// "MCA 303 (1)" -> "MCA303", "MCA-305-2" -> "MCA305", "MCA 305(3)" -> "MCA305"
// This is used for broader category matching (e.g., finding unmatched papers from older syllabus)
func extractBaseCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))

	// Remove common separators
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, "_", "")

	// Remove variant numbers in parentheses: MCA303(1) -> MCA303
	if idx := strings.Index(code, "("); idx > 0 {
		code = code[:idx]
	}

	// Handle cases like MCA3031, MCA3052 - extract base MCA303, MCA305
	if len(code) == 7 && code[6] >= '1' && code[6] <= '9' {
		code = code[:6]
	}

	return code
}

// extractSemesterFromCode extracts the semester number from a subject code
// Subject codes follow pattern: COURSE + SEMESTER + SUBJECT_NUM + (VARIANT)
// Examples: "MCA 303 (1)" -> 3, "MCA-505" -> 5, "MCA 101" -> 1, "BCA-201" -> 2
// The first digit after the course prefix indicates the semester
func extractSemesterFromCode(code string) int {
	// Normalize the code
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")

	// Find the first digit in the code (after letters)
	for i, c := range code {
		if c >= '0' && c <= '9' {
			// The first digit is the semester number
			semester := int(c - '0')
			// Validate it's a reasonable semester (1-8)
			if semester >= 1 && semester <= 8 {
				_ = i // suppress unused variable warning
				return semester
			}
			break
		}
	}

	return 0 // Unknown semester
}

// extractCourseFromCode extracts the course prefix from a subject code
// Examples: "MCA 303 (1)" -> "MCA", "BCA-201" -> "BCA"
func extractCourseFromCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))

	// Extract letters from the beginning
	var course strings.Builder
	for _, c := range code {
		if c >= 'A' && c <= 'Z' {
			course.WriteRune(c)
		} else {
			break
		}
	}

	return course.String()
}

// extractKeywords extracts meaningful keywords from a subject name
// "Data Mining" -> ["data", "mining"]
// "Artificial Intelligence" -> ["artificial", "intelligence"]
func extractKeywords(name string) []string {
	// Common words to exclude (stop words)
	stopWords := map[string]bool{
		"and": true, "or": true, "the": true, "a": true, "an": true,
		"of": true, "in": true, "to": true, "for": true, "with": true,
		"using": true, "based": true, "introduction": true, "advanced": true,
		"basics": true, "fundamentals": true,
	}

	words := strings.Fields(strings.ToLower(name))
	keywords := make([]string, 0, len(words))

	for _, word := range words {
		// Clean the word - remove non-alphanumeric chars
		cleaned := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, word)

		if len(cleaned) >= 3 && !stopWords[cleaned] {
			keywords = append(keywords, cleaned)
		}
	}

	return keywords
}

// matchesSubjectKeywords checks if a paper name/title matches subject keywords
// Uses strict matching: ALL keywords must be present for a match
// This prevents "Python Programming" from matching "Web Programming" or "Python" alone
func matchesSubjectKeywords(paperName, paperTitle string, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}

	combined := paperName + " " + paperTitle

	// Count how many keywords match
	matchCount := 0
	for _, kw := range keywords {
		if strings.Contains(combined, kw) {
			matchCount++
		}
	}

	// STRICT MATCHING: Require ALL keywords to match
	// "Python Programming" -> both "python" AND "programming" must be in paper name/title
	// This prevents showing "Web Programming" when searching for "Python Programming"
	return matchCount == len(keywords)
}

// GetAvailableSources returns all registered crawler sources
func (s *PYQCrawlerService) GetAvailableSources() []CrawlerSourceInfo {
	crawlers := s.factory.GetAllCrawlers()
	sources := make([]CrawlerSourceInfo, len(crawlers))

	for i, crawler := range crawlers {
		sources[i] = CrawlerSourceInfo{
			Name:        crawler.GetName(),
			DisplayName: crawler.GetDisplayName(),
			BaseURL:     crawler.GetBaseURL(),
		}
	}

	return sources
}

// CrawlerSourceInfo contains basic info about a crawler source
type CrawlerSourceInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	BaseURL     string `json:"base_url"`
}

// ValidatePDFURL checks if a PDF URL is still accessible
func (s *PYQCrawlerService) ValidatePDFURL(ctx context.Context, pdfURL, sourceName string) (bool, error) {
	crawler, err := s.factory.GetCrawler(sourceName)
	if err != nil {
		return false, fmt.Errorf("crawler not found: %w", err)
	}

	return crawler.ValidatePDFURL(ctx, pdfURL)
}
