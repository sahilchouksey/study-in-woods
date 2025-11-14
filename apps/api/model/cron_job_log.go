package model

import (
	"gorm.io/gorm"
	"time"
)

// CronJobLog represents execution logs for background cron jobs
type CronJobLog struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	JobName     string         `gorm:"type:varchar(100);not null;index" json:"job_name"`
	Status      string         `gorm:"type:varchar(20);not null" json:"status"` // started, completed, failed
	StartedAt   time.Time      `gorm:"not null" json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	Duration    int            `json:"duration_ms"` // Duration in milliseconds
	Message     string         `gorm:"type:text" json:"message"`
	ErrorMsg    string         `gorm:"type:text" json:"error_msg"`
	Metadata    string         `gorm:"type:jsonb" json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for CronJobLog
func (CronJobLog) TableName() string {
	return "cron_job_logs"
}
