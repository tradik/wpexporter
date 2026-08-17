package exportcli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// stubFetcher answers for a fixed set of addresses and records what it was
// asked, so a test can say what the walk decided without a network.
type stubFetcher struct {
	pages  map[string]string
	asked  []string
	silent map[string]bool
}

func (s *stubFetcher) FetchDocument(pageURL string) (models.WordPressPost, bool) {
	s.asked = append(s.asked, pageURL)

	if s.silent[pageURL] {
		return models.WordPressPost{}, false
	}

	body, ok := s.pages[pageURL]
	if !ok {
		return models.WordPressPost{}, false
	}

	document := models.WordPressPost{Link: pageURL, Slug: "x", Status: "publish"}
	document.Content.Rendered = body

	return document, true
}

// pre47Inventory is what a WordPress 4.5.3 offers: a sitemap listing its pages,
// and no content API behind them.
func pre47Inventory() api.Inventory {
	return api.Inventory{
		Sitemap: "https://knine.test/sitemap.xml",
		SitemapURLs: []string{
			"https://knine.test/about-knine/",
			"https://knine.test/guard-dog-training/",
			"https://knine.test/photo-gallery/",
		},
	}
}

// TestSitemapIsTheWholeSource: what reopened #68. 1.8.15 read the sitemap,
// printed the addresses it found and exported none of them — a README, a
// metadata.json of zeroes, and eleven years of content left on a server that
// answers 200 to anyone who asks.
func TestSitemapIsTheWholeSource(t *testing.T) {
	fetcher := &stubFetcher{pages: map[string]string{
		"https://knine.test/about-knine/":        "<p>About us</p>",
		"https://knine.test/guard-dog-training/": "<p>Training</p>",
		"https://knine.test/photo-gallery/":      "<p>Gallery</p>",
	}}

	data := &models.ExportData{}
	recovered := recoverPagesFromSitemap(&config.Config{FromSitemap: true}, fetcher, pre47Inventory(), data)

	assert.Equal(t, 3, recovered)
	require.Len(t, data.Pages, 3)
	assert.Equal(t, 3, data.Stats.TotalPages)
	assert.Equal(t, 3, data.Stats.RecoveredPages,
		"a page read from rendered HTML is thinner than one from the API, and says so")
}

// TestSitemapSkipsWhatTheExportHas: the guard that keeps this off every ordinary
// site. A site whose API answered has every published address accounted for
// already, and re-fetching them would replace API records with thinner ones.
func TestSitemapSkipsWhatTheExportHas(t *testing.T) {
	fetcher := &stubFetcher{pages: map[string]string{
		"https://knine.test/photo-gallery/": "<p>Gallery</p>",
	}}

	existing := models.WordPressPost{Link: "https://knine.test/about-knine"}
	other := models.WordPressPost{Link: "https://knine.test/guard-dog-training/"}
	data := &models.ExportData{Pages: []models.WordPressPost{existing}, Posts: []models.WordPressPost{other}}

	recovered := recoverPagesFromSitemap(&config.Config{FromSitemap: true}, fetcher, pre47Inventory(), data)

	assert.Equal(t, 1, recovered)
	assert.Equal(t, []string{"https://knine.test/photo-gallery/"}, fetcher.asked,
		"a trailing slash does not make one address look like two")
}

// TestSitemapRecoveryIsAskedFor: without --from-sitemap nothing is fetched. REST
// is the better source in every respect, and it is never quietly replaced.
func TestSitemapRecoveryIsAskedFor(t *testing.T) {
	fetcher := &stubFetcher{pages: map[string]string{"https://knine.test/about-knine/": "<p>x</p>"}}

	assert.Zero(t, recoverPagesFromSitemap(&config.Config{}, fetcher, pre47Inventory(), &models.ExportData{}))
	assert.Zero(t, recoverPagesFromSitemap(&config.Config{FromSitemap: true, NoPages: true},
		fetcher, pre47Inventory(), &models.ExportData{}))
	assert.Zero(t, recoverPagesFromSitemap(&config.Config{FromSitemap: true},
		fetcher, api.Inventory{}, &models.ExportData{}))
	assert.Empty(t, fetcher.asked)
}

// TestSitemapRecoveryHonorsTheLimit: a preview of five pages must not fetch a
// thousand, which is the whole point of #60 and #62.
func TestSitemapRecoveryHonorsTheLimit(t *testing.T) {
	fetcher := &stubFetcher{pages: map[string]string{
		"https://knine.test/about-knine/":        "<p>a</p>",
		"https://knine.test/guard-dog-training/": "<p>b</p>",
		"https://knine.test/photo-gallery/":      "<p>c</p>",
	}}

	data := &models.ExportData{}
	recovered := recoverPagesFromSitemap(&config.Config{FromSitemap: true, Limit: 2},
		fetcher, pre47Inventory(), data)

	assert.Equal(t, 2, recovered)
	assert.Len(t, fetcher.asked, 2, "the walk stops rather than fetching and discarding")
}

// TestSitemapSkipsAddressesThatGiveNothing: an address in a sitemap can be a
// redirect, a feed or an attachment page. One that yields no content is passed
// over rather than written as an empty document.
func TestSitemapSkipsAddressesThatGiveNothing(t *testing.T) {
	fetcher := &stubFetcher{
		pages:  map[string]string{"https://knine.test/photo-gallery/": "<p>Gallery</p>"},
		silent: map[string]bool{"https://knine.test/about-knine/": true},
	}

	data := &models.ExportData{}
	assert.Equal(t, 1, recoverPagesFromSitemap(&config.Config{FromSitemap: true},
		fetcher, pre47Inventory(), data))
	require.Len(t, data.Pages, 1)
	assert.Equal(t, "https://knine.test/photo-gallery/", data.Pages[0].Link)
}

// TestExportedAddressesCoversEveryKind: a custom type's entry and a product are
// documents the export carries, and re-fetching their addresses from the
// sitemap would replace API records with thinner ones read from HTML.
func TestExportedAddressesCoversEveryKind(t *testing.T) {
	entry := models.WordPressPost{Link: "https://knine.test/services/grooming/"}
	data := &models.ExportData{
		CustomTypes: []models.CustomTypeSet{{Slug: "services", Posts: []models.WordPressPost{entry}}},
		Products:    []models.WooCommerceProduct{{Permalink: "https://knine.test/produkt/lead/"}},
		Pages:       []models.WordPressPost{{Link: ""}},
	}

	covered := exportedAddresses(data)

	assert.True(t, covered["/services/grooming/"])
	assert.True(t, covered["/produkt/lead/"])
	assert.Len(t, covered, 2, "a document with no address covers nothing")
}
