package database

import (
	"archive-lite/models"
	"log"
	"os" // Added for environment variable access
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	DB   *gorm.DB
	once sync.Once
	err  error
)

const (
	defaultDBPath    = "archive.db"
	dbPathEnvVar     = "ARCHIVE_DB_PATH"
)

// getDBPath determines the database path from an environment variable or uses the default.
func getDBPath() string {
	if path := os.Getenv(dbPathEnvVar); path != "" {
		log.Printf("Using database path from environment variable %s: %s", dbPathEnvVar, path)
		return path
	}
	log.Printf("Using default database path: %s", defaultDBPath)
	return defaultDBPath
}

// Init initializes the database connection and auto-migrates schemas.
func Init() (*gorm.DB, error) {
	once.Do(func() {
		dbPath := getDBPath()
		DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		if err != nil {
			log.Printf("Failed to connect to database at %s: %v", dbPath, err)
			return
		}

		log.Printf("Database connection established at %s.", dbPath)

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
