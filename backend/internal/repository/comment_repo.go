package repository

import (
	"github.com/graydovee/todolist/internal/model"
	"gorm.io/gorm"
)

type CommentRepo struct {
	db *gorm.DB
}

func NewCommentRepo(db *gorm.DB) *CommentRepo {
	return &CommentRepo{db: db}
}

func (r *CommentRepo) Create(tx *gorm.DB, comment *model.Comment) error {
	db := tx
	if db == nil {
		db = r.db
	}
	return db.Create(comment).Error
}

func (r *CommentRepo) FindByTodoID(tx *gorm.DB, todoID uint) ([]*model.Comment, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	var comments []*model.Comment
	if err := db.Where("todo_id = ?", todoID).Order("created_at ASC").Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *CommentRepo) FindByID(tx *gorm.DB, id uint) (*model.Comment, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	var comment model.Comment
	if err := db.Where("id = ?", id).First(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *CommentRepo) Delete(tx *gorm.DB, id uint, userID uint) error {
	db := tx
	if db == nil {
		db = r.db
	}
	return db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Comment{}).Error
}
