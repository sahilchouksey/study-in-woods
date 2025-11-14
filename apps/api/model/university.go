package model

import (
	"gorm.io/gorm"
	"time"
)

// University represents an educational institution
type University struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"not null;uniqueIndex" json:"name"`
	Code      string         `gorm:"uniqueIndex;not null" json:"code"` // e.g., "AKTU", "DU"
	Location  string         `gorm:"type:varchar(255)" json:"location"`
	Website   string         `gorm:"type:varchar(255)" json:"website"`
	IsActive  bool           `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Courses []Course `gorm:"foreignKey:UniversityID;constraint:OnDelete:CASCADE" json:"courses,omitempty"`
}
