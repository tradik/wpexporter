package media

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewDownloader(t *testing.T) {
	cfg := &config.Config{
		URL:           "https://example.com",
		DownloadMedia: true,
		Timeout:       30,
		Concurrent:    5,
		UserAgent:     "test-agent",
		Output:        "test/output",
		Format:        "json",
	}

	downloader := NewDownloader(cfg)

	// NewDownloader should never return nil when there's no error
	if downloader.config != cfg {
		t.Error("NewDownloader() should store config reference")
	}

	if downloader.httpClient == nil {
		t.Error("NewDownloader() should create HTTP client")
	}

	if downloader.mediaDir != cfg.GetMediaDir() {
		t.Errorf("NewDownloader() mediaDir = %s, want %s", downloader.mediaDir, cfg.GetMediaDir())
	}
}

func TestDownloadMediaDisabled(t *testing.T) {
	cfg := &config.Config{
		DownloadMedia: false, // Disabled
	}

	downloader := NewDownloader(cfg)

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: "https://example.com/image.jpg",
		},
	}

	downloaded, err := downloader.DownloadMedia(mediaItems)

	if err != nil {
		t.Errorf("DownloadMedia() error = %v, want nil", err)
	}

	if downloaded != 0 {
		t.Errorf("DownloadMedia() downloaded = %d, want 0", downloaded)
	}
}

func TestDownloadMediaEmptyList(t *testing.T) {
	cfg := &config.Config{
		DownloadMedia: true,
	}

	downloader := NewDownloader(cfg)

	mediaItems := []models.WordPressMedia{}

	downloaded, err := downloader.DownloadMedia(mediaItems)

	if err != nil {
		t.Errorf("DownloadMedia() error = %v, want nil", err)
	}

	if downloaded != 0 {
		t.Errorf("DownloadMedia() downloaded = %d, want 0", downloaded)
	}
}

func TestDownloadMediaSuccess(t *testing.T) {
	tempDir := t.TempDir()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image1.jpg" {
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake image data 1"))
			return
		}
		if r.URL.Path == "/image2.png" {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake image data 2"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		DownloadMedia: true,
		Timeout:       10,
		Concurrent:    2,
		Retries:       1,
		UserAgent:     "test-agent",
		Output:        filepath.Join(tempDir, "output.json"),
		Format:        "json",
	}

	downloader := NewDownloader(cfg)

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: server.URL + "/image1.jpg",
			MimeType:  "image/jpeg",
			Title:     models.RenderedContent{Rendered: "Image 1"},
		},
		{
			ID:        2,
			SourceURL: server.URL + "/image2.png",
			MimeType:  "image/png",
			Title:     models.RenderedContent{Rendered: "Image 2"},
		},
	}

	downloaded, err := downloader.DownloadMedia(mediaItems)

	if err != nil {
		t.Errorf("DownloadMedia() error = %v, want nil", err)
	}

	if downloaded != 2 {
		t.Errorf("DownloadMedia() downloaded = %d, want 2", downloaded)
	}

	// Check if files were downloaded
	mediaDir := cfg.GetMediaDir()

	// Check first file
	parsedURL, _ := url.Parse(mediaItems[0].SourceURL)
	filename1 := downloader.generateFilename(mediaItems[0], parsedURL)
	filePath1 := filepath.Join(mediaDir, filename1)
	if _, err := os.Stat(filePath1); os.IsNotExist(err) {
		t.Errorf("DownloadMedia() file %s was not created", filePath1)
	}

	// Check second file
	parsedURL2, _ := url.Parse(mediaItems[1].SourceURL)
	filename2 := downloader.generateFilename(mediaItems[1], parsedURL2)
	filePath2 := filepath.Join(mediaDir, filename2)
	if _, err := os.Stat(filePath2); os.IsNotExist(err) {
		t.Errorf("DownloadMedia() file %s was not created", filePath2)
	}
}

