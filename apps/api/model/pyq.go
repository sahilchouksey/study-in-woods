package model

import (
	"time"

	"gorm.io/gorm"
)

// PYQExtractionStatus represents the status of PYQ extraction
type PYQExtractionStatus string

const (
	PYQExtractionPending    PYQExtractionStatus = "pending"
	PYQExtractionProcessing PYQExtractionStatus = "processing"
	PYQExtractionCompleted  PYQExtractionStatus = "completed"
	PYQExtractionFailed     PYQExtractionStatus = "failed"
)

// PYQPaper represents a previous year question paper for a subject
type PYQPaper struct {
	ID               uint                `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	DeletedAt        gorm.DeletedAt      `gorm:"index" json:"-"`
	SubjectID        uint                `gorm:"index;not null" json:"subject_id"`
	DocumentID       uint                `gorm:"index" json:"document_id"`                    // Source document
	Year             int                 `gorm:"not null" json:"year"`                        // e.g., 2023, 2024
	Month            string              `gorm:"type:varchar(20)" json:"month,omitempty"`     // e.g., "December", "May"
	ExamType         string              `gorm:"type:varchar(50)" json:"exam_type,omitempty"` // e.g., "End Semester", "Mid Semester", "Supplementary"
	TotalMarks       int                 `gorm:"default:0" json:"total_marks"`
	Duration         string              `gorm:"type:varchar(50)" json:"duration,omitempty"` // e.g., "3 hours"
	TotalQuestions   int                 `gorm:"default:0" json:"total_questions"`
	Instructions     string              `gorm:"type:text" json:"instructions,omitempty"`
	ExtractionStatus PYQExtractionStatus `gorm:"type:varchar(20);default:'pending'" json:"extraction_status"`
	ExtractionError  string              `gorm:"type:text" json:"extraction_error,omitempty"`
	RawExtraction    string              `gorm:"type:text" json:"raw_extraction,omitempty"` // Store raw LLM output

	// Relationships
	Subject   Subject       `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	Document  Document      `gorm:"foreignKey:DocumentID;constraint:OnDelete:SET NULL" json:"document,omitempty"`
	Questions []PYQQuestion `gorm:"foreignKey:PaperID;constraint:OnDelete:CASCADE" json:"questions,omitempty"`
}

// PYQQuestion represents a question in the paper
type PYQQuestion struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	PaperID        uint           `gorm:"not null;index" json:"paper_id"`
	QuestionNumber string         `gorm:"type:varchar(20);not null" json:"question_number"` // e.g., "1", "2a", "2b"
	SectionName    string         `gorm:"type:varchar(50)" json:"section_name,omitempty"`   // e.g., "Section A", "Part I"
	QuestionText   string         `gorm:"type:text;not null" json:"question_text"`
	Marks          int            `gorm:"default:0" json:"marks"`
	IsCompulsory   bool           `gorm:"default:true" json:"is_compulsory"`
	HasChoices     bool           `gorm:"default:false" json:"has_choices"`               // If true, student can choose between choices
	ChoiceGroup    string         `gorm:"type:varchar(20)" json:"choice_group,omitempty"` // Groups questions that are alternatives (e.g., "Q1")
	UnitNumber     int            `gorm:"default:0" json:"unit_number,omitempty"`         // Which unit this question is from
	TopicKeywords  string         `gorm:"type:text" json:"topic_keywords,omitempty"`      // Comma-separated keywords

	// Relationships
	Paper   PYQPaper            `gorm:"foreignKey:PaperID;constraint:OnDelete:CASCADE" json:"-"`
	Choices []PYQQuestionChoice `gorm:"foreignKey:QuestionID;constraint:OnDelete:CASCADE" json:"choices,omitempty"`
}

// PYQQuestionChoice represents an alternative choice within a question
// This is for questions like "Answer any ONE: (a) Explain X OR (b) Explain Y"
type PYQQuestionChoice struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	QuestionID  uint           `gorm:"not null;index" json:"question_id"`
	ChoiceLabel string         `gorm:"type:varchar(10)" json:"choice_label"` // e.g., "a", "b", "OR"
	ChoiceText  string         `gorm:"type:text;not null" json:"choice_text"`
	Marks       int            `gorm:"default:0" json:"marks,omitempty"` // If different from parent question

	// Relationships
	Question PYQQuestion `gorm:"foreignKey:QuestionID;constraint:OnDelete:CASCADE" json:"-"`
}

// ============= Response Types =============

