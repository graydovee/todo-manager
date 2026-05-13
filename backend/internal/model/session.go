package model

import "time"

type Session struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID string    `gorm:"not null;uniqueIndex" json:"session_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Data      []byte    `gorm:"type:blob" json:"-"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
}
