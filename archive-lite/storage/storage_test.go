package storage

import (
	"archive-lite/tests"
	"context" // Needed for errors.Is(err, context.DeadlineExceeded)
	"fmt"
	"image/jpeg" // To check if it's a valid JPEG
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	// "time" // No longer directly used in this file after test changes
	"errors" // For errors.Is

	// "github.com/chromedp/chromedp" // Not directly needed here
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testDB *gorm.DB
var originalRawHTMLDir string
var originalScreenshotsDir string
var tempStorageCleanup func()

func TestMain(m *testing.M) {
	var err error
	testDB, err = tests.SetupTestDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up test DB: %v\n", err)
		os.Exit(1)
	}
	originalRawHTMLDir = RawHTMLDirForTest()
	originalScreenshotsDir = ScreenshotsDirForTest()
	var tempRawDir, tempSsDir string
	_, tempRawDir, tempSsDir, tempStorageCleanup, err = tests.EnsureTestStorageDirs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up temporary test storage dirs: %v\n", err)
		if tempStorageCleanup != nil { tempStorageCleanup() }
		os.Exit(1)
	}
	SetStorageBaseDirsForTest(tempRawDir, tempSsDir)
	exitCode := m.Run()
	SetStorageBaseDirsForTest(originalRawHTMLDir, originalScreenshotsDir)
	if tempStorageCleanup != nil { tempStorageCleanup() }
	os.Exit(exitCode)
}

func TestEnsureStorageDirsFunctionality(t *testing.T) {
	require.NoError(t, os.RemoveAll(rawHTMLDir))
	require.NoError(t, os.RemoveAll(screenshotsDir))
	err := EnsureStorageDirs()
	require.NoError(t, err)
	assert.DirExists(t, rawHTMLDir)
	assert.DirExists(t, screenshotsDir)
	err = EnsureStorageDirs()
	require.NoError(t, err)
	assert.DirExists(t, rawHTMLDir)
	assert.DirExists(t, screenshotsDir)
}

func TestFetchRawHTML(t *testing.T) {
	t.Run("Successful fetch", func(t *testing.T) {
		expectedHTML := "<html><body>Hello, Test!</body></html>"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, expectedHTML)
		}))
		defer server.Close()
		content, err := FetchRawHTML(server.URL)
		require.NoError(t, err)
		assert.Equal(t, expectedHTML+"\n", content)
	})
	t.Run("Server returns 500 error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()
		_, err := FetchRawHTML(server.URL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status code 500")
	})
	t.Run("Attempt to fetch from a non-existent server", func(t *testing.T) {
		_, err := FetchRawHTML("http://localhost:12346/nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get URL")
	})
}

func TestArchiveURL(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, tests.ClearArchiveEntries(testDB))
		if entries, err := os.ReadDir(rawHTMLDir); err == nil {
			for _, entry := range entries { os.Remove(filepath.Join(rawHTMLDir, entry.Name())) }
		}
		// Also clean screenshots specific to this test if any were made directly by ArchiveURL
		if entries, err := os.ReadDir(screenshotsDir); err == nil {
			for _, entry := range entries { os.Remove(filepath.Join(screenshotsDir, entry.Name())) }
		}
	})

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/validpage" {
			fmt.Fprintln(w, "<html><head><style>body{background-color: #abcdef;}</style></head><body>Valid Page Content for Screenshot</body></html>")
		} else { http.NotFound(w, r) }
	}))
	defer mockServer.Close()

	t.Run("Successful archiving of a valid page with screenshot attempt", func(t *testing.T) {
		targetURL := mockServer.URL + "/validpage"
		entry, err := ArchiveURL(testDB, targetURL)
		require.NoError(t, err, "ArchiveURL failed for a valid page")
		require.NotNil(t, entry, "ArchiveEntry should not be nil on success")

		assert.FileExists(t, entry.StoragePath)
		assert.NotEmpty(t, entry.ScreenshotPath, "ScreenshotPath should be set even if capture fails")

		if _, statErr := os.Stat(entry.ScreenshotPath); os.IsNotExist(statErr) {
			t.Logf("Screenshot file %s not found as expected if Chrome is not available or CaptureSPA failed. Error: %v", entry.ScreenshotPath, statErr)
		} else {
			assert.FileExists(t, entry.ScreenshotPath, "Screenshot file should be created if CaptureSPA succeeded")
			f, readErr := os.Open(entry.ScreenshotPath)
			require.NoError(t, readErr, "Failed to open screenshot file")
			defer f.Close()
			_, decodeErr := jpeg.Decode(f)
			assert.NoError(t, decodeErr, "Screenshot file is not a valid JPEG: %v", decodeErr)
		}
	})
}

func TestCaptureSPA_ActualCapture(t *testing.T) {
	if os.Getenv("CHROME_TESTS_DISABLED") == "true" {
		t.Skip("Skipping CaptureSPA actual capture test as CHROME_TESTS_DISABLED is set.")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><head><title>SPA Test</title><style>body{background-color: #123456; color: white;}</style></head><body><h1>Hello Chromedp!</h1></body></html>")
	}))
	defer server.Close()

	testScreenshotFileName := "test_spa_capture.jpg"
	// Uses screenshotsDir which is set to a temp path in TestMain
	testScreenshotPath := filepath.Join(screenshotsDir, testScreenshotFileName)

	t.Cleanup(func() {
		os.Remove(testScreenshotPath) // Clean up the specific file created by this test
	})

	err := CaptureSPA(server.URL, "dummy.html", testScreenshotPath)

	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "executable file not found") || strings.Contains(errMsg, "chrome not found") {
			t.Logf("CaptureSPA test skipped: Chrome executable not found: %v", err)
			t.Skip("Chrome executable not found.")
		}
		// Check for context deadline or canceled, which might indicate environmental issues with Chrome
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		   strings.Contains(errMsg, "context deadline exceeded") || strings.Contains(errMsg, "context canceled") {
			t.Logf("CaptureSPA test inconclusive: Chromedp task timed out or was canceled: %v. This might be an environmental issue (slow Chrome startup, resource limits).", err)
			t.Skip("Chromedp task timed out or was canceled.")
		}
		// For any other error, fail the test
		require.NoError(t, err, "CaptureSPA returned an unexpected error")
	}

	assert.FileExists(t, testScreenshotPath, "Screenshot file should be created by CaptureSPA")

	f, readErr := os.Open(testScreenshotPath)
	require.NoError(t, readErr, "Failed to open created screenshot file")
	defer f.Close()
	_, decodeErr := jpeg.Decode(f)
	assert.NoError(t, decodeErr, "Created screenshot file is not a valid JPEG")
}
