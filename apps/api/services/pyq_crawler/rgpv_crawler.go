package pyq_crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// RGPVCrawler implements PYQCrawlerInterface for RGPV Online
type RGPVCrawler struct {
	BaseCrawler
	httpClient *http.Client
}

// NewRGPVCrawler creates a new RGPV crawler instance
func NewRGPVCrawler() *RGPVCrawler {
	config := CrawlerConfig{
		Name:           "rgpv",
		DisplayName:    "RGPV Online",
		BaseURL:        "https://www.rgpvonline.com",
		Timeout:        30,
		MaxRetries:     3,
		RateLimitDelay: 1000, // 1 second between requests
		CustomHeaders: map[string]string{
			"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		},
	}

	return &RGPVCrawler{
		BaseCrawler: BaseCrawler{Config: config},
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// SearchPapers searches for papers matching the criteria
// subjectCode is REQUIRED - always filters by code first
// subjectName is OPTIONAL - used as additional fuzzy search filter within the code group
func (c *RGPVCrawler) SearchPapers(ctx context.Context, subjectCode, subjectName, course, semester string, limit int) ([]PYQCrawlerResult, error) {
	// Get all papers for the course
	allPapers, err := c.GetAllPapers(ctx, course)
	if err != nil {
		return nil, err
	}

	// Filter papers based on search criteria
	var results []PYQCrawlerResult

	// Normalize subject code for matching while PRESERVING variant number
	// "MCA 303 (1)" -> "MCA3031", "MCA 305(2)" -> "MCA3052"
	// This ensures we only match papers with the EXACT same subject code including variant
	normalizedSearchCode := c.normalizeSubjectCodeForSearch(subjectCode)

	// Optional search query (for fuzzy name filtering within the code group)
	searchQuery := strings.ToUpper(strings.TrimSpace(subjectName))

	for _, paper := range allPapers {
		if limit > 0 && len(results) >= limit {
			break
		}

		// STEP 1: Always filter by full subject code including variant (REQUIRED)
		if normalizedSearchCode != "" {
			paperNormalizedCode := c.normalizeSubjectCodeForSearch(paper.SubjectCode)
			if paperNormalizedCode != normalizedSearchCode {
				continue // Skip papers that don't match the exact code (including variant)
			}
		}

		// STEP 2: If search query provided, filter by name (OPTIONAL fuzzy search)
		if searchQuery != "" {
			paperNameUpper := strings.ToUpper(paper.SubjectName)
			paperTitleUpper := strings.ToUpper(paper.Title)

			// Check if search query matches name or title
			if !strings.Contains(paperNameUpper, searchQuery) && !strings.Contains(paperTitleUpper, searchQuery) {
				continue // Skip papers that don't match the search query
			}
		}

		results = append(results, paper)
	}

	return results, nil
}

// extractBaseSubjectCode extracts the base code without variant numbers
// "MCA 303 (1)" -> "MCA303", "MCA-305-2" -> "MCA305", "MCA 305(3)" -> "MCA305"
func (c *RGPVCrawler) extractBaseSubjectCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))

	// Remove common separators first
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, "_", "")

	// Remove variant numbers in parentheses: MCA303(1) -> MCA303
	if idx := strings.Index(code, "("); idx > 0 {
		code = code[:idx]
	}

	// Handle cases like MCA3031, MCA3052 - extract base MCA303, MCA305
	// Pattern: 3 letters + 3 digits + optional variant digit
	if len(code) >= 6 {
		// Check if it's format like MCA3031 (letter+letter+letter+digit+digit+digit+variant)
		if len(code) == 7 && code[6] >= '1' && code[6] <= '9' {
			// Remove the last digit (variant number)
			code = code[:6]
		}
	}

	return code
}

// normalizeSubjectCodeForSearch normalizes subject codes for comparison (kept for backward compatibility)
// "MCA 303 (1)" -> "MCA303", "MCA-303" -> "MCA303", "MCA-303-1" -> "MCA3031"
func (c *RGPVCrawler) normalizeSubjectCodeForSearch(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	// Remove spaces, hyphens, parentheses, and other common separators
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, "(", "")
	code = strings.ReplaceAll(code, ")", "")
	code = strings.ReplaceAll(code, "_", "")
	return code
}

// GetAllPapers retrieves all papers for a course
func (c *RGPVCrawler) GetAllPapers(ctx context.Context, course string) ([]PYQCrawlerResult, error) {
	// Normalize course name to URL format
	courseURL := c.normalizeCourseToURL(course)
	pageURL := fmt.Sprintf("%s/%s.html", c.Config.BaseURL, courseURL)

	// Fetch the HTML page
	htmlContent, err := c.fetchHTML(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}

	// Parse the HTML and extract paper links
	papers, err := c.extractPapersFromHTML(htmlContent, pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract papers: %w", err)
	}

	return papers, nil
}

