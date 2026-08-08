package media

import (
	"strings"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// newRewriteDownloader builds a downloader wired for content rewriting only.
func newRewriteDownloader(pathStyle string) *Downloader {
	return &Downloader{
		config: &config.Config{
			DownloadMedia:  true,
			MediaPathStyle: pathStyle,
		},
	}
}

// hawanasMedia mirrors the attachment shape reported in issue #11: a multisite
// upload whose REST source_url is https:// while post_content still carries the
// http:// form written years earlier.
func hawanasMedia() []models.WordPressMedia {
	return []models.WordPressMedia{
		{
			ID:        391,
			SourceURL: "https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg",
			MimeType:  "image/jpeg",
			MediaDetails: models.MediaDetails{
				Width: float64(400),
				Sizes: map[string]models.MediaSize{
					"medium": {
						SourceURL: "https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1-300x225.jpg",
						Width:     float64(300),
					},
					"thumbnail": {
						SourceURL: "https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1-150x150.jpg",
						Width:     float64(150),
					},
				},
			},
		},
	}
}

func TestUpdateMediaPathsRewritesSrcAndHref(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRoot)

	// src/href carry the historic http:// form, srcset the current https:// one.
	content := `<a href="http://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg">` +
		`<img src="http://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg" ` +
		`srcset="https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg 400w, ` +
		`https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1-300x225.jpg 300w"></a>`

	updated := downloader.UpdateMediaPaths(content, hawanasMedia())

	if strings.Contains(updated, "hawanas.com") {
		t.Errorf("UpdateMediaPaths() left an absolute source URL behind: %s", updated)
	}
	if !strings.Contains(updated, `href="/media/images/391_fran1.jpg"`) {
		t.Errorf("UpdateMediaPaths() did not localize href, got: %s", updated)
	}
	if !strings.Contains(updated, `src="/media/images/391_fran1.jpg"`) {
		t.Errorf("UpdateMediaPaths() did not localize src, got: %s", updated)
	}
	if !strings.Contains(updated, "/media/images/391_fran1-300x225.jpg 300w") {
		t.Errorf("UpdateMediaPaths() did not localize srcset candidate, got: %s", updated)
	}
}

func TestUpdateMediaPathsSchemeAndHostVariants(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRoot)

	tests := []struct {
		name string
		ref  string
	}{
		{"https", "https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg"},
		{"http", "http://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg"},
		{"www host", "https://www.hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg"},
		{"former domain", "http://old-domain.example/wp-content/uploads/sites/2/2010/07/fran1.jpg"},
		{"protocol relative", "//hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg"},
		{"root relative", "/wp-content/uploads/sites/2/2010/07/fran1.jpg"},
		{"query string", "https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg?ver=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := downloader.UpdateMediaPaths(`<img src="`+tt.ref+`">`, hawanasMedia())

			if updated != `<img src="/media/images/391_fran1.jpg">` {
				t.Errorf("UpdateMediaPaths(%s) = %s", tt.ref, updated)
			}
		})
	}
}

func TestUpdateMediaPathsRemapsStaleSizeVariant(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRoot)

	// -300x199 was retired when the registered size changed; only -300x225 exists.
	content := `<img src="https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1-300x199.jpg">`

	updated := downloader.UpdateMediaPaths(content, hawanasMedia())

	if updated != `<img src="/media/images/391_fran1-300x225.jpg">` {
		t.Errorf("UpdateMediaPaths() should remap a stale size to the nearest surviving width, got: %s", updated)
	}
}

func TestUpdateMediaPathsLeavesUnknownURLsAlone(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRoot)

	content := `<a href="https://example.org/page/">link</a> ` +
		`<img src="https://cdn.example.net/wp-content/uploads/2024/01/other.jpg">`

	if updated := downloader.UpdateMediaPaths(content, hawanasMedia()); updated != content {
		t.Errorf("UpdateMediaPaths() rewrote a URL that is not an exported attachment: %s", updated)
	}
}

