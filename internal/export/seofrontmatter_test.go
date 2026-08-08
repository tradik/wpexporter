package export

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func seoWriter() *Exporter {
	return NewExporter(&config.Config{Output: "out", Format: "markdown"})
}

func TestWriteSEOFrontMatterNamedFields(t *testing.T) {
	var builder strings.Builder

	seoWriter().writeSEOFrontMatter(&builder, models.SEOData{
		Title:            "Swimming",
		MetaDescription:  "Learn to swim",
		Robots:           "index, follow",
		OGType:           "article",
		OGSiteName:       "Hawanas",
		TwitterCard:      "summary_large_image",
		ArticlePublished: "2010-07-21T12:00:00+00:00",
		CanonicalURL:     "https://hawanas.com/swim/",
		Lang:             "en-GB",
	})

	written := builder.String()

	assert.Contains(t, written, `seo_title: "Swimming"`)
	assert.Contains(t, written, `robots: "index, follow"`)
	assert.Contains(t, written, `og_type: "article"`)
	assert.Contains(t, written, `og_site_name: "Hawanas"`)
	assert.Contains(t, written, `twitter_card: "summary_large_image"`)
	assert.Contains(t, written, `article_published_time: "2010-07-21T12:00:00+00:00"`)
	assert.Contains(t, written, `canonical_url: "https://hawanas.com/swim/"`)
	assert.Contains(t, written, `lang: "en-GB"`)

	// Fields with no value emit no key.
	assert.NotContains(t, written, "og_title:")
	assert.NotContains(t, written, "twitter_image:")
}

func TestWriteSEOFrontMatterHreflangs(t *testing.T) {
	var builder strings.Builder

	seoWriter().writeSEOFrontMatter(&builder, models.SEOData{
		Hreflangs: []models.HreflangLink{
			{Lang: "en", Href: "https://x.test/en/"},
			{Lang: "pl", Href: "https://x.test/pl/"},
		},
	})

	written := builder.String()

	assert.Contains(t, written, "hreflangs:\n")
	assert.Contains(t, written, `  - lang: "en"`)
	assert.Contains(t, written, `    href: "https://x.test/en/"`)
	assert.Contains(t, written, `  - lang: "pl"`)
}

// TestWriteMetaMapIsSorted pins reproducible output: the same export run twice
// must produce byte-identical front matter.
func TestWriteMetaMapIsSorted(t *testing.T) {
	var builder strings.Builder

	seoWriter().writeMetaMap(&builder, map[string]string{
		"zeta":      "last",
		"alpha":     "first",
		"generator": "Rank Math",
	})

	written := builder.String()

	assert.Equal(t, "meta:\n"+
		"  \"alpha\": \"first\"\n"+
		"  \"generator\": \"Rank Math\"\n"+
		"  \"zeta\": \"last\"\n", written)
}

func TestWriteMetaMapEmpty(t *testing.T) {
	var builder strings.Builder

	seoWriter().writeMetaMap(&builder, nil)
	seoWriter().writeMetaMap(&builder, map[string]string{})

	assert.Empty(t, builder.String(), "an empty map emits no key at all")
}

func TestWriteMetaMapEscapesValues(t *testing.T) {
	var builder strings.Builder

	seoWriter().writeMetaMap(&builder, map[string]string{"quoted": `say "hi"`})

	assert.Contains(t, builder.String(), `\"hi\"`)
}

func TestWriteJSONLD(t *testing.T) {
	var builder strings.Builder

	seoWriter().writeJSONLD(&builder, []string{"{\n  \"@type\": \"Article\"\n}", `{"b":2}`})

	written := builder.String()

	assert.Contains(t, written, "json_ld:\n")
	assert.Contains(t, written, "  - |\n")
	assert.Contains(t, written, `    {"b":2}`)
	assert.Contains(t, written, `      "@type": "Article"`, "block scalar keeps JSON readable and unescaped")
}

func TestWriteJSONLDEmpty(t *testing.T) {
	var builder strings.Builder

	seoWriter().writeJSONLD(&builder, nil)

	assert.Empty(t, builder.String())
}

// TestWriteJSONLDStripsCarriageReturns pins that CRLF source does not leak stray
// \r into the YAML block scalar.
func TestWriteJSONLDStripsCarriageReturns(t *testing.T) {
	var builder strings.Builder

	seoWriter().writeJSONLD(&builder, []string{"{\r\n  \"a\": 1\r\n}"})

	assert.NotContains(t, builder.String(), "\r")
}

// TestSSGCarriesUnanticipatedMeta covers the end-to-end path: a tag with no
// dedicated field still reaches the ssg document.
func TestSSGCarriesUnanticipatedMeta(t *testing.T) {
	tmpDir := t.TempDir()
	data := ssgFixture()
	data.Posts[0].SEO.Meta = map[string]string{"generator": "Rank Math 1.0.219"}
	data.Posts[0].SEO.JSONLD = []string{`{"@type":"Article"}`}

	runSSGExport(t, ssgConfig(tmpDir), data)

	written := readFileString(t, tmpDir+"/posts/swimming/swimming-lesson.md")

	assert.Contains(t, written, `"generator": "Rank Math 1.0.219"`)
	assert.Contains(t, written, `{"@type":"Article"}`)
}
