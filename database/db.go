package database

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// InitDatabase initializes the SQLite database connection and runs migrations
func InitDatabase() (*gorm.DB, error) {
	var err error

	// Open SQLite database
	DB, err := gorm.Open(sqlite.Open("folo.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// CRITICAL: Enable foreign key constraints for SQLite
	// Without this, CASCADE DELETE and other FK constraints are ignored
	if err := DB.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, err
	}

	log.Println("Database connection established (foreign keys enabled)")

	return DB, err
}

// AutoMigrate runs database migrations for the provided models
func AutoMigrate(db *gorm.DB, models ...any) error {
	return db.AutoMigrate(models...)
}
