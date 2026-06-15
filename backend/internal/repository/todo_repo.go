package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/graydovee/todo-manager/internal/model"
	"gorm.io/gorm"
)

type TodoRepo struct {
	db *gorm.DB
}

func NewTodoRepo(db *gorm.DB) *TodoRepo {
	return &TodoRepo{db: db}
}

type TodoFilters struct {
	Query        string
	Code         string
	Tags         []string
	Category     string
	Priority     string
	Status       string
	UpdatedAfter *time.Time
	Page         int
	PageSize     int
	SortBy       string
	SortOrder    string
}

func (r *TodoRepo) Create(tx *gorm.DB, todo *model.Todo) error {
	db := r.getDB(tx)
	return db.Create(todo).Error
}

func (r *TodoRepo) FindByID(tx *gorm.DB, id, userID uint) (*model.Todo, error) {
	db := r.getDB(tx)
	var todo model.Todo
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&todo).Error; err != nil {
		return nil, err
	}
	return &todo, nil
}

func (r *TodoRepo) FindByIDWithDetails(tx *gorm.DB, id, userID uint) (*model.Todo, error) {
	db := r.getDB(tx)
	var todo model.Todo
	if err := db.Where("id = ? AND user_id = ?", id, userID).
		Preload("Tags").
		Preload("Relations").
		First(&todo).Error; err != nil {
		return nil, err
	}
	return &todo, nil
}

func (r *TodoRepo) Update(tx *gorm.DB, todo *model.Todo) error {
	db := r.getDB(tx)
	return db.Save(todo).Error
}

func (r *TodoRepo) Delete(tx *gorm.DB, id, userID uint) error {
	db := r.getDB(tx)
	return db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Todo{}).Error
}

func (r *TodoRepo) List(tx *gorm.DB, userID uint, filters TodoFilters) ([]*model.Todo, int64, error) {
	db := r.getDB(tx).Model(&model.Todo{}).Where("todos.user_id = ?", userID)

	if filters.Query != "" {
		q := "%" + filters.Query + "%"
		db = db.Where("todos.title LIKE ? OR todos.description LIKE ?", q, q)
	}

	if filters.Code != "" {
		db = db.Where("todos.code = ?", filters.Code)
	}

	if filters.Category != "" {
		categories := strings.Split(filters.Category, ",")
		if len(categories) == 1 {
			db = db.Where("todos.category = ?", categories[0])
		} else {
			db = db.Where("todos.category IN ?", categories)
		}
	}

	if filters.Priority != "" {
		priorities := strings.Split(filters.Priority, ",")
		if len(priorities) == 1 {
			db = db.Where("todos.priority = ?", priorities[0])
		} else {
			db = db.Where("todos.priority IN ?", priorities)
		}
	}

	if filters.Status != "" {
		statuses := strings.Split(filters.Status, ",")
		if len(statuses) == 1 {
			db = db.Where("todos.status = ?", statuses[0])
		} else {
			db = db.Where("todos.status IN ?", statuses)
		}
	}

	if len(filters.Tags) > 0 {
		db = db.Joins("JOIN todo_tags ON todo_tags.todo_id = todos.id").
			Where("todo_tags.tag IN ?", filters.Tags).
			Group("todos.id")
	}

	if filters.UpdatedAfter != nil {
		db = db.Where("todos.updated_at >= ?", *filters.UpdatedAfter)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortBy := "todos.created_at"
	sortOrder := "DESC"
	allowedSortBy := map[string]bool{
		"created_at": true, "updated_at": true, "due_at": true, "priority": true, "status": true,
	}
	if allowedSortBy[filters.SortBy] {
		sortBy = "todos." + filters.SortBy
	}
	if strings.EqualFold(filters.SortOrder, "asc") {
		sortOrder = "ASC"
	}
	db = db.Order(fmt.Sprintf("todos.pinned DESC, %s %s", sortBy, sortOrder))

	page := filters.Page
	if page < 1 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var todos []*model.Todo
	if err := db.Preload("Tags").Offset(offset).Limit(pageSize).Find(&todos).Error; err != nil {
		return nil, 0, err
	}

	return todos, total, nil
}

func (r *TodoRepo) ListAllWithTags(tx *gorm.DB, userID uint) ([]*model.Todo, error) {
	db := r.getDB(tx)

	var todos []*model.Todo
	if err := db.
		Where("todos.user_id = ?", userID).
		Preload("Tags").
		Order("todos.id ASC").
		Find(&todos).Error; err != nil {
		return nil, err
	}

	return todos, nil
}

// FindByUserAndUpdatedAtRange returns todos belonging to the given user whose
// updated_at falls within [start, end], with tags preloaded.
func (r *TodoRepo) FindByUserAndUpdatedAtRange(tx *gorm.DB, userID uint, start, end time.Time) ([]*model.Todo, error) {
	db := r.getDB(tx)
	var todos []*model.Todo
	if err := db.Where("user_id = ? AND updated_at >= ? AND updated_at <= ?", userID, start, end).
		Preload("Tags").
		Find(&todos).Error; err != nil {
		return nil, err
	}
	return todos, nil
}

// FindByIDsAndUser returns todos matching the given IDs that belong to the specified user.
func (r *TodoRepo) FindByIDsAndUser(tx *gorm.DB, ids []uint, userID uint) ([]*model.Todo, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db := r.getDB(tx)
	var todos []*model.Todo
	if err := db.Where("id IN ? AND user_id = ?", ids, userID).
		Preload("Tags").
		Find(&todos).Error; err != nil {
		return nil, err
	}
	return todos, nil
}

func (r *TodoRepo) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}
