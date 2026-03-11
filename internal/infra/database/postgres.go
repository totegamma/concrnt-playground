package database

import (
	"log"
	"log/slog"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/totegamma/concrnt-playground/internal/infra/database/models"
)

func NewPostgres(dsn string) (*gorm.DB, error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             300 * time.Millisecond, // Slow SQL threshold
			LogLevel:                  logger.Warn,            // Log level
			IgnoreRecordNotFoundError: true,                   // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,                   // Enable color
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true,
		Logger:         gormLogger,
	})
	if err != nil {
		return nil, err
	}

	// set connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	if err = db.Use(tracing.NewPlugin(
		tracing.WithDBSystem("postgresql"),
	)); err != nil {
		slog.Error("failed to enable tracing plugin for postgres", slog.String("error", err.Error()))
	}

	return db, err
}

func MigratePostgres(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.CommitLog{},
		&models.CommitOwner{},
		&models.Record{},
		&models.RecordKey{},
		&models.Association{},
		&models.Server{},
		&models.Entity{},
		&models.EntityMeta{},
	)
}
