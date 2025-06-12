package storage

import (
	"archive-lite/models"
	"archive-lite/tests" // Test helpers
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testDB *gorm.DB

// Store original directory paths from storage package to restore them after tests.
var originalRawHTMLDir string
var originalScreenshotsDir string
var tempStorageCleanup func() // Function to clean up temporary storage directories

// TestMain sets up the test DB and temporary directories for file storage tests.
func TestMain(m *testing.M) {
	var err error
	testDB, err = tests.SetupTestDB() // Uses in-memory SQLite
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up test DB: %v\n", err)
		os.Exit(1)
	}

	originalRawHTMLDir = rawHTMLDir         // Save original value from storage.go (now a var)
	originalScreenshotsDir = screenshotsDir // Save original value from storage.go (now a var)

	var tempRawDir, tempSsDir string
	// tests.EnsureTestStorageDirs is from test_helpers.go, creates temp dirs for test artifacts
	_, tempRawDir, tempSsDir, tempStorageCleanup, err = tests.EnsureTestStorageDirs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up temporary test storage dirs: %v\n", err)
		if tempStorageCleanup != nil { tempStorageCleanup() }
		os.Exit(1)
	}
	// Configure storage package (SUT) to use these temporary directories
	SetStorageBaseDirsForTest(tempRawDir, tempSsDir)

	exitCode := m.Run() // Run all tests in the package

	// Restore original paths in storage package (SUT)
	SetStorageBaseDirsForTest(originalRawHTMLDir, originalScreenshotsDir)
	if tempStorageCleanup != nil {
		tempStorageCleanup() // Clean up temporary directories created by tests.EnsureTestStorageDirs
	}
	os.Exit(exitCode)
}

// TestEnsureStorageDirsFunctionality tests the EnsureStorageDirs function from the storage package.
// It ensures that the function correctly creates directories it's configured to use.
func TestEnsureStorageDirsFunctionality(t *testing.T) {
	// The global temp dirs (rawHTMLDir, screenshotsDir) are set by TestMain.
	// To test EnsureStorageDirs' creation capability, we remove them first.
	require.NoError(t, os.RemoveAll(rawHTMLDir), "Failed to remove temp rawHTMLDir for test setup")
	require.NoError(t, os.RemoveAll(screenshotsDir), "Failed to remove temp screenshotsDir for test setup")

	err := EnsureStorageDirs() // Call the function under test
	require.NoError(t, err, "EnsureStorageDirs should not return an error on first run")
	assert.DirExists(t, rawHTMLDir, "Raw HTML directory should be created by EnsureStorageDirs")
	assert.DirExists(t, screenshotsDir, "Screenshots directory should be created by EnsureStorageDirs")

	// Test idempotency: running again should not cause an error
	err = EnsureStorageDirs()
	require.NoError(t, err, "EnsureStorageDirs should be idempotent")
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
		// Note: Fprintln adds a newline
		assert.Equal(t, expectedHTML+"\n", content, "Fetched HTML content mismatch")
	})

	t.Run("Server returns 500 error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := FetchRawHTML(server.URL)
		require.Error(t, err, "Expected an error for 500 status code")
		assert.Contains(t, err.Error(), "status code 500", "Error message should indicate 500 status")
	})

	t.Run("Attempt to fetch from a non-existent server", func(t *testing.T) {
		// Use a port that is highly unlikely to be in use
		_, err := FetchRawHTML("http://localhost:12346/nonexistent")
		require.Error(t, err, "Expected an error when server is not reachable")
		assert.Contains(t, err.Error(), "failed to get URL", "Error message should indicate failure to get URL")
	})
}

