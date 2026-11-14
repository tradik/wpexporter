package media

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// Media under the site's own host is downloaded from the origin (FEAT-001).
func TestDownloaderFetchesThroughOrigin(t *testing.T) {
	var host string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host = r.Host
		_, _ = w.Write([]byte("jpeg-bytes"))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.URL = "https://example.com"
	cfg.Origin = srv.URL
	cfg.Output = t.TempDir()

	d := NewDownloader(cfg)
	target := filepath.Join(d.mediaDir, "a.jpg")
	require.NoError(t, os.MkdirAll(d.mediaDir, 0o750))

	ok := d.downloadFile("https://example.com/wp-content/uploads/a.jpg", target)
	require.True(t, ok)

	data, err := os.ReadFile(target) // #nosec G304 -- test file in a temp dir
	require.NoError(t, err)
	assert.Equal(t, "jpeg-bytes", string(data))
	assert.Equal(t, "example.com", host)
}
