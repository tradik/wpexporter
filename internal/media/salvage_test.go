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

// salvageSetup builds a downloader and rewriter pointed at a test server, with one
// ordinary attachment already in the library.
func salvageSetup(t *testing.T, handler http.HandlerFunc) (*Downloader, *URLRewriter, string) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	dir := t.TempDir()
	downloader := &Downloader{
		config: &config.Config{
			URL:            server.URL,
			DownloadMedia:  true,
			MediaPathStyle: PathStyleRoot,
			Timeout:        5,
		},
		httpClient: server.Client(),
		mediaDir:   dir,
	}

	rewriter := downloader.NewURLRewriter([]models.WordPressMedia{{
		ID:        1,
		SourceURL: server.URL + "/wp-content/uploads/2024/03/known.jpg",
		MimeType:  "image/jpeg",
	}})

	return downloader, rewriter, server.URL
}

// TestCollectorSkipsWhatIsNotOurs covers the selection rules of the salvage pass
// (#30): only same-host asset URLs the index cannot already resolve.
func TestCollectorSkipsWhatIsNotOurs(t *testing.T) {
	_, rewriter, base := salvageSetup(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	collector := rewriter.NewAssetCollector()
	collector.Scan(`
		<img src="` + base + `/wp-content/uploads/2024/03/known.jpg">
		<img src="` + base + `/wp-content/uploads/elementor/thumbs/post-52-abc.jpg">
		<img src="https://cdn.example.net/wp-content/uploads/foreign.jpg">
		<a href="` + base + `/about-us/">About</a>
		<img src="/wp-content/uploads/2024/03/relative.png">
	`)

	assets := collector.Assets()
	if len(assets) != 2 {
		t.Fatalf("collected %d assets, want 2: %+v", len(assets), assets)
	}

	var keys []string
	for _, a := range assets {
		keys = append(keys, a.Key)
	}
	joined := strings.Join(keys, " ")

	if !strings.Contains(joined, "elementor/thumbs/post-52-abc.jpg") {
		t.Errorf("the Elementor rendition should be salvaged: %v", keys)
	}
	if !strings.Contains(joined, "relative.png") {
		t.Errorf("a root-relative reference is the site's own: %v", keys)
	}
	if strings.Contains(joined, "known.jpg") {
		t.Error("an attachment already in the library must not be salvaged again")
	}
	if strings.Contains(joined, "foreign.jpg") {
		t.Error("a CDN image is somebody else's file and must be left alone")
	}
	if strings.Contains(joined, "about-us") {
		t.Error("a page address is not media")
	}
}

// TestSalvageDownloadsAndRewrites is the end-to-end promise of #30: after the pass,
// a reference the library never listed resolves to a local file.
func TestSalvageDownloadsAndRewrites(t *testing.T) {
	downloader, rewriter, base := salvageSetup(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".jpg") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("jpeg-bytes"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	orphan := base + "/wp-content/uploads/elementor/thumbs/post-52-copyright.jpg"
	content := `<img src="` + orphan + `">`

	collector := rewriter.NewAssetCollector()
	collector.Scan(content)

	if saved := downloader.SalvageAssets(rewriter, collector.Assets()); saved != 1 {
		t.Fatalf("SalvageAssets saved %d files, want 1", saved)
	}

	rewritten := rewriter.Rewrite(content)
	if strings.Contains(rewritten, base) {
		t.Errorf("salvaged reference still points at the source host: %s", rewritten)
	}
	if !strings.Contains(rewritten, "/media/images/") {
		t.Errorf("salvaged reference should resolve under media/images/: %s", rewritten)
	}

	// The file is really on disk under the rewritten name. Take the path straight
	// from the index rather than parsing it back out of the HTML.
	local, _, ok := rewriter.index.resolve("/wp-content/uploads/elementor/thumbs/post-52-copyright.jpg")
	if !ok {
		t.Fatal("salvaged asset was not registered in the index")
	}
	onDisk := filepath.Join(downloader.mediaDir, filepath.FromSlash(strings.TrimPrefix(local, "/media/")))
	if _, err := os.Stat(onDisk); err != nil {
		t.Errorf("salvaged file missing on disk at %s: %v", onDisk, err)
	}
}

// TestSalvageDistinguishesRepeatedBasenames covers the reason the name carries a
// hash: Elementor reuses basenames across its thumbs directories, so two distinct
// files would otherwise overwrite each other.
func TestSalvageDistinguishesRepeatedBasenames(t *testing.T) {
	downloader, rewriter, base := salvageSetup(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("bytes"))
	})

	first := base + "/wp-content/uploads/elementor/thumbs/a/photo.jpg"
	second := base + "/wp-content/uploads/elementor/thumbs/b/photo.jpg"

	collector := rewriter.NewAssetCollector()
	collector.Scan(`<img src="` + first + `"><img src="` + second + `">`)

	if saved := downloader.SalvageAssets(rewriter, collector.Assets()); saved != 2 {
		t.Fatalf("saved %d files, want 2", saved)
	}

	firstPath := rewriter.Rewrite(first)
	secondPath := rewriter.Rewrite(second)

	if firstPath == secondPath {
		t.Errorf("same-basename files collided on %q", firstPath)
	}
}

// TestSalvageIsStableAcrossRuns confirms the generated name is derived from the
// source path, so a re-export does not duplicate every salvaged file.
func TestSalvageIsStableAcrossRuns(t *testing.T) {
	key := "/wp-content/uploads/elementor/thumbs/post-52-copyright.jpg"

	first := salvageRelativePath(key)
	second := salvageRelativePath(strings.Clone(key))

	if first != second {
		t.Errorf("salvage path must be deterministic: %q vs %q", first, second)
	}
	if !strings.HasPrefix(first, "images/") {
		t.Errorf("salvageRelativePath(%q) = %q, want it under images/", key, first)
	}
	// A different source path must not produce the same name, or Elementor's
	// repeated basenames overwrite each other.
	if other := salvageRelativePath("/wp-content/uploads/elementor/thumbs/b/post-52-copyright.jpg"); other == first {
		t.Errorf("distinct sources collided on %q", first)
	}
}

// TestSalvageHonorsExcludedTypes ensures the pass cannot reintroduce a category the
// export was told to skip.
func TestSalvageHonorsExcludedTypes(t *testing.T) {
	downloader, rewriter, base := salvageSetup(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("bytes"))
	})
	downloader.config.ExcludeMediaTypes = []string{"videos"}

	collector := rewriter.NewAssetCollector()
	collector.Scan(`<video src="` + base + `/wp-content/uploads/2024/clip.mp4"></video>`)

	if saved := downloader.SalvageAssets(rewriter, collector.Assets()); saved != 0 {
		t.Errorf("saved %d excluded files, want 0", saved)
	}
}
