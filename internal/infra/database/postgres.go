package database

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

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