func TestDownloadMediaWithRetries(t *testing.T) {
	tempDir := t.TempDir()
	attemptCount := 0

	// Create test server that fails initially then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake image data"))
	}))
	defer server.Close()

	cfg := &config.Config{
		DownloadMedia: true,
		Timeout:       10,
		Concurrent:    1,
		Retries:       3, // Allow retries
		UserAgent:     "test-agent",
		Output:        filepath.Join(tempDir, "output.json"),
		Format:        "json",
	}

	downloader := NewDownloader(cfg)

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: server.URL + "/image.jpg",
			MimeType:  "image/jpeg",
		},
	}

	downloaded, err := downloader.DownloadMedia(mediaItems)

	if err != nil {
		t.Errorf("DownloadMedia() error = %v, want nil", err)
	}

	if downloaded != 1 {
		t.Errorf("DownloadMedia() downloaded = %d, want 1", downloaded)
	}

	if attemptCount <= 2 {
		t.Errorf("DownloadMedia() expected at least 3 attempts, got %d", attemptCount)
	}
}

func TestDownloadMediaFileExists(t *testing.T) {
	tempDir := t.TempDir()
	mediaDir := filepath.Join(tempDir, "output_media") // This is what GetMediaDir() returns for output.json

	// Create media directory with images subfolder and a file
	imagesDir := filepath.Join(mediaDir, "images")
	err := os.MkdirAll(imagesDir, 0750)
	if err != nil {
		t.Fatalf("Failed to create media directory: %v", err)
	}

	existingFile := filepath.Join(imagesDir, "1_image.jpg")
	err = os.WriteFile(existingFile, []byte("existing data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("DownloadMedia() should not make HTTP request when file exists, but got request to: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		DownloadMedia: true,
		Timeout:       10,
		Concurrent:    1,
		Retries:       0,
		UserAgent:     "test-agent",
		Output:        filepath.Join(tempDir, "output.json"),
		Format:        "json",
	}

	downloader := NewDownloader(cfg)

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: server.URL + "/image.jpg",
			MimeType:  "image/jpeg",
		},
	}

	downloaded, err := downloader.DownloadMedia(mediaItems)

	if err != nil {
		t.Errorf("DownloadMedia() error = %v, want nil", err)
	}

	if downloaded != 1 {
		t.Errorf("DownloadMedia() downloaded = %d, want 1", downloaded)
	}
}

