package origin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectPicksFirstAnswerBelow500(t *testing.T) {
	broken := &seen{}
	brokenSrv := httptest.NewServer(broken.handler(http.StatusBadGateway, ""))
	defer brokenSrv.Close()
	good := &seen{}
	goodSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://example.com/elsewhere")
		good.handler(http.StatusMovedPermanently, "")(w, r)
	}))
	defer goodSrv.Close()

	cfg := siteConfig("https://example.com", "")
	got, ok := detect(cfg, []string{"not a url ::", "http://127.0.0.1:1", brokenSrv.URL, goodSrv.URL})

	assert.True(t, ok)
	assert.Equal(t, goodSrv.URL, got.String())
	assert.Equal(t, "example.com", good.host, "the probe names the site, not the address")
	assert.Equal(t, "https", good.proto)
	assert.Equal(t, 1, broken.count)
	assert.Equal(t, 1, good.count, "the redirect is the answer; it is not followed")
}

func TestDetectNothingAnswers(t *testing.T) {
	cfg := siteConfig("https://example.com", "")
	_, ok := detect(cfg, []string{"http://127.0.0.1:1"})
	assert.False(t, ok)
}

func TestDetectWithoutSiteURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	cfg := siteConfig("", "")
	_, ok := detect(cfg, []string{srv.URL})
	assert.False(t, ok)

	cfg.URL = "http://%zz"
	_, ok = detect(cfg, []string{srv.URL})
	assert.False(t, ok)
}

func TestDetectUsesDefaultCandidates(t *testing.T) {
	// Nothing listens on the default candidates in a test environment that
	// matters; the call must simply return without error or panic.
	cfg := siteConfig("https://example.invalid", "")
	cfg.Timeout = 1
	_, _ = Detect(cfg)
	assert.Equal(t, []string{"https://127.0.0.1:443", "http://127.0.0.1:80"}, defaultCandidates)
}

func TestProbeTimeout(t *testing.T) {
	cfg := siteConfig("https://example.com", "")
	cfg.Timeout = 60
	assert.Equal(t, maxProbeTimeout, probeTimeout(cfg))
	cfg.Timeout = 2
	assert.Equal(t, 2*maxProbeTimeout/5, probeTimeout(cfg))
}