// PYQPaperResponse is used for API responses
type PYQPaperResponse struct {
	ID               uint                  `json:"id"`
	SubjectID        uint                  `json:"subject_id"`
	Year             int                   `json:"year"`
	Month            string                `json:"month,omitempty"`
	ExamType         string                `json:"exam_type,omitempty"`
	TotalMarks       int                   `json:"total_marks"`
	Duration         string                `json:"duration,omitempty"`
	TotalQuestions   int                   `json:"total_questions"`
	Instructions     string                `json:"instructions,omitempty"`
	ExtractionStatus PYQExtractionStatus   `json:"extraction_status"`
	ExtractionError  string                `json:"extraction_error,omitempty"`
	Questions        []PYQQuestionResponse `json:"questions"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// PYQQuestionResponse is used for API responses
type PYQQuestionResponse struct {
	ID             uint                        `json:"id"`
	QuestionNumber string                      `json:"question_number"`
	SectionName    string                      `json:"section_name,omitempty"`
	QuestionText   string                      `json:"question_text"`
	Marks          int                         `json:"marks"`
	IsCompulsory   bool                        `json:"is_compulsory"`
	HasChoices     bool                        `json:"has_choices"`
	ChoiceGroup    string                      `json:"choice_group,omitempty"`
	UnitNumber     int                         `json:"unit_number,omitempty"`
	TopicKeywords  string                      `json:"topic_keywords,omitempty"`
	Choices        []PYQQuestionChoiceResponse `json:"choices,omitempty"`
}

// PYQQuestionChoiceResponse is used for API responses
type PYQQuestionChoiceResponse struct {
	ID          uint   `json:"id"`
	ChoiceLabel string `json:"choice_label"`
	ChoiceText  string `json:"choice_text"`
	Marks       int    `json:"marks,omitempty"`
}

// ToResponse converts PYQPaper model to PYQPaperResponse
func (p *PYQPaper) ToResponse() *PYQPaperResponse {
	response := &PYQPaperResponse{
		ID:               p.ID,
		SubjectID:        p.SubjectID,
		Year:             p.Year,
		Month:            p.Month,
		ExamType:         p.ExamType,
		TotalMarks:       p.TotalMarks,
		Duration:         p.Duration,
		TotalQuestions:   p.TotalQuestions,
		Instructions:     p.Instructions,
		ExtractionStatus: p.ExtractionStatus,
		ExtractionError:  p.ExtractionError,
		Questions:        make([]PYQQuestionResponse, 0),
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}

	for _, question := range p.Questions {
		questionResp := PYQQuestionResponse{
			ID:             question.ID,
			QuestionNumber: question.QuestionNumber,
			SectionName:    question.SectionName,
			QuestionText:   question.QuestionText,
			Marks:          question.Marks,
			IsCompulsory:   question.IsCompulsory,
			HasChoices:     question.HasChoices,
			ChoiceGroup:    question.ChoiceGroup,
			UnitNumber:     question.UnitNumber,
			TopicKeywords:  question.TopicKeywords,
			Choices:        make([]PYQQuestionChoiceResponse, 0),
		}

		for _, choice := range question.Choices {
			questionResp.Choices = append(questionResp.Choices, PYQQuestionChoiceResponse{
				ID:          choice.ID,
				ChoiceLabel: choice.ChoiceLabel,
				ChoiceText:  choice.ChoiceText,
				Marks:       choice.Marks,
			})
		}

		response.Questions = append(response.Questions, questionResp)
	}

	return response
}

// PYQPapersListResponse for listing multiple papers
type PYQPapersListResponse struct {
	Papers []PYQPaperSummary `json:"papers"`
	Total  int               `json:"total"`
}

// PYQPaperSummary is a lightweight version for listing
type PYQPaperSummary struct {
	ID               uint                `json:"id"`
	Year             int                 `json:"year"`
	Month            string              `json:"month,omitempty"`
	ExamType         string              `json:"exam_type,omitempty"`
	TotalMarks       int                 `json:"total_marks"`
	TotalQuestions   int                 `json:"total_questions"`
	ExtractionStatus PYQExtractionStatus `json:"extraction_status"`
	CreatedAt        time.Time           `json:"created_at"`
}

// ToSummary converts PYQPaper to PYQPaperSummary
func (p *PYQPaper) ToSummary() PYQPaperSummary {
	return PYQPaperSummary{
		ID:               p.ID,
		Year:             p.Year,
		Month:            p.Month,
		ExamType:         p.ExamType,
		TotalMarks:       p.TotalMarks,
		TotalQuestions:   p.TotalQuestions,
		ExtractionStatus: p.ExtractionStatus,
		CreatedAt:        p.CreatedAt,
	}
}
