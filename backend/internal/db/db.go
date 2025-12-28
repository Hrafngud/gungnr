package db

import (
	"fmt"
	"log"

	"go-notes/internal/config"
	"go-notes/internal/models"

	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg config.Config) (*gorm.DB, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database url is empty")
	}

	gormDB, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("retrieve generic db handle: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		log.Printf("warn: db ping failed: %v", err)
	}

	return gormDB, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.Note{})
}
