package exportcli

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/localdb"
	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
	"github.com/tradik/wpexporter/internal/origin"
	"github.com/tradik/wpexporter/internal/wpconfig"
	"github.com/tradik/wpexporter/pkg/models"
)

// noOrigin makes origin detection find nothing for the test.
func noOrigin(t *testing.T) {
	detectOrigin = func(*config.Config) (origin.Origin, bool) { return origin.Origin{}, false }
	t.Cleanup(func() { detectOrigin = origin.Detect })
}

// A failing statement anywhere in a local run ends it, or degrades it the way
// the REST export degrades, and never leaks the password.
func TestLocalExportFailures(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		args           []any
		flags          map[string]string
		wantErr        string
		wantLog        string
	}{
		{name: "site options", fragment: "WHERE option_name IN", wantErr: "address could not be read"},
		{name: "uploads, with --url", fragment: "WHERE option_name IN", flags: map[string]string{"url": "https://example.com"},
			wantErr: "reading site options"},
		{name: "pages", fragment: "WHERE post_type = ?", args: []any{"page", 0, 500}, wantErr: "failed to get site info"},
		{name: "posts", fragment: "WHERE post_type = ?", args: []any{"post", 0, 500}, wantErr: "failed to get posts"},
		{name: "post types", fragment: "SELECT post_type, COUNT(*)", wantErr: "listing post types"},
		{name: "categories", fragment: "WHERE tt.taxonomy = ?", args: []any{"category"}, wantErr: "failed to get categories"},
		{name: "tags", fragment: "WHERE tt.taxonomy = ?", args: []any{"post_tag"}, wantErr: "failed to get tags"},
		{name: "media", fragment: "FROM wp_posts a LEFT JOIN", wantErr: "failed to get media"},
		{name: "users", fragment: "SELECT DISTINCT post_author", wantLog: "Warning: could not read users"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var opened wpconfig.Settings
			mockDatabaseWith(t, &opened, func(mock sqlmock.Sqlmock) {
				args := make([]driver.Value, len(tc.args))
				for i, a := range tc.args {
					args[i] = a
				}
				localdbtest.Fail(mock, tc.fragment, args...)
			})
			noOrigin(t)
			// The database password is the one the failure quotes.
			t.Setenv("WORDPRESS_DB_PASSWORD", localdbtest.Password)
			flags := map[string]string{"local": localSite(t), "format": "json", "output": filepath.Join(t.TempDir(), "o")}
			for k, v := range tc.flags {
				flags[k] = v
			}
			log, err := runWithFlags(t, flags)
			if tc.wantErr != "" {
				require.Error(t, err, log)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.NotContains(t, err.Error(), localdbtest.Password)
				return
			}
			require.NoError(t, err, log)
			assert.Contains(t, log, tc.wantLog)
		})
	}
}

// Everything that can go wrong before the database is read.
func TestLocalExportRefusals(t *testing.T) {
	noOrigin(t)
	site := localSite(t)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))

	for name, flags := range map[string]map[string]string{
		"conflict":        {"local": site, "brute-force": "true"},
		"max-id":          {"local": site, "max-id": "5"},
		"no wp-config":    {"local": dir},
		"db flags alone":  {"url": "https://example.com", "db-host": "db"},
		"unwritable":      {"local": site, "output": filepath.Join(blocker, "x", "y")},
		"bad url for api": {"local": site, "url": "ftp://example.com", "origin": "http://127.0.0.1:1"},
	} {
		t.Run(name, func(t *testing.T) {
			var opened wpconfig.Settings
			mockDatabase(t, &opened)
			_, err := runWithFlags(t, flags)
			assert.Error(t, err)
		})
	}

	t.Run("database will not open", func(t *testing.T) {
		openDatabase = func(context.Context, *wpconfig.Settings, localdb.Options) (*localdb.Source, error) {
			return nil, errors.New("connection refused")
		}
		t.Cleanup(func() { openDatabase = localdb.Open })
		_, err := runWithFlags(t, map[string]string{"local": site})
		assert.ErrorContains(t, err, "connection refused")
	})

	t.Run("snap", func(t *testing.T) {
		t.Cleanup(func() { executable = os.Executable })
		t.Setenv("SNAP", "/snap/wpexporter/12")

		executable = func() (string, error) { return "/snap/wpexporter/12/bin/wpexporter", nil }
		assert.ErrorContains(t, localSnapError(), "strictly confined snap")

		// Started from inside another snap: the variables are inherited, the
		// binary is not confined.
		executable = func() (string, error) { return "/usr/local/bin/wpexporter", nil }
		assert.NoError(t, localSnapError())
		executable = func() (string, error) { return "", errors.New("unknown") }
		assert.NoError(t, localSnapError())

		t.Setenv("SNAP", "")
		assert.NoError(t, localSnapError())
	})

	t.Run("export cannot be written", func(t *testing.T) {
		var opened wpconfig.Settings
		mockDatabase(t, &opened)
		_, err := runWithFlags(t, map[string]string{"local": site, "output": blocker, "no-media": "true"})
		assert.Error(t, err)
	})
}

