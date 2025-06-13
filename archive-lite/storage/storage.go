package storage

import (
	"archive-lite/models"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv" // For parsing env vars
	"strings" // For parsing env vars
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	rawHTMLDir                     = "data/raw"
	screenshotsDir                 = "data/screenshots"
	chromedpTimeout                time.Duration // Now set in init()
	screenshotQuality              int           // Now set in init()
	customChromedpAllocatorOptions []chromedp.ExecAllocatorOption
)

const (
	defaultChromedpTimeoutSeconds = 20
	defaultScreenshotQuality      = 85
)

func init() {
	// -- Timeout --
	timeoutStr := os.Getenv("CHROMEDP_TIMEOUT_SECONDS")
	timeoutSeconds := defaultChromedpTimeoutSeconds
	if parsedTimeout, err := strconv.Atoi(timeoutStr); err == nil && parsedTimeout > 0 {
		timeoutSeconds = parsedTimeout
		log.Printf("Using CHROMEDP_TIMEOUT_SECONDS: %d from environment.", timeoutSeconds)
	} else {
		if timeoutStr != "" { // Log if var was set but invalid
			log.Printf("Invalid CHROMEDP_TIMEOUT_SECONDS value: '%s'. Using default: %d seconds.", timeoutStr, defaultChromedpTimeoutSeconds)
		} else {
			log.Printf("Using default chromedp timeout: %d seconds.", defaultChromedpTimeoutSeconds)
		}
	}
	chromedpTimeout = time.Duration(timeoutSeconds) * time.Second

	// -- Screenshot Quality --
	qualityStr := os.Getenv("SCREENSHOT_QUALITY")
	screenshotQuality = defaultScreenshotQuality
	if parsedQuality, err := strconv.Atoi(qualityStr); err == nil && parsedQuality > 0 && parsedQuality <= 100 {
		screenshotQuality = parsedQuality
		log.Printf("Using SCREENSHOT_QUALITY: %d from environment.", screenshotQuality)
	} else {
		if qualityStr != "" { // Log if var was set but invalid
			log.Printf("Invalid SCREENSHOT_QUALITY value: '%s'. Using default: %d.", qualityStr, defaultScreenshotQuality)
		} else {
			log.Printf("Using default screenshot quality: %d.", defaultScreenshotQuality)
		}
	}

	// -- Allocator Options --
	// Start with a fresh list of options
	options := []chromedp.ExecAllocatorOption{
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("single-process", true),
		chromedp.Flag("ignore-certificate-errors", true),
	}

	// Append default flags that might not be covered by the above simplified ones.
	// This ensures we get a good base from DefaultExecAllocatorOptions and then layer our specifics.
	// customChromedpAllocatorOptions = append(chromedp.DefaultExecAllocatorOptions[:], options...)
    // Revision: It's usually better to start with DefaultExecAllocatorOptions and then selectively add or ensure specific flags.
    // However, for full control and explicitness, especially with NoSandbox, starting clean is also common.
    // Let's stick to the plan of building options explicitly and then potentially adding from Default if some are missed.
    // For now, the explicit list `options` will be the base for customChromedpAllocatorOptions.

	if execPath := os.Getenv("CHROME_BIN_PATH"); execPath != "" {
		options = append(options, chromedp.ExecPath(execPath))
		log.Printf("Using CHROME_BIN_PATH: %s from environment.", execPath)
	} else {
		log.Println("CHROME_BIN_PATH not set, relying on chromedp to find Chrome/Chromium in PATH.")
	}

    if extraFlagsStr := os.Getenv("CHROMEDP_EXTRA_FLAGS"); extraFlagsStr != "" {
        flags := strings.Split(extraFlagsStr, ",")
        for _, flag := range flags {
            trimmedFlag := strings.TrimSpace(flag)
            if trimmedFlag != "" {
                // Assume flags starting with -- are proper flags.
                // chromedp.Flag expects key (flag name without --) and value.
                // For boolean flags, value is interface{}(true) or interface{}(false).
                // For flags like --remote-debugging-port=0, key is "remote-debugging-port", value is 0.
                // This simple parser assumes boolean flags.
                if strings.HasPrefix(trimmedFlag, "--") {
                    options = append(options, chromedp.Flag(strings.TrimPrefix(trimmedFlag, "--"), true))
                    log.Printf("Adding extra chromedp flag: %s", trimmedFlag)
                } else {
                    log.Printf("Skipping malformed extra flag: %s (should start with --)", trimmedFlag)
                }
            }
        }
    }
	customChromedpAllocatorOptions = options // Assign the constructed options
}

