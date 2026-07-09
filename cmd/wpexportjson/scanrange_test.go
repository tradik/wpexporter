package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/bruteforce"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// TestAppendScanRange exercises the --scan-range wiring of Scanner.ScanSpecificRange
// (GO-002): it rescans a specific ID range and merges only items not already fetched.
func TestAppendScanRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// Only post id 2 exists in the scanned range.
		case strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/posts/2"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":2,"title":{"rendered":"Post 2"}}`))
		// Only media id 3 exists.
		case strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/media/3"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":3,"title":{"rendered":"Media 3"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.URL = server.URL
	cfg.Timeout = 5
	cfg.Concurrent = 2
	cfg.ScanRange = "1-3"

	client, err := api.NewClient(cfg)
	require.NoError(t, err)
	sc := bruteforce.NewScanner(cfg, client)

	posts := []models.WordPressPost{{ID: 1}} // id 1 already fetched -> must be deduped
	var pages []models.WordPressPost
	var media []models.WordPressMedia

	n, err := appendScanRange(sc, cfg, &posts, &pages, &media)
	require.NoError(t, err)

	// New items: post 2 and media 3. Post 1 was already present (deduped).
	assert.Equal(t, 2, n)
	assert.Len(t, posts, 2)
	assert.Len(t, media, 1)
	assert.Empty(t, pages)
}

// TestAppendScanRangeDisabled verifies the no-op path when --scan-range is unset.
func TestAppendScanRangeDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.URL = "https://example.com"
	cfg.ScanRange = ""
	client, err := api.NewClient(cfg)
	require.NoError(t, err)
	sc := bruteforce.NewScanner(cfg, client)

	posts := []models.WordPressPost{{ID: 1}}
	var pages []models.WordPressPost
	var media []models.WordPressMedia

	n, err := appendScanRange(sc, cfg, &posts, &pages, &media)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Len(t, posts, 1)
}
