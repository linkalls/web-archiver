package storage

import (
	"archive-lite/models"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/html"
	"gorm.io/gorm"
)

var (
	rawHTMLDir      = "data/raw"
	assetsDir       = "data/assets"
	lastRequestTime time.Time
	requestDelay    = 2 * time.Second // Delay between requests to avoid bot detection
	httpClient      *http.Client
)

// init initializes the HTTP client with cookie support
func init() {
	jar, err := cookiejar.New(nil)
	if err != nil {
		// Fallback to client without cookies if jar creation fails
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	} else {
		httpClient = &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		}
	}
}

func SetStorageBaseDirsForTest(testRawHTMLDir, testAssetsDir string) {
	rawHTMLDir = testRawHTMLDir
	assetsDir = testAssetsDir
}

func RawHTMLDirForTest() string { return rawHTMLDir }
func AssetsDirForTest() string  { return assetsDir }

func EnsureStorageDirs() error {
	if err := os.MkdirAll(rawHTMLDir, 0755); err != nil {
		return fmt.Errorf("failed to create raw HTML directory '%s': %w", rawHTMLDir, err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory '%s': %w", assetsDir, err)
	}
	return nil
}

// waitBetweenRequests implements a simple rate limiting to avoid bot detection
func waitBetweenRequests() {
	if !lastRequestTime.IsZero() {
		elapsed := time.Since(lastRequestTime)
		if elapsed < requestDelay {
			time.Sleep(requestDelay - elapsed)
		}
	}
	lastRequestTime = time.Now()
}

// setProperHeaders sets headers to mimic a real browser
func setProperHeaders(req *http.Request, referer ...string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ja,en-US;q=0.9,en;q=0.8")
	// Temporarily disable compression to avoid encoding issues
	// req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	// Add referer if provided
	if len(referer) > 0 && referer[0] != "" {
		req.Header.Set("Referer", referer[0])
	}
}

// resolveRedirects follows redirects and returns the final URL
func resolveRedirects(originalURL string) (string, error) {
	return resolveRedirectsWithReferer(originalURL, "")
}

// resolveRedirectsWithReferer follows redirects with a specific referer and returns the final URL
func resolveRedirectsWithReferer(originalURL, referer string) (string, error) {
	client := httpClient

	req, err := http.NewRequest("GET", originalURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for '%s': %w", originalURL, err)
	}
	setProperHeaders(req, referer)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to resolve redirects for '%s': %w", originalURL, err)
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()

	// Check if we hit a CAPTCHA or sorry page
	if strings.Contains(finalURL, "sorry") || strings.Contains(finalURL, "captcha") {
		return "", fmt.Errorf("access blocked by CAPTCHA or sorry page: %s", finalURL)
	}

	return finalURL, nil
}

// extractFinalURLFromGoogleNews extracts the actual URL from Google News redirect URLs
func extractFinalURLFromGoogleNews(googleNewsURL string) (string, error) {
	// Try to extract URL from Google News format
	if strings.Contains(googleNewsURL, "news.google.com") {
		// Prime Google cookies before accessing Google News
		if err := primeGoogleCookies(); err != nil {
			fmt.Printf("Warning: failed to prime Google cookies: %v\n", err)
		}

		// Wait before accessing Google News
		waitBetweenRequests()

		// First try to follow redirects normally with proper referer
		finalURL, err := resolveRedirectsWithReferer(googleNewsURL, "https://www.google.com")
		if err == nil && !strings.Contains(finalURL, "news.google.com") && !strings.Contains(finalURL, "sorry") {
			return finalURL, nil
		}

		// If that doesn't work, try to parse the URL parameter
		parsedURL, err := url.Parse(googleNewsURL)
		if err != nil {
			return "", fmt.Errorf("failed to parse Google News URL: %w", err)
		}

		// Look for common URL parameters in Google News
		if q := parsedURL.Query().Get("url"); q != "" {
			decodedURL, err := url.QueryUnescape(q)
			if err == nil {
				return decodedURL, nil
			}
		}
	}

	// For other redirect services, just follow redirects
	return resolveRedirects(googleNewsURL)
}

func FetchRawHTML(url string) (string, error) {
	waitBetweenRequests()

	client := httpClient

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for '%s': %w", url, err)
	}
	setProperHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get URL '%s': %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get URL '%s': status code %d", url, resp.StatusCode)
	}

	// Handle gzip-compressed responses
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader for '%s': %w", url, err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body from '%s': %w", url, err)
	}

	return string(bodyBytes), nil
}

