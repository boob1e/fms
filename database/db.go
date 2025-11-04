package database

import (
	"database/sql"
	"log"

	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDatabase initializes the SQLite database connection and runs migrations
func InitDatabase() error {
	var err error

	// Open SQLite database
	DB, err = gorm.Open(sql.Open("turso", "encrypted.db"))
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
