package origin

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
)

// siteRouter sends requests for the site's host to the origin and everything
// else to the ordinary transport.
type siteRouter struct {
	cfg      *config.Config
	origin   Origin
	direct   http.RoundTripper
	toOrigin http.RoundTripper
}

// failing answers every request with the error that made the origin unusable.
// An origin that cannot be parsed must not quietly fall back to DNS: that would
// send the run through the very proxy the operator asked to avoid.
type failing struct{ err error }

func (f failing) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

// NewTransport wraps base so that requests for the site's own host — and its
// www./bare twin — are sent to cfg.Origin. The request keeps the site's name in
// the Host header and the TLS server name; only the address dialed changes.
// When the origin speaks plain HTTP for an https site, X-Forwarded-Proto: https
// is added, without which WordPress redirects every request back to https.
//
// With no origin configured, base is returned unchanged (nil stays nil, which
// an http.Client reads as the default transport). A nil base otherwise means
// http.DefaultTransport.
func NewTransport(cfg *config.Config, base http.RoundTripper) http.RoundTripper {
	if strings.TrimSpace(cfg.Origin) == "" {
		return base
	}

	o, err := Parse(cfg.Origin)
	if err != nil {
		return failing{err: err}
	}

	if base == nil {
		base = http.DefaultTransport
	}

	return &siteRouter{cfg: cfg, origin: o, direct: base, toOrigin: originTransport(o, cfg.OriginInsecure, base)}
}

// HTTPClient returns the client the crawler and the media downloader use: the
// configured timeout, and the origin routing when --origin is set. Without it
// the client is exactly what those packages built before.
func HTTPClient(cfg *config.Config) *http.Client {
	client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
	if strings.TrimSpace(cfg.Origin) != "" {
		client.Transport = NewTransport(cfg, nil)
	}
	return client
}

// RoundTrip implements http.RoundTripper.
func (s *siteRouter) RoundTrip(req *http.Request) (*http.Response, error) {
	if !matchesSite(s.cfg, req.URL.Hostname()) {
		return s.direct.RoundTrip(req)
	}

	routed := req.Clone(req.Context())
	if req.URL.Scheme == "https" && s.origin.Scheme == "http" {
		routed.Header.Set("X-Forwarded-Proto", "https")
	}
	routed.URL.Scheme = s.origin.Scheme
	if routed.Host == "" {
		routed.Host = req.URL.Host
	}

	return s.toOrigin.RoundTrip(routed)
}

// originTransport is a copy of base whose every connection goes to the origin.
// The URL keeps the site's host, so the TLS server name and certificate check
// are the site's and connections are pooled per site host.
func originTransport(o Origin, insecure bool, base http.RoundTripper) *http.Transport {
	var t *http.Transport
	if bt, ok := base.(*http.Transport); ok {
		t = bt.Clone()
	} else {
		t = http.DefaultTransport.(*http.Transport).Clone()
	}

	// Straight to the origin: a proxy from the environment would be the
	// detour this flag exists to remove.
	t.Proxy = nil
	t.DialTLSContext = nil
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	t.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, o.Host)
	}

	t.TLSClientConfig = tlsConfigOrDefault(t.TLSClientConfig)
	if insecure {
		// Only this transport — the one that dials the origin — skips
		// verification. Foreign hosts go through the base transport, verified.
		t.TLSClientConfig.InsecureSkipVerify = true // #nosec G402 -- opt-in --origin-insecure, origin connections only
	}

	return t
}

// tlsConfigOrDefault returns c, or a fresh TLS 1.2+ configuration when c is nil
// (a base transport that never set one).
func tlsConfigOrDefault(c *tls.Config) *tls.Config {
	if c == nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return c
}

// matchesSite reports whether host is the configured site's host or its www./
// bare twin. config.IsSameHost decides sameness, so origin routing and the
// credential guard agree on what "the site" is.
func matchesSite(cfg *config.Config, host string) bool {
	if host == "" {
		return false
	}
	if cfg.IsSameHost("//" + host) {
		return true
	}

	site, err := url.Parse(cfg.URL)
	if err != nil {
		return false
	}
	return strings.EqualFold(twin(site.Hostname()), host)
}

// twin returns the www. form of a bare host and the bare form of a www. host.
func twin(host string) string {
	if strings.HasPrefix(strings.ToLower(host), "www.") {
		return host[len("www."):]
	}
	return "www." + host
}
