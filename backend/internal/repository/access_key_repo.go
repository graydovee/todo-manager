package repository

import (
	"time"

	"github.com/graydovee/todolist/internal/model"
	"gorm.io/gorm"
)

type AccessKeyRepo struct {
	db *gorm.DB
}

func NewAccessKeyRepo(db *gorm.DB) *AccessKeyRepo {
	return &AccessKeyRepo{db: db}
}

func (r *AccessKeyRepo) ListByUser(tx *gorm.DB, userID uint) ([]*model.AccessKey, error) {
	var keys []*model.AccessKey
	if err := r.getDB(tx).Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *AccessKeyRepo) FindByID(tx *gorm.DB, id, userID uint) (*model.AccessKey, error) {
	var key model.AccessKey
	if err := r.getDB(tx).Where("id = ? AND user_id = ?", id, userID).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *AccessKeyRepo) FindByPrefix(tx *gorm.DB, prefix string) (*model.AccessKey, error) {
	var key model.AccessKey
	if err := r.getDB(tx).Where("key_prefix = ?", prefix).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *AccessKeyRepo) Create(tx *gorm.DB, key *model.AccessKey) error {
	return r.getDB(tx).Create(key).Error
}

func (r *AccessKeyRepo) Update(tx *gorm.DB, key *model.AccessKey) error {
	return r.getDB(tx).Save(key).Error
}

func (r *AccessKeyRepo) Delete(tx *gorm.DB, id, userID uint) error {
	return r.getDB(tx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.AccessKey{}).Error
}

func (r *AccessKeyRepo) TouchLastUsed(tx *gorm.DB, id uint, at time.Time) error {
	return r.getDB(tx).Model(&model.AccessKey{}).Where("id = ?", id).Update("last_used_at", at).Error
}

func (r *AccessKeyRepo) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}
