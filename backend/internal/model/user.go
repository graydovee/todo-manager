package model

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AuthProvider string    `gorm:"not null;uniqueIndex:idx_provider_subject" json:"auth_provider"`
	AuthSubject  string    `gorm:"not null;uniqueIndex:idx_provider_subject" json:"auth_subject"`
	DisplayName  string    `gorm:"not null" json:"display_name"`
	CreatedAt    time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time `gorm:"not null" json:"updated_at"`
}