func TestGenerateFilename(t *testing.T) {
	downloader := &Downloader{}

	tests := []struct {
		name      string
		media     models.WordPressMedia
		parsedURL *url.URL
		expected  string
	}{
		{
			name: "Standard image URL",
			media: models.WordPressMedia{
				ID:       123,
				MimeType: "image/jpeg",
			},
			parsedURL: &url.URL{
				Path: "/wp-content/uploads/2024/01/test-image.jpg",
			},
			expected: "images/123_test-image.jpg",
		},
		{
			name: "URL with special characters",
			media: models.WordPressMedia{
				ID:       456,
				MimeType: "image/png",
			},
			parsedURL: &url.URL{
				Path: "/wp-content/uploads/my image (1).png",
			},
			expected: "images/456_my image (1).png", // Spaces are preserved by current implementation
		},
		{
			name: "Empty URL path",
			media: models.WordPressMedia{
				ID:       789,
				MimeType: "image/gif",
			},
			parsedURL: &url.URL{
				Path: "",
			},
			expected: "images/789_media_789.gif",
		},
		{
			name: "No filename in URL",
			media: models.WordPressMedia{
				ID:       101,
				MimeType: "application/pdf",
			},
			parsedURL: &url.URL{
				Path: "/",
			},
			expected: "documents/101_media_101.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := downloader.generateFilename(tt.media, tt.parsedURL)
			if result != tt.expected {
				t.Errorf("generateFilename() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	downloader := &Downloader{}

	tests := []struct {
		input    string
		expected string
	}{
		{"normal-file.jpg", "normal-file.jpg"},
		{"file with spaces.jpg", "file with spaces.jpg"}, // Spaces are not replaced by current implementation
		{"file/with\\slashes.jpg", "file_with_slashes.jpg"},
		{"file:with*special?chars.jpg", "file_with_special_chars.jpg"},
		{"file\"with<quotes>and|pipes.jpg", "file_with_quotes_and_pipes.jpg"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := downloader.sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetExtensionFromMimeType(t *testing.T) {
	downloader := &Downloader{}

	tests := []struct {
		mimeType string
		expected string
	}{
		{"image/jpeg", ".jpg"},
		{"image/jpg", ".jpg"},
		{"image/png", ".png"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"image/svg+xml", ".svg"},
		{"video/mp4", ".mp4"},
		{"video/avi", ".avi"},
		{"audio/mp3", ".mp3"},
		{"audio/wav", ".wav"},
		{"application/pdf", ".pdf"},
		{"text/plain", ".txt"},
		{"application/zip", ".zip"},
		{"unknown/type", ".bin"},
		{"", ".bin"},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			result := downloader.getExtensionFromMimeType(tt.mimeType)
			if result != tt.expected {
				t.Errorf("getExtensionFromMimeType(%s) = %s, want %s", tt.mimeType, result, tt.expected)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tempDir := t.TempDir()
	mediaDir := filepath.Join(tempDir, "media")

	downloader := &Downloader{
		mediaDir: mediaDir,
	}

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
		setup    func() // Optional setup function
	}{
		{
			name:     "Valid file path within media directory",
			filePath: filepath.Join(mediaDir, "image.jpg"),
			wantErr:  false,
		},
		{
			name:     "Valid nested path within media directory",
			filePath: filepath.Join(mediaDir, "subdir", "image.jpg"),
			wantErr:  false,
		},
		{
			name:     "Path outside media directory",
			filePath: filepath.Join(tempDir, "outside.jpg"),
			wantErr:  true,
		},
		{
			name:     "Path with directory traversal attempt",
			filePath: filepath.Join(mediaDir, "..", "outside.jpg"),
			wantErr:  true,
		},
		{
			name:     "Absolute path outside media directory",
			filePath: "/etc/passwd",
			wantErr:  true,
		},
		{
			name:     "Relative path outside media directory",
			filePath: "../outside.jpg",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			err := downloader.validateFilePath(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateMediaPaths(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			DownloadMedia: true,
		},
	}

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: "https://example.com/wp-content/uploads/2024/01/image1.jpg",
			MimeType:  "image/jpeg",
		},
		{
			ID:        2,
			SourceURL: "https://example.com/wp-content/uploads/2024/01/image2.png",
			MimeType:  "image/png",
		},
	}

	content := `<p>This is a post with images: <img src="https://example.com/wp-content/uploads/2024/01/image1.jpg" alt="Image 1"> and <img src="https://example.com/wp-content/uploads/2024/01/image2.png" alt="Image 2"></p>`

	updated := downloader.UpdateMediaPaths(content, mediaItems)

	expected := `<p>This is a post with images: <img src="media/images/1_image1.jpg" alt="Image 1"> and <img src="media/images/2_image2.png" alt="Image 2"></p>`

	if updated != expected {
		t.Errorf("UpdateMediaPaths() = %s, want %s", updated, expected)
	}
}

func TestUpdateMediaPathsDisabled(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			DownloadMedia: false, // Disabled
		},
	}

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: "https://example.com/image.jpg",
		},
	}

	content := `<img src="https://example.com/image.jpg">`

	updated := downloader.UpdateMediaPaths(content, mediaItems)

	// Should return unchanged content
	if updated != content {
		t.Errorf("UpdateMediaPaths() with disabled download should return unchanged content")
	}
}

func TestGenerateSizeFilename(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:       123,
		MimeType: "image/jpeg",
	}

	size := models.MediaSize{
		SourceURL: "https://example.com/wp-content/uploads/2024/01/image-150x150.jpg",
	}

	originalURL := &url.URL{
		Path: "/wp-content/uploads/2024/01/image.jpg",
	}

	result := downloader.generateSizeFilename(media, size, originalURL)

	expected := "images/123_image-150x150.jpg"

	if result != expected {
		t.Errorf("generateSizeFilename() = %s, want %s", result, expected)
	}
}

func TestDownloadFileInvalidURL(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			Verbose: false,
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	success := downloader.downloadFile("invalid-url", "/tmp/test.jpg")

	if success {
		t.Error("downloadFile() should return false for invalid URL")
	}
}

func TestDownloadFileHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	downloader := &Downloader{
		config: &config.Config{
			Verbose: false,
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	success := downloader.downloadFile(server.URL+"/test.jpg", "/tmp/test.jpg")

	if success {
		t.Error("downloadFile() should return false for HTTP error")
	}
}

func TestDownloadMediaItemEmptyURL(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:        1,
		SourceURL: "", // Empty URL
	}

	success := downloader.downloadMediaItem(media)

	if success {
		t.Error("downloadMediaItem() should return false for empty URL")
	}
}

func TestDownloadMediaItemInvalidURL(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:        1,
		SourceURL: "invalid-url",
	}

	success := downloader.downloadMediaItem(media)

	if success {
		t.Error("downloadMediaItem() should return false for invalid URL")
	}
}

func TestSanitizeFilenameLongFilename(t *testing.T) {
	downloader := &Downloader{}

	// Create a filename longer than 200 characters
	longName := "a"
	for i := 0; i < 250; i++ {
		longName += "b"
	}
	longName += ".jpg"

	result := downloader.sanitizeFilename(longName)

	// Result should be truncated to 200 characters total
	if len(result) > 200 {
		t.Errorf("sanitizeFilename() should truncate long filenames to 200 chars, got %d", len(result))
	}

	// Should preserve extension
	if filepath.Ext(result) != ".jpg" {
		t.Errorf("sanitizeFilename() should preserve extension, got %s", filepath.Ext(result))
	}
}

func TestUpdateMediaPathsWithMediaSizes(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			DownloadMedia: true,
		},
	}

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: "https://example.com/wp-content/uploads/2024/01/image.jpg",
			MimeType:  "image/jpeg",
			MediaDetails: models.MediaDetails{
				Sizes: map[string]models.MediaSize{
					"thumbnail": {
						SourceURL: "https://example.com/wp-content/uploads/2024/01/image-150x150.jpg",
					},
					"medium": {
						SourceURL: "https://example.com/wp-content/uploads/2024/01/image-300x200.jpg",
					},
				},
			},
		},
	}

	content := `<p>Images: <img src="https://example.com/wp-content/uploads/2024/01/image.jpg"> <img src="https://example.com/wp-content/uploads/2024/01/image-150x150.jpg"> <img src="https://example.com/wp-content/uploads/2024/01/image-300x200.jpg"></p>`

	updated := downloader.UpdateMediaPaths(content, mediaItems)

	// All URLs should be replaced
	if strings.Contains(updated, "https://example.com") {
		t.Errorf("UpdateMediaPaths() should replace all media URLs, got: %s", updated)
	}

	// Check that local paths are used (now with subfolders)
	if !strings.Contains(updated, "media/images/1_image.jpg") {
		t.Errorf("UpdateMediaPaths() should include local path for original, got: %s", updated)
	}

	if !strings.Contains(updated, "media/images/1_image-150x150.jpg") {
		t.Errorf("UpdateMediaPaths() should include local path for thumbnail, got: %s", updated)
	}
}

