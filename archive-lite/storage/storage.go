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

const (
	rawHTMLDir      = "data/raw"
	screenshotsDir  = "data/screenshots" // For future use
)

// EnsureStorageDirs creates the necessary storage directories if they don't exist.
func EnsureStorageDirs() error {
	dirs := []string{rawHTMLDir, screenshotsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// FetchRawHTML fetches the HTML content from a given URL.
func FetchRawHTML(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get URL %s: status code %d", url, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body from %s: %w", url, err)
	}
	return string(bodyBytes), nil
}

// CaptureSPA is a placeholder for future SPA capturing logic using a headless browser.
// For now, it doesn't perform any headless browser actions.
// It might be enhanced to save the raw HTML as a starting point or do nothing.
func CaptureSPA(url string, htmlFilePath string, screenshotPath string) error {
	// Placeholder: In a real SPA capture, we would use chromedp here to:
	// 1. Navigate to the URL.
	// 2. Wait for dynamic content to load.
	// 3. Extract the rendered HTML and save it to htmlFilePath.
	// 4. Take a screenshot and save it to screenshotPath.
	// For now, we can log that this is a placeholder.
	fmt.Printf("CaptureSPA called for URL: %s. This is a placeholder for actual SPA rendering.\n", url)
	fmt.Printf("SPA HTML would be saved to: %s\n", htmlFilePath)
	fmt.Printf("SPA Screenshot would be saved to: %s\n", screenshotPath)
	// As a minimal step, we could copy the raw HTML here if needed,
	// or this function could do nothing until SPA rendering is fully implemented.
	return nil
}

// ArchiveURL orchestrates the archiving of a given URL.
// It fetches raw HTML, (optionally captures SPA content), saves it,
// and records the entry in the database.
func ArchiveURL(db *gorm.DB, urlToArchive string) (*models.ArchiveEntry, error) {
	if err := EnsureStorageDirs(); err != nil {
		return nil, fmt.Errorf("failed to ensure storage directories: %w", err)
	}

	// Fetch raw HTML first
	rawHTML, err := FetchRawHTML(urlToArchive)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw HTML for %s: %w", urlToArchive, err)
	}

	entryUUID := uuid.New().String()
	rawHTMLFileName := fmt.Sprintf("%s.html", entryUUID)
	rawHTMLFilePath := filepath.Join(rawHTMLDir, rawHTMLFileName)

	// Save raw HTML
	err = os.WriteFile(rawHTMLFilePath, []byte(rawHTML), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write raw HTML to %s: %w", rawHTMLFilePath, err)
	}

	// Placeholder for screenshot path
	screenshotFileName := fmt.Sprintf("%s.png", entryUUID)
	screenshotFilePath := filepath.Join(screenshotsDir, screenshotFileName) // Path for future screenshot

	// Call CaptureSPA (currently a placeholder)
	// In the future, CaptureSPA might save its own version of HTML (e.g., fully rendered)
	// and a screenshot. For now, rawHTMLFilePath is the primary content.
	if err := CaptureSPA(urlToArchive, rawHTMLFilePath /* or a different path for SPA rendered HTML */, screenshotFilePath); err != nil {
		// Non-critical for now as it's a placeholder, but good to log
		fmt.Printf("CaptureSPA for %s encountered an error (non-critical): %v\n", urlToArchive, err)
	}

	// For now, Title will be empty. It can be populated later, perhaps by parsing the HTML.
	archiveEntry := models.ArchiveEntry{
		URL:            urlToArchive,
		Title:          "", // Placeholder for title
		StoragePath:    rawHTMLFilePath,
		ScreenshotPath: screenshotFilePath, // Store path even if file isn't created yet
		ArchivedAt:     time.Now(),
	}

	result := db.Create(&archiveEntry)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to create archive entry in database for %s: %w", urlToArchive, result.Error)
	}

	return &archiveEntry, nil
}
