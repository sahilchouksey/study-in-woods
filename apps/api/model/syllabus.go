package model

import (
	"time"

	"gorm.io/gorm"
)

// SyllabusExtractionStatus represents the status of syllabus extraction
type SyllabusExtractionStatus string

const (
	SyllabusExtractionPending    SyllabusExtractionStatus = "pending"
	SyllabusExtractionProcessing SyllabusExtractionStatus = "processing"
	SyllabusExtractionCompleted  SyllabusExtractionStatus = "completed"
	SyllabusExtractionFailed     SyllabusExtractionStatus = "failed"
)

// Syllabus represents parsed syllabus data for a subject
type Syllabus struct {
	ID               uint                     `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
	DeletedAt        gorm.DeletedAt           `gorm:"index" json:"-"`
	SubjectID        uint                     `gorm:"uniqueIndex;not null" json:"subject_id"` // One syllabus per subject
	DocumentID       uint                     `gorm:"index" json:"document_id"`               // Source document
	SubjectName      string                   `gorm:"type:varchar(255)" json:"subject_name"`
	SubjectCode      string                   `gorm:"type:varchar(50)" json:"subject_code"`
	TotalCredits     int                      `gorm:"default:0" json:"total_credits"`
	ExtractionStatus SyllabusExtractionStatus `gorm:"type:varchar(20);default:'pending'" json:"extraction_status"`
	ExtractionError  string                   `gorm:"type:text" json:"extraction_error,omitempty"`
	RawExtraction    string                   `gorm:"type:text" json:"raw_extraction,omitempty"` // Store raw LLM output for debugging

	// Relationships
	Subject  Subject         `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	Document Document        `gorm:"foreignKey:DocumentID;constraint:OnDelete:SET NULL" json:"document,omitempty"`
	Units    []SyllabusUnit  `gorm:"foreignKey:SyllabusID;constraint:OnDelete:CASCADE" json:"units,omitempty"`
	Books    []BookReference `gorm:"foreignKey:SyllabusID;constraint:OnDelete:CASCADE" json:"books,omitempty"`
}

// SyllabusUnit represents a unit/module in the syllabus
type SyllabusUnit struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	SyllabusID  uint           `gorm:"not null;index" json:"syllabus_id"`
	UnitNumber  int            `gorm:"not null" json:"unit_number"` // 1, 2, 3, etc.
	Title       string         `gorm:"type:varchar(255);not null" json:"title"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	Hours       int            `gorm:"default:0" json:"hours"` // Teaching hours allocated

	// Relationships
	Syllabus Syllabus        `gorm:"foreignKey:SyllabusID;constraint:OnDelete:CASCADE" json:"-"`
	Topics   []SyllabusTopic `gorm:"foreignKey:UnitID;constraint:OnDelete:CASCADE" json:"topics,omitempty"`
}

// SyllabusTopic represents a topic within a unit
type SyllabusTopic struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	UnitID      uint           `gorm:"not null;index" json:"unit_id"`
	TopicNumber int            `gorm:"not null" json:"topic_number"` // Order within the unit
	Title       string         `gorm:"type:varchar(255);not null" json:"title"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	Keywords    string         `gorm:"type:text" json:"keywords,omitempty"` // Comma-separated keywords for search

	// Relationships
	Unit SyllabusUnit `gorm:"foreignKey:UnitID;constraint:OnDelete:CASCADE" json:"-"`
}

