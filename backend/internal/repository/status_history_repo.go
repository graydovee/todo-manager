package repository

import (
	"time"

	"github.com/graydovee/todo-manager/internal/model"
	"gorm.io/gorm"
)

// StatusHistoryRepo handles persistence operations for todo status history records.
type StatusHistoryRepo struct {
	db *gorm.DB
}

// NewStatusHistoryRepo creates a new StatusHistoryRepo instance.
func NewStatusHistoryRepo(db *gorm.DB) *StatusHistoryRepo {
	return &StatusHistoryRepo{db: db}
}

// Create inserts a new status history record.
func (r *StatusHistoryRepo) Create(tx *gorm.DB, record *model.TodoStatusHistory) error {
	db := r.getDB(tx)
	return db.Create(record).Error
}

// FindByTodoID returns all status history records for a given todo, ordered by changed_at ascending.
func (r *StatusHistoryRepo) FindByTodoID(tx *gorm.DB, todoID uint) ([]*model.TodoStatusHistory, error) {
	db := r.getDB(tx)
	var records []*model.TodoStatusHistory
	if err := db.Where("todo_id = ?", todoID).Order("changed_at ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// FindByTodoIDAndTimeRange returns status history records for a given todo
// whose changed_at falls within [start, end].
func (r *StatusHistoryRepo) FindByTodoIDAndTimeRange(tx *gorm.DB, todoID uint, start, end time.Time) ([]*model.TodoStatusHistory, error) {
	db := r.getDB(tx)
	var records []*model.TodoStatusHistory
	if err := db.Where("todo_id = ? AND changed_at >= ? AND changed_at <= ?", todoID, start, end).
		Order("changed_at ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// FindByTodoIDs returns status history records for multiple todos, grouped by todo ID.
func (r *StatusHistoryRepo) FindByTodoIDs(tx *gorm.DB, todoIDs []uint) (map[uint][]*model.TodoStatusHistory, error) {
	db := r.getDB(tx)
	if len(todoIDs) == 0 {
		return make(map[uint][]*model.TodoStatusHistory), nil
	}
	var records []*model.TodoStatusHistory
	if err := db.Where("todo_id IN ?", todoIDs).Order("changed_at ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make(map[uint][]*model.TodoStatusHistory)
	for _, rec := range records {
		result[rec.TodoID] = append(result[rec.TodoID], rec)
	}
	return result, nil
}

// FindByTodoIDsAndTimeRange returns status history records for multiple todos
// whose changed_at falls within [start, end], grouped by todo ID.
func (r *StatusHistoryRepo) FindByTodoIDsAndTimeRange(tx *gorm.DB, todoIDs []uint, start, end time.Time) (map[uint][]*model.TodoStatusHistory, error) {
	db := r.getDB(tx)
	if len(todoIDs) == 0 {
		return make(map[uint][]*model.TodoStatusHistory), nil
	}
	var records []*model.TodoStatusHistory
	if err := db.Where("todo_id IN ? AND changed_at >= ? AND changed_at <= ?", todoIDs, start, end).
		Order("changed_at ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make(map[uint][]*model.TodoStatusHistory)
	for _, rec := range records {
		result[rec.TodoID] = append(result[rec.TodoID], rec)
	}
	return result, nil
}

// DeleteByTodoID removes all status history records for a given todo.
func (r *StatusHistoryRepo) DeleteByTodoID(tx *gorm.DB, todoID uint) error {
	db := r.getDB(tx)
	return db.Where("todo_id = ?", todoID).Delete(&model.TodoStatusHistory{}).Error
}

// getDB returns the transaction if provided, otherwise the default db.
func (r *StatusHistoryRepo) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}
