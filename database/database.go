package database

import (
	"archive-lite/models"
	"log"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	DB   *gorm.DB
	once sync.Once
	err  error
)

// Init initializes the database connection and auto-migrates schemas.
func Init() (*gorm.DB, error) {
	once.Do(func() {
		DB, err = gorm.Open(sqlite.Open("archive.db"), &gorm.Config{})
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
			return
		}

		log.Println("Database connection established.")

		// Auto-migrate the schema
		err = DB.AutoMigrate(&models.ArchiveEntry{})
		if err != nil {
			log.Printf("Failed to auto-migrate database schema: %v", err)
			return
		}
		log.Println("Database schema migrated.")
	})
	return DB, err
}
