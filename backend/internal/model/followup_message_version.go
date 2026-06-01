package model

import "time"

// FollowupMessageVersion represents a versioned AI response to a follow-up question.
type FollowupMessageVersion struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	FollowupMessageID uint      `gorm:"not null;index" json:"followup_message_id"`
	Content           string    `gorm:"type:text;not null" json:"content"`
	VersionNumber     int       `gorm:"not null;default:1" json:"version_number"`
	CreatedAt         time.Time `gorm:"not null" json:"created_at"`
}
