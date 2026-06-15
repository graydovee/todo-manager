package repository

import (
	"github.com/graydovee/todo-manager/internal/model"
	"gorm.io/gorm"
)

type RelationRepo struct {
	db *gorm.DB
}

func NewRelationRepo(db *gorm.DB) *RelationRepo {
	return &RelationRepo{db: db}
}

func (r *RelationRepo) Create(tx *gorm.DB, relation *model.TodoRelation) error {
	db := tx
	if db == nil {
		db = r.db
	}
	return db.Create(relation).Error
}

func (r *RelationRepo) Delete(tx *gorm.DB, sourceID, targetID uint, relationType model.RelationType) error {
	db := tx
	if db == nil {
		db = r.db
	}
	return db.Where("source_id = ? AND target_id = ? AND relation_type = ?",
		sourceID, targetID, relationType).Delete(&model.TodoRelation{}).Error
}

func (r *RelationRepo) DeleteBySourceOrTarget(tx *gorm.DB, todoID uint) error {
	db := tx
	if db == nil {
		db = r.db
	}
	return db.Where("source_id = ? OR target_id = ?", todoID, todoID).Delete(&model.TodoRelation{}).Error
}

func (r *RelationRepo) FindBySource(tx *gorm.DB, sourceID uint) ([]*model.TodoRelation, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	var relations []*model.TodoRelation
	if err := db.Where("source_id = ?", sourceID).Find(&relations).Error; err != nil {
		return nil, err
	}
	return relations, nil
}

func (r *RelationRepo) FindByTarget(tx *gorm.DB, targetID uint) ([]*model.TodoRelation, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	var relations []*model.TodoRelation
	if err := db.Where("target_id = ?", targetID).Find(&relations).Error; err != nil {
		return nil, err
	}
	return relations, nil
}

func (r *RelationRepo) FindBySourceAndType(tx *gorm.DB, sourceID uint, relationType model.RelationType) ([]*model.TodoRelation, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	var relations []*model.TodoRelation
	if err := db.Where("source_id = ? AND relation_type = ?", sourceID, relationType).Find(&relations).Error; err != nil {
		return nil, err
	}
	return relations, nil
}

func (r *RelationRepo) FindByUserAndType(tx *gorm.DB, userID uint, relationType model.RelationType) ([]*model.TodoRelation, error) {
	db := tx
	if db == nil {
		db = r.db
	}

	var relations []*model.TodoRelation
	if err := db.
		Table("todo_relations").
		Select("todo_relations.id, todo_relations.source_id, todo_relations.target_id, todo_relations.relation_type").
		Joins("JOIN todos source_todos ON source_todos.id = todo_relations.source_id").
		Joins("JOIN todos target_todos ON target_todos.id = todo_relations.target_id").
		Where("source_todos.user_id = ? AND target_todos.user_id = ? AND todo_relations.relation_type = ?", userID, userID, relationType).
		Order("todo_relations.id ASC").
		Scan(&relations).Error; err != nil {
		return nil, err
	}

	return relations, nil
}

func (r *RelationRepo) ReplaceRelations(tx *gorm.DB, sourceID uint, relations []*model.TodoRelation) error {
	db := tx
	if db == nil {
		db = r.db
	}

	if err := db.Where("source_id = ?", sourceID).Delete(&model.TodoRelation{}).Error; err != nil {
		return err
	}

	for _, rel := range relations {
		if err := db.Create(rel).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *RelationRepo) HasCycle(tx *gorm.DB, sourceID, targetID uint) (bool, error) {
	db := tx
	if db == nil {
		db = r.db
	}

	visited := make(map[uint]bool)
	stack := []uint{targetID}

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if current == sourceID {
			return true, nil
		}
		if visited[current] {
			continue
		}
		visited[current] = true

		// Find all targets that current depends on
		var nextTargets []uint
		db.Model(&model.TodoRelation{}).
			Where("source_id = ? AND relation_type = ?", current, model.RelationDependsOn).
			Pluck("target_id", &nextTargets)

		stack = append(stack, nextTargets...)
	}

	return false, nil
}
