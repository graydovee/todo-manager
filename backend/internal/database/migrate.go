package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/*
var migrationFS embed.FS

func RunMigrations(db *gorm.DB, driver string) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}

	dir := fmt.Sprintf("migrations/%s", driver)
	entries, err := migrationFS.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var count int64
		db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", name).Scan(&count)
		if count > 0 {
			continue
		}

		content, err := migrationFS.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		slog.Info("running migration", "version", name)
		if err := executeMigrationSQL(sqlDB, string(content), driver); err != nil {
			return fmt.Errorf("execute migration %s: %w", name, err)
		}

		db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name)
	}

	return nil
}

func executeMigrationSQL(sqlDB *sql.DB, sqlStr string, driver string) error {
	if driver == "sqlite" || driver == "postgres" {
		_, err := sqlDB.Exec(sqlStr)
		return err
	}
	statements := strings.Split(sqlStr, ";\n")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := sqlDB.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
