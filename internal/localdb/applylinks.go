package localdb

import (
	"slices"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// applyLinks sets Link on a type's documents. ancestors are pages outside docs
// that a page's address still runs through — a limited read leaves them out.
func (s *Source) applyLinks(postType string, docs, ancestors []models.WordPressPost) error {
	l, err := s.linker()
	if err != nil {
		return err
	}

	switch postType {
	case "post":
		return s.linkPosts(l, docs)
	case "page":
		byID := make(map[int]models.WordPressPost, len(docs)+len(ancestors))
		for _, d := range append(ancestors, docs...) {
			byID[d.ID] = d
		}
		for i := range docs {
			docs[i].Link = l.page(byID, docs[i])
		}
	default:
		for i := range docs {
			docs[i].Link = l.custom(postType, docs[i].ID)
		}
	}
	return nil
}

// linkPosts sets the blog posts' addresses, reading categories and author
// slugs only when the permalink structure uses them.
func (s *Source) linkPosts(l *linker, docs []models.WordPressPost) error {
	var cats []models.WordPressCategory
	if strings.Contains(l.structure, "%category%") {
		var err error
		if cats, err = s.GetCategories(); err != nil {
			return err
		}
	}
	authors := map[int]string{}
	if strings.Contains(l.structure, "%author%") {
		var err error
		if authors, err = s.authorSlugs(authorIDs(docs)); err != nil {
			return err
		}
	}
	for i := range docs {
		docs[i].Link = l.post(docs[i], primaryCategory(cats, docs[i].Categories), authors[docs[i].Author])
	}
	return nil
}

// primaryCategory is the category %category% stands for: WordPress takes the
// one with the lowest ID, written through its parents.
func primaryCategory(cats []models.WordPressCategory, ids []int) string {
	if len(ids) == 0 {
		return "uncategorized"
	}
	return categoryPath(cats, slices.Min(ids))
}

// authorIDs lists the distinct authors of docs.
func authorIDs(docs []models.WordPressPost) []int {
	seen := map[int]bool{}
	var out []int
	for _, d := range docs {
		if !seen[d.Author] {
			seen[d.Author] = true
			out = append(out, d.Author)
		}
	}
	return out
}
