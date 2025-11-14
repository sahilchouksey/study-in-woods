package model

import (
	"gorm.io/gorm"
	"time"
)

// CoursePayment represents a payment record for course enrollment
type CoursePayment struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	UserID            uint           `gorm:"not null;index" json:"user_id"`
	CourseID          uint           `gorm:"not null;index" json:"course_id"`
	RazorpayOrderID   string         `gorm:"type:varchar(100);uniqueIndex" json:"razorpay_order_id"`
	RazorpayPaymentID string         `gorm:"type:varchar(100)" json:"razorpay_payment_id"`
	Amount            float64        `gorm:"not null" json:"amount"`
	Currency          string         `gorm:"type:varchar(10);default:'INR'" json:"currency"`
	Status            string         `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending, completed, failed, refunded
	PaymentMethod     string         `gorm:"type:varchar(50)" json:"payment_method"`
	StorageQuotaMB    int            `gorm:"default:500" json:"storage_quota_mb"`
	ValidUntil        *time.Time     `json:"valid_until"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User   User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Course Course `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"course,omitempty"`
}

// TableName specifies the table name for CoursePayment
func (CoursePayment) TableName() string {
	return "course_payments"
}
