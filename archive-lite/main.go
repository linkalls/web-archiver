package main

import (
	"archive-lite/database"
	"archive-lite/handlers" // Import handlers
	"archive-lite/storage"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger" // Optional: add logger
)

func main() {
	// Initialize Database
	_, err := database.Init()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialized successfully.")

	// Ensure storage directories exist
	if err := storage.EnsureStorageDirs(); err != nil {
		log.Fatalf("Failed to create storage directories: %v", err)
	}
	log.Println("Storage directories ensured.")

	app := fiber.New()

	// Middleware
	app.Use(logger.New()) // Add basic request logging

	// 静的ファイル配信: WebUIとアーカイブデータ
	app.Static("/webui.html", "./webui.html")
	app.Static("/data", "./data")

	// Setup Routes
	handlers.SetupRoutes(app) // Configure API routes

	// Simple welcome route
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Archive-Lite API is running. Use /api/archive endpoints.")
	})

	log.Println("Starting server on port 3000...")
	log.Fatal(app.Listen(":3000"))
}
