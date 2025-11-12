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

	if err == nil {
	log.Println("Database connection established")
	}

	return DB, err 
}

// AutoMigrate runs database migrations for the provided models
func AutoMigrate(db *gorm.DB, models ...interface{}) error {
	return db.AutoMigrate(models...)
}
