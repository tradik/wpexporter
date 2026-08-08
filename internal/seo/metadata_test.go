package seo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// richPage carries the shapes a real WordPress head emits: Rank Math OG/Twitter
// tags, article metadata, a plugin tag nobody anticipated, JSON-LD, and both a
// GA4 and a GTM snippet.
const richPage = `<html><head>
<title>Swimming lessons</title>
<meta name="description" content="Learn to swim">
<meta name="robots" content="index, follow, max-snippet:-1">
<meta property="og:title" content="Swimming lessons &amp; more">
<meta property="og:type" content="article">
<meta property="og:url" content="https://hawanas.com/swim/">
<meta property="og:site_name" content="Hawanas">
<meta property="og:locale" content="en_GB">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="Swimming lessons">
<meta name="twitter:site" content="@hawanas">
<meta property="article:published_time" content="2010-07-21T12:00:00+00:00">
<meta property="article:author" content="hawanass">
<meta name="generator" content="Rank Math 1.0.219">
<meta name="msvalidate.01" content="ABC123">
<script type="application/ld+json">{"@type":"Article","headline":"Swimming"}</script>
<script async src="https://www.googletagmanager.com/gtag/js?id=G-ABC1234567"></script>
<script>gtag('config', 'UA-123456-1');</script>
<script>(function(w,d,s,l,i){})(window,document,'script','dataLayer','GTM-WXYZ123');</script>
</head><body>x</body></html>`

func newTestCrawler(extractMeta string) *Crawler {
	return NewCrawler(&config.Config{ExtractMeta: extractMeta, Quiet: true})
}

func TestPopulateSEONamedFields(t *testing.T) {
	var seo models.SEOData
	newTestCrawler("all").populateSEO(&seo, richPage)

	assert.Equal(t, "Swimming lessons", seo.Title)
	assert.Equal(t, "Learn to swim", seo.MetaDescription)
	assert.Equal(t, "index, follow, max-snippet:-1", seo.Robots)
	assert.Equal(t, "Swimming lessons & more", seo.OGTitle, "entities decoded")
	assert.Equal(t, "article", seo.OGType)
	assert.Equal(t, "https://hawanas.com/swim/", seo.OGURL)
	assert.Equal(t, "Hawanas", seo.OGSiteName)
	assert.Equal(t, "en_GB", seo.OGLocale)
	assert.Equal(t, "summary_large_image", seo.TwitterCard)
	assert.Equal(t, "Swimming lessons", seo.TwitterTitle)
	assert.Equal(t, "@hawanas", seo.TwitterSite)
	assert.Equal(t, "2010-07-21T12:00:00+00:00", seo.ArticlePublished)
	assert.Equal(t, "hawanass", seo.ArticleAuthor)
}

// TestPopulateSEOKeepsUnanticipatedTags is the point of the catch-all: a tag
// with no dedicated field must still survive the export.
func TestPopulateSEOKeepsUnanticipatedTags(t *testing.T) {
	var seo models.SEOData
	newTestCrawler("all").populateSEO(&seo, richPage)

	assert.Equal(t, "Rank Math 1.0.219", seo.Meta["generator"])
	assert.Equal(t, "ABC123", seo.Meta["msvalidate.01"])

	// Named fields are promoted, not duplicated into the catch-all.
	assert.NotContains(t, seo.Meta, "description")
	assert.NotContains(t, seo.Meta, "og:title")
}

func TestPopulateSEOJSONLD(t *testing.T) {
	var seo models.SEOData
	newTestCrawler("all").populateSEO(&seo, richPage)

	require.Len(t, seo.JSONLD, 1)
	assert.Contains(t, seo.JSONLD[0], `"@type":"Article"`)
}

