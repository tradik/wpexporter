package exportcli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/checkpoint"
	"github.com/tradik/wpexporter/internal/config"
)

// scenario is one REST export of the fake site, with some routes changed.
type scenario struct {
	name      string
	overrides map[string]http.HandlerFunc
	flags     map[string]string
	wantErr   string
	wantLog   []string
}

// withList answers a collection route with a body built around the server URL.
func withList(body func(base string) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeList(w, r, body("http://"+r.Host))
	}
}

func TestRESTExportScenarios(t *testing.T) {
	scenarios := []scenario{
		{
			name: "shop, menus, missing featured image, custom type, limit",
			overrides: map[string]http.HandlerFunc{
				"/wp-json/wc/v3/products": withList(func(string) string {
					return `[{"id":40,"name":"Mug","slug":"mug","type":"simple","status":"publish"}]`
				}),
				"/wp-json/wp/v2/menus": answer(200, `[{"id":7,"name":"Main","slug":"main"}]`),
				"/wp-json/wp/v2/menu-items": withList(func(base string) string {
					return `[{"id":71,"title":{"rendered":"Home"},"url":"` + base + `/","menus":7}]`
				}),
				"/wp-json/wp/v2/media":   withList(func(string) string { return `[]` }),
				"/wp-json/wp/v2/media/5": answer(200, `{"id":5,"slug":"a","media_type":"image","mime_type":"image/png"}`),
				"/wp-json/wp/v2/types": answer(200, `{"post":{"slug":"post","rest_base":"posts"},`+
					`"service":{"slug":"service","name":"Services","rest_base":"service"}}`),
				"/wp-json/wp/v2/service": withList(func(base string) string {
					return `[{"id":50,"slug":"audit","link":"` + base + `/service/audit/","title":{"rendered":"Audit"}}]`
				}),
				"/wp-json/wp/v2/users": answer(500, `{}`),
			},
			flags: map[string]string{"limit": "10", "verbose": "true", "flat-html": "true", "preserve-classes": "x"},
			wantLog: []string{"Found 1 WooCommerce products", "Found 1 menus", "Fetched 1 additional featured images",
				"Services (service): 1", "Warning: could not fetch users", "Limiting media", "Filtered to"},
		},
		{
			name: "a shop that needs keys, products that fail, gaps and notices",
			overrides: map[string]http.HandlerFunc{
				"/wp-json/wc/v3/products": answer(401, `{}`),
				"/wp-json/wp/v2/product":  withList(func(string) string { return `[]` }),
				"/wp-json/wp/v2/menus":    answer(500, `{}`),
				"/wp-json/wp/v2/tags": func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Query().Get("page") == "2" {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					// Two pages stated, the second refused: a gap. (writeList
					// would state one page, which now ends the walk.)
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-WP-TotalPages", "2")
					w.Header().Set("X-WP-Total", "101")
					_, _ = w.Write([]byte(`[{"id":8,"name":"t","slug":"t"}]`))
				},
				"/sitemap.xml": func(w http.ResponseWriter, r *http.Request) {
					_, _ = fmt.Fprintf(w, `<urlset><url><loc>http://%s/elsewhere/</loc></url></urlset>`, r.Host)
				},
			},
			flags:   map[string]string{"basic-html": "true", "format": "markdown"},
			wantLog: []string{"Warning: could not fetch menus", "Incomplete:"},
		},
		{
			name:    "brute force and a range rescan",
			flags:   map[string]string{"brute-force": "true", "max-id": "3", "scan-range": "1-3", "no-media": "true"},
			wantLog: []string{"Performing brute force content discovery", "Rescanning ID range 1-3"},
		},
		{
			name:      "products that fail",
			overrides: map[string]http.HandlerFunc{"/wp-json/wc/v3/products": answer(500, `{}`)},
			flags:     map[string]string{"retries": "0"},
			wantLog:   []string{"Warning: WooCommerce products may be incomplete"},
		},
		{
			name:      "a site without a root document",
			overrides: map[string]http.HandlerFunc{"/wp-json/": answer(500, `{}`), "/wp-json": answer(500, `{}`)},
			wantLog:   []string{"Warning: "},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			srv := fakeWordPressWith(t, sc.overrides)
			flags := map[string]string{"url": srv.URL, "format": "json", "retries": "0", "timeout": "5",
				"output": filepath.Join(t.TempDir(), "out")}
			for k, v := range sc.flags {
				flags[k] = v
			}
			log, err := runWithFlags(t, flags)
			if sc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), sc.wantErr)
				return
			}
			require.NoError(t, err, log)
			for _, want := range sc.wantLog {
				assert.Contains(t, log, want)
			}
		})
	}
}

