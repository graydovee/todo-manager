package service

import (
	"fmt"
	"time"

	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
)

type CommentService struct {
	db          *gorm.DB
	commentRepo *repository.CommentRepo
	todoRepo    *repository.TodoRepo
}

func NewCommentService(db *gorm.DB, commentRepo *repository.CommentRepo, todoRepo *repository.TodoRepo) *CommentService {
	return &CommentService{db: db, commentRepo: commentRepo, todoRepo: todoRepo}
}

func (s *CommentService) Create(userID, todoID uint, content string) (*model.Comment, error) {
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	_, err := s.todoRepo.FindByID(nil, todoID, userID)
	if err != nil {
		return nil, fmt.Errorf("todo not found")
	}

	comment := &model.Comment{
		TodoID:    todoID,
		UserID:    userID,
		Content:   content,
		CreatedAt: time.Now(),
	}

	if err := s.commentRepo.Create(nil, comment); err != nil {
		return nil, err
	}
	return comment, nil
}

func (s *CommentService) ListByTodoID(userID, todoID uint) ([]*model.Comment, error) {
	_, err := s.todoRepo.FindByID(nil, todoID, userID)
	if err != nil {
		return nil, fmt.Errorf("todo not found")
	}
	return s.commentRepo.FindByTodoID(nil, todoID)
}

func (s *CommentService) Delete(userID, commentID uint) error {
	comment, err := s.commentRepo.FindByID(nil, commentID)
	if err != nil {
		return fmt.Errorf("comment not found")
	}
	if comment.UserID != userID {
		return fmt.Errorf("can only delete own comments")
	}
	return s.commentRepo.Delete(nil, commentID, userID)
}
