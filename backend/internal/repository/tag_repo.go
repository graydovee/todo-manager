package repository

import (
	"github.com/graydovee/todolist/internal/model"
	"gorm.io/gorm"
)

type TagRepo struct {
	db *gorm.DB
}

func NewTagRepo(db *gorm.DB) *TagRepo {
	return &TagRepo{db: db}
}

func (r *TagRepo) ReplaceTags(tx *gorm.DB, todoID uint, tags []string) error {
	db := tx
	if db == nil {
		db = r.db
	}

	if err := db.Where("todo_id = ?", todoID).Delete(&model.TodoTag{}).Error; err != nil {
		return err
	}

	for _, tag := range tags {
		if tag == "" {
			continue
		}
		todoTag := model.TodoTag{TodoID: todoID, Tag: tag}
		if err := db.Create(&todoTag).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *TagRepo) FindByTodoID(tx *gorm.DB, todoID uint) ([]string, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	var tags []model.TodoTag
	if err := db.Where("todo_id = ?", todoID).Find(&tags).Error; err != nil {
		return nil, err
	}
	result := make([]string, len(tags))
	for i, t := range tags {
		result[i] = t.Tag
	}
	return result, nil
}

func (r *TagRepo) FindDistinctByUser(tx *gorm.DB, userID uint) ([]string, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	var tags []string
	if err := db.Model(&model.TodoTag{}).
		Distinct("todo_tags.tag").
		Joins("JOIN todos ON todos.id = todo_tags.todo_id").
		Where("todos.user_id = ?", userID).
		Pluck("tag", &tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}