// Every collection can be skipped, and a detected origin is used.
func TestLocalExportSkipsAndDetection(t *testing.T) {
	var opened wpconfig.Settings
	mockDatabase(t, &opened)
	var seen []string
	srv := fakeOrigin(t, &seen)
	detectOrigin = func(*config.Config) (origin.Origin, bool) {
		o, _ := origin.Parse(srv.URL)
		return o, true
	}
	t.Cleanup(func() { detectOrigin = origin.Detect })

	log, err := runWithFlags(t, map[string]string{"local": localSite(t), "format": "json",
		"output": filepath.Join(t.TempDir(), "o"), "no-posts": "true", "no-tags": "true", "no-users": "true",
		"no-products": "true", "no-media": "true", "assisted-crawl": "false", "crawl-content": "false"})
	require.NoError(t, err, log)
	for _, want := range []string{"Found the site's web server", "Skipping posts", "Skipping tags", "Skipping users",
		"Skipping products", "Skipping media"} {
		assert.Contains(t, log, want)
	}
	assert.NotContains(t, log, "Crawling", "the operator switched the crawl off")
}

func TestLocationOf(t *testing.T) {
	srv := fakeWordPressWith(t, map[string]http.HandlerFunc{
		"/away": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://elsewhere.example/x", http.StatusMovedPermanently)
		},
	})
	cfg := config.DefaultConfig()
	cfg.URL = srv.URL
	r := &exportRun{cfg: cfg}
	client := origin.HTTPClient(cfg)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	_, ok := r.locationOf(client, srv.URL+"/away")
	assert.False(t, ok, "a redirect off the site is not an address")
	_, ok = r.locationOf(client, "http://\x7f")
	assert.False(t, ok, "an address that cannot be requested")
	_, ok = r.locationOf(client, "http://127.0.0.1:1/")
	assert.False(t, ok, "nothing answers")
}

func TestResolveLinksEdges(t *testing.T) {
	r := &exportRun{cfg: config.DefaultConfig()}
	r.resolveLinks() // nothing to ask about

	var asked []string
	srv := fakeWordPressWith(t, map[string]http.HandlerFunc{"/": func(w http.ResponseWriter, req *http.Request) {
		asked = append(asked, req.URL.RequestURI())
		w.WriteHeader(http.StatusOK)
	}})
	r.cfg.URL = srv.URL
	r.c.siteInfo = &models.SiteInfo{HomeURL: srv.URL + "/"}
	r.c.posts = []models.WordPressPost{{ID: 1, Type: "post"}}
	r.resolveLinks()
	assert.Equal(t, []string{"/?p=1"}, asked, "the home URL's trailing slash is not doubled")
}

func TestRelink(t *testing.T) {
	c := &collected{
		siteInfo: &models.SiteInfo{FrontPage: &models.SitePage{Link: "old-page"}},
		comments: []models.WordPressComment{{ID: 9, PostURL: "old-post"}, {ID: 10, PostURL: "kept"}},
		menus:    []models.WordPressMenu{{Items: []models.WordPressMenuItem{{URL: "old-page"}, {URL: "kept"}}}},
	}
	relink(c, map[string]string{"old-post": "new-post", "old-page": "new-page"})
	assert.Equal(t, "new-post", c.comments[0].PostURL)
	assert.Equal(t, "new-post#comment-9", c.comments[0].Link)
	assert.Equal(t, "kept", c.comments[1].PostURL)
	assert.Equal(t, "new-page", c.menus[0].Items[0].URL)
	assert.Equal(t, "kept", c.menus[0].Items[1].URL)
	assert.Equal(t, "new-page", c.siteInfo.FrontPage.Link)

	relink(c, nil)
}

func TestApplyAllSourceFlags(t *testing.T) {
	cmd := newSourceCmd()
	for name, value := range map[string]string{"local": "/var/www/html", "db-host": "h", "db-name": "n",
		"db-user": "u", "db-prefix": "p_", "blog-id": "3"} {
		require.NoError(t, cmd.Flags().Set(name, value))
	}
	cfg := config.DefaultConfig()
	applySourceFlagOverrides(cmd, cfg)
	assert.Equal(t, []any{"/var/www/html", "h", "n", "u", "p_", 3},
		[]any{cfg.LocalPath, cfg.DBHost, cfg.DBName, cfg.DBUser, cfg.DBPrefix, cfg.BlogID})
	assert.NoError(t, validateSource(cfg))

	cfg.LocalPath = ""
	assert.ErrorContains(t, validateSource(cfg), "apply to --local only")
}
