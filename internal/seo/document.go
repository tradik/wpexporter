package seo

// Building a document out of a published page (#68).
//
// On a WordPress older than 4.7 there is no content API to read: `wp/v2`
// arrived in that release, `/wp-json/` offers `oembed/1.0` and nothing else,
// and both spellings of every content route answer `rest_no_route`. The site
// still publishes — a sitemap listing its pages, and the pages themselves,
// answering 200 to anyone who asks.
//
// 1.8.15 read the sitemap and stopped there. It reported the addresses it found
// and exported none of them, because `--from-sitemap` recovered *posts* from
// the feed and a site of that age has its content in pages. The reporter's
// export was a README and a metadata.json of zeroes, from a site with eleven
// years of content behind it, and they wrote the walk themselves to get the
// migration done.
//
// So the walk lives here: an address in, a document out, through exactly the
// fetch, cache, SEO extraction and content extraction every crawled page
// already goes through.

import (
	"net/url"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// FetchDocument reads one published page and returns it as a document, or false
// when the address served nothing worth writing.
//
// The record is thinner than a REST payload — no ID, no taxonomy terms, no
// featured image, no authored date — because a rendered page is what the site
// shows a reader rather than what its database holds. It carries the title, the
// address, the SEO metadata the page declares and the content element, which is
// what a migration needs to rebuild the page.
func (c *Crawler) FetchDocument(pageURL string) (models.WordPressPost, bool) {
	result := c.extractSEOAndContent(pageURL, true)
	if strings.TrimSpace(result.Content) == "" {
		return models.WordPressPost{}, false
	}

	document := models.WordPressPost{
		Link:   pageURL,
		Slug:   documentSlug(pageURL),
		Status: "publish",
		SEO:    result.SEO,
	}

	document.Title.Rendered = documentTitle(result.SEO)
	document.Content.Rendered = result.Content

	return document, true
}

// documentTitle prefers what the page declared for sharing over its <title>,
// which carries the site name on most themes — "About us — Knine Training" is
// a browser tab, not a heading.
func documentTitle(seo models.SEOData) string {
	if seo.OGTitle != "" {
		return seo.OGTitle
	}

	return seo.Title
}

// documentSlug is the last segment of the address, which is what the page is
// called on a site that has no ID to offer.
func documentSlug(pageURL string) string {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	last := segments[len(segments)-1]

	if last == "" {
		// The site root: a home page has no segment to be named after.
		return "home"
	}

	return last
}
