package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// ssgFixture mirrors the site from #11: a multisite post with a category, a
// featured image, WordPress-flavored front matter and a nested page.
func ssgFixture() *models.ExportData {
	stamp := models.WordPressTime{Time: time.Date(2010, 7, 21, 12, 0, 0, 0, time.UTC)}

	return &models.ExportData{
		Categories: []models.WordPressCategory{{ID: 2, Name: "Swimming", Slug: "swimming"}},
		Users:      []models.WordPressUser{{ID: 4, Name: "hawanass"}},
		Media: []models.WordPressMedia{
			{
				ID:        391,
				SourceURL: "https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg",
				MimeType:  "image/jpeg",
				AltText:   "Swimmer mid-stroke",
			},
		},
		Posts: []models.WordPressPost{
			{
				ID:            389,
				Slug:          "swimming-lesson",
				Status:        "publish",
				Link:          "https://hawanas.com/2010/07/21/389/",
				Date:          stamp,
				Modified:      stamp,
				Author:        4,
				Categories:    []int{2},
				FeaturedMedia: 391,
				Title:         models.RenderedContent{Rendered: "Swimming &#8211; lesson"},
				Content: models.RenderedContent{
					Rendered: `<p>Text &#8211; more&hellip;</p>` +
						`<img src="https://hawanas.com/wp-content/uploads/sites/2/2010/07/fran1.jpg" ` +
						`class="wp-image-391 size-full" title="fran1" loading="lazy">`,
				},
				Excerpt: models.RenderedContent{
					Rendered: `<p>Summary&hellip; <a href="/389/" class="more-link">Continue reading</a></p>`,
				},
				SEO: models.SEOData{MetaDescription: "A swimming lesson"},
			},
		},
		Pages: []models.WordPressPost{
			{
				ID:     12,
				Slug:   "cost",
				Status: "publish",
				Link:   "https://hawanas.com/baby-water-instructor/cost/",
				Date:   stamp,
				Title:  models.RenderedContent{Rendered: "Cost"},
			},
		},
	}
}

func ssgConfig(output string) *config.Config {
	return &config.Config{
		URL:           "https://hawanas.com",
		Output:        output,
		Format:        "ssg",
		DownloadMedia: false,
	}
}

// runSSGExport runs the ssg writer through the same pre-format pass Export uses.
func runSSGExport(t *testing.T, cfg *config.Config, data *models.ExportData) {
	t.Helper()

	e := NewExporter(cfg)
	require.NoError(t, cfg.EnsureOutputDir())

	e.localizeAddresses(data)

	require.NoError(t, e.exportSSG(data))
}

func readFileString(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path) // #nosec G304 -- path comes from the test's own temp dir
	require.NoError(t, err, "expected %s to exist", path)

	return string(body)
}

// TestSSGLayout pins the directory contract from #11: posts at least one level
// below posts/, pages nested to mirror their URL.
func TestSSGLayout(t *testing.T) {
	tmpDir := t.TempDir()
	runSSGExport(t, ssgConfig(tmpDir), ssgFixture())

	assert.FileExists(t, filepath.Join(tmpDir, "posts", "swimming", "swimming-lesson.md"))
	assert.FileExists(t, filepath.Join(tmpDir, "pages", "baby-water-instructor", "cost.md"))
	assert.FileExists(t, filepath.Join(tmpDir, "metadata.json"))
}

// TestSSGFrontMatter pins the single-spelling front-matter contract.
func TestSSGFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()
	runSSGExport(t, ssgConfig(tmpDir), ssgFixture())

	written := readFileString(t, filepath.Join(tmpDir, "posts", "swimming", "swimming-lesson.md"))

	assert.Contains(t, written, `title: "Swimming – lesson"`, "entities decoded in title")
	assert.Contains(t, written, `slug: "swimming-lesson"`)
	assert.Contains(t, written, `status: "publish"`)
	assert.Contains(t, written, `type: "post"`)
	assert.Contains(t, written, `author: "hawanass"`)
	assert.Contains(t, written, `category: "Swimming"`)
	assert.Contains(t, written, `description: "A swimming lesson"`)
	assert.Contains(t, written, `excerpt: "Summary…"`, "read-more chrome stripped")

	// WordPress's competing spellings must not appear.
	for _, absent := range []string{"seo_title:", "meta_description:", "og_title:", "categories:", "category_ids:", "author_id:"} {
		assert.NotContains(t, written, absent)
	}
}

// TestSSGLinkIsRootRelativeByDefault pins that ssg defaults to --link-style root.
func TestSSGLinkIsRootRelativeByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	runSSGExport(t, ssgConfig(tmpDir), ssgFixture())

	written := readFileString(t, filepath.Join(tmpDir, "posts", "swimming", "swimming-lesson.md"))

	assert.Contains(t, written, `link: "/2010/07/21/389/"`)
	assert.NotContains(t, written, "hawanas.com/2010")
}

// TestSSGLinkStyleOverride pins that an explicit flag still wins.
func TestSSGLinkStyleOverride(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := ssgConfig(tmpDir)
	cfg.LinkStyle = "absolute"

	runSSGExport(t, cfg, ssgFixture())

	written := readFileString(t, filepath.Join(tmpDir, "posts", "swimming", "swimming-lesson.md"))
	assert.Contains(t, written, `link: "https://hawanas.com/2010/07/21/389/"`)
}

