package model

import (
	"gorm.io/gorm"
	"time"
)

// AdminAuditLog represents audit trail for admin actions
type AdminAuditLog struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	AdminID     uint           `gorm:"not null;index" json:"admin_id"`
	Action      string         `gorm:"type:varchar(100);not null" json:"action"` // e.g., "user_delete", "settings_update"
	Resource    string         `gorm:"type:varchar(100)" json:"resource"`        // e.g., "users", "courses"
	ResourceID  uint           `json:"resource_id"`
	OldValue    string         `gorm:"type:jsonb" json:"old_value"`
	NewValue    string         `gorm:"type:jsonb" json:"new_value"`
	IPAddress   string         `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent   string         `gorm:"type:text" json:"user_agent"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Admin User `gorm:"foreignKey:AdminID;constraint:OnDelete:CASCADE" json:"admin,omitempty"`
}

// TableName specifies the table name for AdminAuditLog
func (AdminAuditLog) TableName() string {
	return "admin_audit_logs"
}
