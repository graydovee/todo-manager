// Package model defines the data models for the application.
package model

import "time"

// TodoStatusHistory records each status transition for a todo item.
type TodoStatusHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TodoID    uint      `gorm:"not null;index:idx_status_history_todo_id" json:"todo_id"`
	OldStatus string    `gorm:"not null;size:20" json:"old_status"`
	NewStatus string    `gorm:"not null;size:20" json:"new_status"`
	ChangedAt time.Time `gorm:"not null;index:idx_status_history_changed_at" json:"changed_at"`
}

// TableName overrides the default GORM table name to match the migration.
func (TodoStatusHistory) TableName() string {
	return "todo_status_history"
}