// TestSSGContentIsCleaned covers the body transforms: entities decoded, alt
// filled in, WordPress presentation attributes dropped.
func TestSSGContentIsCleaned(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := ssgConfig(tmpDir)
	cfg.DownloadMedia = true

	runSSGExport(t, cfg, ssgFixture())

	written := readFileString(t, filepath.Join(tmpDir, "posts", "swimming", "swimming-lesson.md"))

	assert.Contains(t, written, "Text – more…", "typographic entities decoded")
	assert.Contains(t, written, `alt="Swimmer mid-stroke"`, "alt filled from alt_text")
	assert.NotContains(t, written, "wp-image-391", "WordPress classes dropped")
	assert.NotContains(t, written, `title="fran1"`, "filename-repeating title dropped")
	assert.NotContains(t, written, "loading=", "browser hints dropped")
	assert.Contains(t, written, `featured_image: "/media/images/391_fran1.jpg"`)
}

// TestSSGUncategorizedPost pins that a post with no category still lands one
// directory below posts/, which the generator requires.
func TestSSGUncategorizedPost(t *testing.T) {
	tmpDir := t.TempDir()
	data := ssgFixture()
	data.Posts[0].Categories = nil
	data.Posts[0].Link = "https://hawanas.com/389/"
	data.Categories = nil

	runSSGExport(t, ssgConfig(tmpDir), data)

	assert.FileExists(t, filepath.Join(tmpDir, "posts", uncategorizedDir, "swimming-lesson.md"))
}

// TestSSGDescriptionFallsBackToExcerpt pins the description precedence.
func TestSSGDescriptionFallsBackToExcerpt(t *testing.T) {
	tmpDir := t.TempDir()
	data := ssgFixture()
	data.Posts[0].SEO.MetaDescription = ""
	data.Posts[0].SEO.OGDescription = "From Open Graph"

	runSSGExport(t, ssgConfig(tmpDir), data)

	written := readFileString(t, filepath.Join(tmpDir, "posts", "swimming", "swimming-lesson.md"))
	assert.Contains(t, written, `description: "From Open Graph"`)

	// With neither, the excerpt carries it.
	tmpDir2 := t.TempDir()
	data2 := ssgFixture()
	data2.Posts[0].SEO = models.SEOData{}

	runSSGExport(t, ssgConfig(tmpDir2), data2)

	written2 := readFileString(t, filepath.Join(tmpDir2, "posts", "swimming", "swimming-lesson.md"))
	assert.Contains(t, written2, `description: "Summary…"`)
}

// TestSSGTitlePrefersSEOTitle pins that the title the site actually rendered wins.
func TestSSGTitlePrefersSEOTitle(t *testing.T) {
	tmpDir := t.TempDir()
	data := ssgFixture()
	data.Posts[0].SEO.Title = "Swimming lessons in Hawanas"

	runSSGExport(t, ssgConfig(tmpDir), data)

	written := readFileString(t, filepath.Join(tmpDir, "posts", "swimming", "swimming-lesson.md"))
	assert.Contains(t, written, `title: "Swimming lessons in Hawanas"`)
}

func TestPageURLPath(t *testing.T) {
	tests := []struct {
		name string
		page models.WordPressPost
		want []string
	}{
		{
			name: "nested link",
			page: models.WordPressPost{Link: "https://x.test/a/b/c/", Slug: "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "root link falls back to slug",
			page: models.WordPressPost{Link: "https://x.test/", Slug: "home"},
			want: []string{"home"},
		},
		{
			name: "no link falls back to slug",
			page: models.WordPressPost{Slug: "about"},
			want: []string{"about"},
		},
		{
			name: "traversal segments dropped",
			page: models.WordPressPost{Link: "https://x.test/a/../../etc/passwd", Slug: "x"},
			want: []string{"a", "etc", "passwd"},
		},
		{
			name: "invalid link falls back to slug",
			page: models.WordPressPost{Link: "://nope", Slug: "fallback"},
			want: []string{"fallback"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pageURLPath(tt.page))
		})
	}
}

// TestSSGPageTraversalIsContained pins that a crafted link cannot write outside
// the export directory.
func TestSSGPageTraversalIsContained(t *testing.T) {
	tmpDir := t.TempDir()
	data := ssgFixture()
	data.Posts = nil
	data.Pages[0].Link = "https://hawanas.com/../../../etc/evil/"

	runSSGExport(t, ssgConfig(tmpDir), data)

	assert.FileExists(t, filepath.Join(tmpDir, "pages", "etc", "evil.md"))
	assert.NoFileExists(t, filepath.Join(filepath.Dir(tmpDir), "etc", "evil.md"))
}

func TestSanitizePathSegment(t *testing.T) {
	assert.Equal(t, "a-b", sanitizePathSegment("a/b"))
	assert.Equal(t, "a-b", sanitizePathSegment(`a\b`))
	assert.Equal(t, "c-", sanitizePathSegment("c:"))
}

func TestWriteYAMLStringSkipsEmpty(t *testing.T) {
	var builder strings.Builder

	writeYAMLString(&builder, "title", "")
	assert.Empty(t, builder.String(), "an empty value should emit no key at all")

	writeYAMLString(&builder, "title", "Set")
	assert.Equal(t, "title: \"Set\"\n", builder.String())
}
