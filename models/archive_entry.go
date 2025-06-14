package models

import (
	"time"
)

// ArchiveEntry represents an archived URL in the database
type ArchiveEntry struct {
	ID             string    `gorm:"primaryKey;type:varchar(36)"` // Random UUID as primary key
	URL            string    `gorm:"index;not null"`              // The original URL that was archived
	Title          string    // Optional: Title of the webpage
	StoragePath    string    `gorm:"not null"` // Path to the stored raw HTML content
	ScreenshotPath string    // Optional: Path to the stored screenshot
	ArchivedAt     time.Time `gorm:"not null"` // Timestamp when the archiving process was completed for this entry
	CreatedAt      time.Time // Creation timestamp
	UpdatedAt      time.Time // Update timestamp
}
