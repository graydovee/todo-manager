package model

type TodoTag struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	TodoID uint   `gorm:"not null;uniqueIndex:idx_todo_tag" json:"todo_id"`
	Tag    string `gorm:"not null;uniqueIndex:idx_todo_tag;size:100" json:"tag"`
}
