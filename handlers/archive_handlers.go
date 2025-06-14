package handlers

import (
	"archive-lite/database"
	"archive-lite/models"
	"archive-lite/storage"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
)

// CreateArchivePayload is the expected payload for the CreateArchive handler
type CreateArchivePayload struct {
	URL string `json:"url"`
}

// CreateArchive handles the request to archive a new URL
func CreateArchive(c *fiber.Ctx) error {
	payload := new(CreateArchivePayload)
	if err := c.BodyParser(payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON payload",
		})
	}

	if payload.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "URL cannot be empty",
		})
	}

	entry, err := storage.ArchiveURL(database.DB, payload.URL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to archive URL: %s", err.Error()),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(entry)
}

// ListArchives handles the request to list all archived entries
func ListArchives(c *fiber.Ctx) error {
	var entries []models.ArchiveEntry
	result := database.DB.Order("archived_at desc").Find(&entries)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to list archives: %s", result.Error.Error()),
		})
	}
	return c.JSON(entries)
}

// GetArchiveDetails handles the request to get details for a specific archive entry
func GetArchiveDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Archive ID cannot be empty",
		})
	}

	var entry models.ArchiveEntry
	result := database.DB.Where("id = ?", id).First(&entry)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("Archive entry with ID %s not found: %s", id, result.Error.Error()),
		})
	}
	return c.JSON(entry)
}

// GetArchiveContent handles the request to retrieve the stored HTML content for an archive
func GetArchiveContent(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Archive ID cannot be empty",
		})
	}

	var entry models.ArchiveEntry
	result := database.DB.Where("id = ?", id).First(&entry)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("Archive entry with ID %s not found: %s", id, result.Error.Error()),
		})
	}

	if entry.StoragePath == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("Storage path not found for archive ID %s", id),
		})
	}

	// Check if file exists
	if _, err := os.Stat(entry.StoragePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("Archived content file not found at %s for ID %s", entry.StoragePath, id),
		})
	}

	// Correctly send the file as text/html
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.SendFile(entry.StoragePath)
}

// GetArchiveScreenshot handles the request to retrieve a screenshot for an archive
// This is a placeholder for now, as screenshot functionality is not yet implemented.
func GetArchiveScreenshot(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Archive ID cannot be empty",
		})
	}

	var entry models.ArchiveEntry
	result := database.DB.Where("id = ?", id).First(&entry)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("Archive entry with ID %s not found: %s", id, result.Error.Error()),
		})
	}

	// Check if screenshot file exists
	if entry.ScreenshotPath == "" {
		// If SPA/screenshot is not yet implemented, or file doesn't exist
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": fmt.Sprintf("Screenshot not available for archive ID %s. This feature might still be under development or the screenshot was not captured.", id),
		})
	}

	if _, err := os.Stat(entry.ScreenshotPath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("Screenshot file not found at %s for ID %s. It might not have been captured.", entry.ScreenshotPath, id),
		})
	}

	// Assuming PNG for now, adjust if other formats are used
	c.Set(fiber.HeaderContentType, "image/png")
	return c.SendFile(entry.ScreenshotPath)
}

// SetupRoutes configures the API routes for the application
func SetupRoutes(app *fiber.App) {
	api := app.Group("/api") // Base path for API routes

	archiveRoutes := api.Group("/archive")
	archiveRoutes.Post("/", CreateArchive)
	archiveRoutes.Get("/", ListArchives)
	archiveRoutes.Get("/:id", GetArchiveDetails)
	archiveRoutes.Get("/:id/content", GetArchiveContent)
	archiveRoutes.Get("/:id/screenshot", GetArchiveScreenshot)
}