func TestUpdateMediaPathsRelativeStyle(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRelative)

	content := `<img src="https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg">`

	if updated := downloader.UpdateMediaPaths(content, hawanasMedia()); updated != `<img src="media/images/391_fran1.jpg">` {
		t.Errorf("UpdateMediaPaths() with relative style = %s", updated)
	}
}

func TestUpdateMediaPathsNoIndexableMedia(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRoot)

	content := `<img src="https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg">`
	mediaItems := []models.WordPressMedia{{ID: 1, SourceURL: ""}, {ID: 2, SourceURL: "://invalid-url"}}

	if updated := downloader.UpdateMediaPaths(content, mediaItems); updated != content {
		t.Errorf("UpdateMediaPaths() with no indexable media should be a no-op, got: %s", updated)
	}
}

func TestUpdateMediaPathsSkipsEmptySizeURL(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRoot)

	mediaItems := []models.WordPressMedia{
		{
			ID:        7,
			SourceURL: "https://example.com/wp-content/uploads/2024/01/pic.jpg",
			MimeType:  "image/jpeg",
			MediaDetails: models.MediaDetails{
				Sizes: map[string]models.MediaSize{
					"broken": {SourceURL: "", Width: float64(150)},
					"bad":    {SourceURL: "://nope", Width: float64(150)},
				},
			},
		},
	}

	content := `<img src="https://example.com/wp-content/uploads/2024/01/pic.jpg">`

	if updated := downloader.UpdateMediaPaths(content, mediaItems); updated != `<img src="/media/images/7_pic.jpg">` {
		t.Errorf("UpdateMediaPaths() = %s", updated)
	}
}

func TestUpdateMediaPathsDownloadDisabled(t *testing.T) {
	downloader := &Downloader{config: &config.Config{DownloadMedia: false}}

	content := `<img src="https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg">`

	if updated := downloader.UpdateMediaPaths(content, hawanasMedia()); updated != content {
		t.Errorf("UpdateMediaPaths() should be a no-op when downloads are disabled")
	}
}

func TestUpdateMediaPathsVerboseRemapLogging(t *testing.T) {
	downloader := &Downloader{
		config: &config.Config{DownloadMedia: true, MediaPathStyle: PathStyleRoot, Verbose: true},
	}

	content := `<img src="https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1-300x199.jpg">`

	if updated := downloader.UpdateMediaPaths(content, hawanasMedia()); !strings.Contains(updated, "391_fran1-300x225.jpg") {
		t.Errorf("UpdateMediaPaths() verbose remap = %s", updated)
	}
}

func TestUpdateMediaPathsIgnoresPathlessTokens(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRoot)

	// Matches the URL token pattern but carries no path to key on.
	content := `<a href="https://hawanas.com">home</a>`

	if updated := downloader.UpdateMediaPaths(content, hawanasMedia()); updated != content {
		t.Errorf("UpdateMediaPaths() should leave a pathless URL untouched, got: %s", updated)
	}
}

func TestBuildURLIndexSkipsPathlessSourceURL(t *testing.T) {
	downloader := newRewriteDownloader(PathStyleRoot)

	mediaItems := []models.WordPressMedia{{ID: 1, SourceURL: "https://example.com", MimeType: "image/jpeg"}}

	if index := downloader.buildURLIndex(mediaItems); len(index.exact) != 0 {
		t.Errorf("buildURLIndex() should skip a source URL with no path, got %d entries", len(index.exact))
	}
}

func TestSplitSizeSuffixOverflowWidth(t *testing.T) {
	// Digits the regex accepts but strconv.Atoi cannot represent.
	if _, _, ok := splitSizeSuffix("/uploads/a-99999999999999999999x100.jpg"); ok {
		t.Error("splitSizeSuffix() should reject a width that overflows an int")
	}
}

