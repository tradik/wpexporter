package export

// Structured frontmatter that survives a flat store (#49).
//
// The exported `meta` map and `hreflangs` list are the right shape for a
// generator reading the files, and the wrong shape for a store whose metadata
// model is key → list of strings. A pipeline through such a store loses them,
// and loses them silently: a loader that stringifies with Go's %v writes
// `map[…]`, which reads back as a string and breaks the template that expected
// the structure (tradik/mddb#187, spagu/ssg#154).

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// postWithStructuredSEO carries both non-flat values.
func postWithStructuredSEO() models.WordPressPost {
	post := models.WordPressPost{ID: 1, Slug: "focaccia", Link: "https://x.test/focaccia/"}
	post.Title.Rendered = "Focaccia"
	post.SEO = models.SEOData{
		Hreflangs: []models.HreflangLink{
			{Lang: "en", Href: "https://x.test/focaccia/"},
			{Lang: "pl", Href: "https://x.test/pl/focaccia/"},
		},
		Meta:   map[string]string{"twitter:label1": "Prep time", "recipe:yield": "8"},
		JSONLD: []string{`{"@type":"Recipe"}`},
	}

	return post
}

// flatExporter builds an exporter in the flat style. The nested one is the
// default, and comes from newMarkdownExporter.
func flatExporter(t *testing.T) *Exporter {
	t.Helper()

	return NewExporter(&config.Config{
		Output: t.TempDir(), Format: "markdown", Quiet: true, FrontmatterStyle: "flat",
	})
}

// TestNestedIsStillTheDefault: every existing consumer reads YAML structure, and
// must keep reading it.
func TestNestedIsStillTheDefault(t *testing.T) {
	exporter, _ := newMarkdownExporter(t)

	body := exporter.generateMarkdownContent(postWithStructuredSEO(), "post")

	assert.Contains(t, body, "hreflangs:\n  - lang: \"en\"")
	assert.Contains(t, body, "meta:\n  \"recipe:yield\": \"8\"")
	assert.False(t, config.DefaultConfig().FlatFrontmatter(), "nested unless asked otherwise")
}

// TestFlatStyleWritesDecodableJSON: the point of the option. The value has to
// come back as the structure it was, not as something a reader has to parse by
// eye.
func TestFlatStyleWritesDecodableJSON(t *testing.T) {
	exporter := flatExporter(t)
	data := &models.ExportData{Posts: []models.WordPressPost{postWithStructuredSEO()}}
	exporter.buildLookupMaps(data)

	body := exporter.generateMarkdownContent(data.Posts[0], "post")

	meta := frontmatterValue(t, body, "meta")
	var decodedMeta map[string]string
	require.NoError(t, json.Unmarshal([]byte(meta), &decodedMeta))
	assert.Equal(t, "8", decodedMeta["recipe:yield"])
	assert.Equal(t, "Prep time", decodedMeta["twitter:label1"])

	alternates := frontmatterValue(t, body, "hreflangs")
	var decodedAlternates []models.HreflangLink
	require.NoError(t, json.Unmarshal([]byte(alternates), &decodedAlternates))
	require.Len(t, decodedAlternates, 2)
	assert.Equal(t, "pl", decodedAlternates[1].Lang)

	assert.NotContains(t, body, "map[", "Go stringification is the failure this avoids")
}

// TestFlatStyleIsReproducible: two runs of the same export produce the same
// bytes, or every re-export is a diff nobody can read.
func TestFlatStyleIsReproducible(t *testing.T) {
	exporter := flatExporter(t)
	post := postWithStructuredSEO()

	first := exporter.generateMarkdownContent(post, "post")
	for i := 0; i < 5; i++ {
		assert.Equal(t, first, exporter.generateMarkdownContent(post, "post"))
	}
}

// TestFlatStyleQuotesSafely: a value carrying a quote must not end the YAML
// scalar early.
func TestFlatStyleQuotesSafely(t *testing.T) {
	exporter := flatExporter(t)

	post := models.WordPressPost{ID: 2, Slug: "quoted"}
	post.SEO = models.SEOData{Meta: map[string]string{
		"description": `He said "yes" and it's fine`,
	}}

	body := exporter.generateMarkdownContent(post, "post")

	value := frontmatterValue(t, body, "meta")
	var decoded map[string]string
	require.NoError(t, json.Unmarshal([]byte(value), &decoded))
	assert.Equal(t, `He said "yes" and it's fine`, decoded["description"])
}

// TestFlatStyleAppliesToTheSSGFormat: the format most likely to be loaded into
// such a store.
func TestFlatStyleAppliesToTheSSGFormat(t *testing.T) {
	exporter := flatExporter(t)
	exporter.config.Format = "ssg"

	data := &models.ExportData{Posts: []models.WordPressPost{postWithStructuredSEO()}}
	exporter.buildLookupMaps(data)

	document := exporter.generateSSGContent(data.Posts[0], "post")
	assert.Contains(t, document, `meta: '{`)

	// json_ld already traveled as text, and is left exactly as it was: a list
	// of strings is what a flat store holds natively.
	assert.Contains(t, document, "json_ld:\n  - |")
}

// TestFrontmatterStyleIsValidated: a typo must be refused before an export runs
// for forty minutes and writes the wrong shape.
func TestFrontmatterStyleIsValidated(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.URL = "https://x.test"
	cfg.Output = t.TempDir()

	cfg.FrontmatterStyle = "flat"
	require.NoError(t, cfg.Validate())

	cfg.FrontmatterStyle = "JSON"
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frontmatter_style must be one of: nested, flat")
}

// frontmatterValue reads one single-quoted scalar out of the front matter,
// undoing YAML's doubled-quote escaping.
func frontmatterValue(t *testing.T, document, key string) string {
	t.Helper()

	for _, line := range strings.Split(document, "\n") {
		prefix := key + ": '"
		if strings.HasPrefix(line, prefix) {
			return strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(line, prefix), "'"), "''", "'")
		}
	}

	t.Fatalf("no %q key in front matter:\n%s", key, document)

	return ""
}
