package localdb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/pkg/models"
)

// typesQuery lists the post types that have published documents.
const typesQuery = "SELECT post_type, COUNT(*) FROM {posts} WHERE " + published +
	" GROUP BY post_type ORDER BY post_type"

// GetPostTypes lists the post types holding published documents, shaped like
// the REST types document so the export's existing custom-type handling —
// which types are content, --custom-types, --skip-unaddressable-types — runs
// on it unchanged. A type's label lives in PHP, so its name is made from the
// slug; its REST base is the slug, which is what GetCustomPosts takes.
//
// WooCommerce's own types (product*) are left out: products are not read from
// the database in this release.
func (s *Source) GetPostTypes() ([]api.PostType, error) {
	if s.types != nil {
		return s.types, nil
	}
	types := []api.PostType{}
	err := s.q.each(func(r *sql.Rows) error {
		var slug string
		var count int
		if err := r.Scan(&slug, &count); err != nil {
			return err
		}
		s.counts[slug] = count
		if slug == "product" {
			s.products = count
		}
		if !strings.HasPrefix(slug, "product") {
			types = append(types, api.PostType{Slug: slug, Name: humanize(slug), RestBase: slug})
		}
		return nil
	}, typesQuery)
	if err != nil {
		return nil, s.fail(fmt.Errorf("listing post types: %w", err))
	}
	s.types = types
	return types, nil
}

// GetCustomPosts returns the published entries of one custom type the export
// carries, by slug — which is also the name a --limit-per-type gives it.
func (s *Source) GetCustomPosts(slug string) ([]models.WordPressPost, error) {
	return s.exported(slug, slug)
}

// documentTypes lists the types whose documents the export carries: posts,
// pages and the custom types that are content.
func (s *Source) documentTypes() ([]string, error) {
	types, err := s.GetPostTypes()
	if err != nil {
		return nil, err
	}
	out := []string{"post", "page"}
	for _, t := range api.CustomPostTypes(types) {
		out = append(out, t.Slug)
	}
	return out, nil
}

// humanize turns a slug into a label: "case_study" → "Case Study".
func humanize(slug string) string {
	words := strings.FieldsFunc(slug, func(r rune) bool { return r == '_' || r == '-' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// PublishedProducts counts the published WooCommerce products, which this
// release does not read from the database: the caller says they were left out.
func (s *Source) PublishedProducts() (int, error) {
	if _, err := s.GetPostTypes(); err != nil {
		return 0, err
	}
	return s.products, nil
}
