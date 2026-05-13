package model

type RelationType string

const (
	RelationDependsOn  RelationType = "depends_on"
	RelationDuplicateOf RelationType = "duplicate_of"
)

type TodoRelation struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	SourceID     uint         `gorm:"not null;uniqueIndex:idx_relation" json:"source_id"`
	TargetID     uint         `gorm:"not null;uniqueIndex:idx_relation" json:"target_id"`
	RelationType RelationType `gorm:"not null;uniqueIndex:idx_relation;size:20" json:"relation_type"`
}
