package seo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/internal/config"
)

func originConfig(originURL string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.URL = "https://example.com"
	cfg.Origin = originURL
	cfg.Timeout = 5
	cfg.Quiet = true
	return cfg
}

// The crawl reads the rendered page from the origin, by the site's name.
func TestCrawlerReadsThroughOrigin(t *testing.T) {
	var host string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host = r.Host
		_, _ = w.Write([]byte(`<html><head><title>From origin</title></head><body></body></html>`))
	}))
	defer srv.Close()

	got := NewCrawler(originConfig(srv.URL)).extractSEO("https://example.com/about/")

	assert.Equal(t, "From origin", got.Title)
	assert.Equal(t, "example.com", host)
}

// SEC-005: credentials attached for the site must not follow a redirect from
// the origin to a foreign host.
func TestCrawlerOriginRedirectDropsCredentials(t *testing.T) {
	var foreignAuth string
	var foreignHits int
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignHits++
		foreignAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`<html><head><title>Elsewhere</title></head></html>`))
	}))
	defer foreign.Close()

	var originAuth string
	originSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originAuth = r.Header.Get("Authorization")
		http.Redirect(w, r, foreign.URL+"/landing", http.StatusFound)
	}))
	defer originSrv.Close()

	cfg := originConfig(originSrv.URL)
	cfg.AuthUser = "editor"
	cfg.AuthPass = "secret"

	_ = NewCrawler(cfg).extractSEO("https://example.com/private/")

	assert.NotEmpty(t, originAuth, "the site itself does get the credentials")
	assert.Equal(t, 1, foreignHits)
	assert.Empty(t, foreignAuth, "credentials leaked to a foreign host after a redirect")
}
