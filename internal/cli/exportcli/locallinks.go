package exportcli

// Asking the site where each document lives (FEAT-001, decision 3).
//
// The database gives a first answer — the permalink structure, worked through.
// It cannot give the last one: custom types' rewrite slugs, WPML, custom
// permalink plugins all live in PHP. WordPress itself answers the question for
// any document: asked for /?p=ID, it redirects to the canonical address. So
// each document's query address is requested from the origin, without
// following the redirect, and the Location is taken as its address. A document
// nothing answers for keeps the address the database gave it.
//
// Observed on WordPress 7.1.4 (make test-integration): /?p=ID for a post,
// /?page_id=ID for a page and /?post_type=T&p=ID for a custom type entry each
// answer 301 with the canonical address — the last one including a rewrite
// slug registered only in PHP. /?p=ID also redirects for pages and custom
// types; unpublished documents answer 404, so they are never asked about.

import (
	"net/http"
	neturl "net/url"
	"strconv"
	"sync"

	"github.com/tradik/wpexporter/internal/origin"
	"github.com/tradik/wpexporter/pkg/models"
)

// queryAddress is the address WordPress resolves for any document of a type.
func queryAddress(home, postType string, id int) string {
	v := neturl.Values{}
	switch postType {
	case "post":
		v.Set("p", strconv.Itoa(id))
	case "page":
		v.Set("page_id", strconv.Itoa(id))
	default:
		v.Set("post_type", postType)
		v.Set("p", strconv.Itoa(id))
	}
	return home + "/?" + v.Encode()
}

// resolveLinks replaces each document's address with the one the site
// redirects its query address to, and carries the change to the comments,
// menus and site pages that quoted the old one.
func (r *exportRun) resolveLinks() {
	c := &r.c
	var docs []*models.WordPressPost
	eachDocument(c, func(d *models.WordPressPost) { docs = append(docs, d) })
	if len(docs) == 0 {
		return
	}

	logf("Asking the site for the address of %d document(s)...\n", len(docs))
	moved := r.askLocations(docs)
	logf("The site gave %d address(es) different from the database's\n", len(moved))
	relink(c, moved)
}

// askLocations asks the origin for each document's canonical address, a few
// at a time, and returns old address → new for those that differ.
func (r *exportRun) askLocations(docs []*models.WordPressPost) map[string]string {
	client := origin.HTTPClient(r.cfg)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	home := homePageURL(r.c.siteInfo, r.cfg)
	for len(home) > 0 && home[len(home)-1] == '/' {
		home = home[:len(home)-1]
	}

	moved := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, max(r.cfg.Concurrent, 1))
	for _, doc := range docs {
		wg.Add(1)
		slots <- struct{}{}
		go func(doc *models.WordPressPost) {
			defer func() { <-slots; wg.Done() }()
			loc, ok := r.locationOf(client, queryAddress(home, doc.Type, doc.ID))
			if !ok || loc == doc.Link {
				return
			}
			mu.Lock()
			moved[doc.Link] = loc
			doc.Link = loc
			mu.Unlock()
		}(doc)
	}
	wg.Wait()
	return moved
}

// locationOf requests address and returns where the site redirects it, when
// that is an address on the site itself.
func (r *exportRun) locationOf(client *http.Client, address string) (string, bool) {
	req, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", r.cfg.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	_ = resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
	default:
		return "", false
	}
	loc, err := resp.Location()
	if err != nil || !r.cfg.IsSameHost(loc.String()) {
		return "", false
	}
	return loc.String(), true
}

// relink carries moved addresses to everything that quoted them.
func relink(c *collected, moved map[string]string) {
	if len(moved) == 0 {
		return
	}
	for i := range c.comments {
		if to, ok := moved[c.comments[i].PostURL]; ok {
			c.comments[i].PostURL = to
			c.comments[i].Link = to + "#comment-" + strconv.Itoa(c.comments[i].ID)
		}
	}
	for m := range c.menus {
		for i := range c.menus[m].Items {
			if to, ok := moved[c.menus[m].Items[i].URL]; ok {
				c.menus[m].Items[i].URL = to
			}
		}
	}
	for _, page := range []*models.SitePage{c.siteInfo.FrontPage, c.siteInfo.PostsPage} {
		if page == nil {
			continue
		}
		if to, ok := moved[page.Link]; ok {
			page.Link = to
		}
	}
}