func TestArchiveURL(t *testing.T) {
	// Per-test cleanup of database and files created in temp storage
	// This main t.Cleanup will run after all subtests in TestArchiveURL are done.
	t.Cleanup(func() {
		// It's still good to have a final cleanup here, though subtests should manage their own state.
		require.NoError(t, tests.ClearArchiveEntries(testDB))

		// Clean files from the temporary rawHTMLDir
		if entries, err := os.ReadDir(rawHTMLDir); err == nil {
			for _, entry := range entries {
				require.NoError(t, os.Remove(filepath.Join(rawHTMLDir, entry.Name())))
			}
		}
		// Clean files from the temporary screenshotsDir
		if entries, err := os.ReadDir(screenshotsDir); err == nil {
			for _, entry := range entries {
				require.NoError(t, os.Remove(filepath.Join(screenshotsDir, entry.Name())))
			}
		}
	})

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/validpage" {
			fmt.Fprintln(w, "<html><body>Valid Page Content</body></html>")
		} else if r.URL.Path == "/emptypage" {
			fmt.Fprint(w, "") // Empty response body
		} else {
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	t.Run("Successful archiving of a valid page", func(t *testing.T) {
		// Ensure a clean state for this subtest
		require.NoError(t, tests.ClearArchiveEntries(testDB), "Subtest setup: ClearArchiveEntries failed")
		t.Cleanup(func() { // Ensure this subtest cleans up after itself
			require.NoError(t, tests.ClearArchiveEntries(testDB), "Subtest cleanup: ClearArchiveEntries failed")
		})

		targetURL := mockServer.URL + "/validpage"
		entry, err := ArchiveURL(testDB, targetURL)
		require.NoError(t, err, "ArchiveURL failed for a valid page")
		require.NotNil(t, entry, "ArchiveEntry should not be nil on success")

		assert.Equal(t, targetURL, entry.URL)
		assert.Empty(t, entry.Title, "Title should be empty as it's not implemented yet")
		assert.NotEmpty(t, entry.StoragePath, "StoragePath should be set")
		assert.True(t, strings.HasSuffix(entry.StoragePath, ".html"), "StoragePath should end with .html")
		// Check that StoragePath is within the temporary directory configured for tests
		assert.True(t, strings.HasPrefix(entry.StoragePath, rawHTMLDir),
			fmt.Sprintf("StoragePath '%s' should be within the temp rawHTMLDir '%s'", entry.StoragePath, rawHTMLDir))

		assert.FileExists(t, entry.StoragePath, "HTML file should be created in temp dir")
		content, readErr := os.ReadFile(entry.StoragePath)
		require.NoError(t, readErr)
		assert.Equal(t, "<html><body>Valid Page Content</body></html>\n", string(content))

		var dbEntry models.ArchiveEntry
		dbResult := testDB.First(&dbEntry, entry.ID)
		require.NoError(t, dbResult.Error, "Failed to retrieve entry from DB")
		assert.Equal(t, targetURL, dbEntry.URL)
		assert.Equal(t, entry.StoragePath, dbEntry.StoragePath)
		assert.WithinDuration(t, time.Now(), dbEntry.ArchivedAt, 5*time.Second, "ArchivedAt timestamp is incorrect")

		assert.NotEmpty(t, entry.ScreenshotPath)
		assert.True(t, strings.HasPrefix(entry.ScreenshotPath, screenshotsDir),
			fmt.Sprintf("ScreenshotPath '%s' should be within temp screenshotsDir '%s'", entry.ScreenshotPath, screenshotsDir))
		_, statErr := os.Stat(entry.ScreenshotPath)
		assert.True(t, os.IsNotExist(statErr), "Screenshot file should not exist for placeholder CaptureSPA")
	})

	t.Run("Archiving fails if URL fetch fails (404)", func(t *testing.T) {
		// Ensure a clean state for this subtest
		require.NoError(t, tests.ClearArchiveEntries(testDB), "Subtest setup: ClearArchiveEntries failed")
		t.Cleanup(func() { // Ensure this subtest cleans up after itself
			require.NoError(t, tests.ClearArchiveEntries(testDB), "Subtest cleanup: ClearArchiveEntries failed")
		})

		targetURL := mockServer.URL + "/nonexistentpage"
		entry, err := ArchiveURL(testDB, targetURL)
		require.Error(t, err, "ArchiveURL should return an error if fetching fails")
		assert.Nil(t, entry, "ArchiveEntry should be nil on fetch failure")
		assert.Contains(t, err.Error(), "failed to fetch raw HTML")
		assert.Contains(t, err.Error(), "status code 404")

		var count int64
		testDB.Model(&models.ArchiveEntry{}).Count(&count)
		assert.Equal(t, int64(0), count, "No DB entry should be created if fetching fails")
	})

	t.Run("Archiving an empty page successfully", func(t *testing.T) {
		// Ensure a clean state for this subtest
		require.NoError(t, tests.ClearArchiveEntries(testDB), "Subtest setup: ClearArchiveEntries failed")
		t.Cleanup(func() { // Ensure this subtest cleans up after itself
			require.NoError(t, tests.ClearArchiveEntries(testDB), "Subtest cleanup: ClearArchiveEntries failed")
		})

		targetURL := mockServer.URL + "/emptypage"
		entry, err := ArchiveURL(testDB, targetURL)
		require.NoError(t, err, "ArchiveURL failed for an empty page")
		require.NotNil(t, entry)

		assert.FileExists(t, entry.StoragePath)
		content, readErr := os.ReadFile(entry.StoragePath)
		require.NoError(t, readErr)
		assert.Equal(t, "", string(content), "File content for empty page should be empty")

		var dbEntry models.ArchiveEntry
		testDB.First(&dbEntry, entry.ID)
		assert.Equal(t, targetURL, dbEntry.URL)
	})
}

// TestCaptureSPA tests the placeholder CaptureSPA function.
func TestCaptureSPA(t *testing.T) {
	// This test is minimal as CaptureSPA is a placeholder.
	// It just ensures it can be called without error.
	err := CaptureSPA("http://example.com", "test.html", "test.png")
	assert.NoError(t, err, "Placeholder CaptureSPA should currently return no error")
}
