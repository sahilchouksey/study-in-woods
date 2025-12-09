package model

import (
	"time"

	"gorm.io/gorm"
)

// Course represents an academic program (e.g., MCA, BCA)
type Course struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	UniversityID uint           `gorm:"not null;index" json:"university_id"`
	Name         string         `gorm:"not null" json:"name"`
	Code         string         `gorm:"uniqueIndex;not null" json:"code"` // e.g., "MCA", "BCA"
	Description  string         `gorm:"type:text" json:"description"`
	Duration     int            `gorm:"default:4" json:"duration"` // Duration in semesters

	// Relationships
	University University   `gorm:"foreignKey:UniversityID;constraint:OnDelete:CASCADE" json:"university,omitempty"`
	Semesters  []Semester   `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"semesters,omitempty"`
	Users      []UserCourse `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"-"`
}

// Semester represents an academic term within a course
type Semester struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CourseID  uint           `gorm:"not null;index" json:"course_id"`
	Number    int            `gorm:"not null" json:"number"`       // 1, 2, 3, etc.
	Name      string         `gorm:"type:varchar(50)" json:"name"` // e.g., "Semester 1", "First Year"

	// Relationships
	Course   Course    `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"course,omitempty"`
	Subjects []Subject `gorm:"foreignKey:SemesterID;constraint:OnDelete:CASCADE" json:"subjects,omitempty"`
}

// Subject represents an individual academic subject
type Subject struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
	SemesterID        uint           `gorm:"not null;index" json:"semester_id"`
	Name              string         `gorm:"not null" json:"name"`
	Code              string         `gorm:"not null" json:"code"`
	Credits           int            `gorm:"default:0" json:"credits"`
	Description       string         `gorm:"type:text" json:"description"`
	KnowledgeBaseUUID string         `gorm:"type:varchar(100)" json:"knowledge_base_uuid"`
	AgentUUID         string         `gorm:"type:varchar(100)" json:"agent_uuid"`

	// Relationships
	Semester     Semester      `gorm:"foreignKey:SemesterID;constraint:OnDelete:CASCADE" json:"semester,omitempty"`
	Documents    []Document    `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"documents,omitempty"`
	ChatSessions []ChatSession `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"-"`
}