func FetchAsset(assetURL string) ([]byte, error) {
	waitBetweenRequests()

	client := httpClient

	req, err := http.NewRequest("GET", assetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for asset '%s': %w", assetURL, err)
	}
	setProperHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset '%s': %w", assetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get asset '%s': status code %d", assetURL, resp.StatusCode)
	}

	// Handle gzip-compressed responses
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader for asset '%s': %w", assetURL, err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	return io.ReadAll(reader)
}

func extractAssetsFromHTML(htmlContent, baseURL string) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var assets []string
	var extractFunc func(*html.Node)
	extractFunc = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var attrName string
			switch n.Data {
			case "link":
				attrName = "href"
			case "script", "img", "iframe":
				attrName = "src"
			}

			if attrName != "" {
				for _, attr := range n.Attr {
					if attr.Key == attrName {
						assetURL := attr.Val
						if resolvedURL := resolveURL(baseURL, assetURL); resolvedURL != "" {
							assets = append(assets, resolvedURL)
						}
						break
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractFunc(c)
		}
	}

	extractFunc(doc)
	return assets, nil
}

func resolveURL(baseURL, relativeURL string) string {
	if relativeURL == "" {
		return ""
	}

	// Skip data: URLs and already absolute URLs
	if strings.HasPrefix(relativeURL, "data:") || strings.HasPrefix(relativeURL, "http://") || strings.HasPrefix(relativeURL, "https://") {
		if strings.HasPrefix(relativeURL, "http") {
			return relativeURL
		}
		return ""
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	relative, err := url.Parse(relativeURL)
	if err != nil {
		return ""
	}

	return base.ResolveReference(relative).String()
}

func generateAssetFileName(assetURL, entryUUID string) string {
	// Create a hash of the URL to avoid filename conflicts
	hasher := md5.New()
	hasher.Write([]byte(assetURL))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))[:8]

	// Extract file extension
	parsedURL, err := url.Parse(assetURL)
	if err != nil {
		return fmt.Sprintf("%s_%s", entryUUID, hash)
	}

	ext := filepath.Ext(parsedURL.Path)
	if ext == "" {
		// Try to guess extension from URL path
		if strings.Contains(assetURL, ".css") {
			ext = ".css"
		} else if strings.Contains(assetURL, ".js") {
			ext = ".js"
		} else if strings.Contains(assetURL, ".png") {
			ext = ".png"
		} else if strings.Contains(assetURL, ".jpg") || strings.Contains(assetURL, ".jpeg") {
			ext = ".jpg"
		}
	}

	return fmt.Sprintf("%s_%s%s", entryUUID, hash, ext)
}