func TestExtractMetaPolicies(t *testing.T) {
	tests := []struct {
		name    string
		policy  string
		present []string
		absent  []string
	}{
		{"all keeps everything", "all", []string{"generator", "msvalidate.01"}, nil},
		{"empty behaves as all", "", []string{"generator"}, nil},
		{"none drops the catch-all", "none", nil, []string{"generator", "msvalidate.01"}},
		{"allow-list keeps only listed", "generator", []string{"generator"}, []string{"msvalidate.01"}},
		{"allow-list is trimmed", " generator , msvalidate.01 ", []string{"generator", "msvalidate.01"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seo models.SEOData
			newTestCrawler(tt.policy).populateSEO(&seo, richPage)

			for _, key := range tt.present {
				assert.Contains(t, seo.Meta, key)
			}
			for _, key := range tt.absent {
				assert.NotContains(t, seo.Meta, key)
			}

			// Named fields are never affected by the policy.
			assert.Equal(t, "Learn to swim", seo.MetaDescription)
		})
	}
}

// TestPopulateSEOHonoursExcludeTags pins that --exclude-tags still works, in
// both the plain and the "meta:" spelling.
func TestPopulateSEOHonoursExcludeTags(t *testing.T) {
	crawler := NewCrawler(&config.Config{
		ExtractMeta: "all",
		ExcludeTags: []string{"meta:description", "og:type", "generator"},
		Quiet:       true,
	})

	var seo models.SEOData
	crawler.populateSEO(&seo, richPage)

	assert.Empty(t, seo.MetaDescription)
	assert.Empty(t, seo.OGType)
	assert.NotContains(t, seo.Meta, "generator")
	assert.Equal(t, "Hawanas", seo.OGSiteName, "unexcluded fields still extracted")
}

func TestExtractAnalytics(t *testing.T) {
	found := extractAnalytics(richPage)

	assert.Equal(t, []string{"G-ABC1234567"}, found.GA4)
	assert.Equal(t, []string{"UA-123456-1"}, found.UniversalAnalytics)
	assert.Equal(t, []string{"GTM-WXYZ123"}, found.GoogleTagManager)
}

func TestExtractAnalyticsOtherVendors(t *testing.T) {
	html := `<script>fbq('init', '1234567890123456');</script>
	<script>h._hjSettings={hjid:1234567,hjsv:6};</script>
	<script src="https://www.clarity.ms/tag/abcdef1234"></script>
	<script>_linkedin_partner_id = "987654";</script>
	<script>ttq.load('CABCDEFGHIJKLMNOPQR');</script>
	<script>gtag('config','AW-123456789');</script>`

	found := extractAnalytics(html)

	assert.Equal(t, []string{"1234567890123456"}, found.MetaPixel)
	assert.Equal(t, []string{"1234567"}, found.HotjarSiteID)
	assert.Equal(t, []string{"abcdef1234"}, found.ClarityProjectID)
	assert.Equal(t, []string{"987654"}, found.LinkedInPartnerID)
	assert.Equal(t, []string{"CABCDEFGHIJKLMNOPQR"}, found.TikTokPixel)
	assert.Equal(t, []string{"AW-123456789"}, found.GoogleAdsConversion)
}

func TestExtractAnalyticsEmptyPage(t *testing.T) {
	assert.True(t, isEmptyAnalytics(extractAnalytics("<html><body>nothing</body></html>")))
}

// TestRecordAnalyticsDeduplicatesAcrossPages pins that the same ID seen on every
// page is reported once, and that the result is a site-wide union.
func TestRecordAnalyticsDeduplicatesAcrossPages(t *testing.T) {
	crawler := newTestCrawler("all")

	crawler.recordAnalytics(extractAnalytics(richPage))
	crawler.recordAnalytics(extractAnalytics(richPage))
	crawler.recordAnalytics(extractAnalytics(`<script src="...?id=G-ZZZ9999999"></script>`))

	analytics := crawler.Analytics()
	require.NotNil(t, analytics)

	assert.Equal(t, []string{"G-ABC1234567", "G-ZZZ9999999"}, analytics.GA4, "union, sorted, deduplicated")
	assert.Equal(t, []string{"GTM-WXYZ123"}, analytics.GoogleTagManager)
}

