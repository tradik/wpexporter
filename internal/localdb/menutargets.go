package localdb

import (
	"net/url"

	"github.com/tradik/wpexporter/pkg/models"
)

// menuTargets is what a menu item can point at: published documents and terms.
type menuTargets struct {
	docs  map[int]models.WordPressPost
	terms map[int]struct{ name, link string }
	l     *linker
}

// menuTargets gathers the published documents and the terms with addresses.
func (s *Source) menuTargets(l *linker) (*menuTargets, error) {
	t := &menuTargets{docs: map[int]models.WordPressPost{}, terms: map[int]struct{ name, link string }{}, l: l}

	types, err := s.documentTypes()
	if err != nil {
		return nil, err
	}
	for _, typ := range types {
		docs, err := s.documents(typ)
		if err != nil {
			return nil, err
		}
		for _, d := range docs {
			t.docs[d.ID] = d
		}
	}

	cats, err := s.GetCategories()
	if err != nil {
		return nil, err
	}
	for _, c := range cats {
		t.terms[c.ID] = struct{ name, link string }{c.Name, c.Link}
	}
	tags, err := s.GetTags()
	if err != nil {
		return nil, err
	}
	for _, tg := range tags {
		t.terms[tg.ID] = struct{ name, link string }{tg.Name, tg.Link}
	}
	return t, nil
}

// resolve fills an item's title and address from what it points at. ok is
// false for an item whose target is not published.
func (t *menuTargets) resolve(it models.WordPressMenuItem) (models.WordPressMenuItem, bool) {
	switch it.Type {
	case "post_type":
		doc, ok := t.docs[it.ObjectID]
		if !ok {
			return it, false
		}
		it.URL = doc.Link
		if it.Title == "" {
			it.Title = doc.Title.Rendered
		}
	case "taxonomy":
		term, ok := t.terms[it.ObjectID]
		if !ok {
			return it, false
		}
		it.URL = term.link
		if it.Title == "" {
			it.Title = term.name
		}
	case "post_type_archive":
		// The archive's slug is registered in PHP; the query form always works.
		it.URL = t.l.query(url.Values{"post_type": {it.Object}})
	}
	return it, true
}
