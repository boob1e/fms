package database

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDatabase initializes the SQLite database connection and runs migrations
func InitDatabase() error {
	var err error

	// Open SQLite database
	DB, err = gorm.Open(sqlite.Open("folo.db"), &gorm.Config{})
	if err != nil {
		return err
	}

	log.Println("Database connection established")

	return nil
}

// AutoMigrate runs database migrations for the provided models
func AutoMigrate(models ...interface{}) error {
	return DB.AutoMigrate(models...)
}