func SetStorageBaseDirsForTest(testRawHTMLDir, testScreenshotsDir string) {
	rawHTMLDir = testRawHTMLDir
	screenshotsDir = testScreenshotsDir
}

func RawHTMLDirForTest() string { return rawHTMLDir }
func ScreenshotsDirForTest() string { return screenshotsDir }

func EnsureStorageDirs() error {
	dirs := []string{rawHTMLDir, screenshotsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", dir, err)
		}
	}
	return nil
}

func FetchRawHTML(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil { return "", fmt.Errorf("failed to get URL '%s': %w", url, err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("failed to get URL '%s': status code %d", url, resp.StatusCode) }
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil { return "", fmt.Errorf("failed to read response body from '%s': %w", url, err) }
	return string(bodyBytes), nil
}

func CaptureSPA(urlToCapture string, _ string, screenshotFilePath string) error {
	// Create a parent context for the allocator.
	// Allocator context should ideally not have a short timeout itself,
    // the timeout is better applied to the task context.
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), customChromedpAllocatorOptions...)
	defer cancelAlloc()

    // Create a new task context with the configured timeout.
	taskCtx, cancelTask := context.WithTimeout(allocCtx, chromedpTimeout)
	defer cancelTask()

    // Create the browser context from the task context
	ctx, cancel := chromedp.NewContext(taskCtx, chromedp.WithLogf(log.Printf)) // Consider conditional logging based on debug env var
	defer cancel()


	var buf []byte
	log.Printf("Chromedp: Navigating to %s for screenshot. Timeout: %s, Quality: %d", urlToCapture, chromedpTimeout, screenshotQuality)
	tasks := chromedp.Tasks{
		chromedp.Navigate(urlToCapture),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(3 * time.Second), // Consider making this configurable too
		chromedp.FullScreenshot(&buf, screenshotQuality), // Uses package-level screenshotQuality
	}

	if err := chromedp.Run(ctx, tasks); err != nil {
		return fmt.Errorf("chromedp tasks failed for URL '%s': %w", urlToCapture, err)
	}
    if len(buf) == 0 {
        return fmt.Errorf("captured screenshot is empty for URL '%s'", urlToCapture)
    }
	if err := os.WriteFile(screenshotFilePath, buf, 0644); err != nil {
		return fmt.Errorf("failed to write screenshot to '%s': %w", screenshotFilePath, err)
	}
	log.Printf("Screenshot captured successfully for %s and saved to %s", urlToCapture, screenshotFilePath)
	return nil
}

func ArchiveURL(db *gorm.DB, urlToArchive string) (*models.ArchiveEntry, error) {
	if err := EnsureStorageDirs(); err != nil { return nil, fmt.Errorf("failed to ensure storage directories: %w", err) }
	rawHTML, err := FetchRawHTML(urlToArchive)
	if err != nil { return nil, fmt.Errorf("failed to fetch raw HTML for '%s': %w", urlToArchive, err) }
	entryUUID := uuid.New().String()
	rawHTMLFileName := fmt.Sprintf("%s.html", entryUUID)
	rawHTMLFilePath := filepath.Join(rawHTMLDir, rawHTMLFileName)
	err = os.WriteFile(rawHTMLFilePath, []byte(rawHTML), 0644)
	if err != nil { return nil, fmt.Errorf("failed to write raw HTML to '%s': %w", rawHTMLFilePath, err) }
	screenshotFileName := fmt.Sprintf("%s.jpg", entryUUID)
	screenshotFilePath := filepath.Join(screenshotsDir, screenshotFileName)
	if errCapture := CaptureSPA(urlToArchive, rawHTMLFilePath, screenshotFilePath); errCapture != nil {
		log.Printf("CaptureSPA for '%s' failed (screenshot may be missing): %v", urlToArchive, errCapture)
	}
	archiveEntry := models.ArchiveEntry{
		URL: urlToArchive, Title: "", StoragePath: rawHTMLFilePath,
		ScreenshotPath: screenshotFilePath, ArchivedAt: time.Now(),
	}
	result := db.Create(&archiveEntry)
	if result.Error != nil {
		os.Remove(rawHTMLFilePath)
		return nil, fmt.Errorf("failed to create archive entry in database for '%s': %w", urlToArchive, result.Error)
	}
	return &archiveEntry, nil
}
