package tests

import (
	"archive-lite/models"
	"fmt" // Added for EnsureTestStorageDirs error formatting
	"log"
	"os"
	"path/filepath" // Added for EnsureTestStorageDirs
	"sync"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	testDB    *gorm.DB
	onceDB    sync.Once
	dbInitErr error
)

// SetupTestDB initializes an in-memory SQLite database for testing
// and migrates the schema.
func SetupTestDB() (*gorm.DB, error) {
	onceDB.Do(func() {
		testDB, dbInitErr = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if dbInitErr != nil {
			log.Fatalf("Failed to connect to in-memory test database: %v", dbInitErr)
			return
		}

		log.Println("In-memory test database connection established.")

		dbInitErr = testDB.AutoMigrate(&models.ArchiveEntry{})
		if dbInitErr != nil {
			log.Fatalf("Failed to auto-migrate test database schema: %v", dbInitErr)
			return
		}
		log.Println("Test database schema migrated.")
	})
	return testDB, dbInitErr
}

// CreateTestApp initializes a new Fiber app for testing purposes.
func CreateTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			ctx.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
			return ctx.Status(code).SendString(err.Error())
		},
	})
	return app
}

// TeardownTestDB closes the test database connection and cleans up.
func TeardownTestDB(db *gorm.DB) {
	if db != nil {
		sqlDB, err := db.DB()
		if err == nil {
			err := sqlDB.Close()
			if err != nil {
				log.Printf("Error closing test database: %v", err)
			} else {
				log.Println("Test database closed.")
			}
		}
	}
}

// ClearArchiveEntries deletes all entries from the ArchiveEntry table.
func ClearArchiveEntries(db *gorm.DB) error {
	// Use GORM's batch delete feature. AllowGlobalUpdate is needed for deleting without conditions.
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.ArchiveEntry{}).Error; err != nil {
		return fmt.Errorf("failed to delete archive entries: %w", err)
	}
	// Reset autoincrement sequence for sqlite
	// This is important so that tests expecting specific IDs (if any) are consistent.
	if err := db.Exec("DELETE FROM sqlite_sequence WHERE name='archive_entries'").Error; err != nil {
		// This might fail if the table was empty initially or if the DB is not SQLite.
		// We can log this but not necessarily fail the cleanup.
		// log.Printf("Could not reset sequence for archive_entries (this might be okay if table was empty or DB is not SQLite): %v", err)
	}
	return nil
}

// EnsureTestStorageDirs creates necessary storage directories for tests.
func EnsureTestStorageDirs() (dataDir string, rawDir string, assetsDir string, screenshotsDir string, cleanup func(), err error) {
	tempDir, err := os.MkdirTemp("", "archive_lite_test_*")
	if err != nil {
		return "", "", "", "", nil, fmt.Errorf("failed to create temp dir for tests: %w", err)
	}

	testDataDir := filepath.Join(tempDir, "data")
	testRawHTMLDir := filepath.Join(testDataDir, "raw")
	testAssetsDir := filepath.Join(testDataDir, "assets")
	testScreenshotsDir := filepath.Join(testDataDir, "screenshots")

	dirs := []string{testRawHTMLDir, testAssetsDir, testScreenshotsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			os.RemoveAll(tempDir) // Cleanup on error
			return "", "", "", "", nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	cleanupFunc := func() {
		os.RemoveAll(tempDir)
		log.Println("Test storage directories cleaned up.")
	}

	return testDataDir, testRawHTMLDir, testAssetsDir, testScreenshotsDir, cleanupFunc, nil
}
