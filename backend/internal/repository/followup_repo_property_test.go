package repository

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todolist/internal/model"
	"gorm.io/gorm"
	"pgregory.net/rapid"
)

// setupTestDB creates an in-memory SQLite database with all required tables
// using raw SQL to ensure ON DELETE CASCADE constraints are properly defined.
func setupTestDB(t testing.TB) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	// Enable foreign key enforcement for SQLite
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	// Create tables with raw SQL to ensure ON DELETE CASCADE is properly set
	statements := []string{
		`CREATE TABLE IF NOT EXISTS summaries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			status TEXT NOT NULL DEFAULT 'analyzing',
			result_content TEXT,
			todo_ids TEXT,
			language TEXT,
			custom_prompt TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS followup_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			summary_id INTEGER NOT NULL,
			question TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (summary_id) REFERENCES summaries(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_followup_messages_summary_id ON followup_messages(summary_id)`,
		`CREATE TABLE IF NOT EXISTS followup_message_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			followup_message_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			version_number INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (followup_message_id) REFERENCES followup_messages(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_fmv_followup_message_id ON followup_message_versions(followup_message_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_fmv_message_version ON followup_message_versions(followup_message_id, version_number)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("failed to execute SQL: %s\nerror: %v", stmt, err)
		}
	}

	return db
}

// Feature: ai-summary-followup, Property 8: Cascade delete completeness
// **Validates: Requirements 7.3**
//
// Property: For any summary with associated followup messages and message versions,
// deleting the summary SHALL result in zero followup_messages and zero
// followup_message_versions records referencing that summary's ID.
func TestProperty_CascadeDeleteCompleteness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupTestDB(t)

		// Create a summary
		summary := &model.Summary{
			UserID:    1,
			StartDate: time.Now().Add(-24 * time.Hour),
			EndDate:   time.Now(),
			Status:    model.SummaryStatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := db.Create(summary).Error; err != nil {
			rt.Fatalf("failed to create summary: %v", err)
		}

		repo := NewFollowupRepo(db)

		// Generate a random number of followup messages (1 to 10)
		numMessages := rapid.IntRange(1, 10).Draw(rt, "numMessages")

		for i := 0; i < numMessages; i++ {
			msg := &model.FollowupMessage{
				SummaryID: summary.ID,
				Question:  rapid.StringMatching(`[a-zA-Z0-9 ]{5,50}`).Draw(rt, "question"),
				CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
			}
			if err := repo.CreateMessage(nil, msg); err != nil {
				rt.Fatalf("failed to create message %d: %v", i, err)
			}

			// Generate a random number of versions per message (1 to 5)
			numVersions := rapid.IntRange(1, 5).Draw(rt, "numVersions")
			for v := 0; v < numVersions; v++ {
				ver := &model.FollowupMessageVersion{
					FollowupMessageID: msg.ID,
					Content:           rapid.StringMatching(`[a-zA-Z0-9 ]{10,100}`).Draw(rt, "content"),
					VersionNumber:     v + 1,
					CreatedAt:         time.Now().Add(time.Duration(v) * time.Second),
				}
				if err := repo.CreateVersion(nil, ver); err != nil {
					rt.Fatalf("failed to create version %d for message %d: %v", v, i, err)
				}
			}
		}

		// Verify records exist before deletion
		var msgCountBefore int64
		db.Model(&model.FollowupMessage{}).Where("summary_id = ?", summary.ID).Count(&msgCountBefore)
		if msgCountBefore == 0 {
			rt.Fatalf("expected followup messages to exist before deletion")
		}

		var verCountBefore int64
		db.Model(&model.FollowupMessageVersion{}).
			Joins("JOIN followup_messages ON followup_messages.id = followup_message_versions.followup_message_id").
			Where("followup_messages.summary_id = ?", summary.ID).
			Count(&verCountBefore)
		if verCountBefore == 0 {
			rt.Fatalf("expected followup message versions to exist before deletion")
		}

		// Delete the summary — cascade should remove all related records
		if err := db.Delete(&model.Summary{}, summary.ID).Error; err != nil {
			rt.Fatalf("failed to delete summary: %v", err)
		}

		// Verify zero followup_messages referencing this summary
		var msgCountAfter int64
		db.Model(&model.FollowupMessage{}).Where("summary_id = ?", summary.ID).Count(&msgCountAfter)
		if msgCountAfter != 0 {
			rt.Fatalf("expected 0 followup messages after cascade delete, got %d", msgCountAfter)
		}

		// Verify zero followup_message_versions referencing this summary's messages
		var verCountAfter int64
		db.Model(&model.FollowupMessageVersion{}).
			Joins("JOIN followup_messages ON followup_messages.id = followup_message_versions.followup_message_id").
			Where("followup_messages.summary_id = ?", summary.ID).
			Count(&verCountAfter)
		if verCountAfter != 0 {
			rt.Fatalf("expected 0 followup message versions after cascade delete, got %d", verCountAfter)
		}

		// Also verify no orphaned versions exist (versions whose message no longer exists)
		var orphanedVersions int64
		db.Model(&model.FollowupMessageVersion{}).
			Where("followup_message_id NOT IN (?)",
				db.Model(&model.FollowupMessage{}).Select("id")).
			Count(&orphanedVersions)
		if orphanedVersions != 0 {
			rt.Fatalf("expected 0 orphaned versions after cascade delete, got %d", orphanedVersions)
		}
	})
}

// Feature: ai-summary-followup, Property 9: Followup messages chronological ordering
// **Validates: Requirements 7.4**
//
// Property: For any set of followup messages for a summary, the GET endpoint SHALL
// return them ordered by created_at ascending, and within each message, versions
// SHALL be ordered by version_number ascending.
func TestProperty_FollowupMessagesChronologicalOrdering(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupTestDB(t)

		// Create a summary
		summary := &model.Summary{
			UserID:    1,
			StartDate: time.Now().Add(-24 * time.Hour),
			EndDate:   time.Now(),
			Status:    model.SummaryStatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := db.Create(summary).Error; err != nil {
			rt.Fatalf("failed to create summary: %v", err)
		}

		repo := NewFollowupRepo(db)

		// Generate messages with random timestamps (not necessarily in order)
		numMessages := rapid.IntRange(2, 15).Draw(rt, "numMessages")
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		// Generate unique random offsets in minutes to create varied timestamps
		offsets := make([]int, numMessages)
		usedOffsets := make(map[int]bool)
		for i := 0; i < numMessages; i++ {
			for {
				offset := rapid.IntRange(0, 100000).Draw(rt, "offset")
				if !usedOffsets[offset] {
					usedOffsets[offset] = true
					offsets[i] = offset
					break
				}
			}
		}

		// Insert messages in a shuffled order (reverse of offset order to ensure
		// insertion order differs from chronological order)
		for i := numMessages - 1; i >= 0; i-- {
			ts := baseTime.Add(time.Duration(offsets[i]) * time.Minute)
			msg := &model.FollowupMessage{
				SummaryID: summary.ID,
				Question:  rapid.StringMatching(`[a-zA-Z0-9 ]{5,50}`).Draw(rt, "question"),
				CreatedAt: ts,
			}
			if err := repo.CreateMessage(nil, msg); err != nil {
				rt.Fatalf("failed to create message: %v", err)
			}

			// Generate versions inserted in reverse order to test ordering
			numVersions := rapid.IntRange(2, 5).Draw(rt, "numVersions")
			for v := numVersions; v >= 1; v-- {
				ver := &model.FollowupMessageVersion{
					FollowupMessageID: msg.ID,
					Content:           rapid.StringMatching(`[a-zA-Z0-9 ]{10,100}`).Draw(rt, "content"),
					VersionNumber:     v,
					CreatedAt:         ts.Add(time.Duration(v) * time.Second),
				}
				if err := repo.CreateVersion(nil, ver); err != nil {
					rt.Fatalf("failed to create version: %v", err)
				}
			}
		}

		// Query using FindBySummaryID
		messages, err := repo.FindBySummaryID(nil, summary.ID)
		if err != nil {
			rt.Fatalf("FindBySummaryID failed: %v", err)
		}

		if len(messages) != numMessages {
			rt.Fatalf("expected %d messages, got %d", numMessages, len(messages))
		}

		// Verify messages are ordered by created_at ascending
		for i := 1; i < len(messages); i++ {
			if messages[i].CreatedAt.Before(messages[i-1].CreatedAt) {
				rt.Fatalf("messages not in chronological order: message[%d].created_at=%v is before message[%d].created_at=%v",
					i, messages[i].CreatedAt, i-1, messages[i-1].CreatedAt)
			}
		}

		// Verify within each message, versions are ordered by version_number ascending
		for i, msg := range messages {
			if len(msg.Versions) < 2 {
				rt.Fatalf("expected at least 2 versions for message %d, got %d", i, len(msg.Versions))
			}
			for j := 1; j < len(msg.Versions); j++ {
				if msg.Versions[j].VersionNumber <= msg.Versions[j-1].VersionNumber {
					rt.Fatalf("versions not in ascending order for message %d: version[%d].number=%d <= version[%d].number=%d",
						i, j, msg.Versions[j].VersionNumber, j-1, msg.Versions[j-1].VersionNumber)
				}
			}
		}
	})
}
