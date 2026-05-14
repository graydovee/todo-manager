package repository

import (
	"fmt"
	"strings"

	"github.com/graydovee/todolist/internal/model"
	"gorm.io/gorm"
)

type TodoRepo struct {
	db *gorm.DB
}

func NewTodoRepo(db *gorm.DB) *TodoRepo {
	return &TodoRepo{db: db}
}

type TodoFilters struct {
	Query     string
	Code      string
	Tags      []string
	Category  string
	Priority  string
	Status    string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
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
		db = db.Where("todos.category = ?", filters.Category)
	}

	if filters.Priority != "" {
		db = db.Where("todos.priority = ?", filters.Priority)
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
	db = db.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

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

func (r *TodoRepo) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}
