package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// seoFrontMatterFields are the named SEO keys emitted in order, so exports are
// reproducible and diffable between runs.
var seoFrontMatterFields = []struct {
	key   string
	value func(models.SEOData) string
}{
	{"seo_title", func(s models.SEOData) string { return s.Title }},
	{"meta_description", func(s models.SEOData) string { return s.MetaDescription }},
	{"meta_keywords", func(s models.SEOData) string { return s.MetaKeywords }},
	{"robots", func(s models.SEOData) string { return s.Robots }},
	{"og_title", func(s models.SEOData) string { return s.OGTitle }},
	{"og_description", func(s models.SEOData) string { return s.OGDescription }},
	{"og_image", func(s models.SEOData) string { return s.OGImage }},
	{"og_type", func(s models.SEOData) string { return s.OGType }},
	{"og_url", func(s models.SEOData) string { return s.OGURL }},
	{"og_site_name", func(s models.SEOData) string { return s.OGSiteName }},
	{"og_locale", func(s models.SEOData) string { return s.OGLocale }},
	{"twitter_card", func(s models.SEOData) string { return s.TwitterCard }},
	{"twitter_title", func(s models.SEOData) string { return s.TwitterTitle }},
	{"twitter_description", func(s models.SEOData) string { return s.TwitterDesc }},
	{"twitter_image", func(s models.SEOData) string { return s.TwitterImage }},
	{"twitter_site", func(s models.SEOData) string { return s.TwitterSite }},
	{"article_published_time", func(s models.SEOData) string { return s.ArticlePublished }},
	{"article_modified_time", func(s models.SEOData) string { return s.ArticleModified }},
	{"article_author", func(s models.SEOData) string { return s.ArticleAuthor }},
	{"article_section", func(s models.SEOData) string { return s.ArticleSection }},
	{"canonical_url", func(s models.SEOData) string { return s.CanonicalURL }},
}

// writeSEOFrontMatter emits the named SEO fields, the hreflang alternates, the
// catch-all meta map and any structured data.
//
// The meta map matters: plugins and themes put real information in tags nobody
// anticipated, and a generator can ignore a key it does not recognize — but it
// cannot recover one the export dropped.
func (e *Exporter) writeSEOFrontMatter(builder *strings.Builder, seo models.SEOData) {
	for _, field := range seoFrontMatterFields {
		if value := field.value(seo); value != "" {
			fmt.Fprintf(builder, "%s: \"%s\"\n", field.key, e.escapeYAML(value))
		}
	}

	if seo.Lang != "" {
		fmt.Fprintf(builder, "lang: \"%s\"\n", e.escapeYAML(seo.Lang))
	}

	if len(seo.Hreflangs) > 0 {
		builder.WriteString("hreflangs:\n")
		for _, alternate := range seo.Hreflangs {
			fmt.Fprintf(builder, "  - lang: \"%s\"\n", e.escapeYAML(alternate.Lang))
			fmt.Fprintf(builder, "    href: \"%s\"\n", e.escapeYAML(alternate.Href))
		}
	}

	e.writeMetaMap(builder, seo.Meta)
	e.writeJSONLD(builder, seo.JSONLD)
}

// writeMetaMap emits the tags that have no dedicated key, sorted for stable output.
func (e *Exporter) writeMetaMap(builder *strings.Builder, meta map[string]string) {
	if len(meta) == 0 {
		return
	}

	keys := make([]string, 0, len(meta))
	for key := range meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	builder.WriteString("meta:\n")
	for _, key := range keys {
		fmt.Fprintf(builder, "  %q: \"%s\"\n", key, e.escapeYAML(meta[key]))
	}
}

// writeJSONLD emits the raw structured-data blocks as YAML block scalars, which
// keeps the JSON readable and needs no escaping of its quotes.
func (e *Exporter) writeJSONLD(builder *strings.Builder, blocks []string) {
	if len(blocks) == 0 {
		return
	}

	builder.WriteString("json_ld:\n")
	for _, block := range blocks {
		builder.WriteString("  - |\n")
		for _, line := range strings.Split(block, "\n") {
			fmt.Fprintf(builder, "    %s\n", strings.TrimRight(line, "\r"))
		}
	}
}
