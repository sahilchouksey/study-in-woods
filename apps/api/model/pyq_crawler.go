package model

import (
	"time"

	"gorm.io/gorm"
)

// PYQCrawlerSource represents a source website for PYQ papers
type PYQCrawlerSource struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"type:varchar(100);not null;unique" json:"name"`  // e.g., "RGPV", "AKTU"
	DisplayName string         `gorm:"type:varchar(100);not null" json:"display_name"` // e.g., "RGPV Online"
	BaseURL     string         `gorm:"type:varchar(500);not null" json:"base_url"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	Priority    int            `gorm:"default:0" json:"priority"` // Higher priority sources are checked first
	Description string         `gorm:"type:text" json:"description,omitempty"`

	// Metadata for crawler configuration
	CrawlerType    string     `gorm:"type:varchar(50);not null" json:"crawler_type"` // e.g., "rgpv", "generic_html"
	ConfigJSON     string     `gorm:"type:text" json:"config_json,omitempty"`        // Additional config specific to crawler
	LastCrawledAt  *time.Time `json:"last_crawled_at,omitempty"`
	CrawlFrequency int        `gorm:"default:86400" json:"crawl_frequency"` // In seconds, default 24 hours
}

// PYQCrawledPaper represents a paper discovered by the crawler (before ingestion)
type PYQCrawledPaper struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	SourceID  uint           `gorm:"not null;index" json:"source_id"`

	// Paper identification
	Title     string `gorm:"type:varchar(500);not null" json:"title"`
	SourceURL string `gorm:"type:varchar(1000);not null" json:"source_url"` // HTML page URL
	PDFURL    string `gorm:"type:varchar(1000);not null" json:"pdf_url"`
	FileType  string `gorm:"type:varchar(10);default:'pdf'" json:"file_type"` // pdf, doc, docx, txt

	// Extracted metadata from title/URL
	SubjectCode string `gorm:"type:varchar(50)" json:"subject_code,omitempty"`
	SubjectName string `gorm:"type:varchar(200)" json:"subject_name,omitempty"`
	Year        int    `json:"year,omitempty"`
	Month       string `gorm:"type:varchar(20)" json:"month,omitempty"`
	ExamType    string `gorm:"type:varchar(50)" json:"exam_type,omitempty"`

	// Ingestion status
	IsIngested      bool   `gorm:"default:false;index" json:"is_ingested"`
	IngestedPaperID *uint  `gorm:"index" json:"ingested_paper_id,omitempty"` // FK to PYQPaper
	IngestionError  string `gorm:"type:text" json:"ingestion_error,omitempty"`

	// Relationships
	Source        PYQCrawlerSource `gorm:"foreignKey:SourceID;constraint:OnDelete:CASCADE" json:"source,omitempty"`
	IngestedPaper *PYQPaper        `gorm:"foreignKey:IngestedPaperID;constraint:OnDelete:SET NULL" json:"ingested_paper,omitempty"`
}

// PYQCrawlerSourceResponse for API
type PYQCrawlerSourceResponse struct {
	ID             uint       `json:"id"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"display_name"`
	BaseURL        string     `json:"base_url"`
	IsActive       bool       `json:"is_active"`
	Priority       int        `json:"priority"`
	Description    string     `json:"description,omitempty"`
	LastCrawledAt  *time.Time `json:"last_crawled_at,omitempty"`
	CrawlFrequency int        `json:"crawl_frequency"`
}

// PYQCrawledPaperResponse for API
type PYQCrawledPaperResponse struct {
	ID              uint      `json:"id"`
	SourceID        uint      `json:"source_id"`
	SourceName      string    `json:"source_name"`
	Title           string    `json:"title"`
	PDFURL          string    `json:"pdf_url"`
	FileType        string    `json:"file_type"`
	SubjectCode     string    `json:"subject_code,omitempty"`
	SubjectName     string    `json:"subject_name,omitempty"`
	Year            int       `json:"year,omitempty"`
	Month           string    `json:"month,omitempty"`
	ExamType        string    `json:"exam_type,omitempty"`
	IsIngested      bool      `json:"is_ingested"`
	IngestedPaperID *uint     `json:"ingested_paper_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// ToResponse converts PYQCrawlerSource to response
func (s *PYQCrawlerSource) ToResponse() PYQCrawlerSourceResponse {
	return PYQCrawlerSourceResponse{
		ID:             s.ID,
		Name:           s.Name,
		DisplayName:    s.DisplayName,
		BaseURL:        s.BaseURL,
		IsActive:       s.IsActive,
		Priority:       s.Priority,
		Description:    s.Description,
		LastCrawledAt:  s.LastCrawledAt,
		CrawlFrequency: s.CrawlFrequency,
	}
}

// ToResponse converts PYQCrawledPaper to response
func (p *PYQCrawledPaper) ToResponse() PYQCrawledPaperResponse {
	sourceName := ""
	if p.Source.Name != "" {
		sourceName = p.Source.DisplayName
	}

	return PYQCrawledPaperResponse{
		ID:              p.ID,
		SourceID:        p.SourceID,
		SourceName:      sourceName,
		Title:           p.Title,
		PDFURL:          p.PDFURL,
		FileType:        p.FileType,
		SubjectCode:     p.SubjectCode,
		SubjectName:     p.SubjectName,
		Year:            p.Year,
		Month:           p.Month,
		ExamType:        p.ExamType,
		IsIngested:      p.IsIngested,
		IngestedPaperID: p.IngestedPaperID,
		CreatedAt:       p.CreatedAt,
	}
}
