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
	rawHTMLDir     = "data/raw"
	screenshotsDir = "data/screenshots"
)

func SetStorageBaseDirsForTest(testRawHTMLDir, testScreenshotsDir string) {
	rawHTMLDir = testRawHTMLDir
	screenshotsDir = testScreenshotsDir
}

// RawHTMLDirForTest returns the current rawHTMLDir (for test purposes).
func RawHTMLDirForTest() string {
	return rawHTMLDir
}

// ScreenshotsDirForTest returns the current screenshotsDir (for test purposes).
func ScreenshotsDirForTest() string {
	return screenshotsDir
}

// EnsureStorageDirs creates the necessary storage directories if they don't exist.
func EnsureStorageDirs() error {
	dirs := []string{rawHTMLDir, screenshotsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", dir, err)
		}
	}
	return nil
}

// FetchRawHTML fetches the HTML content from a given URL.
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

// CaptureSPA is a placeholder for future SPA capturing logic.
func CaptureSPA(url string, htmlFilePath string, screenshotPath string) error {
	return nil
}

// ArchiveURL orchestrates archiving a URL.
func ArchiveURL(db *gorm.DB, urlToArchive string) (*models.ArchiveEntry, error) {
	if err := EnsureStorageDirs(); err != nil {
		return nil, fmt.Errorf("failed to ensure storage directories: %w", err)
	}

	rawHTML, err := FetchRawHTML(urlToArchive)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw HTML for '%s': %w", urlToArchive, err)
	}

	entryUUID := uuid.New().String()
	rawHTMLFileName := fmt.Sprintf("%s.html", entryUUID)
	rawHTMLFilePath := filepath.Join(rawHTMLDir, rawHTMLFileName)

	err = os.WriteFile(rawHTMLFilePath, []byte(rawHTML), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write raw HTML to '%s': %w", rawHTMLFilePath, err)
	}

	screenshotFileName := fmt.Sprintf("%s.png", entryUUID)
	screenshotFilePath := filepath.Join(screenshotsDir, screenshotFileName)

	if err := CaptureSPA(urlToArchive, rawHTMLFilePath, screenshotFilePath); err != nil {
		// log.Printf("CaptureSPA for %s encountered a non-critical error: %v", urlToArchive, err)
	}

	archiveEntry := models.ArchiveEntry{
		URL:            urlToArchive,
		Title:          "",
		StoragePath:    rawHTMLFilePath,
		ScreenshotPath: screenshotFilePath,
		ArchivedAt:     time.Now(),
	}

	result := db.Create(&archiveEntry)
	if result.Error != nil {
		os.Remove(rawHTMLFilePath)
		return nil, fmt.Errorf("failed to create archive entry in database for '%s': %w", urlToArchive, result.Error)
	}

	return &archiveEntry, nil
}