// BookReference represents a book/reference material in the syllabus
type BookReference struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	SyllabusID uint           `gorm:"not null;index" json:"syllabus_id"`
	Title      string         `gorm:"type:varchar(500);not null" json:"title"`
	Authors    string         `gorm:"type:varchar(500)" json:"authors"` // Comma-separated authors
	Publisher  string         `gorm:"type:varchar(255)" json:"publisher,omitempty"`
	Edition    string         `gorm:"type:varchar(50)" json:"edition,omitempty"`
	Year       int            `gorm:"default:0" json:"year,omitempty"`
	ISBN       string         `gorm:"type:varchar(20)" json:"isbn,omitempty"`
	IsTextbook bool           `gorm:"default:false" json:"is_textbook"`                      // Primary textbook vs reference
	BookType   string         `gorm:"type:varchar(50);default:'reference'" json:"book_type"` // textbook, reference, recommended

	// Relationships
	Syllabus Syllabus `gorm:"foreignKey:SyllabusID;constraint:OnDelete:CASCADE" json:"-"`
}

// SyllabusResponse is used for API responses with complete syllabus data
type SyllabusResponse struct {
	ID               uint                     `json:"id"`
	SubjectID        uint                     `json:"subject_id"`
	SubjectName      string                   `json:"subject_name"`
	SubjectCode      string                   `json:"subject_code"`
	TotalCredits     int                      `json:"total_credits"`
	ExtractionStatus SyllabusExtractionStatus `json:"extraction_status"`
	Units            []SyllabusUnitResponse   `json:"units"`
	Books            []BookReferenceResponse  `json:"books"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
}

// SyllabusUnitResponse is used for API responses
type SyllabusUnitResponse struct {
	ID          uint                    `json:"id"`
	UnitNumber  int                     `json:"unit_number"`
	Title       string                  `json:"title"`
	Description string                  `json:"description,omitempty"`
	Hours       int                     `json:"hours,omitempty"`
	Topics      []SyllabusTopicResponse `json:"topics"`
}

// SyllabusTopicResponse is used for API responses
type SyllabusTopicResponse struct {
	ID          uint   `json:"id"`
	TopicNumber int    `json:"topic_number"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Keywords    string `json:"keywords,omitempty"`
}

// BookReferenceResponse is used for API responses
type BookReferenceResponse struct {
	ID         uint   `json:"id"`
	Title      string `json:"title"`
	Authors    string `json:"authors"`
	Publisher  string `json:"publisher,omitempty"`
	Edition    string `json:"edition,omitempty"`
	Year       int    `json:"year,omitempty"`
	ISBN       string `json:"isbn,omitempty"`
	IsTextbook bool   `json:"is_textbook"`
	BookType   string `json:"book_type"`
}

// ToResponse converts Syllabus model to SyllabusResponse
func (s *Syllabus) ToResponse() *SyllabusResponse {
	response := &SyllabusResponse{
		ID:               s.ID,
		SubjectID:        s.SubjectID,
		SubjectName:      s.SubjectName,
		SubjectCode:      s.SubjectCode,
		TotalCredits:     s.TotalCredits,
		ExtractionStatus: s.ExtractionStatus,
		Units:            make([]SyllabusUnitResponse, 0),
		Books:            make([]BookReferenceResponse, 0),
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}

	for _, unit := range s.Units {
		unitResp := SyllabusUnitResponse{
			ID:          unit.ID,
			UnitNumber:  unit.UnitNumber,
			Title:       unit.Title,
			Description: unit.Description,
			Hours:       unit.Hours,
			Topics:      make([]SyllabusTopicResponse, 0),
		}

		for _, topic := range unit.Topics {
			unitResp.Topics = append(unitResp.Topics, SyllabusTopicResponse{
				ID:          topic.ID,
				TopicNumber: topic.TopicNumber,
				Title:       topic.Title,
				Description: topic.Description,
				Keywords:    topic.Keywords,
			})
		}

		response.Units = append(response.Units, unitResp)
	}

	for _, book := range s.Books {
		response.Books = append(response.Books, BookReferenceResponse{
			ID:         book.ID,
			Title:      book.Title,
			Authors:    book.Authors,
			Publisher:  book.Publisher,
			Edition:    book.Edition,
			Year:       book.Year,
			ISBN:       book.ISBN,
			IsTextbook: book.IsTextbook,
			BookType:   book.BookType,
		})
	}

	return response
}
