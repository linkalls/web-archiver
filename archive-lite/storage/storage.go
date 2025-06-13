package storage

import (
	"archive-lite/models"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	rawHTMLDir = "data/raw"
)

func SetStorageBaseDirsForTest(testRawHTMLDir string) {
	rawHTMLDir = testRawHTMLDir
}

func RawHTMLDirForTest() string { return rawHTMLDir }

func EnsureStorageDirs() error {
	if err := os.MkdirAll(rawHTMLDir, 0755); err != nil {
		return fmt.Errorf("failed to create raw HTML directory '%s': %w", rawHTMLDir, err)
	}
	return nil
}

func FetchRawHTML(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get URL '%s': %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get URL '%s': status code %d", url, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body from '%s': %w", url, err)
	}

	return string(bodyBytes), nil
}

func ArchiveURL(db *gorm.DB, urlToArchive string) (*models.ArchiveEntry, error) {
	if err := EnsureStorageDirs(); err != nil {
		return nil, fmt.Errorf("failed to ensure storage directories: %w", err)
	}

	// Fetch raw HTML content
	htmlContent, err := FetchRawHTML(urlToArchive)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch HTML content for '%s': %w", urlToArchive, err)
	}

	// Generate unique filename
	entryUUID := uuid.New().String()
	htmlFileName := fmt.Sprintf("%s.html", entryUUID)
	htmlFilePath := filepath.Join(rawHTMLDir, htmlFileName)

	// Save HTML content to file
	if err := os.WriteFile(htmlFilePath, []byte(htmlContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write HTML to '%s': %w", htmlFilePath, err)
	}

	// Create archive entry in database
	archiveEntry := models.ArchiveEntry{
		URL:         urlToArchive,
		Title:       "",
		StoragePath: htmlFilePath,
		ArchivedAt:  time.Now(),
	}

	result := db.Create(&archiveEntry)
	if result.Error != nil {
		os.Remove(htmlFilePath)
		return nil, fmt.Errorf("failed to create archive entry in database for '%s': %w", urlToArchive, result.Error)
	}

	return &archiveEntry, nil
}
