package service

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"

	"github.com/graydovee/todo-manager/internal/model"
	"github.com/graydovee/todo-manager/internal/repository"
	"gorm.io/gorm"
)

// MigrationState represents the detected state of a user's todo codes.
type MigrationState int

const (
	// NeedsMigration indicates all codes match the old format ^[A-Z]+-\d+$
	NeedsMigration MigrationState = iota
	// AlreadyMigrated indicates all codes match the numeric format ^\d+$
	AlreadyMigrated
	// Corrupted indicates a mix of old-format and numeric codes
	Corrupted
)

var (
	oldFormatRegex = regexp.MustCompile(`^[A-Z]+-\d+$`)
	numericRegex   = regexp.MustCompile(`^\d+$`)
)

// MigrationService handles migrating old-format todo codes to the new
// sequential numeric format on application startup.
type MigrationService struct {
	db          *gorm.DB
	todoRepo    *repository.TodoRepo
	counterRepo *repository.CodeCounterRepo
}

// NewMigrationService creates a new MigrationService instance.
func NewMigrationService(db *gorm.DB, todoRepo *repository.TodoRepo, counterRepo *repository.CodeCounterRepo) *MigrationService {
	return &MigrationService{
		db:          db,
		todoRepo:    todoRepo,
		counterRepo: counterRepo,
	}
}

// Run executes the data migration for all users. It gets distinct user IDs
// from the todos table and calls migrateUser for each.
func (s *MigrationService) Run() error {
	var userIDs []uint
	if err := s.db.Model(&model.Todo{}).Distinct("user_id").Pluck("user_id", &userIDs).Error; err != nil {
		return fmt.Errorf("get distinct user IDs: %w", err)
	}

	for _, userID := range userIDs {
		if err := s.migrateUser(userID); err != nil {
			log.Printf("[migration] ERROR: failed to migrate user_id=%d: %v", userID, err)
			// Continue with remaining users
		}
	}

	return nil
}

// migrateUser handles migration for a single user. It loads all todos,
// detects the migration state, and if needed, reassigns sequential codes
// within a transaction.
func (s *MigrationService) migrateUser(userID uint) error {
	// Load all todos for the user (outside transaction for state detection)
	var todos []*model.Todo
	if err := s.db.Where("user_id = ?", userID).Find(&todos).Error; err != nil {
		return fmt.Errorf("load todos for user %d: %w", userID, err)
	}

	// Detect migration state
	state := s.detectMigrationState(todos)

	switch state {
	case AlreadyMigrated:
		log.Printf("[migration] user_id=%d: already migrated, skipping", userID)
		return nil
	case Corrupted:
		log.Printf("[migration] WARNING: user_id=%d has mixed code formats (corrupted state), skipping", userID)
		return nil
	case NeedsMigration:
		// Proceed with migration below
	}

	// Begin transaction for this user's migration
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Sort todos by (created_at ASC, id ASC)
		sort.Slice(todos, func(i, j int) bool {
			if todos[i].CreatedAt.Equal(todos[j].CreatedAt) {
				return todos[i].ID < todos[j].ID
			}
			return todos[i].CreatedAt.Before(todos[j].CreatedAt)
		})

		// Assign codes "1", "2", ..., "N" in order
		for i, todo := range todos {
			newCode := strconv.Itoa(i + 1)
			if err := tx.Model(&model.Todo{}).Where("id = ?", todo.ID).Update("code", newCode).Error; err != nil {
				return fmt.Errorf("update todo id=%d code: %w", todo.ID, err)
			}
		}

		// Delete old CodeCounter rows for this user
		if err := tx.Where("user_id = ?", userID).Delete(&model.CodeCounter{}).Error; err != nil {
			return fmt.Errorf("delete old code counters for user %d: %w", userID, err)
		}

		// Create single CodeCounter with last_code = N
		counter := model.CodeCounter{
			UserID:   userID,
			LastCode: len(todos),
		}
		if err := tx.Create(&counter).Error; err != nil {
			return fmt.Errorf("create code counter for user %d: %w", userID, err)
		}

		log.Printf("[migration] user_id=%d: migrated %d todos successfully", userID, len(todos))
		return nil
	})
}

// detectMigrationState examines the codes of the given todos and determines
// whether they need migration, are already migrated, or are in a corrupted state.
func (s *MigrationService) detectMigrationState(todos []*model.Todo) MigrationState {
	if len(todos) == 0 {
		return AlreadyMigrated
	}

	oldFormatCount := 0
	numericCount := 0

	for _, todo := range todos {
		switch {
		case oldFormatRegex.MatchString(todo.Code):
			oldFormatCount++
		case numericRegex.MatchString(todo.Code):
			numericCount++
		default:
			// Code matches neither pattern — treat as corrupted
			log.Printf("[migration] WARNING: todo id=%d has unrecognized code format: %q", todo.ID, todo.Code)
			return Corrupted
		}
	}

	switch {
	case oldFormatCount == len(todos):
		return NeedsMigration
	case numericCount == len(todos):
		return AlreadyMigrated
	default:
		// Mix of old-format and numeric codes
		return Corrupted
	}
}
