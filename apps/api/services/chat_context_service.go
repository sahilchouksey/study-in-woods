package services

import (
	"context"
	"fmt"

	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/gorm"
)

// ChatContextService provides data for chat context selection dropdowns
type ChatContextService struct {
	db *gorm.DB
}

// NewChatContextService creates a new chat context service
func NewChatContextService(db *gorm.DB) *ChatContextService {
	return &ChatContextService{
		db: db,
	}
}

// UniversityOption represents a university in the dropdown
type UniversityOption struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// CourseOption represents a course in the dropdown
type CourseOption struct {
	ID           uint   `json:"id"`
	UniversityID uint   `json:"university_id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Duration     int    `json:"duration"` // Duration in semesters
}

// SemesterOption represents a semester in the dropdown
type SemesterOption struct {
	ID       uint   `json:"id"`
	CourseID uint   `json:"course_id"`
	Number   int    `json:"number"`
	Name     string `json:"name"`
}

// SubjectOption represents a subject in the dropdown (only those with KB and Agent)
type SubjectOption struct {
	ID                uint   `json:"id"`
	SemesterID        uint   `json:"semester_id"`
	Name              string `json:"name"`
	Code              string `json:"code"`
	Credits           int    `json:"credits"`
	Description       string `json:"description,omitempty"`
	KnowledgeBaseUUID string `json:"knowledge_base_uuid"`
	AgentUUID         string `json:"agent_uuid"`
	HasSyllabus       bool   `json:"has_syllabus"`
}

// ChatContextResponse contains all dropdown data for chat setup
type ChatContextResponse struct {
	Universities []UniversityOption `json:"universities"`
	Courses      []CourseOption     `json:"courses"`
	Semesters    []SemesterOption   `json:"semesters"`
	Subjects     []SubjectOption    `json:"subjects"`
}

// GetChatContext retrieves all dropdown data in a single call
// Only returns subjects that have both KnowledgeBaseUUID and AgentUUID set
func (s *ChatContextService) GetChatContext(ctx context.Context) (*ChatContextResponse, error) {
	response := &ChatContextResponse{
		Universities: []UniversityOption{},
		Courses:      []CourseOption{},
		Semesters:    []SemesterOption{},
		Subjects:     []SubjectOption{},
	}

	// Get all active universities
	var universities []model.University
	if err := s.db.Where("is_active = ?", true).
		Order("name ASC").
		Find(&universities).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch universities: %w", err)
	}

	for _, u := range universities {
		response.Universities = append(response.Universities, UniversityOption{
			ID:   u.ID,
			Name: u.Name,
			Code: u.Code,
		})
	}

	// Get all courses
	var courses []model.Course
	if err := s.db.Order("name ASC").Find(&courses).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch courses: %w", err)
	}

	for _, c := range courses {
		response.Courses = append(response.Courses, CourseOption{
			ID:           c.ID,
			UniversityID: c.UniversityID,
			Name:         c.Name,
			Code:         c.Code,
			Duration:     c.Duration,
		})
	}

	// Get all semesters
	var semesters []model.Semester
	if err := s.db.Order("course_id ASC, number ASC").Find(&semesters).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch semesters: %w", err)
	}

	for _, sem := range semesters {
		response.Semesters = append(response.Semesters, SemesterOption{
			ID:       sem.ID,
			CourseID: sem.CourseID,
			Number:   sem.Number,
			Name:     sem.Name,
		})
	}

	// Get subjects that have BOTH KnowledgeBaseUUID AND AgentUUID set (non-empty)
	var subjects []model.Subject
	if err := s.db.Where("knowledge_base_uuid != '' AND knowledge_base_uuid IS NOT NULL").
		Where("agent_uuid != '' AND agent_uuid IS NOT NULL").
		Order("semester_id ASC, name ASC").
		Find(&subjects).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch subjects: %w", err)
	}

	// Check which subjects have syllabus data
	subjectIDs := make([]uint, len(subjects))
	for i, sub := range subjects {
		subjectIDs[i] = sub.ID
	}

	// Get subjects that have completed syllabus extraction
	var syllabusSubjectIDs []uint
	if len(subjectIDs) > 0 {
		if err := s.db.Model(&model.Syllabus{}).
			Where("subject_id IN ?", subjectIDs).
			Where("extraction_status = ?", model.SyllabusExtractionCompleted).
			Distinct().
			Pluck("subject_id", &syllabusSubjectIDs).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch syllabus data: %w", err)
		}
	}

	// Create a map for quick lookup
	hasSyllabusMap := make(map[uint]bool)
	for _, id := range syllabusSubjectIDs {
		hasSyllabusMap[id] = true
	}

	for _, sub := range subjects {
		response.Subjects = append(response.Subjects, SubjectOption{
			ID:                sub.ID,
			SemesterID:        sub.SemesterID,
			Name:              sub.Name,
			Code:              sub.Code,
			Credits:           sub.Credits,
			Description:       sub.Description,
			KnowledgeBaseUUID: sub.KnowledgeBaseUUID,
			AgentUUID:         sub.AgentUUID,
			HasSyllabus:       hasSyllabusMap[sub.ID],
		})
	}

	return response, nil
}

// GetUniversities retrieves all active universities
func (s *ChatContextService) GetUniversities(ctx context.Context) ([]UniversityOption, error) {
	var universities []model.University
	if err := s.db.Where("is_active = ?", true).
		Order("name ASC").
		Find(&universities).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch universities: %w", err)
	}

	options := make([]UniversityOption, len(universities))
	for i, u := range universities {
		options[i] = UniversityOption{
			ID:   u.ID,
			Name: u.Name,
			Code: u.Code,
		}
	}

	return options, nil
}

// GetCoursesByUniversity retrieves courses for a specific university
func (s *ChatContextService) GetCoursesByUniversity(ctx context.Context, universityID uint) ([]CourseOption, error) {
	var courses []model.Course
	if err := s.db.Where("university_id = ?", universityID).
		Order("name ASC").
		Find(&courses).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch courses: %w", err)
	}

	options := make([]CourseOption, len(courses))
	for i, c := range courses {
		options[i] = CourseOption{
			ID:           c.ID,
			UniversityID: c.UniversityID,
			Name:         c.Name,
			Code:         c.Code,
			Duration:     c.Duration,
		}
	}

	return options, nil
}

// GetSemestersByCourse retrieves semesters for a specific course
func (s *ChatContextService) GetSemestersByCourse(ctx context.Context, courseID uint) ([]SemesterOption, error) {
	var semesters []model.Semester
	if err := s.db.Where("course_id = ?", courseID).
		Order("number ASC").
		Find(&semesters).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch semesters: %w", err)
	}

	options := make([]SemesterOption, len(semesters))
	for i, sem := range semesters {
		options[i] = SemesterOption{
			ID:       sem.ID,
			CourseID: sem.CourseID,
			Number:   sem.Number,
			Name:     sem.Name,
		}
	}

	return options, nil
}

// GetSubjectsBySemester retrieves subjects for a specific semester
// Only returns subjects that have both KnowledgeBaseUUID and AgentUUID set
func (s *ChatContextService) GetSubjectsBySemester(ctx context.Context, semesterID uint) ([]SubjectOption, error) {
	var subjects []model.Subject
	if err := s.db.Where("semester_id = ?", semesterID).
		Where("knowledge_base_uuid != '' AND knowledge_base_uuid IS NOT NULL").
		Where("agent_uuid != '' AND agent_uuid IS NOT NULL").
		Order("name ASC").
		Find(&subjects).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch subjects: %w", err)
	}

	// Check which subjects have syllabus data
	subjectIDs := make([]uint, len(subjects))
	for i, sub := range subjects {
		subjectIDs[i] = sub.ID
	}

	var syllabusSubjectIDs []uint
	if len(subjectIDs) > 0 {
		if err := s.db.Model(&model.Syllabus{}).
			Where("subject_id IN ?", subjectIDs).
			Where("extraction_status = ?", model.SyllabusExtractionCompleted).
			Distinct().
			Pluck("subject_id", &syllabusSubjectIDs).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch syllabus data: %w", err)
		}
	}

	hasSyllabusMap := make(map[uint]bool)
	for _, id := range syllabusSubjectIDs {
		hasSyllabusMap[id] = true
	}

	options := make([]SubjectOption, len(subjects))
	for i, sub := range subjects {
		options[i] = SubjectOption{
			ID:                sub.ID,
			SemesterID:        sub.SemesterID,
			Name:              sub.Name,
			Code:              sub.Code,
			Credits:           sub.Credits,
			Description:       sub.Description,
			KnowledgeBaseUUID: sub.KnowledgeBaseUUID,
			AgentUUID:         sub.AgentUUID,
			HasSyllabus:       hasSyllabusMap[sub.ID],
		}
	}

	return options, nil
}

// SubjectSyllabusContext contains syllabus information for chat prompts
type SubjectSyllabusContext struct {
	SubjectName   string                `json:"subject_name"`
	SubjectCode   string                `json:"subject_code"`
	TotalCredits  int                   `json:"total_credits"`
	Units         []SyllabusUnitContext `json:"units"`
	Books         []SyllabusBookContext `json:"books"`
	FormattedText string                `json:"formatted_text"` // Pre-formatted for prompt injection
}

// SyllabusUnitContext represents a unit for chat context
type SyllabusUnitContext struct {
	UnitNumber  int      `json:"unit_number"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Topics      []string `json:"topics,omitempty"`
	Hours       int      `json:"hours,omitempty"`
}