func TestNormalizeURLKey(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{"absolute", "https://Example.com/WP-Content/Uploads/A.JPG", "/wp-content/uploads/a.jpg", true},
		{"protocol relative", "//example.com/a/b.jpg", "/a/b.jpg", true},
		{"query stripped", "/a/b.jpg?ver=1", "/a/b.jpg", true},
		{"fragment stripped", "/a/b.jpg#x", "/a/b.jpg", true},
		{"percent decoded", "/a/my%20file.jpg", "/a/my file.jpg", true},
		{"dot segments cleaned", "/a/./c/../b.jpg", "/a/b.jpg", true},
		{"host only", "https://example.com", "", false},
		{"root only", "https://example.com/", "", false},
		{"invalid", "://nope", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeURLKey(tt.raw)
			if ok != tt.ok || got != tt.want {
				t.Errorf("normalizeURLKey(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSplitSizeSuffix(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantBase  string
		wantWidth int
		wantOK    bool
	}{
		{"size variant", "/uploads/fran1-300x225.jpg", "/uploads/fran1.jpg", 300, true},
		{"no suffix", "/uploads/fran1.jpg", "", 0, false},
		{"suffix without extension", "/uploads/fran1-300x225", "/uploads/fran1", 300, true},
		{"not a size", "/uploads/fran1-abcxdef.jpg", "", 0, false},
		{"trailing dash digits only", "/uploads/fran1-300.jpg", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, width, ok := splitSizeSuffix(tt.key)
			if ok != tt.wantOK || base != tt.wantBase || width != tt.wantWidth {
				t.Errorf("splitSizeSuffix(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tt.key, base, width, ok, tt.wantBase, tt.wantWidth, tt.wantOK)
			}
		})
	}
}

func TestNearestVariant(t *testing.T) {
	variants := []sizeVariant{
		{width: 150, localPath: "/media/images/1_a-150x150.jpg"},
		{width: 300, localPath: "/media/images/1_a-300x225.jpg"},
		{width: 400, localPath: "/media/images/1_a.jpg"},
	}

	tests := []struct {
		name string
		want int
		path string
	}{
		{"exact width", 300, "/media/images/1_a-300x225.jpg"},
		{"closest below", 380, "/media/images/1_a.jpg"},
		{"closest above", 160, "/media/images/1_a-150x150.jpg"},
		{"tie prefers larger", 225, "/media/images/1_a-300x225.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nearestVariant(variants, tt.want)
			if !ok || got != tt.path {
				t.Errorf("nearestVariant(%d) = (%q, %v), want %q", tt.want, got, ok, tt.path)
			}
		})
	}

	if _, ok := nearestVariant(nil, 300); ok {
		t.Error("nearestVariant() on an empty set should report no match")
	}
}

func TestURLIndexResolveMiss(t *testing.T) {
	index := &urlIndex{exact: map[string]string{}, variants: map[string][]sizeVariant{}}

	if _, _, ok := index.resolve("/uploads/absent.jpg"); ok {
		t.Error("resolve() should miss on an unknown path with no size suffix")
	}
	if _, _, ok := index.resolve("/uploads/absent-300x200.jpg"); ok {
		t.Error("resolve() should miss when no variant of the base path is exported")
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  int
	}{
		{"int", 300, 300},
		{"int64", int64(300), 300},
		{"float64", float64(300), 300},
		{"float32", float32(300), 300},
		{"numeric string", "300", 300},
		{"non-numeric string", "wide", 0},
		{"nil", nil, 0},
		{"unsupported", []int{1}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInt(tt.value); got != tt.want {
				t.Errorf("toInt(%v) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestLocalMediaPathNilConfig(t *testing.T) {
	downloader := &Downloader{}

	if got := downloader.localMediaPath("images/1_a.jpg"); got != "/media/images/1_a.jpg" {
		t.Errorf("localMediaPath() with nil config = %q", got)
	}
}
