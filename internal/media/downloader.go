package media

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// Downloader handles media file downloads
type Downloader struct {
	config     *config.Config
	httpClient *http.Client
	mediaDir   string
	progress   *progressbar.ProgressBar
}

// NewDownloader creates a new media downloader
func NewDownloader(cfg *config.Config) *Downloader {
	return &Downloader{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
		mediaDir: cfg.GetMediaDir(),
	}
}

// DownloadMedia downloads all media files from the provided media items
func (d *Downloader) DownloadMedia(mediaItems []models.WordPressMedia) (int, error) {
	if !d.config.DownloadMedia || len(mediaItems) == 0 {
		return 0, nil
	}

	// Ensure media directory exists
	if err := os.MkdirAll(d.mediaDir, 0750); err != nil {
		return 0, fmt.Errorf("failed to create media directory: %w", err)
	}

	// Validate media directory path
	if !filepath.IsAbs(d.mediaDir) {
		return 0, fmt.Errorf("media directory path must be absolute")
	}

	// Create progress bar
	d.progress = progressbar.NewOptions(len(mediaItems),
		progressbar.OptionSetDescription("Downloading media"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// Create worker pool for concurrent downloads
	jobs := make(chan models.WordPressMedia, len(mediaItems))
	results := make(chan bool, len(mediaItems))

	// Start workers
	for i := 0; i < d.config.Concurrent; i++ {
		go d.worker(jobs, results)
	}

	// Send jobs
	for _, media := range mediaItems {
		jobs <- media
	}
	close(jobs)

	// Collect results
	downloaded := 0
	for i := 0; i < len(mediaItems); i++ {
		if <-results {
			downloaded++
		}
		if err := d.progress.Add(1); err != nil {
			return downloaded, err
		}
	}

	if err := d.progress.Finish(); err != nil {
		return downloaded, err
	}
	return downloaded, nil
}

// worker processes media download jobs
func (d *Downloader) worker(jobs <-chan models.WordPressMedia, results chan<- bool) {
	for media := range jobs {
		success := d.downloadMediaItem(media)
		results <- success
	}
}

// downloadMediaItem downloads a single media item
func (d *Downloader) downloadMediaItem(media models.WordPressMedia) bool {
	if media.SourceURL == "" {
		return false
	}

	// Parse URL to get filename
	parsedURL, err := url.Parse(media.SourceURL)
	if err != nil {
		if d.config.Verbose {
			fmt.Printf("Invalid media URL: %s\n", media.SourceURL)
		}
		return false
	}

	// Generate filename
	filename := d.generateFilename(media, parsedURL)
	filePath := filepath.Join(d.mediaDir, filename)

	// Validate file path
	if !filepath.IsAbs(filePath) {
		return false
	}

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return true // File already exists
	}

	// Download file with retries
	for attempt := 0; attempt <= d.config.Retries; attempt++ {
		if d.downloadFile(media.SourceURL, filePath) {
			return true
		}

		if attempt < d.config.Retries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	return false
}

// downloadFile downloads a file from URL to local path
func (d *Downloader) downloadFile(url, filePath string) bool {
	// Validate file path to prevent directory traversal
	if err := d.validateFilePath(filePath); err != nil {
		if d.config.Verbose {
			fmt.Printf("Invalid file path %s: %v\n", filePath, err)
		}
		return false
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", d.config.UserAgent)

	// Make request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Clean and validate file path before creation to prevent directory traversal
	cleanFilePath := filepath.Clean(filePath)

	// Create file
	file, err := os.Create(cleanFilePath)
	if err != nil {
		return false
	}
	defer func() {
		_ = file.Close()
	}()

	// Copy data
	_, err = io.Copy(file, resp.Body)
	return err == nil
}

// validateFilePath validates that the file path is safe and within the media directory
func (d *Downloader) validateFilePath(filePath string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(filePath)

	// Get absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get absolute media directory path
	absMediaDir, err := filepath.Abs(d.mediaDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute media directory: %w", err)
	}

	// Check if the file path is within the media directory
	relPath, err := filepath.Rel(absMediaDir, absPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Ensure the relative path doesn't start with .. (which would indicate it's outside the media dir)
	if strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, "/") {
		return fmt.Errorf("file path is outside media directory")
	}

	return nil
}

// generateFilename generates a unique filename for a media item
func (d *Downloader) generateFilename(media models.WordPressMedia, parsedURL *url.URL) string {
	// Get original filename from URL
	originalName := filepath.Base(parsedURL.Path)

	// If no filename in URL, generate one
	if originalName == "" || originalName == "." || originalName == "/" {
		ext := d.getExtensionFromMimeType(media.MimeType)
		originalName = fmt.Sprintf("media_%d%s", media.ID, ext)
	}

	// Sanitize filename
	filename := d.sanitizeFilename(originalName)

	// Add ID prefix to avoid conflicts
	name := filepath.Base(filename)
	ext := filepath.Ext(name)
	nameWithoutExt := strings.TrimSuffix(name, ext)

	return fmt.Sprintf("%d_%s%s", media.ID, nameWithoutExt, ext)
}

// sanitizeFilename removes invalid characters from filename
func (d *Downloader) sanitizeFilename(filename string) string {
	// Replace invalid characters
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	sanitized := filename

	for _, char := range invalid {
		sanitized = strings.ReplaceAll(sanitized, char, "_")
	}

	// Limit length
	if len(sanitized) > 200 {
		ext := filepath.Ext(sanitized)
		name := strings.TrimSuffix(sanitized, ext)
		sanitized = name[:200-len(ext)] + ext
	}

	return sanitized
}

// getExtensionFromMimeType returns file extension based on MIME type
func (d *Downloader) getExtensionFromMimeType(mimeType string) string {
	extensions := map[string]string{
		"image/jpeg":      ".jpg",
		"image/jpg":       ".jpg",
		"image/png":       ".png",
		"image/gif":       ".gif",
		"image/webp":      ".webp",
		"image/svg+xml":   ".svg",
		"image/bmp":       ".bmp",
		"image/tiff":      ".tiff",
		"video/mp4":       ".mp4",
		"video/avi":       ".avi",
		"video/mov":       ".mov",
		"video/wmv":       ".wmv",
		"video/flv":       ".flv",
		"video/webm":      ".webm",
		"audio/mp3":       ".mp3",
		"audio/wav":       ".wav",
		"audio/ogg":       ".ogg",
		"application/pdf": ".pdf",
		"text/plain":      ".txt",
		"application/zip": ".zip",
	}

	if ext, exists := extensions[mimeType]; exists {
		return ext
	}

	return ".bin" // Default extension
}

// UpdateMediaPaths updates media URLs in content to point to local files
func (d *Downloader) UpdateMediaPaths(content string, mediaItems []models.WordPressMedia) string {
	if !d.config.DownloadMedia {
		return content
	}

	updated := content

	for _, media := range mediaItems {
		if media.SourceURL == "" {
			continue
		}

		// Parse URL to generate local filename
		parsedURL, err := url.Parse(media.SourceURL)
		if err != nil {
			continue
		}

		filename := d.generateFilename(media, parsedURL)
		localPath := filepath.Join("media", filename)

		// Replace absolute URLs with relative paths
		updated = strings.ReplaceAll(updated, media.SourceURL, localPath)

		// Also check for different size variants
		if media.MediaDetails.Sizes != nil {
			for _, size := range media.MediaDetails.Sizes {
				if size.SourceURL != "" {
					sizeFilename := d.generateSizeFilename(media, size, parsedURL)
					sizePath := filepath.Join("media", sizeFilename)
					updated = strings.ReplaceAll(updated, size.SourceURL, sizePath)
				}
			}
		}
	}

	return updated
}

// generateSizeFilename generates filename for media size variants
func (d *Downloader) generateSizeFilename(media models.WordPressMedia, size models.MediaSize, originalURL *url.URL) string {
	// Parse size URL
	sizeURL, err := url.Parse(size.SourceURL)
	if err != nil {
		return d.generateFilename(media, originalURL)
	}

	// Get size filename
	sizeFilename := filepath.Base(sizeURL.Path)
	if sizeFilename == "" {
		return d.generateFilename(media, originalURL)
	}

	// Sanitize and add ID prefix
	sanitized := d.sanitizeFilename(sizeFilename)
	name := filepath.Base(sanitized)
	ext := filepath.Ext(name)
	nameWithoutExt := strings.TrimSuffix(name, ext)

	return fmt.Sprintf("%d_%s%s", media.ID, nameWithoutExt, ext)
}
