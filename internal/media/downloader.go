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

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// Downloader handles media file downloads
type Downloader struct {
	config     *config.Config
	httpClient *http.Client
	mediaDir   string
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

	if !d.config.Quiet {
		fmt.Printf("Downloading %d media files to: %s\n", len(mediaItems), d.mediaDir)
	}

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
	}

	if !d.config.Quiet {
		fmt.Printf("Downloaded %d/%d media files\n", downloaded, len(mediaItems))
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

	// Check if media type should be excluded
	if d.shouldExcludeMedia(media) {
		if d.config.Verbose {
			fmt.Printf("  ⊘ Skipping excluded type: %s (%s)\n", media.MimeType, media.SourceURL)
		}
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

	// Generate filename (includes subfolder like images/123_file.jpg)
	filename := d.generateFilename(media, parsedURL)
	filePath := filepath.Join(d.mediaDir, filename)

	// Validate file path
	if !filepath.IsAbs(filePath) {
		return false
	}

	// Ensure subfolder exists
	fileDir := filepath.Dir(filePath)
	if err := os.MkdirAll(fileDir, 0750); err != nil {
		if d.config.Verbose {
			fmt.Printf("Failed to create directory %s: %v\n", fileDir, err)
		}
		return false
	}

	// Check if file already exists
	mainFileExists := false
	if _, err := os.Stat(filePath); err == nil {
		if !d.config.Quiet {
			fmt.Printf("  ✓ %s (exists)\n", filename)
		}
		mainFileExists = true
	}

	// Download file with retries (only if it doesn't exist)
	mainDownloadSuccess := mainFileExists
	if !mainFileExists {
		for attempt := 0; attempt <= d.config.Retries; attempt++ {
			if d.downloadFile(media.SourceURL, filePath) {
				if !d.config.Quiet {
					fmt.Printf("  ↓ %s\n", filename)
				}
				mainDownloadSuccess = true
				break
			}

			if attempt < d.config.Retries {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
		}
	}

	if !mainDownloadSuccess {
		if !d.config.Quiet {
			fmt.Printf("  ✗ %s (failed)\n", filename)
		}
		return false
	}

	// Download media sizes
	if media.MediaDetails.Sizes != nil {
		for _, size := range media.MediaDetails.Sizes {
			if size.SourceURL == "" {
				continue
			}

			// Generate filename for size (includes subfolder)
			sizeFilename := d.generateSizeFilename(media, size, parsedURL)
			sizeFilePath := filepath.Join(d.mediaDir, sizeFilename)

			// Ensure subfolder exists for size variant
			sizeDir := filepath.Dir(sizeFilePath)
			if err := os.MkdirAll(sizeDir, 0750); err != nil {
				continue
			}

			// Check if file already exists
			if _, err := os.Stat(sizeFilePath); err == nil {
				continue
			}

			// Download size variant
			for attempt := 0; attempt <= d.config.Retries; attempt++ {
				if d.downloadFile(size.SourceURL, sizeFilePath) {
					if !d.config.Quiet {
						fmt.Printf("  ↓ %s\n", sizeFilename)
					}
					break
				}
				if attempt < d.config.Retries {
					time.Sleep(time.Duration(attempt+1) * time.Second)
				}
			}
		}
	}

	return true
}

// downloadFile downloads a file from URL to local path
func (d *Downloader) downloadFile(downloadURL, filePath string) bool {
	// Validate file path to prevent directory traversal
	if err := d.validateFilePath(filePath); err != nil {
		if d.config.Verbose {
			fmt.Printf("Invalid file path %s: %v\n", filePath, err)
		}
		return false
	}

	// Create request
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", d.config.UserAgent)

	// Apply authentication only for URLs on the configured WordPress host.
	// Media source URLs come from remote API data and frequently point to a
	// separate CDN host; attaching credentials there would leak them (SEC-001).
	if d.config.IsSameHost(downloadURL) {
		if d.config.AuthToken != "" {
			req.Header.Set("Authorization", "Bearer "+d.config.AuthToken)
		} else if d.config.AuthUser != "" && d.config.AuthPass != "" {
			req.SetBasicAuth(d.config.AuthUser, d.config.AuthPass)
		}
	}

	// Make request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		if d.config.Verbose {
			fmt.Printf("Download error %s: %v\n", downloadURL, err)
		}
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		if d.config.Verbose {
			fmt.Printf("Download failed %s: HTTP %d\n", downloadURL, resp.StatusCode)
		}
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

// generateFilename generates a unique filename for a media item with subfolder by type
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

	// Get subfolder based on MIME type
	subfolder := d.getSubfolderForMimeType(media.MimeType)

	return filepath.Join(subfolder, fmt.Sprintf("%d_%s%s", media.ID, nameWithoutExt, ext))
}

// getSubfolderForMimeType returns the appropriate subfolder based on MIME type
func (d *Downloader) getSubfolderForMimeType(mimeType string) string {
	if strings.HasPrefix(mimeType, "image/") {
		return "images"
	}
	if strings.HasPrefix(mimeType, "video/") {
		return "videos"
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return "audio"
	}

	// Document types
	documentTypes := []string{
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument",
		"application/vnd.ms-excel",
		"application/vnd.ms-powerpoint",
		"application/vnd.oasis.opendocument",
		"text/plain",
		"text/csv",
		"text/markdown",
		"application/epub",
	}
	for _, docType := range documentTypes {
		if strings.HasPrefix(mimeType, docType) {
			return "documents"
		}
	}

	// Archive types
	archiveTypes := []string{
		"application/zip",
		"application/x-zip",
		"application/x-rar",
		"application/vnd.rar",
		"application/x-7z",
		"application/x-tar",
		"application/gzip",
		"application/x-gzip",
		"application/x-bzip",
		"application/x-xz",
		"application/x-compressed",
	}
	for _, archiveType := range archiveTypes {
		if strings.HasPrefix(mimeType, archiveType) {
			return "archives"
		}
	}

	// Code/text types
	codeTypes := []string{
		"text/html",
		"text/css",
		"text/javascript",
		"application/javascript",
		"application/json",
		"text/xml",
		"application/xml",
	}
	for _, codeType := range codeTypes {
		if mimeType == codeType {
			return "code"
		}
	}

	return "other"
}

// shouldExcludeMedia checks if a media item should be excluded based on config
func (d *Downloader) shouldExcludeMedia(media models.WordPressMedia) bool {
	if d.config == nil || len(d.config.ExcludeMediaTypes) == 0 {
		return false
	}

	mimeType := media.MimeType
	subfolder := d.getSubfolderForMimeType(mimeType)
	ext := d.getExtensionFromMimeType(mimeType)

	for _, exclude := range d.config.ExcludeMediaTypes {
		exclude = strings.ToLower(exclude)

		// Check by category (images, videos, audio, documents, archives, code, other)
		if exclude == subfolder {
			return true
		}

		// Check by extension (without dot)
		if "."+exclude == ext {
			return true
		}

		// Check by MIME type prefix (e.g., "image", "video", "audio")
		if strings.HasPrefix(mimeType, exclude+"/") {
			return true
		}

		// Check exact MIME type match
		if mimeType == exclude {
			return true
		}
	}

	return false
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
		// Images
		"image/jpeg":    ".jpg",
		"image/jpg":     ".jpg",
		"image/png":     ".png",
		"image/gif":     ".gif",
		"image/webp":    ".webp",
		"image/svg+xml": ".svg",
		"image/bmp":     ".bmp",
		"image/tiff":    ".tiff",
		"image/x-icon":  ".ico",
		"image/avif":    ".avif",
		"image/heic":    ".heic",
		"image/heif":    ".heif",

		// Video
		"video/mp4":        ".mp4",
		"video/mpeg":       ".mpeg",
		"video/avi":        ".avi",
		"video/x-msvideo":  ".avi",
		"video/quicktime":  ".mov",
		"video/x-ms-wmv":   ".wmv",
		"video/x-flv":      ".flv",
		"video/webm":       ".webm",
		"video/x-matroska": ".mkv",
		"video/3gpp":       ".3gp",
		"video/3gpp2":      ".3g2",
		"video/ogg":        ".ogv",
		"video/x-m4v":      ".m4v",

		// Audio
		"audio/mpeg":     ".mp3",
		"audio/mp3":      ".mp3",
		"audio/wav":      ".wav",
		"audio/x-wav":    ".wav",
		"audio/ogg":      ".ogg",
		"audio/flac":     ".flac",
		"audio/x-flac":   ".flac",
		"audio/aac":      ".aac",
		"audio/mp4":      ".m4a",
		"audio/x-m4a":    ".m4a",
		"audio/webm":     ".weba",
		"audio/x-ms-wma": ".wma",
		"audio/midi":     ".midi",
		"audio/x-midi":   ".midi",
		"audio/x-aiff":   ".aiff",

		// Documents - PDF
		"application/pdf": ".pdf",

		// Documents - Microsoft Office
		"application/msword": ".doc",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
		"application/vnd.ms-excel": ".xls",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
		"application/vnd.ms-powerpoint":                                             ".ppt",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",

		// Documents - OpenDocument
		"application/vnd.oasis.opendocument.text":         ".odt",
		"application/vnd.oasis.opendocument.spreadsheet":  ".ods",
		"application/vnd.oasis.opendocument.presentation": ".odp",

		// Archives
		"application/zip":              ".zip",
		"application/x-zip-compressed": ".zip",
		"application/x-rar-compressed": ".rar",
		"application/vnd.rar":          ".rar",
		"application/x-7z-compressed":  ".7z",
		"application/x-tar":            ".tar",
		"application/gzip":             ".gz",
		"application/x-gzip":           ".gz",
		"application/x-bzip2":          ".bz2",
		"application/x-xz":             ".xz",
		"application/x-compressed-tar": ".tar.gz",

		// Text and code
		"text/plain":             ".txt",
		"text/html":              ".html",
		"text/css":               ".css",
		"text/javascript":        ".js",
		"application/javascript": ".js",
		"application/json":       ".json",
		"text/xml":               ".xml",
		"application/xml":        ".xml",
		"text/csv":               ".csv",
		"text/markdown":          ".md",

		// Other
		"application/octet-stream":      ".bin",
		"application/x-shockwave-flash": ".swf",
		"application/epub+zip":          ".epub",
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

	// Get subfolder based on MIME type (sizes are always images)
	subfolder := d.getSubfolderForMimeType(media.MimeType)

	return filepath.Join(subfolder, fmt.Sprintf("%d_%s%s", media.ID, nameWithoutExt, ext))
}
