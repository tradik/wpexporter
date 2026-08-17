package api

// What the site publishes, as opposed to what the export fetched (#40).
//
// An export reports what it read. It never reports what the site *offers*, so a
// content type the REST API does not expose — a plugin's events, a membership
// area — is invisible: the run ends with a success summary and a site missing a
// section. Measured on one real site, the sitemap listed 477 URLs against 155
// exported documents, 57 of them a plugin's post type nobody was told about.
//
// Two inventories cost one request each and are published by almost every
// WordPress: the sitemap and the main feed. Neither is required, neither is
// always complete, and a site that publishes neither is normal rather than
// broken — so nothing here returns an error. What it returns is the list of
// addresses the site says it has, for the caller to subtract from.

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tradik/wpexporter/pkg/models"
)

// sitemapPaths are the names WordPress and its SEO plugins use, in the order
// worth trying: core's own first, then the plugin spellings. Anything else is
// found by asking the site — see discovery.go.
var sitemapPaths = []string{
	"/wp-sitemap.xml",
	"/sitemap.xml",
	"/sitemap_index.xml",
	"/sitemap-index.xml",
	"/wp-sitemap-index.xml",
}

// maxSitemapDocuments bounds an index walk. WordPress writes one child per
// 2,000 URLs, so twenty of them is a 40,000-URL site — past that, a
// completeness check has already learnt what it can. The bound matters because
// this runs by default: an export that quietly turned into a second crawl would
// be a worse surprise than an incomplete count.
const maxSitemapDocuments = 20

// Inventory is what the site says it publishes.
type Inventory struct {
	// SitemapURLs are every address the sitemap lists, deduplicated.
	SitemapURLs []string
	// FeedURLs are the addresses of the main feed's recent items.
	FeedURLs []string
	// FeedPosts are the feed's items as records: title, address, date, author
	// and body. They are what a site whose REST routes are broken still serves,
	// and the difference between a partial export and no export at all (#40).
	FeedPosts []models.WordPressPost
	// Sitemap and Feed name the documents that answered, empty when none did.
	// A report that cannot say where a number came from is not worth printing.
	Sitemap string
	Feed    string
}

// Published reports whether the site offered either inventory.
func (i Inventory) Published() bool {
	return i.Sitemap != "" || i.Feed != ""
}

// sitemapIndex is both shapes the protocol allows: an index of sitemaps, and a
// set of URLs. A document carries one or the other, so both are parsed at once
// and whichever filled decides what it was.
type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

// feedDocument is the RSS a completeness check reads — and, when the REST API
// cannot answer at all, the only copy of the content left to read (#40).
type feedDocument struct {
	Items []feedItem `xml:"channel>item"`
}

// feedItem is one entry of the main feed. WordPress publishes the whole post in
// content:encoded on most sites and an excerpt in description on the rest.
type feedItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Creator     string `xml:"creator"`
	Description string `xml:"description"`
	Encoded     string `xml:"encoded"`
	GUID        string `xml:"guid"`
}

// FetchInventory reads the site's sitemap and main feed. Anything missing is
// simply absent from the result: no sitemap and no feed is a normal site, not
// an error, and the export proceeds exactly as it would have.
func (c *Client) FetchInventory() Inventory {
	inventory := Inventory{}

	c.findSitemap(&inventory)
	c.findFeed(&inventory)

	return inventory
}

// findSitemap tries the addresses a default WordPress uses and, when none of
// them answers, the ones the site names in its own robots.txt.
//
// The second step costs a request and is spent only on the site that needs it:
// an SEO plugin can put the sitemap at a path nobody would guess, and robots.txt
// is the file written to answer exactly that question (#68).
func (c *Client) findSitemap(inventory *Inventory) {
	for _, address := range c.sitemapCandidates() {
		if urls, ok := c.readSitemap(address); ok {
			inventory.Sitemap = address
			inventory.SitemapURLs = urls

			return
		}
	}

	for _, address := range c.sitemapsFromRobots() {
		if urls, ok := c.readSitemap(address); ok {
			inventory.Sitemap = address
			inventory.SitemapURLs = urls

			return
		}
	}
}

// findFeed asks the site where its feed is before guessing.
//
// `/feed/` is a permalink, and a site with permalinks set to plain — the same
// site that serves its REST API only at ?rest_route= (#66) — has no such
// address. Its feed is at /?feed=rss2, and its home page says so in the
// <head>, which is how every feed reader finds one.
func (c *Client) findFeed(inventory *Inventory) {
	for _, address := range c.feedCandidates(c.declaredFeeds()) {
		feed, items, ok := c.readFeed(address)
		if !ok {
			continue
		}

		inventory.Feed = feed
		inventory.FeedURLs = feedLinks(items)
		inventory.FeedPosts = feedPosts(items)

		return
	}
}

