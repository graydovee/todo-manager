package model

type CodeCounter struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	UserID   uint `gorm:"not null;uniqueIndex:idx_user_id" json:"user_id"`
	LastCode int  `gorm:"not null;default:0" json:"last_code"`
}