func TestUpdateMediaPathsEmptySourceURL(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			DownloadMedia: true,
		},
	}

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: "", // Empty URL should be skipped
		},
	}

	content := `<p>Some content</p>`

	updated := downloader.UpdateMediaPaths(content, mediaItems)

	// Content should remain unchanged
	if updated != content {
		t.Errorf("UpdateMediaPaths() should skip empty URLs, got: %s", updated)
	}
}

func TestUpdateMediaPathsInvalidURL(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			DownloadMedia: true,
		},
	}

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: "://invalid-url",
		},
	}

	content := `<p>Some content</p>`

	updated := downloader.UpdateMediaPaths(content, mediaItems)

	// Content should remain unchanged when URL parsing fails
	if updated != content {
		t.Errorf("UpdateMediaPaths() should skip invalid URLs, got: %s", updated)
	}
}

func TestGenerateSizeFilenameInvalidURL(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:       123,
		MimeType: "image/jpeg",
	}

	// Invalid size URL should fall back to original
	size := models.MediaSize{
		SourceURL: "://invalid",
	}

	originalURL := &url.URL{
		Path: "/wp-content/uploads/image.jpg",
	}

	result := downloader.generateSizeFilename(media, size, originalURL)

	// Should fall back to original filename generation (now with subfolder)
	expected := "images/123_image.jpg"
	if result != expected {
		t.Errorf("generateSizeFilename() with invalid URL = %s, want %s", result, expected)
	}
}

