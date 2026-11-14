package origin

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// seen records what a test server was asked for.
type seen struct {
	mu      sync.Mutex
	host    string
	path    string
	proto   string
	auth    string
	tlsName string
	count   int
}

func (s *seen) handler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.host = r.Host
		s.path = r.URL.Path
		s.proto = r.Header.Get("X-Forwarded-Proto")
		s.auth = r.Header.Get("Authorization")
		if r.TLS != nil {
			s.tlsName = r.TLS.ServerName
		}
		s.count++
		s.mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// recorder is a base transport that answers every request itself, so a test
// can tell a request that bypassed the origin from one that reached it.
type recorder struct {
	mu   sync.Mutex
	urls []string
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.urls = append(r.urls, req.URL.String())
	r.mu.Unlock()
	return &http.Response{StatusCode: http.StatusTeapot, Body: http.NoBody, Request: req}, nil
}

func siteConfig(site, originURL string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.URL = site
	cfg.Origin = originURL
	cfg.Timeout = 5
	return cfg
}

func get(t *testing.T, rt http.RoundTripper, target string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// The done-when criterion of phase 1: a request for https://example.com/x lands
// on the origin with Host: example.com and the site's name in the TLS handshake.
func TestTransportSendsSiteRequestsToOrigin(t *testing.T) {
	rec := &seen{}
	srv := httptest.NewTLSServer(rec.handler(http.StatusOK, "hello"))
	defer srv.Close()

	cfg := siteConfig("https://example.com", "https://"+srv.Listener.Addr().String())
	// The test server's certificate names example.com; trusting its CA (and
	// nothing else) proves the name checked is the site's, not the origin's IP.
	rt := NewTransport(cfg, srv.Client().Transport)

	resp := get(t, rt, "https://example.com/x")
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "hello", string(body))
	assert.Equal(t, "example.com", rec.host)
	assert.Equal(t, "/x", rec.path)
	assert.Equal(t, "example.com", rec.tlsName)
	assert.Equal(t, "", rec.proto, "same scheme needs no forwarded-proto")
}

func TestTransportSendsWWWTwinToOrigin(t *testing.T) {
	rec := &seen{}
	srv := httptest.NewServer(rec.handler(http.StatusOK, ""))
	defer srv.Close()

	for _, site := range []string{"https://example.com", "https://www.example.com"} {
		cfg := siteConfig(site, srv.URL)
		rt := NewTransport(cfg, &recorder{})
		for _, target := range []string{"https://www.example.com/a", "https://EXAMPLE.com/b"} {
			before := rec.count
			resp := get(t, rt, target)
			assert.Equal(t, http.StatusOK, resp.StatusCode, "%s via %s", target, site)
			assert.Equal(t, before+1, rec.count)
		}
	}
}

func TestTransportDowngradeAddsForwardedProto(t *testing.T) {
	rec := &seen{}
	srv := httptest.NewServer(rec.handler(http.StatusOK, ""))
	defer srv.Close()

	cfg := siteConfig("https://example.com", srv.URL)
	rt := NewTransport(cfg, nil)

	req, err := http.NewRequest(http.MethodGet, "https://example.com/y", nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, "https", rec.proto)
	assert.Equal(t, "example.com", rec.host)
	assert.Equal(t, "https", req.URL.Scheme, "the caller's request is not modified")
}

func TestTransportLeavesForeignHostsAlone(t *testing.T) {
	rec := &seen{}
	srv := httptest.NewServer(rec.handler(http.StatusOK, ""))
	defer srv.Close()

	base := &recorder{}
	rt := NewTransport(siteConfig("https://example.com", srv.URL), base)

	resp := get(t, rt, "https://cdn.example.net/img.jpg")
	assert.Equal(t, http.StatusTeapot, resp.StatusCode)
	resp = get(t, rt, "https://notexample.com/")
	assert.Equal(t, http.StatusTeapot, resp.StatusCode)

	assert.Equal(t, 0, rec.count, "a foreign host must never reach the origin")
	assert.Equal(t, []string{"https://cdn.example.net/img.jpg", "https://notexample.com/"}, base.urls)
}

func TestTransportDisabledReturnsBase(t *testing.T) {
	base := &recorder{}
	assert.Same(t, base, NewTransport(siteConfig("https://example.com", ""), base))
	assert.Nil(t, NewTransport(siteConfig("https://example.com", ""), nil))
}

