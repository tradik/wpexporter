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
	"strings"
)

// sitemapPaths are the three names WordPress and its SEO plugins use, in the
// order worth trying: core's own first, then the two Yoast/RankMath spellings.
var sitemapPaths = []string{"/wp-sitemap.xml", "/sitemap.xml", "/sitemap_index.xml"}

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

// feedDocument is the part of RSS a completeness check reads.
type feedDocument struct {
	Items []struct {
		Link string `xml:"link"`
	} `xml:"channel>item"`
}

// FetchInventory reads the site's sitemap and main feed. Anything missing is
// simply absent from the result: no sitemap and no feed is a normal site, not
// an error, and the export proceeds exactly as it would have.
func (c *Client) FetchInventory() Inventory {
	inventory := Inventory{}

	root := strings.TrimSuffix(strings.TrimSuffix(c.baseURL, "/wp/v2"), "/wp-json")
	root = strings.TrimSuffix(root, "/")

	for _, path := range sitemapPaths {
		urls, ok := c.readSitemap(root + path)
		if !ok {
			continue
		}

		inventory.Sitemap = root + path
		inventory.SitemapURLs = urls

		break
	}

	if feed, urls, ok := c.readFeed(root + "/feed/"); ok {
		inventory.Feed = feed
		inventory.FeedURLs = urls
	}

	return inventory
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

// readFeed fetches the main RSS feed and returns the addresses of its items.
func (c *Client) readFeed(url string) (string, []string, bool) {
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

	urls := make([]string, 0, len(document.Items))
	for _, item := range document.Items {
		if link := strings.TrimSpace(item.Link); link != "" {
			urls = append(urls, link)
		}
	}

	return url, urls, true
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
