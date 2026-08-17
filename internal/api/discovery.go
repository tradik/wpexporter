package api

// Finding the site's own inventories, rather than guessing their addresses.
//
// The sitemap was looked for at three fixed paths and the feed at `/feed/`.
// Both are guesses that hold for a default WordPress and break on the sites
// that need them most:
//
//   - a site with plain permalinks — the same site that serves its REST API
//     only at `?rest_route=` (#66) — publishes its feed at `/?feed=rss2` and
//     has no `/feed/` at all;
//   - an SEO plugin can put the sitemap anywhere, and every one of them
//     announces where in `robots.txt`, which is a file written for exactly this
//     question;
//   - a site can declare its feed in the home page's `<head>`, which is how
//     every feed reader in existence finds one.
//
// So the addresses are read from what the site says about itself first, and the
// fixed paths are the fallback rather than the whole method. Nothing here costs
// a request on a site whose guessable addresses answer: robots.txt is read only
// when the known sitemap paths have already failed.

import (
	"net/http"
	"regexp"
	"strings"
)

// robotsSitemapRe reads the `Sitemap:` lines robots.txt is required to carry an
// absolute URL on. Case-insensitive because the file is written by plugins,
// themes and hand, and half of them lowercase it.
var robotsSitemapRe = regexp.MustCompile(`(?im)^\s*sitemap:\s*(\S+)\s*$`)

// feedLinkRe reads a declared feed from a page's <head>. Both attribute orders
// occur in the wild, so the type and the href are found separately within one
// <link> element rather than in a fixed sequence.
var feedLinkRe = regexp.MustCompile(`(?is)<link\b[^>]*>`)

// hrefRe and feedTypeRe read one <link>'s parts.
var (
	hrefRe     = regexp.MustCompile(`(?is)\bhref\s*=\s*["']([^"']+)["']`)
	feedTypeRe = regexp.MustCompile(`(?is)\btype\s*=\s*["']application/(?:rss\+xml|atom\+xml)["']`)
	relAltRe   = regexp.MustCompile(`(?is)\brel\s*=\s*["'][^"']*\balternate\b[^"']*["']`)
)

// feedPaths are the addresses a WordPress serves its main feed at. The query
// form is not a legacy spelling: it is what a site with no permalink structure
// has, and it answers on every site whether or not the pretty one does.
var feedPaths = []string{"/feed/", "/?feed=rss2", "/feed/atom/"}

// sitemapCandidates is the addresses a default WordPress and its SEO plugins
// use. Whatever the site names in robots.txt is asked for separately and only
// once these have failed, which on almost every site they do not.
func (c *Client) sitemapCandidates() []string {
	root := c.siteRoot()

	candidates := make([]string, 0, len(sitemapPaths))
	for _, path := range sitemapPaths {
		candidates = append(candidates, root+path)
	}

	return candidates
}

// sitemapsFromRobots reads the addresses the site names in robots.txt.
//
// Every SEO plugin writes this line, and it is the only place a sitemap at a
// path nobody would guess — /sitemap-index-1.xml, /custom/sitemap.xml — can be
// found without crawling for it.
func (c *Client) sitemapsFromRobots() []string {
	resp, err := c.httpClient.R().Get(c.siteRoot() + "/robots.txt")
	if err != nil || resp.StatusCode() != http.StatusOK {
		return nil
	}

	var found []string

	for _, match := range robotsSitemapRe.FindAllStringSubmatch(string(resp.Body()), -1) {
		if address := strings.TrimSpace(match[1]); address != "" {
			found = append(found, address)
		}
	}

	return found
}

// feedCandidates returns the feed addresses worth trying: the one the home page
// declares, then the paths a WordPress serves.
//
// The declared one comes first because it is the site's own answer rather than
// this tool's guess — a feed moved by a plugin, or a site whose only feed is
// `/?feed=rss2` because it has no permalink structure at all, is found no other
// way.
func (c *Client) feedCandidates(declared []string) []string {
	root := c.siteRoot()

	candidates := make([]string, 0, len(declared)+len(feedPaths))
	candidates = append(candidates, declared...)

	for _, path := range feedPaths {
		candidates = append(candidates, root+path)
	}

	return candidates
}

// declaredFeeds reads the feeds a page announces in its <head>, which is how
// every feed reader in existence finds one.
func (c *Client) declaredFeeds() []string {
	resp, err := c.httpClient.R().Get(c.siteRoot() + "/")
	if err != nil || resp.StatusCode() != http.StatusOK {
		return nil
	}

	return feedsDeclaredIn(string(resp.Body()), c.siteRoot())
}

// feedsDeclaredIn pulls the alternate-feed links out of a page's markup.
func feedsDeclaredIn(html, root string) []string {
	var found []string

	for _, element := range feedLinkRe.FindAllString(html, -1) {
		if !relAltRe.MatchString(element) || !feedTypeRe.MatchString(element) {
			continue
		}

		href := hrefRe.FindStringSubmatch(element)
		if href == nil {
			continue
		}

		found = append(found, absoluteAddress(strings.TrimSpace(href[1]), root))
	}

	return found
}

// absoluteAddress resolves a declared address against the site, since a theme
// may write it root-relative.
func absoluteAddress(address, root string) string {
	switch {
	case strings.HasPrefix(address, "http://"), strings.HasPrefix(address, "https://"):
		return address
	case strings.HasPrefix(address, "//"):
		return "https:" + address
	case strings.HasPrefix(address, "/"):
		return root + address
	default:
		return root + "/" + address
	}
}
