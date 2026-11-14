package localdb

import (
	"context"
	"database/sql/driver"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
)

// Fragments of the limited reads, as the mock sees them.
const (
	newestDocs  = "ORDER BY post_date DESC, ID DESC LIMIT ? OFFSET ?"
	newestMedia = "ORDER BY a.post_date DESC, a.ID DESC LIMIT ? OFFSET ?"
	countMedia  = "SELECT COUNT(*) FROM wp_posts a LEFT JOIN"
	ancestors   = "SELECT ID, post_name, post_parent FROM wp_posts WHERE ID IN"
)

var docCols = []string{"ID", "post_author", "post_date", "post_date_gmt", "post_content", "post_title",
	"post_excerpt", "comment_status", "ping_status", "post_name", "post_modified", "post_modified_gmt",
	"post_parent", "guid"}

// docRow is one document row of the limited read.
func docRow(id, parent int, slug string) []driver.Value {
	return []driver.Value{id, 1, "2024-05-06 07:08:09", "2024-05-06 05:08:09", "<p>x</p>", slug, "",
		"open", "open", slug, "2024-05-07 07:08:09", "2024-05-07 05:08:09", parent, "https://example.com/?p=x"}
}

// docRows builds a result set of documents.
func docRows(rows ...[]driver.Value) *sqlmock.Rows {
	r := sqlmock.NewRows(docCols)
	for _, row := range rows {
		r.AddRow(row...)
	}
	return r
}

// expectNewestPosts registers the meta and terms of posts 2 and 1, in the
// order a newest-first read asks for them.
func expectNewestPosts(m sqlmock.Sqlmock) {
	m.ExpectQuery("FROM wp_postmeta WHERE post_id IN").WithArgs(2, 1, `\_%`).WillReturnRows(
		sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}).AddRow(1, "_thumbnail_id", "20"))
	m.ExpectQuery("FROM wp_term_relationships tr").WithArgs(2, 1).WillReturnRows(
		sqlmock.NewRows([]string{"object_id", "term_id", "taxonomy", "slug"}))
}

// openLimited opens the canned site with lim, after registering the test's
// own expectations, which win over the canned ones.
func openLimited(t *testing.T, lim *api.Limits, register func(sqlmock.Sqlmock)) *Source {
	t.Helper()
	db, mock, rec := localdbtest.New(t)
	localdbtest.ExpectSnapshot(mock)
	register(mock)
	localdbtest.ExpectSite(mock)

	src, err := New(context.Background(), db, settings(), Options{Limits: lim})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, src.Close())
		require.Empty(t, rec.Violations())
	})
	return src
}

// A limited read asks for the newest documents and no more: the budget is the
// SQL LIMIT, not a truncation of everything read.
func TestLimitedPostsAreTheNewest(t *testing.T) {
	src := openLimited(t, api.NewLimits(0, 0, map[string]int{api.CollectionPosts: 5}), func(m sqlmock.Sqlmock) {
		m.ExpectQuery(newestDocs).WithArgs("post", 5, 0).WillReturnRows(docRows(docRow(2, 0, "second"), docRow(1, 0, "hello-world")))
		expectNewestPosts(m)
	})

	posts, err := src.GetPosts()
	require.NoError(t, err)
	require.Len(t, posts, 2, "a short batch ends the walk")
	assert.Equal(t, []int{2, 1}, []int{posts[0].ID, posts[1].ID}, "newest first")
	assert.Equal(t, "https://example.com/2024/05/second/", posts[0].Link)
	assert.True(t, posts[1].Sticky, "meta and terms are read for the limited batch too")
	assert.Equal(t, 2, src.StatedTotal(api.CollectionPosts), "the database's own count, under the same filter")
	assert.Zero(t, src.StatedTotal(api.CollectionPages), "a collection never read stated nothing")
}

// A budget larger than one batch is read in batches, and stops at the budget.
func TestLimitedReadInBatches(t *testing.T) {
	first := make([][]driver.Value, batchSize)
	for i := range first {
		first[i] = docRow(5000-i, 0, fmt.Sprintf("p%d", i))
	}
	second := make([][]driver.Value, 100)
	for i := range second {
		second[i] = docRow(4000-i, 0, fmt.Sprintf("q%d", i))
	}
	src := openLimited(t, api.NewLimits(600, 0, nil), func(m sqlmock.Sqlmock) {
		m.ExpectQuery(newestDocs).WithArgs("post", batchSize, 0).WillReturnRows(docRows(first...))
		m.ExpectQuery("FROM wp_postmeta WHERE post_id IN").WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}))
		m.ExpectQuery("FROM wp_term_relationships tr").WillReturnRows(sqlmock.NewRows([]string{"object_id", "term_id", "taxonomy", "slug"}))
		m.ExpectQuery(newestDocs).WithArgs("post", 100, batchSize).WillReturnRows(docRows(second...))
		m.ExpectQuery("FROM wp_postmeta WHERE post_id IN").WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}))
		m.ExpectQuery("FROM wp_term_relationships tr").WillReturnRows(sqlmock.NewRows([]string{"object_id", "term_id", "taxonomy", "slug"}))
	})

	posts, err := src.GetPosts()
	require.NoError(t, err)
	assert.Len(t, posts, 600)

	// The total is spent: the next collection does not ask at all.
	pages, err := src.GetPages()
	require.NoError(t, err)
	assert.Empty(t, pages)
}

