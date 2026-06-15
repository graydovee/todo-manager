package repository

import (
	"github.com/graydovee/todo-manager/internal/model"
	"gorm.io/gorm"
)

type SummaryRepo struct {
	db *gorm.DB
}

func NewSummaryRepo(db *gorm.DB) *SummaryRepo {
	return &SummaryRepo{db: db}
}

func (r *SummaryRepo) Create(tx *gorm.DB, summary *model.Summary) error {
	db := r.getDB(tx)
	return db.Create(summary).Error
}

func (r *SummaryRepo) FindByID(tx *gorm.DB, id, userID uint) (*model.Summary, error) {
	db := r.getDB(tx)
	var summary model.Summary
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&summary).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

// FindByIDOnly retrieves a summary by ID without user ownership check.
func (r *SummaryRepo) FindByIDOnly(tx *gorm.DB, id uint) (*model.Summary, error) {
	db := r.getDB(tx)
	var summary model.Summary
	if err := db.Where("id = ?", id).First(&summary).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

func (r *SummaryRepo) ListByUser(tx *gorm.DB, userID uint, limit int) ([]*model.Summary, error) {
	db := r.getDB(tx)
	if limit <= 0 {
		limit = 50
	}
	var summaries []*model.Summary
	if err := db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

func (r *SummaryRepo) Update(tx *gorm.DB, summary *model.Summary) error {
	db := r.getDB(tx)
	return db.Save(summary).Error
}

func (r *SummaryRepo) Delete(tx *gorm.DB, id, userID uint) error {
	db := r.getDB(tx)
	return db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Summary{}).Error
}

func (r *SummaryRepo) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}
