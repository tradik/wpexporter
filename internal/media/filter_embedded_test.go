package media

import (
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// TestFilterRelevantMedia_GutenbergFigure covers issue #22: an image embedded via a
// Gutenberg figure block must be kept by --relevant-media-only, so it is downloaded
// and therefore indexed for URL rewriting — even when the embedded URL is a size
// variant the media registry does not list.
func TestFilterRelevantMedia_GutenbergFigure(t *testing.T) {
	posts := []models.WordPressPost{{
		ID: 1,
		Content: models.RenderedContent{Rendered: `
			<figure class="wp-block-image size-large">
				<img src="https://site.test/wp-content/uploads/2024/05/cake-1024x768.jpg"
				     srcset="https://site.test/wp-content/uploads/2024/05/cake-300x225.jpg 300w,
				             https://site.test/wp-content/uploads/2024/05/cake-1024x768.jpg 1024w"
				     alt="cake"/>
			</figure>
			<figure class="wp-block-image">
				<img data-src="https://site.test/wp-content/uploads/2024/05/lazy.jpg" alt="lazy"/>
			</figure>`},
	}}

	all := []models.WordPressMedia{
		// source_url is the -scaled rescale; content embeds a -1024x768 variant.
		{ID: 10, SourceURL: "https://site.test/wp-content/uploads/2024/05/cake-scaled.jpg"},
		// only referenced through data-src (lazy-loading theme)
		{ID: 11, SourceURL: "https://site.test/wp-content/uploads/2024/05/lazy.jpg"},
		// not referenced anywhere
		{ID: 12, SourceURL: "https://site.test/wp-content/uploads/2024/05/unused.jpg"},
	}

	got := NewFilter().FilterRelevantMedia(posts, nil, all)

	kept := make(map[int]bool, len(got))
	for _, m := range got {
		kept[m.ID] = true
	}

	if !kept[10] {
		t.Error("figure-embedded image (size variant vs -scaled source_url) should be kept")
	}
	if !kept[11] {
		t.Error("data-src (lazy-loaded) image should be kept")
	}
	if kept[12] {
		t.Error("unreferenced image should be filtered out")
	}
}

// TestFilterRelevantMedia_SrcsetOnly confirms an image referenced only through a
// srcset candidate is still recognized.
func TestFilterRelevantMedia_SrcsetOnly(t *testing.T) {
	posts := []models.WordPressPost{{
		ID:      1,
		Content: models.RenderedContent{Rendered: `<img srcset="/wp-content/uploads/2024/05/only-800x600.jpg 800w" alt="x">`},
	}}
	all := []models.WordPressMedia{
		{ID: 20, SourceURL: "https://site.test/wp-content/uploads/2024/05/only.jpg"},
	}

	got := NewFilter().FilterRelevantMedia(posts, nil, all)
	if len(got) != 1 || got[0].ID != 20 {
		t.Errorf("srcset-only reference should keep the attachment, got %+v", got)
	}
}

// TestCanonicalMediaKey checks size and -scaled suffixes collapse to one key.
func TestCanonicalMediaKey(t *testing.T) {
	f := NewFilter()
	want := "2024/05/photo.jpg"
	for _, in := range []string{
		"https://site.test/wp-content/uploads/2024/05/photo.jpg",
		"https://site.test/wp-content/uploads/2024/05/photo-scaled.jpg",
		"https://site.test/wp-content/uploads/2024/05/photo-1024x768.jpg",
		"https://cdn.test/wp-content/uploads/2024/05/photo-300x225.jpg?ver=2",
	} {
		if got := f.canonicalMediaKey(in); got != want {
			t.Errorf("canonicalMediaKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRewriteScaledOriginal covers issue #22 part 1: content embedding the original
// filename must resolve to the attachment whose source_url carries "-scaled".
func TestRewriteScaledOriginal(t *testing.T) {
	d := &Downloader{config: &config.Config{DownloadMedia: true, MediaPathStyle: PathStyleRoot}}
	rw := d.NewURLRewriter([]models.WordPressMedia{{
		ID:        5,
		SourceURL: "https://site.test/wp-content/uploads/2024/05/photo-scaled.jpg",
		MimeType:  "image/jpeg",
	}})

	out := rw.Rewrite(`<img src="https://site.test/wp-content/uploads/2024/05/photo.jpg">`)
	if out == `<img src="https://site.test/wp-content/uploads/2024/05/photo.jpg">` {
		t.Errorf("original-name URL should resolve to the -scaled attachment, got %s", out)
	}
}