// ValidatePDFURL checks if a PDF URL is accessible
func (c *RGPVCrawler) ValidatePDFURL(ctx context.Context, pdfURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", pdfURL, nil)
	if err != nil {
		return false, err
	}

	for key, value := range c.Config.CustomHeaders {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// GetPaperMetadata extracts metadata from title and URL
func (c *RGPVCrawler) GetPaperMetadata(title, url string) (PYQCrawlerResult, error) {
	result := PYQCrawlerResult{
		Title:     title,
		SourceURL: url,
		PDFURL:    c.convertHTMLToPDFURL(url),
		FileType:  "pdf",
	}

	// Extract metadata from title
	// Example: "MCA-301-DATA-MINING-DEC-2024"
	c.parseRGPVTitle(title, &result)

	return result, nil
}

// Helper methods

func (c *RGPVCrawler) normalizeCourseToURL(course string) string {
	// Convert course name to RGPV URL format
	// e.g., "MCA" -> "mca", "B.Tech CSE" -> "btech"
	course = strings.ToLower(strings.TrimSpace(course))
	course = strings.ReplaceAll(course, " ", "")
	course = strings.ReplaceAll(course, ".", "")

	// Map common course names
	courseMap := map[string]string{
		"mca":       "mca",
		"btech":     "btech", // Default to main page
		"mtech":     "mtech",
		"bpharmacy": "bpharmacy",
		"mpharmacy": "mpharmacy",
		"diploma":   "rgpv-diploma",
		"mba":       "mba",
		"barch":     "barch",
		"march":     "march",
	}

	if mapped, ok := courseMap[course]; ok {
		return mapped
	}

	return course
}

func (c *RGPVCrawler) fetchHTML(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	for key, value := range c.Config.CustomHeaders {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (c *RGPVCrawler) extractPapersFromHTML(htmlContent, sourceURL string) ([]PYQCrawlerResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var papers []PYQCrawlerResult
	var extract func(*html.Node)

	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			var href, title string

			// Get href attribute
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href = attr.Val
				} else if attr.Key == "target" && attr.Val == "_blank" {
					// This is likely a paper link
				}
			}

			// Get link text (title)
			if n.FirstChild != nil {
				if n.FirstChild.Type == html.TextNode {
					title = strings.TrimSpace(n.FirstChild.Data)
				} else if n.FirstChild.Type == html.ElementNode && n.FirstChild.Data == "font" {
					// Handle <font> tags
					if n.FirstChild.FirstChild != nil && n.FirstChild.FirstChild.Type == html.TextNode {
						title = strings.TrimSpace(n.FirstChild.FirstChild.Data)
					}
				}
			}

			// Check if this is a paper link
			if href != "" && title != "" && strings.Contains(href, "/papers/") && strings.HasSuffix(href, ".html") {
				// Convert relative URL to absolute
				if !strings.HasPrefix(href, "http") {
					href = c.Config.BaseURL + href
				}

				// Create paper result
				paper, err := c.GetPaperMetadata(title, href)
				if err == nil {
					papers = append(papers, paper)
				}
			}
		}

		// Traverse children
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			extract(child)
		}
	}

	extract(doc)
	return papers, nil
}

func (c *RGPVCrawler) convertHTMLToPDFURL(htmlURL string) string {
	// Convert .html to .pdf
	// Example: https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.html
	//       -> https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.pdf
	return strings.ReplaceAll(htmlURL, ".html", ".pdf")
}

func (c *RGPVCrawler) parseRGPVTitle(title string, result *PYQCrawlerResult) {
	// Parse title format: "MCA-301-DATA-MINING-DEC-2024"
	// Or: "MCA-105-COMMUNICATION-SKILLS-NOV-2022"

	title = strings.ToUpper(strings.TrimSpace(title))
	parts := strings.Split(title, "-")

	if len(parts) < 3 {
		return
	}

	// Extract subject code (first 2 parts usually)
	// MCA-301 or MCA-101
	if len(parts) >= 2 {
		subjectCode := parts[0] + "-" + parts[1]
		result.SubjectCode = subjectCode
	}

	// Extract year from end
	if len(parts) >= 1 {
		lastPart := parts[len(parts)-1]
		if year, err := strconv.Atoi(lastPart); err == nil && year >= 2000 && year <= 2100 {
			result.Year = year

			// Extract month (second to last)
			if len(parts) >= 2 {
				month := parts[len(parts)-2]
				result.Month = c.normalizeMonth(month)

				// Extract subject name (everything between subject code and month-year)
				if len(parts) > 4 {
					subjectParts := parts[2 : len(parts)-2]
					result.SubjectName = strings.Join(subjectParts, " ")
					result.SubjectName = strings.Title(strings.ToLower(result.SubjectName))
				}
			}
		}
	}

	// Try to extract exam type
	result.ExamType = c.detectExamType(title)
}

func (c *RGPVCrawler) normalizeMonth(month string) string {
	month = strings.ToUpper(month)
	monthMap := map[string]string{
		"JAN": "January", "JANUARY": "January",
		"FEB": "February", "FEBRUARY": "February",
		"MAR": "March", "MARCH": "March",
		"APR": "April", "APRIL": "April",
		"MAY": "May",
		"JUN": "June", "JUNE": "June",
		"JUL": "July", "JULY": "July",
		"AUG": "August", "AUGUST": "August",
		"SEP": "September", "SEPTEMBER": "September",
		"OCT": "October", "OCTOBER": "October",
		"NOV": "November", "NOVEMBER": "November",
		"DEC": "December", "DECEMBER": "December",
	}

	if normalized, ok := monthMap[month]; ok {
		return normalized
	}
	return month
}

func (c *RGPVCrawler) detectExamType(title string) string {
	title = strings.ToUpper(title)

	// Check for common patterns
	patterns := map[string]*regexp.Regexp{
		"Supplementary": regexp.MustCompile(`SUPP|SUPPLEMENTARY|BACK`),
		"Mid Semester":  regexp.MustCompile(`MID|MIDTERM|MSE`),
		"Grading":       regexp.MustCompile(`GRADING|GRADE`),
	}

	for examType, pattern := range patterns {
		if pattern.MatchString(title) {
			return examType
		}
	}

	// Default to End Semester
	return "End Semester"
}
