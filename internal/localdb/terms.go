package localdb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// termsQuery reads the categories, tags and post format of a batch.
const termsQuery = "SELECT tr.object_id, tt.term_id, tt.taxonomy, t.slug FROM {term_relationships} tr " +
	"JOIN {term_taxonomy} tt ON tt.term_taxonomy_id = tr.term_taxonomy_id " +
	"JOIN {terms} t ON t.term_id = tt.term_id " +
	"WHERE tr.object_id IN (?) AND tt.taxonomy IN ('category', 'post_tag', 'post_format') " +
	"ORDER BY tr.object_id, tt.term_id"

// attachTerms fills Categories, Tags and Format.
func (s *Source) attachTerms(batch []models.WordPressPost, postType string) error {
	index := indexByID(batch)
	if postType == "post" {
		for i := range batch {
			batch[i].Format = "standard"
		}
	}
	return s.q.each(s.ctx, func(r *sql.Rows) error {
		var id, termID int
		var taxonomy, slug string
		if err := r.Scan(&id, &termID, &taxonomy, &slug); err != nil {
			return err
		}
		p := &batch[index[id]]
		switch taxonomy {
		case "category":
			p.Categories = append(p.Categories, termID)
		case "post_tag":
			p.Tags = append(p.Tags, termID)
		default:
			p.Format = strings.TrimPrefix(slug, "post-format-")
		}
		return nil
	}, termsQuery, list(ids(batch)))
}

// taxonomyQuery reads every term of one taxonomy.
const taxonomyQuery = "SELECT t.term_id, t.name, t.slug, tt.description, tt.parent, tt.count FROM {terms} t " +
	"JOIN {term_taxonomy} tt ON tt.term_id = t.term_id WHERE tt.taxonomy = ? ORDER BY t.term_id"

// GetCategories returns every category, with its address.
func (s *Source) GetCategories() ([]models.WordPressCategory, error) {
	if s.categories != nil {
		return s.categories, nil
	}
	cats := []models.WordPressCategory{}
	err := s.q.each(s.ctx, func(r *sql.Rows) error {
		c := models.WordPressCategory{Taxonomy: "category"}
		if err := r.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Parent, &c.Count); err != nil {
			return err
		}
		cats = append(cats, c)
		return nil
	}, taxonomyQuery, "category")
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading categories: %w", err))
	}

	l, err := s.linker()
	if err != nil {
		return nil, err
	}
	for i := range cats {
		cats[i].Link = l.category(cats, cats[i].ID)
	}
	s.categories = cats
	return cats, nil
}

// GetTags returns every tag, with its address.
func (s *Source) GetTags() ([]models.WordPressTag, error) {
	if s.tags != nil {
		return s.tags, nil
	}
	tags := []models.WordPressTag{}
	err := s.q.each(s.ctx, func(r *sql.Rows) error {
		t := models.WordPressTag{Taxonomy: "post_tag"}
		var parent int
		if err := r.Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &parent, &t.Count); err != nil {
			return err
		}
		tags = append(tags, t)
		return nil
	}, taxonomyQuery, "post_tag")
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading tags: %w", err))
	}

	l, err := s.linker()
	if err != nil {
		return nil, err
	}
	for i := range tags {
		tags[i].Link = l.tag(tags[i].Slug)
	}
	s.tags = tags
	return tags, nil
}
