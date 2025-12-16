package pyq_crawler

import (
	"context"
)

// PYQCrawlerResult represents a single crawled PYQ paper
type PYQCrawlerResult struct {
	Title       string
	SourceURL   string
	PDFURL      string
	FileType    string
	SubjectCode string
	SubjectName string
	Year        int
	Month       string
	ExamType    string
}

// PYQCrawlerInterface defines the contract for all PYQ crawlers
type PYQCrawlerInterface interface {
	// GetName returns the unique identifier for this crawler
	GetName() string

	// GetDisplayName returns the human-readable name for this crawler
	GetDisplayName() string

	// GetBaseURL returns the base URL of the source
	GetBaseURL() string

	// SearchPapers searches for papers matching the given criteria
	// subjectCode: e.g., "MCA-301", "CS-101"
	// subjectName: e.g., "Data Mining", "Operating Systems"
	// course: e.g., "MCA", "B.Tech CSE"
	// semester: e.g., "3", "5"
	// limit: maximum number of results to return
	SearchPapers(ctx context.Context, subjectCode, subjectName, course, semester string, limit int) ([]PYQCrawlerResult, error)

	// GetAllPapers retrieves all available papers from the source
	// This is used for bulk indexing
	// course: e.g., "MCA", "B.Tech CSE"
	GetAllPapers(ctx context.Context, course string) ([]PYQCrawlerResult, error)

	// ValidatePDFURL checks if a PDF URL is still accessible
	ValidatePDFURL(ctx context.Context, pdfURL string) (bool, error)

	// GetPaperMetadata extracts metadata from a paper URL/title
	// This is useful for normalizing data from different sources
	GetPaperMetadata(title, url string) (PYQCrawlerResult, error)
}

// CrawlerConfig holds configuration for a crawler instance
type CrawlerConfig struct {
	Name           string
	DisplayName    string
	BaseURL        string
	Timeout        int // in seconds
	MaxRetries     int
	RateLimitDelay int // milliseconds between requests
	CustomHeaders  map[string]string
	CustomConfig   map[string]interface{} // crawler-specific config
}

// BaseCrawler provides common functionality for all crawlers
type BaseCrawler struct {
	Config CrawlerConfig
}

// GetName implements PYQCrawlerInterface
func (c *BaseCrawler) GetName() string {
	return c.Config.Name
}

// GetDisplayName implements PYQCrawlerInterface
func (c *BaseCrawler) GetDisplayName() string {
	return c.Config.DisplayName
}

// GetBaseURL implements PYQCrawlerInterface
func (c *BaseCrawler) GetBaseURL() string {
	return c.Config.BaseURL
}