func TestTransportInvalidOriginFailsClosed(t *testing.T) {
	base := &recorder{}
	rt := NewTransport(siteConfig("https://example.com", "ftp://nope"), base)

	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	_, err := rt.RoundTrip(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--origin")
	assert.Empty(t, base.urls, "an origin that cannot be parsed must not fall back to DNS")
}

func TestOriginInsecureOnlyAffectsOrigin(t *testing.T) {
	rec := &seen{}
	srv := httptest.NewTLSServer(rec.handler(http.StatusOK, ""))
	defer srv.Close()
	foreign := httptest.NewTLSServer(rec.handler(http.StatusOK, ""))
	defer foreign.Close()

	cfg := siteConfig("https://example.com", "https://"+srv.Listener.Addr().String())

	// Without the flag an untrusted origin certificate is refused.
	strict := NewTransport(cfg, nil)
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	_, err := strict.RoundTrip(req)
	require.Error(t, err)

	// With it the origin is reached...
	cfg.OriginInsecure = true
	loose := NewTransport(cfg, nil)
	resp := get(t, loose, "https://example.com/")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// ...and a foreign host is still verified.
	req, _ = http.NewRequest(http.MethodGet, foreign.URL+"/", nil)
	_, err = loose.RoundTrip(req)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "certificate")
}

func TestHTTPClient(t *testing.T) {
	plain := HTTPClient(siteConfig("https://example.com", ""))
	assert.Nil(t, plain.Transport, "no --origin: the client is exactly what it was before")
	assert.Equal(t, 5*time.Second, plain.Timeout)

	routed := HTTPClient(siteConfig("https://example.com", "http://127.0.0.1:8080"))
	assert.NotNil(t, routed.Transport)
}

// SEC-005: a redirect from the origin to a foreign host must not carry the
// credentials that were attached for the site.
func TestRedirectFromOriginDropsCredentials(t *testing.T) {
	foreign := &seen{}
	foreignSrv := httptest.NewServer(foreign.handler(http.StatusOK, "elsewhere"))
	defer foreignSrv.Close()

	originSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreignSrv.URL+"/landing", http.StatusFound)
	}))
	defer originSrv.Close()

	client := HTTPClient(siteConfig("https://example.com", originSrv.URL))
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	req.SetBasicAuth("editor", "secret")
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, 1, foreign.count)
	assert.Equal(t, "", foreign.auth, "credentials leaked across a redirect to a foreign host")
}

func TestOriginTransportKeepsBaseTLSSettings(t *testing.T) {
	base := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13}}
	cfg := siteConfig("https://example.com", "https://127.0.0.1")
	cfg.OriginInsecure = true

	rt, ok := NewTransport(cfg, base).(*siteRouter)
	require.True(t, ok)
	toOrigin, ok := rt.toOrigin.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, uint16(tls.VersionTLS13), toOrigin.TLSClientConfig.MinVersion)
	assert.True(t, toOrigin.TLSClientConfig.InsecureSkipVerify)
	assert.False(t, base.TLSClientConfig.InsecureSkipVerify, "the base transport is never weakened")
	assert.Nil(t, toOrigin.Proxy, "the origin is reached directly, never through a proxy")
}

func TestMatchesSite(t *testing.T) {
	cfg := siteConfig("https://Example.com/blog", "")
	assert.True(t, matchesSite(cfg, "example.com"))
	assert.True(t, matchesSite(cfg, "www.example.com"))
	assert.False(t, matchesSite(cfg, "www.www.example.com"))
	assert.False(t, matchesSite(cfg, ""))
	assert.False(t, matchesSite(cfg, "evil.com"))

	cfg.URL = "://bad"
	assert.False(t, matchesSite(cfg, "example.com"))
}

func TestTLSConfigOrDefault(t *testing.T) {
	def := tlsConfigOrDefault(nil)
	require.NotNil(t, def)
	assert.Equal(t, uint16(tls.VersionTLS12), def.MinVersion)
	assert.False(t, def.InsecureSkipVerify)

	own := &tls.Config{MinVersion: tls.VersionTLS13}
	assert.Same(t, own, tlsConfigOrDefault(own))
}

func TestTransportFillsMissingHost(t *testing.T) {
	rec := &seen{}
	srv := httptest.NewServer(rec.handler(http.StatusOK, ""))
	defer srv.Close()

	rt := NewTransport(siteConfig("http://example.com", srv.URL), nil)
	req := &http.Request{Method: http.MethodGet, URL: &url.URL{Scheme: "http", Host: "example.com", Path: "/z"}, Header: http.Header{}}
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, "example.com", rec.host)
}
