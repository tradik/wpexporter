package localdb

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestWPTime(t *testing.T) {
	assert.True(t, wpTime("not a date", "").IsZero())
	assert.Equal(t, 2024, wpTime("0000-00-00 00:00:00", "2024-01-02 03:04:05").Year())
}

func TestLinkerPlainPermalinks(t *testing.T) {
	l := &linker{home: "https://example.com"}
	p := models.WordPressPost{ID: 7, Slug: "x"}
	assert.Equal(t, "https://example.com/?p=7", l.post(p, "", ""))
	assert.Equal(t, "https://example.com/?page_id=7", l.page(nil, p))
	assert.Equal(t, "https://example.com/?cat=3", l.category(nil, 3))
	assert.Equal(t, "https://example.com/?tag=go", l.tag("go"))
	assert.Equal(t, "https://example.com/?author=1", l.author("ada", 1))
}

func TestLinkerPrettyDefaults(t *testing.T) {
	l := &linker{home: "https://example.com", structure: "/%postname%"}
	assert.Equal(t, "https://example.com/tag/go", l.tag("go"), "no trailing slash when the structure has none")
	assert.Equal(t, "https://example.com/author/ada", l.author("ada", 1))

	// A page whose parent is not published keeps the part of its path that is.
	byID := map[int]models.WordPressPost{2: {ID: 2, Slug: "mid", Parent: 99}}
	assert.Equal(t, "https://example.com/mid/leaf", l.page(byID, models.WordPressPost{ID: 3, Slug: "leaf", Parent: 2}))
	// And a loop in the parents does not hang.
	loop := map[int]models.WordPressPost{4: {ID: 4, Slug: "a", Parent: 5}, 5: {ID: 5, Slug: "b", Parent: 4}}
	assert.Equal(t, "https://example.com/a/b/c", l.page(loop, models.WordPressPost{ID: 6, Slug: "c", Parent: 5}))
}

func TestCleanRawContent(t *testing.T) {
	cases := map[string]string{
		"":                                 "",
		"<!-- wp:spacer /-->":              "",
		"<p>kept</p>":                      "<p>kept</p>",
		"one\r\ntwo":                       "<p>one<br />\ntwo</p>",
		"<h2>Title</h2>\n\nbody\n\n\n":     "<h2>Title</h2>\n<p>body</p>",
		"<div class=\"x\">a</div>\n\ntext": "<div class=\"x\">a</div>\n<p>text</p>",
		"<strong>inline</strong> start":    "<p><strong>inline</strong> start</p>",
		"[contact-form-7 id=\"1\"]":        "<p>[contact-form-7 id=\"1\"]</p>",
	}
	for in, want := range cases {
		assert.Equal(t, want, CleanRawContent(in), in)
	}
	assert.True(t, HasShortcode(`<p>[gallery ids="1,2"]</p>`))
	assert.True(t, HasShortcode(`[/caption]`))
	assert.False(t, HasShortcode(`<p>[1] a footnote, [ ] a box</p>`))
}

func TestHumanize(t *testing.T) {
	assert.Equal(t, "Case Study", humanize("case_study"))
	assert.Equal(t, "Team Member", humanize("team-member"))
}

func TestGuardAndBindLists(t *testing.T) {
	assert.NoError(t, guard("SELECT a FROM b"))
	assert.ErrorContains(t, guard("DELETE FROM wp_posts"), "not a SELECT")
	assert.ErrorContains(t, guard("SELECT user_pass FROM wp_users"), "user_pass")
	assert.ErrorContains(t, guard("select x, Comment_Author_IP from c"), "Comment_Author_IP")

	stmt, args := bindLists("SELECT a WHERE x = ? AND y IN (?) AND z IN (?) AND w = ?",
		[]any{1, list([]int{2, 3}), list([]string{}), 4})
	assert.Equal(t, "SELECT a WHERE x = ? AND y IN (?,?) AND z IN (NULL) AND w = ?", stmt)
	assert.Equal(t, []any{1, 2, 3, 4}, args)

	stmt, args = bindLists("SELECT ? ?", []any{1})
	assert.Equal(t, "SELECT ? ?", stmt, "a placeholder without an argument is left to the driver to refuse")
	assert.Equal(t, []any{1}, args)
}

func TestQueryRefusesBeforeSending(t *testing.T) {
	src, _ := openSite(t)
	_, err := src.q.query("SELECT user_email FROM {users}")
	assert.ErrorContains(t, err, "user_email")
	_, err = src.q.query("UPDATE {posts} SET post_title = ''")
	assert.ErrorContains(t, err, "not a SELECT")
}

func TestMultisiteTableNames(t *testing.T) {
	q := &querier{prefix: "wp_", blogID: 3}
	assert.Equal(t, "wp_3_posts", q.table("posts"))
	assert.Equal(t, "wp_users", q.table("users"))
	assert.Equal(t, "wp_usermeta", q.table("usermeta"))
	assert.Equal(t, "SELECT x FROM wp_3_options JOIN wp_users", q.expand("SELECT x FROM {options} JOIN {users}"))

	q.blogID = 1
	assert.Equal(t, "wp_posts", q.table("posts"))
}

// Documents are read in batches of batchSize; the next batch starts after the
// last ID of the previous one.
func TestDocumentsArePaged(t *testing.T) {
	db, mock, rec := localdbtest.New(t)
	localdbtest.ExpectSnapshot(mock)

	columns := []string{"ID", "post_author", "post_date", "post_date_gmt", "post_content", "post_title",
		"post_excerpt", "comment_status", "ping_status", "post_name", "post_modified", "post_modified_gmt",
		"post_parent", "guid"}
	first := sqlmock.NewRows(columns)
	for id := 1; id <= batchSize; id++ {
		first.AddRow(id, 1, "2024-01-01 00:00:00", "2024-01-01 00:00:00", "", "t", "", "open", "open",
			"s", "2024-01-01 00:00:00", "2024-01-01 00:00:00", 0, "")
	}
	mock.ExpectQuery("WHERE post_type = ?").WithArgs("post", 0, batchSize).WillReturnRows(first)
	mock.ExpectQuery("WHERE post_type = ?").WithArgs("post", batchSize, batchSize).WillReturnRows(sqlmock.NewRows(columns))
	mock.ExpectQuery("FROM wp_postmeta").WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}))
	mock.ExpectQuery("FROM wp_term_relationships").WillReturnRows(sqlmock.NewRows([]string{"a", "b", "c", "d"}))
	localdbtest.ExpectSite(mock)

	src, err := New(context.Background(), db, settings(), Options{})
	require.NoError(t, err)
	posts, err := src.GetPosts()
	require.NoError(t, err)
	assert.Len(t, posts, batchSize)
	require.NoError(t, src.Close())
	assert.Empty(t, rec.Violations())
}

// With no documents of a type there is no batch to attach meta to, and a
// failing options read surfaces from the link computation instead.
func TestEmptyTypeStillNeedsOptions(t *testing.T) {
	db, mock, _ := localdbtest.New(t)
	localdbtest.ExpectSnapshot(mock)
	mock.ExpectQuery("WHERE post_type = ?").WithArgs("post", 0, batchSize).
		WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	localdbtest.Fail(mock, "WHERE option_name IN")

	src, err := New(context.Background(), db, settings(), Options{})
	require.NoError(t, err)
	_, err = src.GetPosts()
	assert.Error(t, err)
	_ = src.Close()
}
