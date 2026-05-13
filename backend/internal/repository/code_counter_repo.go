package repository

import (
	"fmt"
	"strings"

	"github.com/graydovee/todolist/internal/model"
	"gorm.io/gorm"
)

type CodeCounterRepo struct {
	db *gorm.DB
}

func NewCodeCounterRepo(db *gorm.DB) *CodeCounterRepo {
	return &CodeCounterRepo{db: db}
}

func (r *CodeCounterRepo) GetNextCode(tx *gorm.DB, userID uint, category string) (string, error) {
	db := tx
	if db == nil {
		db = r.db
	}

	var counter model.CodeCounter
	result := db.Where("user_id = ? AND category = ?", userID, category).First(&counter)

	if result.Error == gorm.ErrRecordNotFound {
		counter = model.CodeCounter{
			UserID:   userID,
			Category: category,
			LastCode: 1,
		}
		if err := db.Create(&counter).Error; err != nil {
			return "", fmt.Errorf("create code counter: %w", err)
		}
	} else if result.Error != nil {
		return "", result.Error
	} else {
		if err := db.Model(&counter).Update("last_code", gorm.Expr("last_code + 1")).Error; err != nil {
			return "", err
		}
		counter.LastCode++
	}

	return fmt.Sprintf("%s-%d", strings.ToUpper(category), counter.LastCode), nil
}
