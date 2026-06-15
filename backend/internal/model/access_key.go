package model

import "time"

type AccessKey struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	UserID             uint       `gorm:"not null;index" json:"user_id"`
	Name               string     `gorm:"not null;size:64" json:"name"`
	KeyPrefix          string     `gorm:"not null;uniqueIndex;size:32" json:"key_prefix"`
	KeySalt            string     `gorm:"not null;size:128" json:"-"`
	KeyHash            string     `gorm:"not null;size:256" json:"-"`
	AuthorizedAPIsJSON string     `gorm:"not null;type:text" json:"-"`
	ExpiresAt          *time.Time `gorm:"index" json:"expires_at"`
	LastUsedAt         *time.Time `gorm:"index" json:"last_used_at"`
	CreatedAt          time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"not null" json:"updated_at"`
}
