package exportcli

import (
	"fmt"
	"strconv"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/localdb"
	"github.com/tradik/wpexporter/pkg/models"
)

// rawBodies records each document's body as the database gave it, keyed by
// type and ID, so the run can tell afterwards which ones the crawl replaced.
func rawBodies(c *collected) map[string]string {
	out := map[string]string{}
	eachDocument(c, func(d *models.WordPressPost) { out[docKey(d)] = d.Content.Rendered })
	return out
}

// noteRawContent reports the documents that kept the stored body — the crawl
// gave nothing for them, or there was no crawl — and how many of those still
// hold shortcodes, which only the site's PHP can render. Like any other hole,
// it goes to the summary and to metadata.json.
func (r *exportRun) noteRawContent(raw map[string]string, originUp bool) {
	total, kept, shortcodes := 0, 0, 0
	eachDocument(&r.c, func(d *models.WordPressPost) {
		total++
		if body, ok := raw[docKey(d)]; ok && body == d.Content.Rendered {
			kept++
			if localdb.HasShortcode(body) {
				shortcodes++
			}
		}
	})
	if kept == 0 {
		return
	}

	notice := fmt.Sprintf("%d of %d document(s) carry the body stored in the database rather than the "+
		"rendered page (%d with shortcodes left as written)", kept, total, shortcodes)
	if !originUp {
		notice += "; no web server answered, so nothing was crawled — give --origin"
	}
	r.c.notices = append(r.c.notices, notice)
}

// eachDocument calls fn for every post, page and custom type entry.
func eachDocument(c *collected, fn func(*models.WordPressPost)) {
	for _, set := range [][]models.WordPressPost{c.posts, c.pages} {
		for i := range set {
			fn(&set[i])
		}
	}
	for s := range c.customTypes {
		for i := range c.customTypes[s].Posts {
			fn(&c.customTypes[s].Posts[i])
		}
	}
}

// docKey identifies a document across types.
func docKey(d *models.WordPressPost) string { return d.Type + ":" + strconv.Itoa(d.ID) }

// localStated answers the summary's "limited from" in local mode: the
// database's counts, and the REST client's for the products it read.
type localStated struct {
	src    *localdb.Source
	client *api.Client
	limits *api.Limits
}

// StatedTotal implements statedCounts.
func (l localStated) StatedTotal(kind string) int {
	if kind == api.CollectionProducts {
		if l.client == nil {
			return 0
		}
		return l.client.StatedTotal(kind)
	}
	return l.src.StatedTotal(kind)
}

// Limited implements statedCounts.
func (l localStated) Limited() bool { return l.limits.Active() }
