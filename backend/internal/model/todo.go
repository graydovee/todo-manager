package model

import "time"

type Todo struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"not null;index;uniqueIndex:idx_user_code" json:"user_id"`
	Code        string     `gorm:"not null;uniqueIndex:idx_user_code" json:"code"`
	Title       string     `gorm:"not null;size:500" json:"title"`
	Description string     `gorm:"type:text" json:"description"`
	Category    string     `gorm:"not null;size:20" json:"category"`
	Priority    string     `gorm:"not null;size:10;default:p2" json:"priority"`
	Status      string     `gorm:"not null;size:20;default:open" json:"status"`
	DueAt       *time.Time `json:"due_at"`
	Pinned      bool       `gorm:"not null;default:false" json:"pinned"`
	Highlighted bool       `gorm:"not null;default:false" json:"highlighted"`
	CreatedAt   time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null" json:"updated_at"`

	Tags      []TodoTag      `gorm:"foreignKey:TodoID" json:"tags,omitempty"`
	Relations []TodoRelation `gorm:"foreignKey:SourceID" json:"-"`
	User      User           `gorm:"foreignKey:UserID" json:"-"`
}

const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusDuplicate  = "duplicate"
)

func ValidStatus(s string) bool {
	return s == StatusOpen || s == StatusInProgress || s == StatusCompleted || s == StatusDuplicate
}
