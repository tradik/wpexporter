package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileCache(t *testing.T) {
	tmpDir := t.TempDir()

	cache, err := NewFileCache(tmpDir, 24*time.Hour, "https://example.com")
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}
	defer cache.Close()

	// Check directories were created
	apiDir := filepath.Join(cache.GetCacheDir(), "api")
	seoDir := filepath.Join(cache.GetCacheDir(), "seo")

	if _, err := os.Stat(apiDir); os.IsNotExist(err) {
		t.Error("API cache directory was not created")
	}
	if _, err := os.Stat(seoDir); os.IsNotExist(err) {
		t.Error("SEO cache directory was not created")
	}
}

func TestGetSet(t *testing.T) {
	tmpDir := t.TempDir()

	cache, err := NewFileCache(tmpDir, 24*time.Hour, "https://example.com")
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}
	defer cache.Close()

	// Test cache miss
	_, found, err := cache.Get("api:test")
	if err != nil {
		t.Errorf("Get on cache miss returned error: %v", err)
	}
	if found {
		t.Error("Expected cache miss, got hit")
	}

	// Test Set - use compact JSON to avoid whitespace normalization issues
	testData := []byte(`{"key":"value"}`)
	err = cache.Set("api:test", testData)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test cache hit
	data, found, err := cache.Get("api:test")
	if err != nil {
		t.Errorf("Get returned error: %v", err)
	}
	if !found {
		t.Error("Expected cache hit, got miss")
	}
	if string(data) != string(testData) {
		t.Errorf("Data mismatch: got %s, want %s", string(data), string(testData))
	}
}

func TestTTLExpiration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache with very short TTL
	cache, err := NewFileCache(tmpDir, 100*time.Millisecond, "https://example.com")
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}
	defer cache.Close()

	// Set data
	testData := []byte(`{"key": "value"}`)
	err = cache.Set("api:test", testData)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should be found immediately
	_, found, _ := cache.Get("api:test")
	if !found {
		t.Error("Expected cache hit immediately after set")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	_, found, _ = cache.Get("api:test")
	if found {
		t.Error("Expected cache miss after TTL expiration")
	}
}

func TestUnlimitedTTL(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache with TTL=0 (unlimited)
	cache, err := NewFileCache(tmpDir, 0, "https://example.com")
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}
	defer cache.Close()

	// Set data
	testData := []byte(`{"key": "value"}`)
	err = cache.Set("api:test", testData)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should be found
	_, found, _ := cache.Get("api:test")
	if !found {
		t.Error("Expected cache hit with unlimited TTL")
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()

	cache, err := NewFileCache(tmpDir, 24*time.Hour, "https://example.com")
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}
	defer cache.Close()

	// Set multiple entries
	cache.Set("api:posts_page_1", []byte(`{"posts": []}`))
	cache.Set("api:pages_page_1", []byte(`{"pages": []}`))
	cache.Set("seo:https://example.com/page", []byte(`{"title": "Test"}`))

	// Verify entries exist
	_, found1, _ := cache.Get("api:posts_page_1")
	_, found2, _ := cache.Get("api:pages_page_1")
	_, found3, _ := cache.Get("seo:https://example.com/page")
	if !found1 || !found2 || !found3 {
		t.Error("Entries should exist before clear")
	}

	// Clear cache
	err = cache.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify entries are gone
	_, found1, _ = cache.Get("api:posts_page_1")
	_, found2, _ = cache.Get("api:pages_page_1")
	_, found3, _ = cache.Get("seo:https://example.com/page")
	if found1 || found2 || found3 {
		t.Error("Entries should not exist after clear")
	}

	// Verify directories still exist
	apiDir := filepath.Join(cache.GetCacheDir(), "api")
	seoDir := filepath.Join(cache.GetCacheDir(), "seo")
	if _, err := os.Stat(apiDir); os.IsNotExist(err) {
		t.Error("API directory should be recreated after clear")
	}
	if _, err := os.Stat(seoDir); os.IsNotExist(err) {
		t.Error("SEO directory should be recreated after clear")
	}
}

func TestSiteIsolation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create caches for different sites
	cache1, err := NewFileCache(tmpDir, 24*time.Hour, "https://site1.com")
	if err != nil {
		t.Fatalf("NewFileCache for site1 failed: %v", err)
	}
	defer cache1.Close()

	cache2, err := NewFileCache(tmpDir, 24*time.Hour, "https://site2.com")
	if err != nil {
		t.Fatalf("NewFileCache for site2 failed: %v", err)
	}
	defer cache2.Close()

	// Set data in site1 cache
	cache1.Set("api:test", []byte(`{"site": "site1"}`))

	// site2 should not have the data
	_, found, _ := cache2.Get("api:test")
	if found {
		t.Error("Site2 should not have site1's cached data")
	}

	// Set different data in site2
	cache2.Set("api:test", []byte(`{"site": "site2"}`))

	// Verify both have their own data
	data1, _, _ := cache1.Get("api:test")
	data2, _, _ := cache2.Get("api:test")

	if string(data1) == string(data2) {
		t.Error("Different sites should have isolated caches")
	}
}

