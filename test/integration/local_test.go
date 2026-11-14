//go:build integration

package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/pkg/models"
)

// env is what make test-integration hands the test.
type env struct {
	work, origin, dbHost string
	ids                  map[string]string
}

func setup(t *testing.T) env {
	t.Helper()
	e := env{work: os.Getenv("IT_WORK"), ids: map[string]string{}}
	if e.work == "" {
		t.Skip("run through `make test-integration`")
	}
	e.origin = "http://127.0.0.1:" + os.Getenv("IT_WEB_PORT")
	e.dbHost = "127.0.0.1:" + os.Getenv("IT_DB_PORT")

	f, err := os.Open(filepath.Join(e.work, "out", "ids.env"))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	for s := bufio.NewScanner(f); s.Scan(); {
		if k, v, ok := strings.Cut(s.Text(), "="); ok {
			e.ids[k] = v
		}
	}
	return e
}

// redirect asks the origin for path as example.test, without following.
func redirect(t *testing.T, e env, path string) (int, string) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequest(http.MethodGet, e.origin+path, nil)
	require.NoError(t, err)
	req.Host = "example.test"
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location")
}

// What WordPress itself answers for the three query forms local mode asks
// (FEAT-001, decision 3), and for the cross forms it does not use.
func TestQueryAddressRedirects(t *testing.T) {
	e := setup(t)
	year := time.Now().Format("2006/01")
	for _, tc := range []struct{ path, want string }{
		{"/?p=" + e.ids["POST"], "http://example.test/" + year + "/hello-local/"},
		{"/?page_id=" + e.ids["TEAM"], "http://example.test/about/team/"},
		{"/?post_type=portfolio&p=" + e.ids["WORK"], "http://example.test/work/bridge/"},
	} {
		status, loc := redirect(t, e, tc.path)
		t.Logf("observed: GET %s -> %d %s", tc.path, status, loc)
		assert.Equal(t, http.StatusMovedPermanently, status, tc.path)
		assert.Equal(t, tc.want, loc, tc.path)
	}
	for _, path := range []string{"/?p=" + e.ids["TEAM"], "/?p=" + e.ids["WORK"], "/?page_id=" + e.ids["POST"],
		"/?p=" + e.ids["DRAFT"], "/?page_id=" + e.ids["PRIVATE"]} {
		status, loc := redirect(t, e, path)
		t.Logf("observed (informational): GET %s -> %d %s", path, status, loc)
	}
}

// binary builds wpexporter once.
func binary(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "wpexporter")
	_, file, _, _ := runtime.Caller(0)
	cmd := exec.Command("go", "build", "-o", out, "./cmd/wpexporter")
	cmd.Dir = filepath.Join(filepath.Dir(file), "..", "..")
	b, err := cmd.CombinedOutput()
	require.NoError(t, err, string(b))
	return out
}

func run(t *testing.T, bin string, extraEnv []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"export"}, args...)...) // #nosec G204 -- test binary, fixed args
	cmd.Env = append(os.Environ(), extraEnv...)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

