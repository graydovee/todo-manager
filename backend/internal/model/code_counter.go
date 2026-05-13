package model

type CodeCounter struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	UserID   uint   `gorm:"not null;uniqueIndex:idx_user_category" json:"user_id"`
	Category string `gorm:"not null;uniqueIndex:idx_user_category;size:20" json:"category"`
	LastCode int    `gorm:"not null;default:0" json:"last_code"`
}
