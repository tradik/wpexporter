package media

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// orphanRewriter builds a rewriter that knows one library attachment, so
// everything else in the content is an orphan.
func orphanRewriter(t *testing.T, siteURL string) (*Downloader, *URLRewriter) {
	t.Helper()
	cfg := &config.Config{
		URL: siteURL, Output: t.TempDir(), DownloadMedia: true,
		UserAgent: "test", Timeout: 5,
	}
	d := NewDownloader(cfg)
	r := d.NewURLRewriter([]models.WordPressMedia{{
		ID:        1,
		SourceURL: siteURL + "/wp-content/uploads/2024/03/known.jpg",
		MimeType:  "image/jpeg",
	}})
	return d, r
}

func TestUnresolvedURLs(t *testing.T) {
	_, r := orphanRewriter(t, "https://example.com")

	body := `<img src="https://example.com/wp-content/uploads/2024/03/known.jpg">
	         <img src="https://example.com/wp-content/uploads/elementor/thumbs/hero-abc123.webp">
	         <img src="/wp-content/uploads/2020/01/deleted-attachment.png">
	         <img src="https://cdn.other.net/asset.jpg">
	         <a href="https://example.com/contact/">contact</a>
	         <script src="https://example.com/wp-includes/js/jquery.js"></script>`

	got := r.UnresolvedURLs(body)

	want := []string{
		"https://example.com/wp-content/uploads/2020/01/deleted-attachment.png",
		"https://example.com/wp-content/uploads/elementor/thumbs/hero-abc123.webp",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q (sorted for a stable report)", i, got[i], want[i])
		}
	}
}

func TestUnresolvedURLs_Deduplicates(t *testing.T) {
	_, r := orphanRewriter(t, "https://example.com")
	body := strings.Repeat(`<img src="/wp-content/uploads/x/a.png">`, 3) +
		`<img src="https://example.com/wp-content/uploads/x/a.png">`

	if got := r.UnresolvedURLs(body, body); len(got) != 1 {
		t.Errorf("the same file in any form is one download, got %v", got)
	}
}

func TestDownloadOrphans(t *testing.T) {
	var served int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "gone.png") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		served++
		_, _ = w.Write([]byte("binary"))
	}))
	t.Cleanup(server.Close)

	d, r := orphanRewriter(t, server.URL)
	urls := []string{
		server.URL + "/wp-content/uploads/elementor/thumbs/hero-abc.webp",
		server.URL + "/wp-content/uploads/2020/01/gone.png",
	}

	mapping := d.DownloadOrphans(urls)

	if len(mapping) != 1 {
		t.Fatalf("only the file that downloaded should be recorded, got %v", mapping)
	}
	local := mapping[urls[0]]
	if !strings.HasPrefix(local, "/media/images/") || !strings.HasSuffix(local, "_hero-abc.webp") {
		t.Errorf("unexpected exported path %q", local)
	}
	if _, err := os.Stat(filepath.Join(d.mediaDir, "images", filepath.Base(local))); err != nil {
		t.Errorf("the file should be on disk: %v", err)
	}

	// Registering it makes the ordinary rewrite pass resolve it.
	r.AddOrphans(mapping)
	if got := r.Rewrite(`<img src="` + urls[0] + `">`); !strings.Contains(got, local) {
		t.Errorf("orphan not rewritten: %s", got)
	}
	// The one that 404'd stays absolute rather than pointing at a missing file.
	if got := r.Rewrite(`<img src="` + urls[1] + `">`); !strings.Contains(got, urls[1]) {
		t.Errorf("a failed download must not be rewritten: %s", got)
	}
}

func TestDownloadOrphans_Disabled(t *testing.T) {
	d := NewDownloader(&config.Config{URL: "https://example.com", Output: t.TempDir()})
	if got := d.DownloadOrphans([]string{"https://example.com/a.png"}); got != nil {
		t.Errorf("without --download-media nothing is fetched, got %v", got)
	}
	d = NewDownloader(&config.Config{URL: "https://example.com", Output: t.TempDir(), DownloadMedia: true})
	if got := d.DownloadOrphans(nil); got != nil {
		t.Errorf("no URLs means no work, got %v", got)
	}
}

func TestOrphanFilename_StableAndUnique(t *testing.T) {
	d := NewDownloader(&config.Config{URL: "https://example.com", Output: t.TempDir()})
	taken := map[string]bool{}

	a := d.orphanFilename("https://example.com/uploads/one/photo.jpg", taken)
	b := d.orphanFilename("https://example.com/uploads/two/photo.jpg", taken)

	if a == b {
		t.Errorf("same basename in different directories must not collide: %q", a)
	}
	if !strings.HasSuffix(a, "_photo.jpg") || !strings.HasSuffix(b, "_photo.jpg") {
		t.Errorf("the original name should stay readable: %q, %q", a, b)
	}
	// Stable between runs, so a re-export does not churn every filename.
	if again := d.orphanFilename("https://example.com/uploads/one/photo.jpg", map[string]bool{}); again != a {
		t.Errorf("filename is not stable: %q vs %q", again, a)
	}
	// A name already taken is numbered rather than overwritten.
	taken[a] = true
	if next := d.orphanFilename("https://example.com/uploads/one/photo.jpg", taken); next != "2_"+a {
		t.Errorf("expected a numbered variant, got %q", next)
	}
}

func TestOrphanSubfolder(t *testing.T) {
	cases := map[string]string{
		"/uploads/a.JPG":        "images",
		"/uploads/a.webp":       "images",
		"/uploads/clip.mp4":     "videos",
		"/uploads/tune.mp3":     "audio",
		"/uploads/terms.pdf":    "documents",
		"/uploads/style.css":    "",
		"/wp-admin/admin.php":   "",
		"/uploads/a%20b/c.webp": "images",
		"/uploads/noext":        "",
	}
	for in, want := range cases {
		if got := orphanSubfolder(in); got != want {
			t.Errorf("orphanSubfolder(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAbsoluteURL(t *testing.T) {
	_, r := orphanRewriter(t, "https://example.com")

	cases := map[string]string{
		"/wp-content/uploads/a.png":             "https://example.com/wp-content/uploads/a.png",
		"https://example.com/uploads/b.png":     "https://example.com/uploads/b.png",
		"//example.com/uploads/c.png":           "https://example.com/uploads/c.png",
		"https://cdn.other.net/d.png":           "",
		"https://example.com/page/":             "",
		"https://example.com/uploads/e.png#top": "https://example.com/uploads/e.png",
	}
	for in, want := range cases {
		if got := r.absoluteURL(in); got != want {
			t.Errorf("absoluteURL(%q) = %q, want %q", in, got, want)
		}
	}

	// Without a configured site there is no "same host" to compare against.
	bare := &URLRewriter{config: &config.Config{}, index: &urlIndex{exact: map[string]string{}}}
	if got := bare.absoluteURL("/uploads/a.png"); got != "" {
		t.Errorf("no site URL should resolve nothing, got %q", got)
	}
}

func TestAddOrphans_Empty(t *testing.T) {
	_, r := orphanRewriter(t, "https://example.com")
	before := len(r.index.exact)
	r.AddOrphans(nil)
	if len(r.index.exact) != before {
		t.Error("an empty mapping should change nothing")
	}
}
