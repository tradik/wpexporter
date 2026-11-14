// Package origin reaches a WordPress site at its own web server rather than at
// whatever its DNS name points to.
//
// A site behind Cloudflare (or any reverse proxy) answers its public address
// with the proxy, and the proxy is what blocks the REST API and the crawl. Run
// next to the site, wpexporter can open the connection to the web server itself
// while still asking for the site by name: the Host header and the TLS server
// name stay the site's, only the address dialed changes. That is --origin.
//
// Only requests for the site's own host (and its www./bare twin) are routed.
// A CDN, an embed or any other foreign host is fetched exactly as before.
package origin

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Origin is the address a site's own requests are sent to. The zero value
// means "not set": requests go wherever DNS says.
type Origin struct {
	// Scheme is "http" or "https": what the origin itself speaks, which may
	// differ from the site's public scheme (TLS often ends at the proxy).
	Scheme string
	// Host is host:port, always with an explicit port.
	Host string
}

// Enabled reports whether an origin was configured.
func (o Origin) Enabled() bool {
	return o.Host != ""
}

// String returns the origin as scheme://host:port, or "" when not set.
func (o Origin) String() string {
	if !o.Enabled() {
		return ""
	}
	return o.Scheme + "://" + o.Host
}

// Parse reads an origin given as scheme://host[:port]. An empty value is the
// zero Origin and not an error. Anything that is not a bare server address —
// a path, a query, credentials — is refused rather than silently dropped, since
// the operator evidently meant something this flag does not do.
func Parse(raw string) (Origin, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Origin{}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Origin{}, fmt.Errorf("--origin %q: %w", raw, err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return Origin{}, fmt.Errorf("--origin %q: scheme must be http or https", raw)
	}
	if u.User != nil {
		return Origin{}, fmt.Errorf("--origin %q: credentials do not belong in the origin address", raw)
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return Origin{}, fmt.Errorf("--origin %q: give a server address only, without a path or query", raw)
	}

	host := u.Hostname()
	if host == "" {
		return Origin{}, fmt.Errorf("--origin %q: host is missing", raw)
	}

	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[scheme]
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return Origin{}, fmt.Errorf("--origin %q: port must be between 1 and 65535", raw)
	}

	return Origin{Scheme: scheme, Host: net.JoinHostPort(host, port)}, nil
}