// SyllabusBookContext represents a book reference for chat context
type SyllabusBookContext struct {
	Title      string `json:"title"`
	Authors    string `json:"authors"`
	IsTextbook bool   `json:"is_textbook"`
}

// GetSubjectSyllabusContext retrieves syllabus data formatted for chat prompts
func (s *ChatContextService) GetSubjectSyllabusContext(ctx context.Context, subjectID uint) (*SubjectSyllabusContext, error) {
	// Get the most recent completed syllabus for this subject
	var syllabus model.Syllabus
	if err := s.db.Where("subject_id = ?", subjectID).
		Where("extraction_status = ?", model.SyllabusExtractionCompleted).
		Preload("Units", func(db *gorm.DB) *gorm.DB {
			return db.Order("unit_number ASC")
		}).
		Preload("Units.Topics", func(db *gorm.DB) *gorm.DB {
			return db.Order("topic_number ASC")
		}).
		Preload("Books").
		Order("created_at DESC").
		First(&syllabus).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No syllabus found, return nil without error
		}
		return nil, fmt.Errorf("failed to fetch syllabus: %w", err)
	}

	context := &SubjectSyllabusContext{
		SubjectName:  syllabus.SubjectName,
		SubjectCode:  syllabus.SubjectCode,
		TotalCredits: syllabus.TotalCredits,
		Units:        make([]SyllabusUnitContext, 0, len(syllabus.Units)),
		Books:        make([]SyllabusBookContext, 0, len(syllabus.Books)),
	}

	// Process units
	for _, unit := range syllabus.Units {
		unitCtx := SyllabusUnitContext{
			UnitNumber:  unit.UnitNumber,
			Title:       unit.Title,
			Description: unit.Description,
			Hours:       unit.Hours,
			Topics:      make([]string, 0, len(unit.Topics)),
		}

		for _, topic := range unit.Topics {
			unitCtx.Topics = append(unitCtx.Topics, topic.Title)
		}

		context.Units = append(context.Units, unitCtx)
	}

	// Process books
	for _, book := range syllabus.Books {
		context.Books = append(context.Books, SyllabusBookContext{
			Title:      book.Title,
			Authors:    book.Authors,
			IsTextbook: book.IsTextbook,
		})
	}

	// Generate formatted text for prompt injection
	context.FormattedText = s.formatSyllabusForPrompt(context)

	return context, nil
}

