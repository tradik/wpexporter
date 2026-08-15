package exportcli

// The completeness check (#40). On one measured site the sitemap listed 477
// URLs against 155 exported documents, 57 of them a plugin's post type — and
// the export reported success.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/pkg/models"
)

// exportOf builds an export carrying the given addresses as pages.
func exportOf(links ...string) *models.ExportData {
	data := &models.ExportData{}
	for i, link := range links {
		data.Pages = append(data.Pages, models.WordPressPost{ID: i + 1, Link: link})
	}

	return data
}

// TestCoverageNamesWhatWasMissed: the whole point. A post type the REST API
// never exposed shows up as the biggest number in the report instead of as
// nothing at all.
func TestCoverageNamesWhatWasMissed(t *testing.T) {
	inventory := api.Inventory{Sitemap: "https://x.test/wp-sitemap.xml"}
	for i := 0; i < 57; i++ {
		inventory.SitemapURLs = append(inventory.SitemapURLs, "https://x.test/events/gala-"+string(rune('a'+i%26))+"/")
	}
	inventory.SitemapURLs = append(inventory.SitemapURLs,
		"https://x.test/membership-account/", "https://x.test/about/")

	lines := checkCoverage(inventory, exportOf("https://x.test/about/"))
	require.NotEmpty(t, lines)

	report := strings.Join(lines, "\n")
	assert.Contains(t, report, "URLs this export does not cover")
	assert.Contains(t, report, "/events/")
	assert.Contains(t, report, "/membership-account/")
	assert.NotContains(t, report, "/about/", "an exported page is covered")
	assert.Contains(t, report, "--custom-types", "the report says what to do next")

	// Heaviest group first: the number that means a whole type is missing.
	assert.Less(t, strings.Index(report, "/events/"), strings.Index(report, "/membership-account/"))
}

// TestCoverageIgnoresArchives: a tag page in a sitemap is a view of content the
// export already carries, and a generator builds it for itself.
func TestCoverageIgnoresArchives(t *testing.T) {
	inventory := api.Inventory{
		Sitemap: "https://x.test/sitemap.xml",
		SitemapURLs: []string{
			"https://x.test/",
			"https://x.test/tag/cake/",
			"https://x.test/category/recipes/",
			"https://x.test/mec-category/concerts/",
			"https://x.test/author/ewa/",
			"https://x.test/2019/07/",
			"https://x.test/page/2/",
			"https://x.test/feed/",
		},
	}

	assert.Nil(t, checkCoverage(inventory, exportOf()),
		"nothing here is content an export should have carried")
}

// TestCoverageIsSilentWhenComplete: an export that covers what the site
// advertises says nothing, so the line means something when it appears.
func TestCoverageIsSilentWhenComplete(t *testing.T) {
	inventory := api.Inventory{
		Sitemap:     "https://x.test/sitemap.xml",
		SitemapURLs: []string{"https://x.test/about/", "https://x.test/blog/hello-world"},
	}

	// Trailing slash, case and host spelling differ between a sitemap and the
	// REST payload often enough that a raw comparison would report everything
	// as missing.
	assert.Nil(t, checkCoverage(inventory, exportOf(
		"https://X.test/About/", "https://x.test/blog/hello-world/")))
}

// TestCoverageWithoutInventory: no sitemap and no feed is a normal site, not an
// error, and the check simply has nothing to say.
func TestCoverageWithoutInventory(t *testing.T) {
	assert.Nil(t, checkCoverage(api.Inventory{}, exportOf("https://x.test/about/")))
	assert.Equal(t, "no sitemap or feed published", api.Inventory{}.Describe())
	assert.False(t, api.Inventory{}.Published())
}

// TestCoverageCountsTheFeedToo: the feed is the second opinion, and its items
// are compared exactly as the sitemap's are.
func TestCoverageCountsTheFeedToo(t *testing.T) {
	inventory := api.Inventory{
		Feed:     "https://x.test/feed/",
		FeedURLs: []string{"https://x.test/blog/unexported-post/"},
	}

	report := strings.Join(checkCoverage(inventory, exportOf()), "\n")
	assert.Contains(t, report, "/blog/")
	assert.Contains(t, report, "publishes 1 URLs")
}

// TestCoverageBoundsTheReport: a site whose sitemap disagrees with the export
// in dozens of ways gets a readable summary, not a reprint of the sitemap.
func TestCoverageBoundsTheReport(t *testing.T) {
	inventory := api.Inventory{Sitemap: "https://x.test/sitemap.xml"}
	for i := 0; i < maxReportedSegments+5; i++ {
		inventory.SitemapURLs = append(inventory.SitemapURLs,
			"https://x.test/section"+strings.Repeat("x", i)+"/page/")
	}

	lines := checkCoverage(inventory, exportOf())
	assert.LessOrEqual(t, len(lines), maxReportedSegments+3)
	assert.Contains(t, strings.Join(lines, "\n"), "and 5 more paths")
}

// TestNormalizePath: the comparison's foundation.
func TestNormalizePath(t *testing.T) {
	assert.Equal(t, "/blog/post/", normalizePath("https://x.test/blog/post"))
	assert.Equal(t, "/blog/post/", normalizePath("https://x.test/blog/post/?utm_source=x#top"))
	assert.Equal(t, "/", normalizePath("https://x.test"))
	assert.Equal(t, "", normalizePath("  "))
	assert.Equal(t, "/events/gala/", normalizePath("/EVENTS/Gala/"))
}

// TestCoverageCountsEveryContentKind: a custom type and a product are exported
// content too, and reporting them as missing would train the operator to ignore
// the report.
func TestCoverageCountsEveryContentKind(t *testing.T) {
	data := exportOf()
	data.CustomTypes = []models.CustomTypeSet{{
		Slug:  "services",
		Posts: []models.WordPressPost{{ID: 9, Link: "https://x.test/services/wms/"}},
	}}
	data.Products = []models.WooCommerceProduct{{ID: 4, Permalink: "https://x.test/shop/cake/"}}

	inventory := api.Inventory{
		Sitemap: "https://x.test/sitemap.xml",
		SitemapURLs: []string{
			"https://x.test/services/wms/",
			"https://x.test/shop/cake/",
			"https://x.test/events/gala/",
		},
	}

	report := strings.Join(checkCoverage(inventory, data), "\n")
	assert.Contains(t, report, "publishes 1 URLs")
	assert.Contains(t, report, "/events/")
	assert.NotContains(t, report, "/services/")
	assert.NotContains(t, report, "/shop/")
}

// TestSegmentClassification pins the two edges: a path with one segment, and a
// path with none.
func TestSegmentClassification(t *testing.T) {
	assert.Equal(t, "about", firstSegment("/about/"))
	assert.Equal(t, "", firstSegment("/"))
	assert.True(t, isArchiveSegment(""), "the home page is not missing content")
	assert.True(t, isArchiveSegment("product-tag"))
	assert.False(t, isArchiveSegment("events"))
}
