package model

import "time"

type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TodoID    uint      `gorm:"not null;index" json:"todo_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Content   string    `gorm:"not null;type:text" json:"content"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}