// A real local export: wp-config.php in the Docker image's getenv_docker form,
// the database read only, pages crawled through the origin, media from disk.
func TestLocalExport(t *testing.T) {
	e := setup(t)
	bin := binary(t)
	site := filepath.Join(e.work, "site")

	// From the host, the image's DB_HOST ("db:3306" inside, its default
	// "mysql" outside) does not resolve: the run says what to do.
	log, err := run(t, bin, nil, "--local", site, "-o", filepath.Join(e.work, "out", "refused"))
	require.Error(t, err, log)
	assert.Contains(t, log, "--db-host")

	// As the operator would: the container's environment, and the database
	// address this machine can reach.
	out := filepath.Join(e.work, "out", "json")
	containerEnv := []string{"WORDPRESS_DB_USER=wordpress", "WORDPRESS_DB_NAME=wordpress",
		"WORDPRESS_TABLE_PREFIX=itwp_", "WPEXPORTER_DB_PASSWORD=it-db-password"}
	log, err = run(t, bin, containerEnv, "--local", site, "--db-host", e.dbHost, "--origin", e.origin,
		"-f", "json", "-o", out, "--timeout", "20")
	t.Log(log)
	require.NoError(t, err, log)
	assert.NotContains(t, log, "it-db-password")

	raw, err := os.ReadFile(filepath.Join(out, "export.json")) // #nosec G304 -- test output
	require.NoError(t, err)
	// Never in the export: credentials, e-mail addresses (ours and the one
	// on WordPress's own sample comment), the commenter's IP, unpublished
	// documents. A password-protected post's title is public on the site (the
	// theme links to it), so for it the test is that no document is exported.
	for _, secret := range []string{"it-db-password", "it-admin-password", "itadmin@example.test",
		"reader@example.test", "wapuu@wordpress.example", "203.0.113.9", "Secret draft", "Private page"} {
		assert.NotContains(t, string(raw), secret)
	}

	var data models.ExportData
	require.NoError(t, json.Unmarshal(raw, &data))
	year := time.Now().Format("2006/01")
	assert.Equal(t, "Integration Site", data.Site.Name)
	// wp core install adds "Hello world!" and "Sample Page" of its own.
	assert.ElementsMatch(t, []string{"hello-world", "hello-local"}, slugs(data.Posts), "no draft, no locked post")
	assert.Equal(t, "http://example.test/"+year+"/hello-local/", linkOf(data.Posts, "hello-local"))
	for _, p := range data.Posts {
		assert.NotEmpty(t, p.SEO.Title, "crawled through the origin: %s", p.Slug)
	}
	// The body is the rendered page: WordPress ran the shortcode and dropped
	// the block delimiters; nothing stayed on the stored copy.
	for _, p := range data.Posts {
		if p.Slug == "hello-local" {
			assert.Contains(t, p.Content.Rendered, "Stored body with")
			assert.NotContains(t, p.Content.Rendered, "[gallery")
			assert.NotContains(t, p.Content.Rendered, "wp:paragraph")
		}
	}
	for _, n := range data.Stats.Notices {
		assert.NotContains(t, n, "carry the body stored in the database", "every document was crawled")
	}
	assert.ElementsMatch(t, []string{"sample-page", "about", "team"}, slugs(data.Pages), "no private page")
	assert.Equal(t, "http://example.test/about/team/", linkOf(data.Pages, "team"))
	require.Len(t, data.CustomTypes, 1)
	assert.Equal(t, "http://example.test/work/bridge/", data.CustomTypes[0].Posts[0].Link,
		"the rewrite slug lives in PHP; the origin's redirect supplied it")
	var authors []string
	for _, c := range data.Comments {
		authors = append(authors, c.Author)
	}
	assert.ElementsMatch(t, []string{"Reader", "A WordPress Commenter"}, authors)
	require.Len(t, data.Menus, 1)
	assert.Len(t, data.Menus[0].Items, 2, "the item pointing at a private page is left out")
	require.Len(t, data.Media, 1)
	assert.GreaterOrEqual(t, data.Stats.MediaDownloaded, 1)

	pixel, err := filepath.Glob(filepath.Join(out, "media", "images", "*pixel*"))
	require.NoError(t, err)
	require.NotEmpty(t, pixel)
	fmt.Println("media written:", pixel)

	// A preview: the newest post alone, read with a LIMIT, the crawl cached.
	limited := filepath.Join(e.work, "out", "limited")
	cacheDir := filepath.Join(e.work, "out", "cache")
	log, err = run(t, bin, containerEnv, "--local", site, "--db-host", e.dbHost, "--origin", e.origin,
		"-f", "json", "-o", limited, "--limit-posts", "1", "--limit-pages", "1", "--cache", "--cache-dir", cacheDir, "--timeout", "20")
	t.Log(log)
	require.NoError(t, err, log)
	assert.Contains(t, log, "Posts: 1 (limited from 2)")
	assert.Contains(t, log, "Cache enabled")
	raw, err = os.ReadFile(filepath.Join(limited, "export.json")) // #nosec G304 -- test output
	require.NoError(t, err)
	var preview models.ExportData
	require.NoError(t, json.Unmarshal(raw, &preview))
	assert.Equal(t, []string{"hello-local"}, slugs(preview.Posts), "the newest post")
	assert.Contains(t, log, "Pages: 1 (limited from 3)")
	assert.Equal(t, []string{"team"}, slugs(preview.Pages), "the newest page")
	assert.Equal(t, "http://example.test/about/team/", linkOf(preview.Pages, "team"),
		"its parent, left out by the limit, still in its address")
	cached, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.NotEmpty(t, cached, "the crawl through the origin was cached")
}

func slugs(docs []models.WordPressPost) []string {
	var out []string
	for _, d := range docs {
		out = append(out, d.Slug)
	}
	return out
}

func linkOf(docs []models.WordPressPost, slug string) string {
	for _, d := range docs {
		if d.Slug == slug {
			return d.Link
		}
	}
	return ""
}