// formatSyllabusForPrompt creates a well-structured text representation of the syllabus
func (s *ChatContextService) formatSyllabusForPrompt(ctx *SubjectSyllabusContext) string {
	var result string

	result = fmt.Sprintf("## Syllabus for %s (%s)\n", ctx.SubjectName, ctx.SubjectCode)
	result += fmt.Sprintf("Total Credits: %d\n\n", ctx.TotalCredits)

	// Add units
	result += "### Course Units:\n\n"
	for _, unit := range ctx.Units {
		result += fmt.Sprintf("**Unit %d: %s**", unit.UnitNumber, unit.Title)
		if unit.Hours > 0 {
			result += fmt.Sprintf(" (%d hours)", unit.Hours)
		}
		result += "\n"

		if unit.Description != "" {
			result += unit.Description + "\n"
		}

		if len(unit.Topics) > 0 {
			result += "Topics: "
			for i, topic := range unit.Topics {
				if i > 0 {
					result += ", "
				}
				result += topic
			}
			result += "\n"
		}
		result += "\n"
	}

	// Add recommended books
	if len(ctx.Books) > 0 {
		result += "### Recommended Books:\n"
		for _, book := range ctx.Books {
			bookType := "Reference"
			if book.IsTextbook {
				bookType = "Textbook"
			}
			result += fmt.Sprintf("- %s by %s (%s)\n", book.Title, book.Authors, bookType)
		}
	}

	return result
}
