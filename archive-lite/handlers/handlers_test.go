package handlers

import (
	"archive-lite/database"
	"archive-lite/models"
	"archive-lite/storage" // For storage path configuration
	"archive-lite/tests"   // For test helpers
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testDB *gorm.DB
var app *fiber.App // Fiber app for testing handlers

// Variables to store original storage paths and the cleanup function for temp dirs
var originalRawHTMLDir string
var originalScreenshotsDir string
var tempStorageCleanup func()

// TestMain for handlers package
func TestMain(m *testing.M) {
	var err error
	testDB, err = tests.SetupTestDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up test DB: %v\n", err)
		os.Exit(1)
	}
	database.DB = testDB // Crucial: Handlers use the global database.DB

	app = tests.CreateTestApp()
	SetupRoutes(app) // Setup API routes for the test app

	// Save original storage paths and set up temporary ones for all tests in this package
	originalRawHTMLDir = storage.RawHTMLDirForTest()
	originalScreenshotsDir = storage.ScreenshotsDirForTest()

	var tempRawDir, tempSsDir string
	_, tempRawDir, tempSsDir, tempStorageCleanup, err = tests.EnsureTestStorageDirs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up temporary test storage dirs for handlers: %v\n", err)
		if tempStorageCleanup != nil { tempStorageCleanup() }
		os.Exit(1)
	}
	storage.SetStorageBaseDirsForTest(tempRawDir, tempSsDir) // Configure storage to use temp dirs

	exitCode := m.Run()

	// Teardown: Restore original storage paths and clean up temp dirs
	storage.SetStorageBaseDirsForTest(originalRawHTMLDir, originalScreenshotsDir)
	if tempStorageCleanup != nil {
		tempStorageCleanup()
	}

	os.Exit(exitCode)
}

// createTestArchiveEntry is a helper to seed the DB for GET tests
func createTestArchiveEntry(t *testing.T, url, title, storagePath, screenshotPath string, archivedAt time.Time) models.ArchiveEntry {
	entry := models.ArchiveEntry{
		URL:            url,
		Title:          title,
		StoragePath:    storagePath,
		ScreenshotPath: screenshotPath,
		ArchivedAt:     archivedAt,
	}
	result := testDB.Create(&entry)
	require.NoError(t, result.Error, "Failed to create test archive entry")
	return entry
}

// TestCreateArchiveAPI (Integration Test for POST /api/archive)
func TestCreateArchiveAPI(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, tests.ClearArchiveEntries(testDB))
		// Clean up files created in the temp storage rawHTMLDir
		if entries, err := os.ReadDir(storage.RawHTMLDirForTest()); err == nil {
			for _, entry := range entries {
				os.Remove(filepath.Join(storage.RawHTMLDirForTest(), entry.Name()))
			}
		}
	})

	mockStorageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body>Test content for API</body></html>")
	}))
	defer mockStorageServer.Close()

	t.Run("Valid URL", func(t *testing.T) {
		payload := map[string]string{"url": mockStorageServer.URL}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/archive", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1) // -1 for no timeout
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

		var createdEntry models.ArchiveEntry
		err = json.NewDecoder(resp.Body).Decode(&createdEntry)
		require.NoError(t, err)
		assert.Equal(t, mockStorageServer.URL, createdEntry.URL)
		assert.NotEmpty(t, createdEntry.StoragePath)
		assert.FileExists(t, createdEntry.StoragePath)
	})

	t.Run("Missing URL", func(t *testing.T) {
		payload := map[string]string{"url": ""}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/archive", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var respBody map[string]string
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)
		assert.Equal(t, "URL cannot be empty", respBody["error"])
	})

	t.Run("Malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/archive", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var respBody map[string]string
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)
		assert.Equal(t, "Cannot parse JSON payload", respBody["error"])
	})
}

func TestListArchivesAPI(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, tests.ClearArchiveEntries(testDB)) })

	now := time.Now().Truncate(time.Second)
	entry1 := createTestArchiveEntry(t, "http://example.com/1", "Ex1", "data/raw/ex1.html", "", now.Add(-1*time.Hour))
	entry2 := createTestArchiveEntry(t, "http://example.com/2", "Ex2", "data/raw/ex2.html", "", now) // entry2 is newer

	req := httptest.NewRequest("GET", "/api/archive", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var entries []models.ArchiveEntry
	err = json.NewDecoder(resp.Body).Decode(&entries)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, entry2.URL, entries[0].URL)
	assert.Equal(t, entry1.URL, entries[1].URL)
}

func TestGetArchiveDetailsAPI(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, tests.ClearArchiveEntries(testDB)) })

	entryTime := time.Now().Truncate(time.Second)
	entry := createTestArchiveEntry(t, "http://example.com/details", "Details", "data/raw/details.html", "", entryTime)

	t.Run("Found", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/archive/%d", entry.ID), nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		var fetchedEntry models.ArchiveEntry
		err = json.NewDecoder(resp.Body).Decode(&fetchedEntry)
		require.NoError(t, err)
		assert.Equal(t, entry.URL, fetchedEntry.URL)
		assert.Equal(t, entry.ID, fetchedEntry.ID)
	})

	t.Run("Not Found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/archive/99999", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})

	t.Run("Invalid ID format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/archive/invalid-id", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}

func TestGetArchiveContentAPI(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, tests.ClearArchiveEntries(testDB))
		if entries, err := os.ReadDir(storage.RawHTMLDirForTest()); err == nil {
			for _, entry := range entries {
				os.Remove(filepath.Join(storage.RawHTMLDirForTest(), entry.Name()))
			}
		}
	})

	dummyHTMLContent := "<html><body>Test Content File</body></html>"
	dummyFileName := "test_content_file.html"
	dummyStoragePath := filepath.Join(storage.RawHTMLDirForTest(), dummyFileName)
	err := os.WriteFile(dummyStoragePath, []byte(dummyHTMLContent), 0644)
	require.NoError(t, err)
	entryTime := time.Now().Truncate(time.Second)
	entry := createTestArchiveEntry(t, "http://example.com/content", "Content", dummyStoragePath, "", entryTime)

	t.Run("Found and content served", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/archive/%d/content", entry.ID), nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		assert.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get("Content-Type"))

		bodyBytes, readErr := io.ReadAll(resp.Body)
		require.NoError(t, readErr)
		assert.Equal(t, dummyHTMLContent, string(bodyBytes))
	})

	t.Run("DB Entry found but file missing", func(t *testing.T) {
		missingFilePath := filepath.Join(storage.RawHTMLDirForTest(), "missing.html")
		entryMissingFileTime := time.Now().Truncate(time.Second)
		entryMissingFile := createTestArchiveEntry(t, "http://example.com/missingfile", "Missing", missingFilePath, "", entryMissingFileTime)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/archive/%d/content", entryMissingFile.ID), nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})

	t.Run("Archive entry not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/archive/99999/content", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})
}

func TestGetArchiveScreenshotAPI(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, tests.ClearArchiveEntries(testDB)) })

	entryTime := time.Now().Truncate(time.Second)
	entry := createTestArchiveEntry(t, "http://example.com/screenshotpage", "Screenshot Test",
		"dummy.html",
		filepath.Join(storage.ScreenshotsDirForTest(), "test_ss.png"), entryTime)

	t.Run("Screenshot not available (file does not exist)", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/archive/%d/screenshot", entry.ID), nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)

		var respBody map[string]string
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)
		assert.Contains(t, respBody["error"], "Screenshot file not found")
	})
}
