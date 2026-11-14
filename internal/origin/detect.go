package origin

import (
	"net/http"
	"time"

	"github.com/tradik/wpexporter/internal/config"
)

// defaultCandidates are where a web server on the same machine usually listens,
// in the order tried: TLS first, since that is what a public site normally
// serves, then plain HTTP behind a TLS-terminating proxy or tunnel.
var defaultCandidates = []string{"https://127.0.0.1:443", "http://127.0.0.1:80"}

// maxProbeTimeout bounds each detection attempt: a port that is filtered rather
// than closed would otherwise hold the run for the whole --timeout.
const maxProbeTimeout = 5 * time.Second

// Detect looks for the site's web server on this machine, for local mode run
// without --origin. Each candidate is asked for the site's home page by name;
// the first one answering below 500 wins. Nothing answering is not an error —
// the export then comes from the database alone, and the caller says so.
func Detect(cfg *config.Config) (Origin, bool) {
	return detect(cfg, defaultCandidates)
}

// detect is Detect over an explicit list of candidates.
func detect(cfg *config.Config, candidates []string) (Origin, bool) {
	if cfg.URL == "" {
		return Origin{}, false
	}

	for _, raw := range candidates {
		o, err := Parse(raw)
		if err != nil {
			continue
		}
		if probe(cfg, o) {
			return o, true
		}
	}

	return Origin{}, false
}

// probe asks one candidate for the site's home page, without following a
// redirect: a redirect is an answer, and following it could leave the machine.
func probe(cfg *config.Config, o Origin) bool {
	trial := *cfg
	trial.Origin = o.String()

	client := &http.Client{
		Timeout:   probeTimeout(cfg),
		Transport: NewTransport(&trial, nil),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest(http.MethodGet, cfg.URL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()

	return resp.StatusCode < http.StatusInternalServerError
}

// probeTimeout is the run's --timeout, capped at maxProbeTimeout.
func probeTimeout(cfg *config.Config) time.Duration {
	t := time.Duration(cfg.Timeout) * time.Second
	if t > maxProbeTimeout || t <= 0 {
		return maxProbeTimeout
	}
	return t
}