// Resuming walks the checkpointed routes and deletes the checkpoint at the end.
func TestRESTExportResume(t *testing.T) {
	srv := fakeWordPress(t)
	out := filepath.Join(t.TempDir(), "out")
	log, err := runWithFlags(t, map[string]string{"url": srv.URL, "format": "json", "output": out,
		"retries": "0", "resume": "true"})
	require.NoError(t, err, log)
	assert.NoFileExists(t, filepath.Join(out, ".wpexport_checkpoint.json"))

	// A checkpoint that cannot be read ends the run before it starts.
	require.NoError(t, os.MkdirAll(out, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(out, ".wpexport_checkpoint.json"), []byte("{"), 0o600))
	_, err = runWithFlags(t, map[string]string{"url": srv.URL, "output": out, "resume": "true"})
	assert.ErrorContains(t, err, "checkpoint")
}

// The run's own configuration failures, each refused before any request.
func TestRunConfigurationFailures(t *testing.T) {
	dir := t.TempDir()
	badConfig := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(badConfig, []byte("url: [unclosed"), 0o600))
	fileAsDir := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(fileAsDir, nil, 0o600))

	for name, flags := range map[string]map[string]string{
		"config file":         {"config": badConfig},
		"limit spec":          {"url": "http://127.0.0.1:1", "limit-per-type": "posts=x"},
		"format":              {"url": "http://127.0.0.1:1", "format": "nope"},
		"api client":          {"url": "ftp://example.com"},
		"cache ttl":           {"url": "http://127.0.0.1:1", "cache": "true", "cache-ttl": "soon", "cache-dir": dir},
		"unwritable output":   {"url": "http://127.0.0.1:1", "output": filepath.Join(fileAsDir, "x", "y")},
		"export cannot write": {"url": "", "output": fileAsDir},
	} {
		if name == "export cannot write" {
			srv := fakeWordPress(t)
			flags["url"] = srv.URL
		}
		_, err := runWithFlags(t, flags)
		assert.Error(t, err, name)
	}
}

// The shared stages under the flags that switch them on: path filter, empty
// content, the combined crawl, conversion rules from a config file, the cache,
// a resumed run.
func TestRESTExportSharedStages(t *testing.T) {
	srv := fakeWordPress(t)
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("url: "+srv.URL+"\nflat_html_rules:\n"+
		"  - class: \"lead\"\n    markdown: \"**{content}**\"\n"), 0o600))

	log, err := runWithFlags(t, map[string]string{"config": configFile, "format": "json", "retries": "0",
		"output": filepath.Join(dir, "a"), "path-filter": "/", "skip-empty-content": "true",
		"assisted-crawl": "true", "crawl-content": "true", "flat-html": "true", "cache": "true",
		"cache-dir": filepath.Join(dir, "cache")})
	require.NoError(t, err, log)
	for _, want := range []string{"Path filter '/'", "Skipped empty content", "(combined)",
		"SEO metadata and content extraction complete", "Cache enabled"} {
		assert.Contains(t, log, want)
	}

	// A run resumed from a checkpoint says so.
	out := filepath.Join(dir, "b")
	require.NoError(t, os.MkdirAll(out, 0o750))
	mgr := checkpoint.NewManager(out, true)
	_, err = mgr.Load(srv.URL)
	require.NoError(t, err)
	require.NoError(t, mgr.Save())
	log, err = runWithFlags(t, map[string]string{"url": srv.URL, "format": "json", "retries": "0",
		"output": out, "resume": "true", "basic-html": "true", "preserve-classes": "keep"})
	require.NoError(t, err, log)
	assert.Contains(t, log, "Resuming from checkpoint")

	// A snap may not write to /tmp.
	resetExportFlags(t)
	t.Setenv("SNAP", "/snap/wpexporter/1")
	cfg := config.DefaultConfig()
	cfg.URL, cfg.Output = srv.URL, "/tmp/wpexporter-snap-test"
	assert.ErrorContains(t, checkOutput(cfg), "snap")
}