func TestGenerateSiteHash(t *testing.T) {
	tests := []struct {
		url1     string
		url2     string
		sameSite bool
	}{
		{"https://example.com", "https://example.com", true},
		{"https://example.com", "https://different.com", false},
		{"https://example.com/path1", "https://example.com/path2", false},
	}

	for _, tc := range tests {
		hash1 := GenerateSiteHash(tc.url1)
		hash2 := GenerateSiteHash(tc.url2)

		if tc.sameSite && hash1 != hash2 {
			t.Errorf("Expected same hash for %s and %s", tc.url1, tc.url2)
		}
		if !tc.sameSite && hash1 == hash2 {
			t.Errorf("Expected different hash for %s and %s", tc.url1, tc.url2)
		}

		// Hash should be 12 characters
		if len(hash1) != 12 {
			t.Errorf("Hash should be 12 chars, got %d", len(hash1))
		}
	}
}

func TestGenerateAPIKey(t *testing.T) {
	tests := []struct {
		endpoint string
		page     int
		expected string
	}{
		{"posts", 1, "api:posts_page_1"},
		{"posts", 2, "api:posts_page_2"},
		{"media", 0, "api:media"},
		{"site_info", 0, "api:site_info"},
	}

	for _, tc := range tests {
		key := GenerateAPIKey(tc.endpoint, tc.page)
		if key != tc.expected {
			t.Errorf("GenerateAPIKey(%s, %d) = %s, want %s",
				tc.endpoint, tc.page, key, tc.expected)
		}
	}
}

func TestGenerateSEOKey(t *testing.T) {
	url := "https://example.com/my-page/"
	key := GenerateSEOKey(url)
	expected := "seo:" + url

	if key != expected {
		t.Errorf("GenerateSEOKey(%s) = %s, want %s", url, key, expected)
	}
}

func TestParseTTL(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"24h", 24 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"0", 0, false},
		{"unlimited", 0, false},
		{"", 0, false},
		{"invalid", 0, true},
		{"1h30m45s", 5445 * time.Second, false},
	}

	for _, tc := range tests {
		result, err := ParseTTL(tc.input)

		if tc.hasError {
			if err == nil {
				t.Errorf("ParseTTL(%s) should return error", tc.input)
			}
			continue
		}

		if err != nil {
			t.Errorf("ParseTTL(%s) returned unexpected error: %v", tc.input, err)
			continue
		}

		if result != tc.expected {
			t.Errorf("ParseTTL(%s) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"posts_page_1", "posts_page_1"},
		{"posts/page/1", "posts_page_1"},
		{"test:file", "test_file"},
		{"test  file", "test_file"},
		{"a<b>c", "a_b_c"},
		{"", "unnamed"},
	}

	for _, tc := range tests {
		result := sanitizeFilename(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeFilename(%s) = %s, want %s", tc.input, result, tc.expected)
		}
	}
}

func TestCacheEntryIsExpired(t *testing.T) {
	// No expiry
	entry1 := CacheEntry{ExpiresAt: nil}
	if entry1.IsExpired() {
		t.Error("Entry with nil ExpiresAt should not be expired")
	}

	// Future expiry
	future := time.Now().Add(1 * time.Hour)
	entry2 := CacheEntry{ExpiresAt: &future}
	if entry2.IsExpired() {
		t.Error("Entry with future ExpiresAt should not be expired")
	}

	// Past expiry
	past := time.Now().Add(-1 * time.Hour)
	entry3 := CacheEntry{ExpiresAt: &past}
	if !entry3.IsExpired() {
		t.Error("Entry with past ExpiresAt should be expired")
	}
}

func TestDefaultCacheDir(t *testing.T) {
	dir := DefaultCacheDir()

	// Should contain .wpexporter/cache
	if !filepath.IsAbs(dir) && dir != ".wpexporter/cache" {
		// Either absolute or fallback
		if dir != ".wpexporter/cache" {
			if _, err := filepath.Rel(os.Getenv("HOME"), dir); err != nil {
				t.Errorf("DefaultCacheDir returned unexpected path: %s", dir)
			}
		}
	}
}