func modifyHTMLPaths(htmlContent, entryUUID, baseURL string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var modifyFunc func(*html.Node)
	modifyFunc = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var attrName string
			switch n.Data {
			case "link":
				attrName = "href"
			case "script", "img", "iframe":
				attrName = "src"
			}

			if attrName != "" {
				for i, attr := range n.Attr {
					if attr.Key == attrName {
						originalURL := attr.Val
						if resolvedURL := resolveURL(baseURL, originalURL); resolvedURL != "" {
							newPath := fmt.Sprintf("/data/assets/%s", generateAssetFileName(resolvedURL, entryUUID))
							n.Attr[i].Val = newPath
						}
						break
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			modifyFunc(c)
		}
	}

	modifyFunc(doc)

	// Convert back to HTML string
	var buf strings.Builder
	err = html.Render(&buf, doc)
	if err != nil {
		return "", fmt.Errorf("failed to render modified HTML: %w", err)
	}

	return buf.String(), nil
}

func ArchiveURL(db *gorm.DB, urlToArchive string) (*models.ArchiveEntry, error) {
	if err := EnsureStorageDirs(); err != nil {
		return nil, fmt.Errorf("failed to ensure storage directories: %w", err)
	}

	// Resolve redirects to get the final URL
	finalURL := urlToArchive
	if strings.Contains(urlToArchive, "news.google.com") ||
		strings.Contains(urlToArchive, "t.co") ||
		strings.Contains(urlToArchive, "bit.ly") ||
		strings.Contains(urlToArchive, "tinyurl.com") {
		resolvedURL, err := extractFinalURLFromGoogleNews(urlToArchive)
		if err != nil {
			fmt.Printf("Warning: failed to resolve redirects for '%s': %v, using original URL\n", urlToArchive, err)
		} else {
			finalURL = resolvedURL
			fmt.Printf("Resolved URL: %s -> %s\n", urlToArchive, finalURL)
		}
	}

	// Fetch raw HTML content from the final URL
	htmlContent, err := FetchRawHTML(finalURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch HTML content for '%s': %w", finalURL, err)
	}

	// Generate unique filename
	entryUUID := uuid.New().String()

	// Extract and save assets using the final URL as base
	assets, err := extractAssetsFromHTML(htmlContent, finalURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract assets from HTML for '%s': %w", urlToArchive, err)
	}
	// Save assets
	fmt.Printf("Found %d assets to download\n", len(assets))
	for i, assetURL := range assets {
		fmt.Printf("Downloading asset %d/%d: %s\n", i+1, len(assets), assetURL)

		assetContent, err := FetchAsset(assetURL)
		if err != nil {
			// Log error but continue with other assets
			fmt.Printf("Warning: failed to fetch asset '%s': %v\n", assetURL, err)
			continue
		}

		// Validate asset content
		if !validateAssetContent(assetContent, assetURL) {
			fmt.Printf("Warning: invalid asset content for '%s', skipping\n", assetURL)
			continue
		}

		assetFileName := generateAssetFileName(assetURL, entryUUID)
		assetFilePath := filepath.Join(assetsDir, assetFileName)

		// Ensure the asset file is written in binary mode
		if err := os.WriteFile(assetFilePath, assetContent, 0644); err != nil {
			fmt.Printf("Warning: failed to save asset '%s' to '%s': %v\n", assetURL, assetFilePath, err)
			continue
		}

		fmt.Printf("Successfully saved asset: %s (%d bytes)\n", assetFileName, len(assetContent))
	}
	// Modify HTML to use local asset paths (use finalURL for proper resolution)
	modifiedHTML, err := modifyHTMLPaths(htmlContent, entryUUID, finalURL)
	if err != nil {
		return nil, fmt.Errorf("failed to modify HTML paths for '%s': %w", finalURL, err)
	}

	// Save modified HTML content to file
	htmlFileName := fmt.Sprintf("%s.html", entryUUID)
	htmlFilePath := filepath.Join(rawHTMLDir, htmlFileName)

	if err := os.WriteFile(htmlFilePath, []byte(modifiedHTML), 0644); err != nil {
		return nil, fmt.Errorf("failed to write HTML to '%s': %w", htmlFilePath, err)
	}

	// Create archive entry in database
	// Store the original URL for reference, but the content comes from the final URL
	archiveEntry := models.ArchiveEntry{
		URL:         finalURL, // Store the resolved URL as the primary URL
		Title:       "",
		StoragePath: htmlFilePath,
		ArchivedAt:  time.Now(),
	}

	result := db.Create(&archiveEntry)
	if result.Error != nil {
		os.Remove(htmlFilePath)
		return nil, fmt.Errorf("failed to create archive entry in database for '%s': %w", finalURL, result.Error)
	}

	return &archiveEntry, nil
}

// primeGoogleCookies visits Google's homepage to establish cookies before accessing Google News
func primeGoogleCookies() error {
	req, err := http.NewRequest("GET", "https://www.google.com", nil)
	if err != nil {
		return fmt.Errorf("failed to create request for Google homepage: %w", err)
	}
	setProperHeaders(req)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to access Google homepage: %w", err)
	}
	defer resp.Body.Close()

	// Just read and discard the response to establish cookies
	io.ReadAll(resp.Body)

	// Wait a bit to make it look more natural
	time.Sleep(1 * time.Second)

	return nil
}

// validateAssetContent validates that the downloaded content is a valid asset
func validateAssetContent(content []byte, assetURL string) bool {
	if len(content) == 0 {
		return false
	}

	// Check for common file signatures
	if len(content) >= 4 {
		// PNG signature
		if content[0] == 0x89 && content[1] == 0x50 && content[2] == 0x4E && content[3] == 0x47 {
			return true
		}
		// JPEG signature
		if content[0] == 0xFF && content[1] == 0xD8 {
			return true
		}
		// GIF signature
		if string(content[:3]) == "GIF" {
			return true
		}
	}

	// For CSS and JS files, check if it contains reasonable text content
	if strings.HasSuffix(strings.ToLower(assetURL), ".css") ||
		strings.HasSuffix(strings.ToLower(assetURL), ".js") {
		// Check if it's printable ASCII/UTF-8 text
		contentStr := string(content[:min(500, len(content))]) // Check first 500 bytes
		for _, r := range contentStr {
			if r > 127 && r < 32 && r != '\n' && r != '\r' && r != '\t' {
				return false // Contains suspicious binary data
			}
		}
		return true
	}

	// For other files, assume they're valid if they have content
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
