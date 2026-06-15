package repository

import (
	"github.com/graydovee/todo-manager/internal/model"
	"gorm.io/gorm"
)

// FollowupRepo handles database operations for followup messages and versions.
type FollowupRepo struct {
	db *gorm.DB
}

// NewFollowupRepo creates a new FollowupRepo instance.
func NewFollowupRepo(db *gorm.DB) *FollowupRepo {
	return &FollowupRepo{db: db}
}

// CreateMessage persists a new followup message record.
func (r *FollowupRepo) CreateMessage(tx *gorm.DB, msg *model.FollowupMessage) error {
	db := r.getDB(tx)
	return db.Create(msg).Error
}

// CreateVersion persists a new followup message version record.
func (r *FollowupRepo) CreateVersion(tx *gorm.DB, ver *model.FollowupMessageVersion) error {
	db := r.getDB(tx)
	return db.Create(ver).Error
}

// FindBySummaryID returns all followup messages for a given summary,
// ordered by created_at ASC, with versions preloaded and ordered by version_number ASC.
func (r *FollowupRepo) FindBySummaryID(tx *gorm.DB, summaryID uint) ([]*model.FollowupMessage, error) {
	db := r.getDB(tx)
	var messages []*model.FollowupMessage
	if err := db.Where("summary_id = ?", summaryID).
		Preload("Versions", func(db *gorm.DB) *gorm.DB {
			return db.Order("version_number ASC")
		}).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

// GetNextVersionNumber returns the next version number for a given followup message.
// If no versions exist, returns 1.
func (r *FollowupRepo) GetNextVersionNumber(tx *gorm.DB, messageID uint) (int, error) {
	db := r.getDB(tx)
	var maxVersion *int
	if err := db.Model(&model.FollowupMessageVersion{}).
		Where("followup_message_id = ?", messageID).
		Select("MAX(version_number)").
		Scan(&maxVersion).Error; err != nil {
		return 0, err
	}
	if maxVersion == nil {
		return 1, nil
	}
	return *maxVersion + 1, nil
}

// FindLatestVersionByMessageID returns the latest version for a given followup message.
func (r *FollowupRepo) FindLatestVersionByMessageID(tx *gorm.DB, messageID uint) (*model.FollowupMessageVersion, error) {
	db := r.getDB(tx)
	var version model.FollowupMessageVersion
	if err := db.Where("followup_message_id = ?", messageID).
		Order("version_number DESC").
		First(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

func (r *FollowupRepo) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}