// One --limit across kinds, in fetch order; a page's address keeps the
// ancestors the budget left out, which are read for their slugs alone.
func TestLimitedTotalAcrossKinds(t *testing.T) {
	src := openLimited(t, api.NewLimits(3, 0, nil), func(m sqlmock.Sqlmock) {
		m.ExpectQuery(newestDocs).WithArgs("post", 3, 0).WillReturnRows(docRows(docRow(2, 0, "second"), docRow(1, 0, "hello-world")))
		expectNewestPosts(m)
		m.ExpectQuery(newestDocs).WithArgs("page", 1, 0).WillReturnRows(docRows(docRow(14, 13, "lead")))
		m.ExpectQuery("FROM wp_postmeta WHERE post_id IN").WithArgs(14, `\_%`).WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}))
		m.ExpectQuery("FROM wp_term_relationships tr").WithArgs(14).WillReturnRows(sqlmock.NewRows([]string{"object_id", "term_id", "taxonomy", "slug"}))
		m.ExpectQuery(ancestors).WithArgs(13).WillReturnRows(sqlmock.NewRows([]string{"ID", "post_name", "post_parent"}).AddRow(13, "team", 12))
		m.ExpectQuery(ancestors).WithArgs(12).WillReturnRows(sqlmock.NewRows([]string{"ID", "post_name", "post_parent"}).AddRow(12, "about", 0))
	})

	posts, err := src.GetPosts()
	require.NoError(t, err)
	assert.Len(t, posts, 2)

	pages, err := src.GetPages()
	require.NoError(t, err)
	require.Len(t, pages, 1, "what the posts left of the total")
	assert.Equal(t, "https://example.com/about/team/lead/", pages[0].Link)

	portfolio, err := src.GetCustomPosts("portfolio")
	require.NoError(t, err)
	assert.Empty(t, portfolio, "the budget is spent")
	media, err := src.GetMedia()
	require.NoError(t, err)
	assert.Empty(t, media)
}

// An ancestor that is not in the database ends the chain, as it does for a
// full read.
func TestLimitedPageWithMissingAncestor(t *testing.T) {
	src := openLimited(t, api.NewLimits(0, 1, nil), func(m sqlmock.Sqlmock) {
		m.ExpectQuery(newestDocs).WithArgs("page", 1, 0).WillReturnRows(docRows(docRow(14, 99, "orphan")))
		m.ExpectQuery("FROM wp_postmeta WHERE post_id IN").WithArgs(14, `\_%`).WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}))
		m.ExpectQuery("FROM wp_term_relationships tr").WithArgs(14).WillReturnRows(sqlmock.NewRows([]string{"object_id", "term_id", "taxonomy", "slug"}))
		m.ExpectQuery(ancestors).WithArgs(99).WillReturnRows(sqlmock.NewRows([]string{"ID", "post_name", "post_parent"}))
	})

	pages, err := src.GetPages()
	require.NoError(t, err)
	require.Len(t, pages, 1)
	assert.Equal(t, "https://example.com/orphan/", pages[0].Link)
}

// The media library under a limit: newest first, and its size from a COUNT
// under the same filter as the read.
func TestLimitedMedia(t *testing.T) {
	src := openLimited(t, api.NewLimits(0, 0, map[string]int{api.CollectionMedia: 1}), func(m sqlmock.Sqlmock) {
		m.ExpectQuery(newestMedia).WithArgs(1, 0).WillReturnRows(sqlmock.NewRows([]string{"ID", "post_author", "post_date",
			"post_date_gmt", "post_title", "post_excerpt", "post_content", "post_name", "post_modified", "post_modified_gmt",
			"post_parent", "post_mime_type", "guid"}).AddRow(21, 1, "2024-05-02 00:00:00", "2024-05-02 00:00:00", "Brochure",
			"", "", "brochure", "2024-05-02 00:00:00", "2024-05-02 00:00:00", 0, "application/pdf", "g"))
		m.ExpectQuery(countMedia).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	})

	media, err := src.GetMedia()
	require.NoError(t, err)
	require.Len(t, media, 1)
	assert.Equal(t, 21, media[0].ID)
	assert.Equal(t, "https://example.com/wp-content/uploads/2024/05/brochure.pdf", media[0].SourceURL)
	assert.Equal(t, 2, src.StatedTotal(api.CollectionMedia))
}

