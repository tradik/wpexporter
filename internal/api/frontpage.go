package api

// Which page is the home, and where the posts went (#75).
//
// WordPress has two settings that decide the shape of a migrated site:
// `show_on_front` — the blog archive, or a static page — and, when it is a
// static page, `page_on_front` and `page_for_posts`. The export described the
// site well and walked past all three, so anything building a theme from it had
// to guess, and every available guess is bad:
//
//   - "is there a document claiming /?" answers the first question only, and
//     says nothing about where the archive went;
//   - "is there an empty page called blog?" is a slug convention, and it breaks
//     on every site that calls it news, journal or aktualności.
//
// On the site that reported this, the listing page was never identified, the
// theme was built from the wrong page, and the blog lost its layout.
//
// /wp/v2/settings carries all three and needs authentication, so it cannot be
// the only source. WordPress also states the answer in the markup of every page
// it renders: `<body class="home blog">` when the home is the archive, and
// `home page page-id-4211` when it is a static page. That is a fact the site
// publishes to every visitor, and it needs no credentials.
//
// Nothing here is guessed. A key absent from the export means it could not be
// worked out, which a consumer can tell apart from "there is no posts page".

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// bodyClassRe reads the class attribute of a rendered page's <body>.
var bodyClassRe = regexp.MustCompile(`(?is)<body\b[^>]*\bclass\s*=\s*["']([^"']*)["']`)

// pageIDClassRe reads the numeric id out of WordPress's own `page-id-4211`.
var pageIDClassRe = regexp.MustCompile(`^page-id-(\d+)$`)

// Values of show_on_front, in WordPress's own spelling.
const (
	frontShowsPage  = "page"
	frontShowsPosts = "posts"
)

// readFrontPage fills in the site's front-page settings, from the authenticated
// settings document where it answers and from the rendered home page otherwise.
//
// It is called after the identity fields are in place, and adds only what it
// can establish: a run that learns nothing leaves all three keys absent rather
// than writing a default that reads like an answer.
func (c *Client) readFrontPage(info *models.SiteInfo, settings siteSettings) {
	// The settings document is the site's own record and wins outright, but it
	// needs credentials, so most runs fall to the markup below.
	if settings.ShowOnFront != "" {
		info.ShowOnFront = settings.ShowOnFront
		c.namePage(&info.FrontPage, settings.PageOnFront)
		c.namePage(&info.PostsPage, settings.PageForPosts)

		return
	}

	home, ok := c.renderedHome()
	if !ok {
		return
	}

	classes := bodyClasses(home)
	switch {
	case classes["blog"] && classes["home"]:
		// The home is the archive: there is no static front page and no
		// separate posts page to find.
		info.ShowOnFront = frontShowsPosts
	case classes["home"] && classes["page"]:
		info.ShowOnFront = frontShowsPage
		c.namePage(&info.FrontPage, frontPageID(classes))
	}
}

// renderedHome fetches the site's home page as a visitor sees it.
func (c *Client) renderedHome() (string, bool) {
	resp, err := c.httpClient.R().Get(c.siteRoot() + "/")
	if err != nil || resp.StatusCode() != http.StatusOK {
		return "", false
	}

	return string(resp.Body()), true
}

// bodyClasses is the set of classes on a rendered page's <body>.
func bodyClasses(html string) map[string]bool {
	match := bodyClassRe.FindStringSubmatch(html)
	if match == nil {
		return nil
	}

	classes := map[string]bool{}
	for _, class := range strings.Fields(match[1]) {
		classes[strings.ToLower(class)] = true
	}

	return classes
}

// frontPageID reads the page id WordPress writes into the body classes, or 0
// when the theme did not emit one.
func frontPageID(classes map[string]bool) int {
	for class := range classes {
		match := pageIDClassRe.FindStringSubmatch(class)
		if match == nil {
			continue
		}

		if id, err := strconv.Atoi(match[1]); err == nil {
			return id
		}
	}

	return 0
}

// namePage turns a page id into the record a consumer can act on, asking the
// API for the slug and address that go with it.
//
// An id alone is worth little after a migration — the numbers do not survive it
// — so the slug and the published address travel with it. A page the API will
// not name is still recorded by id, because "the home is page 4211" is more
// than nothing.
func (c *Client) namePage(target **models.SitePage, id int) {
	if id <= 0 {
		return
	}

	page := &models.SitePage{ID: id}

	if named, ok := c.pageByID(id); ok {
		page.Slug = named.Slug
		page.Link = named.Link
	}

	*target = page
}

// pageByID reads one page's slug and address.
func (c *Client) pageByID(id int) (models.WordPressPost, bool) {
	resp, err := c.fetchCollection("pages/"+strconv.Itoa(id), "")
	if err != nil || resp.StatusCode() != http.StatusOK {
		return models.WordPressPost{}, false
	}

	var page models.WordPressPost
	if err := json.Unmarshal(resp.Body(), &page); err != nil {
		return models.WordPressPost{}, false
	}

	return page, page.ID == id
}
