package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// --origin reaches the REST API on the web server behind the proxy, asking for
// the site by name (FEAT-001).
func TestNewClientRoutesThroughOrigin(t *testing.T) {
	var mu sync.Mutex
	var hosts, paths, protos []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hosts = append(hosts, r.Host)
		paths = append(paths, r.URL.Path)
		protos = append(protos, r.Header.Get("X-Forwarded-Proto"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-WP-Total", "0")
		w.Header().Set("X-WP-TotalPages", "1")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.URL = "https://example.com"
	cfg.Origin = srv.URL
	cfg.Retries = 0

	client, err := NewClient(cfg)
	require.NoError(t, err)

	posts, err := client.GetPosts()
	require.NoError(t, err)
	assert.Empty(t, posts)

	require.NotEmpty(t, paths)
	assert.True(t, strings.HasPrefix(paths[0], "/wp-json/wp/v2/posts"), paths[0])
	assert.Equal(t, "example.com", hosts[0])
	assert.Equal(t, "https", protos[0])
}

// Without --origin the resty client keeps its own transport.
func TestNewClientWithoutOriginKeepsTransport(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.URL = "https://example.com"

	client, err := NewClient(cfg)
	require.NoError(t, err)

	_, isHTTPTransport := client.httpClient.GetClient().Transport.(*http.Transport)
	assert.True(t, isHTTPTransport)
}