// Under a limit the site description reads only the two pages it names, not
// every page.
func TestLimitedSiteInfo(t *testing.T) {
	src := openLimited(t, api.NewLimits(1, 0, nil), localdbtest.ExpectNamedPages)

	info, err := src.GetSiteInfo()
	require.NoError(t, err)
	require.NotNil(t, info.FrontPage)
	assert.Equal(t, "https://example.com/", info.FrontPage.Link)
	require.NotNil(t, info.PostsPage)
	assert.Equal(t, "https://example.com/blog/", info.PostsPage.Link)
	budget, proceed := src.opts.Limits.Take(api.CollectionPosts)
	assert.Equal(t, 1, budget, "describing the site spends nothing")
	assert.True(t, proceed)
}

// Failures of the limited reads are reported like any other, without the
// password.
func TestLimitedReadFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		lim      *api.Limits
		register func(sqlmock.Sqlmock)
		read     func(*Source) error
	}{
		"documents": {api.NewLimits(2, 0, nil), func(m sqlmock.Sqlmock) {
			m.ExpectQuery(newestDocs).WillReturnError(fmt.Errorf("gone hunter2"))
		}, func(s *Source) error { _, err := s.GetPosts(); return err }},
		"ancestors": {api.NewLimits(0, 1, nil), func(m sqlmock.Sqlmock) {
			m.ExpectQuery(newestDocs).WithArgs("page", 1, 0).WillReturnRows(docRows(docRow(14, 13, "lead")))
			m.ExpectQuery("FROM wp_postmeta WHERE post_id IN").WithArgs(14, `\_%`).WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}))
			m.ExpectQuery("FROM wp_term_relationships tr").WithArgs(14).WillReturnRows(sqlmock.NewRows([]string{"object_id", "term_id", "taxonomy", "slug"}))
			m.ExpectQuery(ancestors).WillReturnError(fmt.Errorf("gone hunter2"))
		}, func(s *Source) error { _, err := s.GetPages(); return err }},
		"ancestor row": {api.NewLimits(0, 1, nil), func(m sqlmock.Sqlmock) {
			m.ExpectQuery(newestDocs).WithArgs("page", 1, 0).WillReturnRows(docRows(docRow(14, 13, "lead")))
			m.ExpectQuery("FROM wp_postmeta WHERE post_id IN").WithArgs(14, `\_%`).WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}))
			m.ExpectQuery("FROM wp_term_relationships tr").WithArgs(14).WillReturnRows(sqlmock.NewRows([]string{"object_id", "term_id", "taxonomy", "slug"}))
			m.ExpectQuery(ancestors).WillReturnRows(sqlmock.NewRows([]string{"ID", "post_name", "post_parent"}).AddRow("not a number", "x", 0))
		}, func(s *Source) error { _, err := s.GetPages(); return err }},
		"post types": {api.NewLimits(2, 0, nil), func(m sqlmock.Sqlmock) {
			m.ExpectQuery(newestDocs).WithArgs("post", 2, 0).WillReturnRows(docRows())
			m.ExpectQuery("SELECT post_type, COUNT(*)").WillReturnError(fmt.Errorf("gone hunter2"))
		}, func(s *Source) error { _, err := s.GetPosts(); return err }},
		"media": {api.NewLimits(2, 0, nil), func(m sqlmock.Sqlmock) {
			m.ExpectQuery(newestMedia).WillReturnError(fmt.Errorf("gone hunter2"))
		}, func(s *Source) error { _, err := s.GetMedia(); return err }},
		"site pages": {api.NewLimits(2, 0, nil), func(m sqlmock.Sqlmock) {
			m.ExpectQuery("AND ID IN (?,?)").WillReturnError(fmt.Errorf("gone hunter2"))
		}, func(s *Source) error { _, err := s.GetSiteInfo(); return err }},
		"media count": {api.NewLimits(2, 0, nil), func(m sqlmock.Sqlmock) {
			m.ExpectQuery(newestMedia).WillReturnRows(sqlmock.NewRows([]string{"ID"}))
			m.ExpectQuery(countMedia).WillReturnError(fmt.Errorf("gone hunter2"))
		}, func(s *Source) error { _, err := s.GetMedia(); return err }},
	} {
		t.Run(name, func(t *testing.T) {
			err := tc.read(openLimited(t, tc.lim, tc.register))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "hunter2")
		})
	}
}
