# Archive-Lite

Archive-Lite is a lightweight, self-hosted web archiving solution built with Go. It aims to provide core archiving functionalities, an easy-to-use API, and simple Docker deployment. This project is inspired by ArchiveBox and is being developed with a focus on simplicity for potential integration with Pocket alternatives.

```bash
docker build -t archive-lite .
docker run -d -p 3000:3000 --name archive-lite-app \
  -v "$(pwd)/data:/app/data" \
  archive-lite
  ```

## Features

- Archive web pages (raw HTML).
- API to add, list, and retrieve archived content .
- Dockerized for easy deployment (includes Google Chrome ).
- Uses SQLite for metadata storage.
- Support for Single Page Application (SPA) compatible archiving via headless Chrome.

## Technologies Used

- **Go (Golang)**: Backend language.
- **Fiber v2**: Web framework.
- **GORM**: ORM for database interaction.
- **SQLite**: Database for storing archive metadata.
- **Chromedp**: Go library for driving browsers via the Chrome DevTools Protocol
- **Docker**: For containerization.

## Configuration

- **`ARCHIVE_DB_PATH`**: Environment variable to specify the path for the SQLite database file. Defaults to `archive.db` in the application's working directory.
- **`CHROME_BIN_PATH`**: Optional path to the Chrome/Chromium executable if it's not in the system PATH (used by `chromedp`).
- **`CHROMEDP_EXTRA_FLAGS`**: Optional comma-separated list of additional flags to pass to the Chrome/Chromium process started by `chromedp` (e.g., `"--flag1,--flag2"`).

- **Data Directories**:
    - `data/raw/`: Stores the raw HTML content of archived pages.
    These directories are created automatically by the application at startup if they don't exist.

## Getting Started

### Prerequisites

- Go (version 1.21 or higher recommended)
- Docker (for containerized deployment, including Chrome)
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
    (Ensure Chrome or Chromium is installed and in your PATH ).
    ```bash
    # Optional: Set environment variables
    # export ARCHIVE_DB_PATH=./my-archive-data/archive.sqlite3
    # export CHROMEDP_TIMEOUT_SECONDS=30
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
    docker run -d -p 3000:3000 --name archive-lite-app \
      -v $(pwd)/data:/app/data \
      # Optional: Pass environment variables for configuration
      # -e ARCHIVE_DB_PATH=/app/data/archive.db \
      # -e CHROMEDP_TIMEOUT_SECONDS=30 \
      # To use host's Chrome in Docker (advanced, typically not needed with Chrome in image):
      # -e CHROME_BIN_PATH=/path/to/chrome \
      # For additional Chrome flags:
      # -e CHROMEDP_EXTRA_FLAGS="--disable-features=site-per-process,--some-other-flag" \
      archive-lite
    ```
    - This command maps port `3000` of the container to port `3000` on the host.
   
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

## SPA (Single Page Application) Support


-   The system does not currently perform deep SPA rendering (e.g., scrolling to trigger lazy-loaded content, interacting with elements before capture, or extracting fully rendered HTML after all JavaScript execution).
-   Further enhancements for more complex SPA interactions and full HTML rendering from SPAs are future considerations.

## Running Tests

`CHROME_TESTS_DISABLED=true` before running tests (e.g., `CHROME_TESTS_DISABLED=true go test ./...`). If Chrome is not found or encounters issues like timeouts, these tests are designed to skip gracefully or log the issue, allowing other tests to pass.

This project includes unit and integration tests.

1.  **Run all tests:**
    To run all tests in the project from the root directory:
    ```bash
    go test ./... -v
    ```
    The `-v` flag enables verbose output.

2.  **Run tests for a specific package:**
    For example, to run tests only for the `storage` package:
    ```bash
    go test ./storage -v -count=1
    ```
    Or for the `handlers` package:
    ```bash
    go test ./handlers -v -count=1
    ```
    The `-count=1` flag disables test caching, which can be useful to ensure tests run fresh each time.

3.  **Test Coverage (Optional):**
    To generate a test coverage report:
    ```bash
    go test ./... -coverprofile=coverage.out
    go tool cover -html=coverage.out
    ```
    This will open an HTML page in your browser showing code coverage.

## Contributing

Contributions are welcome! Please feel free to open an issue or submit a pull request.

## License

This project is open source. (Consider adding a LICENSE file, e.g., MIT or Apache 2.0)
