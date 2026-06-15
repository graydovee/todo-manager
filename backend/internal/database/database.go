package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todo-manager/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(cfg *config.Config, gormLogger logger.Interface) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.DB.Driver {
	case "sqlite":
		dialector = sqlite.Open(cfg.DB.DSN + "?_journal_mode=WAL")
	case "mysql":
		dialector = mysql.Open(cfg.DB.DSN)
	case "postgres":
		dialector = postgres.Open(cfg.DB.DSN)
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", cfg.DB.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return db, nil
}