// readSitemap fetches one sitemap and, when it is an index, the documents it
// points at. Returns false when the address serves no sitemap.
func (c *Client) readSitemap(url string) ([]string, bool) {
	index, ok := c.readSitemapDocument(url)
	if !ok {
		return nil, false
	}

	seen := make(map[string]struct{})
	var urls []string

	add := func(loc string) {
		loc = strings.TrimSpace(loc)
		if loc == "" {
			return
		}
		if _, duplicate := seen[loc]; duplicate {
			return
		}
		seen[loc] = struct{}{}
		urls = append(urls, loc)
	}

	for _, entry := range index.URLs {
		add(entry.Loc)
	}

	documents := 0
	for _, child := range index.Sitemaps {
		if documents >= maxSitemapDocuments {
			break
		}
		documents++

		c.applyRateLimit()

		childIndex, childOK := c.readSitemapDocument(strings.TrimSpace(child.Loc))
		if !childOK {
			continue
		}

		for _, entry := range childIndex.URLs {
			add(entry.Loc)
		}
	}

	return urls, true
}

// readSitemapDocument fetches and parses one sitemap document.
func (c *Client) readSitemapDocument(url string) (sitemapIndex, bool) {
	if url == "" {
		return sitemapIndex{}, false
	}

	resp, err := c.httpClient.R().Get(url)
	if err != nil || resp.StatusCode() != http.StatusOK {
		return sitemapIndex{}, false
	}

	// A site with no sitemap usually answers the request with its home page —
	// 200 and a full HTML document — so the body has to say what it is before
	// it is believed.
	if !looksLikeSitemap(resp.Body()) {
		return sitemapIndex{}, false
	}

	var document sitemapIndex
	if err := xml.Unmarshal(resp.Body(), &document); err != nil {
		return sitemapIndex{}, false
	}

	return document, true
}

// looksLikeSitemap reports whether a body opens as one of the protocol's two
// documents. Only the head is examined: a sitemap declares itself in its first
// element, and a 400 KB URL list is not worth scanning to find that out.
func looksLikeSitemap(body []byte) bool {
	head := body
	if len(head) > 512 {
		head = head[:512]
	}

	lowered := strings.ToLower(string(head))

	return strings.Contains(lowered, "<urlset") || strings.Contains(lowered, "<sitemapindex")
}

// readFeed fetches the main RSS feed and returns its items.
func (c *Client) readFeed(url string) (string, []feedItem, bool) {
	resp, err := c.httpClient.R().Get(url)
	if err != nil || resp.StatusCode() != http.StatusOK {
		return "", nil, false
	}

	var document feedDocument
	if err := xml.Unmarshal(resp.Body(), &document); err != nil {
		return "", nil, false
	}

	if len(document.Items) == 0 {
		return "", nil, false
	}

	return url, document.Items, true
}

// feedLinks is the addresses alone, for the completeness check.
func feedLinks(items []feedItem) []string {
	urls := make([]string, 0, len(items))
	for _, item := range items {
		if link := strings.TrimSpace(item.Link); link != "" {
			urls = append(urls, link)
		}
	}

	return urls
}

// feedPosts turns feed items into the records the rest of the exporter works
// with.
//
// What comes back is thinner than a REST payload — no IDs, no taxonomy terms,
// no featured image — and it is deliberately not disguised as more than that:
// the ID stays zero, so nothing downstream can mistake a recovered record for
// one WordPress numbered. Titles, addresses, dates, authors and bodies are what
// a migration needs most, and on a site whose /wp/v2/posts answers 500 they are
// the only copy on offer.
func feedPosts(items []feedItem) []models.WordPressPost {
	posts := make([]models.WordPressPost, 0, len(items))

	for _, item := range items {
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}

		post := models.WordPressPost{
			Slug:   slugFromURL(link),
			Status: "publish", // a feed lists what the site published
			Type:   "post",
			Link:   link,
		}
		post.Title.Rendered = strings.TrimSpace(item.Title)
		post.Content.Rendered = firstNonEmpty(item.Encoded, item.Description)
		post.Excerpt.Rendered = strings.TrimSpace(item.Description)

		if published, err := parseFeedDate(item.PubDate); err == nil {
			post.Date = models.WordPressTime{Time: published}
			post.Modified = models.WordPressTime{Time: published}
		}

		posts = append(posts, post)
	}

	return posts
}

// feedDateFormats are the spellings RSS dates arrive in. WordPress writes
// RFC1123 with a numeric zone; plugins and caches rewrite it.
var feedDateFormats = []string{
	time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, time.RFC3339,
}

// parseFeedDate reads a publication date in whichever form the feed used.
func parseFeedDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)

	var err error
	for _, layout := range feedDateFormats {
		var parsed time.Time
		if parsed, err = time.Parse(layout, raw); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, err
}

// slugFromURL is the last path segment of an address, which is a post's slug on
// every permalink structure WordPress offers.
func slugFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	return segments[len(segments)-1]
}

// firstNonEmpty returns the first value with something in it.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// Describe renders where the inventory came from, for a report that has to be
// able to say why it is claiming anything.
func (i Inventory) Describe() string {
	switch {
	case i.Sitemap != "" && i.Feed != "":
		return fmt.Sprintf("sitemap (%d URLs) and feed (%d items)", len(i.SitemapURLs), len(i.FeedURLs))
	case i.Sitemap != "":
		return fmt.Sprintf("sitemap (%d URLs)", len(i.SitemapURLs))
	case i.Feed != "":
		return fmt.Sprintf("feed (%d items)", len(i.FeedURLs))
	default:
		return "no sitemap or feed published"
	}
}