func TestAnalyticsNilWhenNoneFound(t *testing.T) {
	assert.Nil(t, newTestCrawler("all").Analytics())
}

func TestParseMetaTags(t *testing.T) {
	tests := []struct {
		name string
		html string
		key  string
		want string
	}{
		{"name attribute", `<meta name="a" content="1">`, "a", "1"},
		{"property attribute", `<meta property="og:x" content="2">`, "og:x", "2"},
		{"itemprop attribute", `<meta itemprop="c" content="3">`, "c", "3"},
		{"http-equiv attribute", `<meta http-equiv="d" content="4">`, "d", "4"},
		{"reversed order", `<meta content="5" name="e">`, "e", "5"},
		{"single quotes", `<meta name='f' content='6'>`, "f", "6"},
		{"key lower-cased", `<meta name="OG:Y" content="7">`, "og:y", "7"},
		{"entities decoded", `<meta name="g" content="a &amp; b">`, "g", "a & b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := parseMetaTags(tt.html)
			require.Len(t, tags, 1)
			assert.Equal(t, tt.key, tags[0].key)
			assert.Equal(t, tt.want, tags[0].content)
		})
	}
}

// TestParseTagAttributesFirstWins pins that a repeated attribute keeps its first
// value, as browsers do.
func TestParseTagAttributesFirstWins(t *testing.T) {
	attrs := parseTagAttributes(`<meta name="a" name="b" content="1">`)

	assert.Equal(t, "a", attrs["name"])
	assert.Equal(t, "1", attrs["content"])
}

func TestParseMetaTagsSkipsIncomplete(t *testing.T) {
	html := `<meta charset="utf-8">` + // no name/property
		`<meta name="empty" content="">` + // blank content
		`<meta name="ok" content="kept">`

	tags := parseMetaTags(html)

	require.Len(t, tags, 1)
	assert.Equal(t, "ok", tags[0].key)
}

// TestApplyMetaTagsFirstOccurrenceWins pins the tie-break when a page declares
// the same tag twice, which duplicate SEO plugins routinely do.
func TestApplyMetaTagsFirstOccurrenceWins(t *testing.T) {
	var seo models.SEOData

	applyMetaTags(&seo, []metaTag{
		{key: "description", content: "first"},
		{key: "description", content: "second"},
		{key: "custom", content: "one"},
		{key: "custom", content: "two"},
	}, nil)

	assert.Equal(t, "first", seo.MetaDescription)
	assert.Equal(t, "one", seo.Meta["custom"])
}

func TestExtractJSONLD(t *testing.T) {
	html := `<script type="application/ld+json">{"a":1}</script>
	<script type="application/json">{"ignored":true}</script>
	<script type='application/ld+json'>  {"b":2}  </script>
	<script type="application/ld+json"></script>`

	blocks := extractJSONLD(html)

	require.Len(t, blocks, 2)
	assert.Equal(t, `{"a":1}`, blocks[0])
	assert.Equal(t, `{"b":2}`, blocks[1], "surrounding whitespace trimmed")
}

func TestDecodeEntities(t *testing.T) {
	assert.Equal(t, `a & b "c" 'd'`, decodeEntities(`a &amp; b &quot;c&quot; &#39;d&#39;`))
	assert.Equal(t, "unchanged", decodeEntities("unchanged"))
}

func TestAppendUnique(t *testing.T) {
	values := appendUnique(nil, "b")
	values = appendUnique(values, "a")
	values = appendUnique(values, "b")

	assert.Equal(t, []string{"a", "b"}, values, "deduplicated and sorted")
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "b", firstNonEmpty("", "  ", "b", "c"))
	assert.Equal(t, "", firstNonEmpty("", "   "))
}