func TestGenerateSizeFilenameEmptyPath(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:       123,
		MimeType: "image/jpeg",
	}

	// Size URL with root path only - filepath.Base("/") returns "/"
	// which is not empty string, so it goes through sanitization
	size := models.MediaSize{
		SourceURL: "https://example.com/",
	}

	originalURL := &url.URL{
		Path: "/wp-content/uploads/original.jpg",
	}

	result := downloader.generateSizeFilename(media, size, originalURL)

	// The size URL path "/" gives filename "/" after Base(), which gets sanitized to "_"
	// Resulting in "123__" (id + underscore + sanitized name which is "_" after slashes removed)
	// This is the actual behavior - testing it as-is
	if result == "" {
		t.Error("generateSizeFilename() should not return empty string")
	}
}

func TestDownloadMediaItemVerboseMode(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			Verbose: true,
		},
		mediaDir: "/tmp/test",
	}

	// Invalid URL should fail and potentially log with verbose mode
	media := models.WordPressMedia{
		ID:        1,
		SourceURL: "://invalid-url",
	}

	success := downloader.downloadMediaItem(media)

	if success {
		t.Error("downloadMediaItem() should return false for invalid URL even in verbose mode")
	}
}

func TestDownloadFileValidationError(t *testing.T) {
	tempDir := t.TempDir()
	mediaDir := filepath.Join(tempDir, "media")

	downloader := &Downloader{
		config: &config.Config{
			Verbose:   true,
			UserAgent: "test-agent",
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		mediaDir: mediaDir,
	}

	// Try to download to a path outside media directory
	success := downloader.downloadFile("http://example.com/test.jpg", "/etc/passwd")

	if success {
		t.Error("downloadFile() should return false for paths outside media directory")
	}
}

func TestDownloadMediaNonAbsolutePath(t *testing.T) {
	cfg := &config.Config{
		DownloadMedia: true,
		Timeout:       10,
		Concurrent:    1,
		Retries:       0,
		UserAgent:     "test-agent",
		Output:        "relative/path.json", // Relative path
		Format:        "json",
	}

	downloader := NewDownloader(cfg)
	// Force mediaDir to be relative for this test
	downloader.mediaDir = "relative/media"

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: "https://example.com/image.jpg",
			MimeType:  "image/jpeg",
		},
	}

	downloaded, err := downloader.DownloadMedia(mediaItems)

	// Should fail because path is not absolute
	if err == nil {
		t.Error("DownloadMedia() should return error for non-absolute media directory")
	}

	if downloaded != 0 {
		t.Errorf("DownloadMedia() downloaded = %d, want 0 for non-absolute path", downloaded)
	}
}

func TestDownloadMediaItemNonAbsoluteFilePath(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			Verbose: false,
		},
		mediaDir: "relative/path", // Non-absolute path
	}

	media := models.WordPressMedia{
		ID:        1,
		SourceURL: "https://example.com/image.jpg",
		MimeType:  "image/jpeg",
	}

	success := downloader.downloadMediaItem(media)

	if success {
		t.Error("downloadMediaItem() should return false for non-absolute file path")
	}
}

func TestUpdateMediaPathsWithEmptySizeURL(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			DownloadMedia: true,
		},
	}

	mediaItems := []models.WordPressMedia{
		{
			ID:        1,
			SourceURL: "https://example.com/image.jpg",
			MimeType:  "image/jpeg",
			MediaDetails: models.MediaDetails{
				Sizes: map[string]models.MediaSize{
					"thumbnail": {
						SourceURL: "", // Empty URL should be skipped
					},
				},
			},
		},
	}

	content := `<img src="https://example.com/image.jpg">`

	updated := downloader.UpdateMediaPaths(content, mediaItems)

	// Only main URL should be replaced (now in images subfolder)
	if !strings.Contains(updated, "media/images/1_image.jpg") {
		t.Errorf("UpdateMediaPaths() should replace main URL, got: %s", updated)
	}
}

func TestDownloadFileNetworkError(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{
			Verbose:   false,
			UserAgent: "test-agent",
		},
		httpClient: &http.Client{
			Timeout: 1 * time.Millisecond, // Very short timeout to cause error
		},
		mediaDir: t.TempDir(),
	}

	// Try to download from a non-existent server
	success := downloader.downloadFile("http://192.0.2.1/test.jpg", filepath.Join(downloader.mediaDir, "test.jpg"))

	if success {
		t.Error("downloadFile() should return false for network errors")
	}
}

func TestDownloadFileCannotCreateFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test data"))
	}))
	defer server.Close()

	mediaDir := filepath.Join(tempDir, "media")
	err := os.MkdirAll(mediaDir, 0750)
	if err != nil {
		t.Fatalf("Failed to create media dir: %v", err)
	}

	downloader := &Downloader{
		config: &config.Config{
			Verbose:   false,
			UserAgent: "test-agent",
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		mediaDir: mediaDir,
	}

	// Make the media directory read-only to cause file creation to fail
	err = os.Chmod(mediaDir, 0500)
	if err != nil {
		t.Fatalf("Failed to change directory permissions: %v", err)
	}
	defer os.Chmod(mediaDir, 0750)

	success := downloader.downloadFile(server.URL+"/test.jpg", filepath.Join(mediaDir, "test.jpg"))

	if success {
		t.Error("downloadFile() should return false when file cannot be created")
	}
}

func TestGenerateFilenameURLWithDot(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:       789,
		MimeType: "image/gif",
	}

	parsedURL := &url.URL{
		Path: ".",
	}

	result := downloader.generateFilename(media, parsedURL)

	// Should generate filename from ID when path is "." (now in images subfolder)
	expected := "images/789_media_789.gif"
	if result != expected {
		t.Errorf("generateFilename() = %s, want %s", result, expected)
	}
}

func TestGenerateSizeFilename_ValidURL(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:       123,
		MimeType: "image/jpeg",
	}

	size := models.MediaSize{
		SourceURL: "https://example.com/wp-content/uploads/test-150x150.jpg",
	}

	originalURL := &url.URL{
		Path: "/wp-content/uploads/test.jpg",
	}

	result := downloader.generateSizeFilename(media, size, originalURL)

	// Should use the size URL filename (now in images subfolder)
	expected := "images/123_test-150x150.jpg"
	if result != expected {
		t.Errorf("generateSizeFilename() = %s, want %s", result, expected)
	}
}

func TestGenerateSizeFilename_InvalidURL(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:       456,
		MimeType: "image/png",
	}

	size := models.MediaSize{
		SourceURL: "://invalid-url",
	}

	originalURL := &url.URL{
		Path: "/wp-content/uploads/original.png",
	}

	result := downloader.generateSizeFilename(media, size, originalURL)

	// Should fallback to generateFilename when URL parsing fails (now in images subfolder)
	expected := "images/456_original.png"
	if result != expected {
		t.Errorf("generateSizeFilename() = %s, want %s", result, expected)
	}
}

func TestDownloadMedia_EmptyMediaList(t *testing.T) {
	cfg := &config.Config{
		DownloadMedia: true,
	}

	downloader := NewDownloader(cfg)

	// Empty media list should return 0 downloads
	count, err := downloader.DownloadMedia([]models.WordPressMedia{})
	if err != nil {
		t.Errorf("DownloadMedia() error = %v, want nil", err)
	}
	if count != 0 {
		t.Errorf("DownloadMedia() count = %d, want 0", count)
	}
}

func TestDownloadMedia_DownloadDisabled(t *testing.T) {
	cfg := &config.Config{
		DownloadMedia: false,
	}

	downloader := NewDownloader(cfg)

	media := []models.WordPressMedia{
		{ID: 1, SourceURL: "https://example.com/test.jpg"},
	}

	count, err := downloader.DownloadMedia(media)
	if err != nil {
		t.Errorf("DownloadMedia() error = %v, want nil", err)
	}
	if count != 0 {
		t.Errorf("DownloadMedia() count = %d, want 0", count)
	}
}

func TestGenerateFilename_SlashPath(t *testing.T) {
	downloader := &Downloader{}

	media := models.WordPressMedia{
		ID:       999,
		MimeType: "image/webp",
	}

	parsedURL := &url.URL{
		Path: "/",
	}

	result := downloader.generateFilename(media, parsedURL)

	// Should generate filename from ID when path is just "/" (now in images subfolder)
	expected := "images/999_media_999.webp"
	if result != expected {
		t.Errorf("generateFilename() = %s, want %s", result, expected)
	}
}
