package model

import "time"

type Summary struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"not null;index" json:"user_id"`
	StartDate     time.Time `gorm:"not null;type:date" json:"start_date"`
	EndDate       time.Time `gorm:"not null;type:date" json:"end_date"`
	Status        string    `gorm:"not null;size:20;default:analyzing" json:"status"`
	ResultContent string    `gorm:"type:text" json:"result_content,omitempty"`
	TodoIDs       string    `gorm:"type:text" json:"todo_ids,omitempty"`
	CreatedAt     time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt     time.Time `gorm:"not null" json:"updated_at"`
}

const (
	SummaryStatusAnalyzing = "analyzing"
	SummaryStatusCompleted = "completed"
	SummaryStatusError     = "error"
)
