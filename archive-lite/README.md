# Archive-Lite

Archive-Lite is a lightweight, self-hosted web archiving solution built with Go. It aims to provide core archiving functionalities, an easy-to-use API, and simple Docker deployment. This project is inspired by ArchiveBox and is being developed with a focus on simplicity for potential integration with Pocket alternatives.

## Features

- Archive web pages (raw HTML).
- API to add, list, and retrieve archived content.
- Dockerized for easy deployment.
- Uses SQLite for metadata storage.
- (Planned) Support for Single Page Application (SPA) rendering via headless Chrome.

## Technologies Used

- **Go (Golang)**: Backend language.
- **Fiber v2**: Web framework.
- **GORM**: ORM for database interaction.
- **SQLite**: Database for storing archive metadata.
- **Chromedp**: (Integrated for future use) Go library for driving browsers via the Chrome DevTools Protocol, intended for SPA rendering.
- **Docker**: For containerization.

## Configuration

- **`ARCHIVE_DB_PATH`**: Environment variable to specify the path for the SQLite database file. Defaults to `archive.db` in the application's working directory.
- **Data Directories**:
    - `data/raw/`: Stores the raw HTML content of archived pages.
    - `data/screenshots/`: Intended for storing screenshots of archived pages (feature in development).
    These directories are created automatically by the application at startup if they don't exist.

## Getting Started

### Prerequisites

- Go (version 1.21 or higher recommended)
- Docker (for containerized deployment)
- Git

### Local Development

1.  **Clone the repository:**
    ```bash
    git clone <repository-url> archive-lite
    cd archive-lite
    ```

2.  **Build the application:**
    ```bash
    go build -o archive-lite main.go
    ```

3.  **Run the application:**
    ```bash
    # Optional: Set the database path
    # export ARCHIVE_DB_PATH=./my-archive-data/archive.sqlite3
    ./archive-lite
    ```
    The server will start on port `3000` by default.

### Docker Deployment

1.  **Build the Docker image:**
    ```bash
    docker build -t archive-lite .
    ```

2.  **Run the Docker container:**
    ```bash
    docker run -d -p 3000:3000 --name archive-lite-app       -v $(pwd)/data:/app/data       # Optional: Specify a custom database path within the mounted volume
      # -e ARCHIVE_DB_PATH=/app/data/archive.db       archive-lite
    ```
    - This command maps port `3000` of the container to port `3000` on the host.
    - It mounts a local directory named `data` (relative to your current path) into `/app/data` inside the container. This ensures that your archived content and the SQLite database (if `ARCHIVE_DB_PATH` points within `/app/data`) persist across container restarts.
    - If you use the `-e ARCHIVE_DB_PATH=/app/data/archive.db` option, the SQLite database file will be stored in your local `data` directory. Otherwise, it defaults to `/app/archive.db` inside the container (which would be lost if the container is removed without a volume for `/app`).

## API Endpoints

All API endpoints are prefixed with `/api/archive`.

-   **`POST /api/archive`**: Archive a new URL.
    -   **Request Body (JSON):**
        ```json
        {
          "url": "https://example.com"
        }
        ```
    -   **Success Response (201 Created):**
        ```json
        // ArchiveEntry object (see models/archive_entry.go)
        {
          "ID": 1,
          "CreatedAt": "2023-10-27T10:00:00Z",
          "UpdatedAt": "2023-10-27T10:00:00Z",
          "DeletedAt": null,
          "URL": "https://example.com",
          "Title": "", // Title might be empty initially
          "StoragePath": "data/raw/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.html",
          "ScreenshotPath": "data/screenshots/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.png",
          "ArchivedAt": "2023-10-27T10:00:00Z"
        }
        ```
    -   **Error Responses:** `400 Bad Request`, `500 Internal Server Error`.

-   **`GET /api/archive`**: List all archived entries.
    -   **Success Response (200 OK):**
        ```json
        [
          // Array of ArchiveEntry objects
        ]
        ```

-   **`GET /api/archive/:id`**: Get details for a specific archive entry.
    -   `:id` is the numerical ID of the archive entry.
    -   **Success Response (200 OK):**
        ```json
        // ArchiveEntry object
        ```
    -   **Error Responses:** `400 Bad Request`, `404 Not Found`.

-   **`GET /api/archive/:id/content`**: Retrieve the stored HTML content for an archive.
    -   `:id` is the numerical ID of the archive entry.
    -   **Success Response (200 OK):** Returns the HTML content (`text/html`).
    -   **Error Responses:** `400 Bad Request`, `404 Not Found`.

-   **`GET /api/archive/:id/screenshot`**: Retrieve a screenshot for an archive.
    -   `:id` is the numerical ID of the archive entry.
    -   **Success Response (200 OK):** Returns the PNG image (`image/png`) if available.
    -   **Error Responses:** `400 Bad Request`, `404 Not Found` (if screenshot doesn't exist or feature is not fully implemented).
        Currently, this endpoint will likely return a 404 as screenshot capture is a placeholder.

## SPA (Single Page Application) Support

-   The `chromedp` library has been included in the project dependencies, and basic file path generation for screenshots is in place.
-   However, the actual rendering of SPAs and capturing of screenshots using `chromedp` is **not yet implemented** in the `storage.CaptureSPA()` function. This function currently acts as a placeholder.
-   Full SPA support is a planned enhancement. When implemented, the `Dockerfile` will also need to be updated to include a headless Chrome/Chromium instance in both the build and runtime stages.

## Contributing

Contributions are welcome! Please feel free to open an issue or submit a pull request.

## License

This project is open source. (Consider adding a LICENSE file, e.g., MIT or Apache 2.0)
