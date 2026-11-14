package localdb

// Exporting less than everything from the database (--limit, #60), with the
// meaning the REST export gives it: newest first, one total budget spent across
// kinds in the order they are read, and a budget per kind — custom types named
// by their slug. The budget is the SQL LIMIT: a preview of five posts reads
// five rows, not the table.
//
// Menus, comments and users are not budgeted kinds, on REST or here. The link
// maps they need still come from the full read in documents(), unchanged.

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/pkg/models"
)

// newestDocQuery reads one batch of a type's newest published documents. The
// snapshot is consistent, so paging by offset neither skips nor repeats.
const newestDocQuery = docColumns + " ORDER BY post_date DESC, ID DESC LIMIT ? OFFSET ?"

// ancestorQuery reads pages for their slugs alone: the parents a limited read
// left out, which a child's address still runs through.
const ancestorQuery = "SELECT ID, post_name, post_parent FROM {posts} WHERE ID IN (?)"

// newestMediaQuery reads one batch of the newest media.
const newestMediaQuery = mediaColumns + mediaLibrary + " ORDER BY a.post_date DESC, a.ID DESC LIMIT ? OFFSET ?"

// namedPagesQuery reads published pages by ID.
const namedPagesQuery = docColumns + " AND ID IN (?)"

// mediaCountQuery is the size of the library, under the same filter.
const mediaCountQuery = "SELECT COUNT(*) " + mediaLibrary

// exported returns the documents of one type the export carries: every one, or
// under a limit the newest the budget allows.
func (s *Source) exported(postType, collection string) ([]models.WordPressPost, error) {
	budget, proceed := s.opts.Limits.Take(collection)
	if !proceed {
		return nil, nil
	}
	if budget == 0 {
		return s.documents(postType)
	}

	docs, err := s.newest(postType, budget)
	if err != nil {
		return nil, err
	}
	s.opts.Limits.Spend(len(docs))

	if _, err := s.GetPostTypes(); err != nil {
		return nil, err
	}
	s.stated[collection] = s.counts[postType]
	return docs, nil
}

// newest reads up to budget of a type's newest documents, in batches.
func (s *Source) newest(postType string, budget int) ([]models.WordPressPost, error) {
	var all []models.WordPressPost
	for len(all) < budget {
		n := min(batchSize, budget-len(all))
		batch, err := s.documentBatch(postType, newestDocQuery, postType, n, len(all))
		if err != nil {
			return nil, s.fail(fmt.Errorf("reading %s documents: %w", postType, err))
		}
		all = append(all, batch...)
		if len(batch) < n {
			break
		}
	}

	return all, s.linkPart(postType, all)
}

// linkPart sets the addresses of part of a type's documents, reading the
// ancestors a page's address runs through when they are not among them.
func (s *Source) linkPart(postType string, docs []models.WordPressPost) error {
	var ancestors []models.WordPressPost
	if postType == "page" {
		var err error
		if ancestors, err = s.pageAncestors(docs); err != nil {
			return err
		}
	}
	return s.applyLinks(postType, docs, ancestors)
}

// sitePages returns the pages the site description names — the front page
// and the posts page. Under a limit only those two are read; otherwise they
// are found among all pages, which the export reads anyway.
func (s *Source) sitePages(ids ...string) ([]models.WordPressPost, error) {
	if !s.opts.Limits.Active() {
		return s.documents("page")
	}
	named := make([]int, len(ids))
	for i, id := range ids {
		named[i], _ = strconv.Atoi(id)
	}
	pages, err := s.documentBatch("page", namedPagesQuery, "page", list(named))
	if err == nil {
		err = s.linkPart("page", pages)
	}
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading the front and posts pages: %w", err))
	}
	return pages, nil
}

// pageAncestors reads the parents of pages that are not among them, level by
// level, until every chain ends at the top or at a page the database lacks.
func (s *Source) pageAncestors(pages []models.WordPressPost) ([]models.WordPressPost, error) {
	known := make(map[int]bool, len(pages))
	for _, p := range pages {
		known[p.ID] = true
	}
	missing := func(docs []models.WordPressPost) []int {
		var out []int
		for _, d := range docs {
			if d.Parent != 0 && !known[d.Parent] {
				known[d.Parent] = true
				out = append(out, d.Parent)
			}
		}
		return out
	}

	var ancestors []models.WordPressPost
	for wanted := missing(pages); len(wanted) > 0; {
		var level []models.WordPressPost
		err := s.q.each(s.ctx, func(r *sql.Rows) error {
			var p models.WordPressPost
			if err := r.Scan(&p.ID, &p.Slug, &p.Parent); err != nil {
				return err
			}
			level = append(level, p)
			return nil
		}, ancestorQuery, list(wanted))
		if err != nil {
			return nil, s.fail(fmt.Errorf("reading page ancestors: %w", err))
		}
		ancestors = append(ancestors, level...)
		wanted = missing(level)
	}
	return ancestors, nil
}

// newestMedia reads up to budget of the newest media, and the library's size.
func (s *Source) newestMedia(budget int) ([]models.WordPressMedia, error) {
	var all []models.WordPressMedia
	for len(all) < budget {
		n := min(batchSize, budget-len(all))
		batch, err := s.mediaBatch(newestMediaQuery, n, len(all))
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < n {
			break
		}
	}
	s.opts.Limits.Spend(len(all))

	var count int
	if err := s.q.each(s.ctx, func(r *sql.Rows) error { return r.Scan(&count) }, mediaCountQuery); err != nil {
		return nil, s.fail(fmt.Errorf("counting media: %w", err))
	}
	s.stated[api.CollectionMedia] = count
	return all, nil
}

// StatedTotal is how many records of a collection the database holds, for a
// collection a limit cut short; 0 for one read whole or not at all.
func (s *Source) StatedTotal(collection string) int {
	return s.stated[collection]
}
