package model

import (
	"gorm.io/gorm"
	"time"
)

// AppSetting represents application-wide configuration settings
type AppSetting struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Key         string         `gorm:"uniqueIndex;not null" json:"key"`
	Value       string         `gorm:"type:text;not null" json:"value"`
	Type        string         `gorm:"type:varchar(20);default:'string'" json:"type"` // string, int, bool, json
	Description string         `gorm:"type:text" json:"description"`
	IsPublic    bool           `gorm:"default:false" json:"is_public"` // If true, can be accessed without auth
	Category    string         `gorm:"type:varchar(50)" json:"category"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for AppSetting
func (AppSetting) TableName() string {
	return "app_settings"
}
