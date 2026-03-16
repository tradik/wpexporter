package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	if e.ExpiresAt == nil {
		return false // No expiry = never expires
	}
	return time.Now().After(*e.ExpiresAt)
}

// FileCache implements file-based caching with TTL support
type FileCache struct {
	baseDir  string
	ttl      time.Duration
	siteHash string
	mu       sync.RWMutex
}

// NewFileCache creates a new file-based cache for a specific site
func NewFileCache(baseDir string, ttl time.Duration, siteURL string) (*FileCache, error) {
	siteHash := GenerateSiteHash(siteURL)
	cacheDir := filepath.Join(baseDir, siteHash)

	// Create cache directories
	apiDir := filepath.Join(cacheDir, "api")
	seoDir := filepath.Join(cacheDir, "seo")

	if err := os.MkdirAll(apiDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create API cache directory: %w", err)
	}
	if err := os.MkdirAll(seoDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create SEO cache directory: %w", err)
	}

	return &FileCache{
		baseDir:  cacheDir,
		ttl:      ttl,
		siteHash: siteHash,
	}, nil
}

// Get retrieves data from cache, returns (data, found, error)
func (c *FileCache) Get(key string) ([]byte, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filePath := c.keyToPath(key)

	data, err := os.ReadFile(filePath) // #nosec G304 -- path is constructed from validated cache key
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil // Cache miss
		}
		return nil, false, fmt.Errorf("failed to read cache file: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Corrupted cache entry, treat as miss
		return nil, false, nil
	}

	if entry.IsExpired() {
		// Entry expired, treat as miss (will be overwritten on next Set)
		return nil, false, nil
	}

	return entry.Data, true, nil
}

// Set stores data in cache with TTL
func (c *FileCache) Set(key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt *time.Time
	if c.ttl > 0 {
		t := time.Now().Add(c.ttl)
		expiresAt = &t
	}

	entry := CacheEntry{
		Data:      data,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	entryData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	filePath := c.keyToPath(key)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.WriteFile(filePath, entryData, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Clear removes all cached data for this site
func (c *FileCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.RemoveAll(c.baseDir); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	// Recreate directories
	apiDir := filepath.Join(c.baseDir, "api")
	seoDir := filepath.Join(c.baseDir, "seo")

	if err := os.MkdirAll(apiDir, 0750); err != nil {
		return fmt.Errorf("failed to recreate API cache directory: %w", err)
	}
	if err := os.MkdirAll(seoDir, 0750); err != nil {
		return fmt.Errorf("failed to recreate SEO cache directory: %w", err)
	}

	return nil
}

// Close performs any cleanup (no-op for file cache)
func (c *FileCache) Close() error {
	return nil
}

// GetCacheDir returns the cache directory path
func (c *FileCache) GetCacheDir() string {
	return c.baseDir
}

// keyToPath converts a cache key to a file path
func (c *FileCache) keyToPath(key string) string {
	// Key format: "api:posts:1" or "seo:https://example.com/page"
	parts := strings.SplitN(key, ":", 2)
	if len(parts) < 2 {
		// Fallback for simple keys
		return filepath.Join(c.baseDir, sanitizeFilename(key)+".json")
	}

	category := parts[0]
	name := parts[1]

	// For SEO keys, hash the URL to avoid filesystem issues
	if category == "seo" {
		name = hashString(name)
	} else {
		name = sanitizeFilename(name)
	}

	return filepath.Join(c.baseDir, category, name+".json")
}

// GenerateSiteHash creates a unique hash for a site URL
func GenerateSiteHash(siteURL string) string {
	return hashString(siteURL)[:12]
}

// GenerateAPIKey creates a cache key for API responses
func GenerateAPIKey(endpoint string, page int) string {
	if page > 0 {
		return fmt.Sprintf("api:%s_page_%d", endpoint, page)
	}
	return fmt.Sprintf("api:%s", endpoint)
}

// GenerateSEOKey creates a cache key for SEO crawl results
func GenerateSEOKey(pageURL string) string {
	return fmt.Sprintf("seo:%s", pageURL)
}

// hashString returns SHA256 hash of a string (first 16 chars)
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:16]
}

// sanitizeFilename makes a string safe for use as a filename
func sanitizeFilename(s string) string {
	// Replace unsafe characters
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " ", "&", "="}
	result := s
	for _, char := range unsafe {
		result = strings.ReplaceAll(result, char, "_")
	}

	// Remove multiple consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	// Trim underscores
	result = strings.Trim(result, "_")

	// Limit length
	if len(result) > 100 {
		result = result[:100]
	}

	if result == "" {
		result = "unnamed"
	}

	return result
}

// ParseTTL parses a TTL string like "24h", "1h30m", "0", "unlimited"
func ParseTTL(ttlStr string) (time.Duration, error) {
	ttlStr = strings.TrimSpace(strings.ToLower(ttlStr))

	// Handle special cases
	if ttlStr == "0" || ttlStr == "unlimited" || ttlStr == "" {
		return 0, nil // No expiry
	}

	// Parse duration
	d, err := time.ParseDuration(ttlStr)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL format '%s': %w", ttlStr, err)
	}

	return d, nil
}

// DefaultCacheDir returns the default cache directory
func DefaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".wpexporter/cache"
	}
	return filepath.Join(home, ".wpexporter", "cache")
}
