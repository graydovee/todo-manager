package model

import "time"

// FollowupMessage represents a follow-up question asked on a summary.
type FollowupMessage struct {
	ID        uint                     `gorm:"primaryKey" json:"id"`
	SummaryID uint                     `gorm:"not null;index" json:"summary_id"`
	Question  string                   `gorm:"type:text;not null" json:"question"`
	Versions  []FollowupMessageVersion `gorm:"foreignKey:FollowupMessageID" json:"versions"`
	CreatedAt time.Time                `gorm:"not null" json:"created_at"`
}
