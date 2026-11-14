package localdb

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// linker computes the addresses WordPress would give, from the stored
// permalink settings. It is the first answer, not the last: rewrite rules and
// plugin filters live in PHP, so where the site's own web server can be asked,
// its redirect overrides these (FEAT-001, decision 3).
type linker struct {
	home         string
	structure    string
	categoryBase string
	tagBase      string
	frontPage    int
	slash        string
}

// linker builds a linker from the site's options.
func (s *Source) linker() (*linker, error) {
	home, err := s.home()
	if err != nil {
		return nil, err
	}
	o := s.options
	l := &linker{home: home, structure: o["permalink_structure"],
		categoryBase: strings.Trim(o["category_base"], "/"), tagBase: strings.Trim(o["tag_base"], "/")}
	if strings.HasSuffix(l.structure, "/") {
		l.slash = "/"
	}
	if o["show_on_front"] == "page" {
		l.frontPage, _ = strconv.Atoi(o["page_on_front"])
	}
	return l, nil
}

// pretty reports whether the site uses pretty permalinks.
func (l *linker) pretty() bool { return l.structure != "" }

// query returns the home URL with a query string.
func (l *linker) query(v url.Values) string { return l.home + "/?" + v.Encode() }

// path returns the home URL with a path and the site's trailing slash habit.
func (l *linker) path(segments ...string) string {
	return l.home + "/" + strings.Join(segments, "/") + l.slash
}

// post returns a blog post's address.
func (l *linker) post(p models.WordPressPost, categoryPath, author string) string {
	if !l.pretty() {
		return l.query(url.Values{"p": {strconv.Itoa(p.ID)}})
	}
	d := p.Date.Time
	r := strings.NewReplacer(
		"%year%", d.Format("2006"), "%monthnum%", d.Format("01"), "%day%", d.Format("02"),
		"%hour%", d.Format("15"), "%minute%", d.Format("04"), "%second%", d.Format("05"),
		"%postname%", p.Slug, "%post_id%", strconv.Itoa(p.ID),
		"%category%", categoryPath, "%author%", author,
	)
	return l.home + r.Replace(l.structure)
}

// page returns a page's address: its ancestors' slugs, then its own.
func (l *linker) page(byID map[int]models.WordPressPost, p models.WordPressPost) string {
	if p.ID == l.frontPage {
		return l.home + "/"
	}
	if !l.pretty() {
		return l.query(url.Values{"page_id": {strconv.Itoa(p.ID)}})
	}
	segments := []string{p.Slug}
	seen := map[int]bool{p.ID: true}
	for parent := p.Parent; parent != 0 && !seen[parent]; {
		up, ok := byID[parent]
		if !ok {
			break
		}
		seen[parent] = true
		segments = append([]string{up.Slug}, segments...)
		parent = up.Parent
	}
	return l.path(segments...)
}

// custom returns a custom type entry's query-only address. Its pretty form
// depends on a rewrite slug registered in PHP, which the database does not
// hold; the origin's redirect supplies it when the site can be asked.
func (l *linker) custom(postType string, id int) string {
	return l.query(url.Values{"post_type": {postType}, "p": {strconv.Itoa(id)}})
}

// category returns a category's address, through its parents.
func (l *linker) category(cats []models.WordPressCategory, id int) string {
	if !l.pretty() {
		return l.query(url.Values{"cat": {strconv.Itoa(id)}})
	}
	base := l.categoryBase
	if base == "" {
		base = "category"
	}
	return l.path(base, categoryPath(cats, id))
}

// tag returns a tag's address.
func (l *linker) tag(slug string) string {
	if !l.pretty() {
		return l.query(url.Values{"tag": {slug}})
	}
	base := l.tagBase
	if base == "" {
		base = "tag"
	}
	return l.path(base, slug)
}

// author returns an author archive's address.
func (l *linker) author(nicename string, id int) string {
	if !l.pretty() {
		return l.query(url.Values{"author": {strconv.Itoa(id)}})
	}
	return l.path("author", nicename)
}

// categoryPath is a category's slug preceded by its ancestors', as %category%
// and category archives spell it.
func categoryPath(cats []models.WordPressCategory, id int) string {
	byID := make(map[int]models.WordPressCategory, len(cats))
	for _, c := range cats {
		byID[c.ID] = c
	}
	var segments []string
	seen := map[int]bool{}
	for cur, ok := byID[id]; ok && !seen[cur.ID]; cur, ok = byID[cur.Parent] {
		seen[cur.ID] = true
		segments = append([]string{cur.Slug}, segments...)
	}
	return strings.Join(segments, "/")
}
