package localdb

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
)

// openWith opens the canned site after registering overrides, which win over
// the canned answers.
func openWith(t *testing.T, options map[string]any, register func(sqlmock.Sqlmock)) *Source {
	t.Helper()
	db, mock, rec := localdbtest.New(t)
	localdbtest.ExpectSnapshot(mock)
	if register != nil {
		register(mock)
	}
	localdbtest.ExpectSiteWith(mock, options)
	src, err := New(context.Background(), db, settings(), Options{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = src.Close()
		assert.Empty(t, rec.Violations())
	})
	return src
}

func TestSiteInfoWithoutPostsPage(t *testing.T) {
	src := openWith(t, map[string]any{"page_for_posts": "0", "WPLANG": "pl_PL", "show_on_front": "posts"}, nil)
	info, err := src.GetSiteInfo()
	require.NoError(t, err)
	assert.Nil(t, info.PostsPage)
	assert.Equal(t, "pl_PL", info.Language)

	pages, err := src.GetPages()
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/home/", pages[0].Link, "not the front page when posts are on the front")
}

func TestNoMenusNoUsers(t *testing.T) {
	src := openWith(t, nil, func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery("WHERE tt.taxonomy = 'nav_menu' ORDER BY t.term_id").
			WillReturnRows(sqlmock.NewRows([]string{"term_id", "name", "slug", "description"}))
		mock.ExpectQuery("WHERE ID IN (SELECT DISTINCT post_author").
			WillReturnRows(sqlmock.NewRows([]string{"ID", "display_name", "user_nicename", "user_url"}))
	})
	menus, err := src.GetMenus()
	require.NoError(t, err)
	assert.Empty(t, menus)
	assert.NotNil(t, menus)

	users, err := src.GetUsers()
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestMenuItemEdgeCases(t *testing.T) {
	src := openWith(t, nil, func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery("WHERE tt.taxonomy = 'nav_menu' ORDER BY t.term_id").
			WillReturnRows(sqlmock.NewRows([]string{"term_id", "name", "slug", "description"}).
				AddRow(50, "Main", "main", "").AddRow(51, "Other", "other", ""))
		mock.ExpectQuery("WHERE p.post_type = 'nav_menu_item'").
			WillReturnRows(sqlmock.NewRows([]string{"ID", "post_title", "post_content", "menu_order", "term_id"}).
				AddRow(70, "Work", "", 1, 51).AddRow(71, "", "", 2, 51).AddRow(72, "Mine", "", 1, 50))
		mock.ExpectQuery("'_menu_item_type'").
			WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}).
				AddRow(70, "_menu_item_type", "post_type_archive").AddRow(70, "_menu_item_object", "portfolio").
				AddRow(71, "_menu_item_type", "taxonomy").AddRow(71, "_menu_item_object_id", "404").
				AddRow(72, "_menu_item_type", "taxonomy").AddRow(72, "_menu_item_object_id", "7"))
	})
	menus, err := src.GetMenus()
	require.NoError(t, err)
	require.Len(t, menus, 2)
	require.Len(t, menus[0].Items, 1)
	assert.Equal(t, "Mine", menus[0].Items[0].Title, "a stored title wins over the term's name")
	assert.Equal(t, "https://example.com/topics/go/", menus[0].Items[0].URL)
	require.Len(t, menus[1].Items, 1, "the item pointing at a missing term is left out")
	assert.Equal(t, "https://example.com/?post_type=portfolio", menus[1].Items[0].URL)
	assert.Empty(t, menus[1].Locations)
}

func TestMediaPagingAndOddSizes(t *testing.T) {
	columns := []string{"ID", "post_author", "post_date", "post_date_gmt", "post_title", "post_excerpt",
		"post_content", "post_name", "post_modified", "post_modified_gmt", "post_parent", "post_mime_type", "guid"}
	src := openWith(t, nil, func(mock sqlmock.Sqlmock) {
		first := sqlmock.NewRows(columns)
		for id := 1; id <= batchSize; id++ {
			first.AddRow(id, 1, "2024-01-01 00:00:00", "2024-01-01 00:00:00", "t", "", "", "s",
				"2024-01-01 00:00:00", "2024-01-01 00:00:00", 0, "image/png", "")
		}
		mock.ExpectQuery("FROM wp_posts a LEFT JOIN").WithArgs(0, batchSize).WillReturnRows(first)
		mock.ExpectQuery("FROM wp_posts a LEFT JOIN").WithArgs(batchSize, batchSize).WillReturnRows(sqlmock.NewRows(columns))
		mock.ExpectQuery("'_wp_attached_file'").WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}).
			AddRow(1, "_wp_attachment_metadata", `a:2:{s:4:"file";s:5:"a.png";s:5:"sizes";a:2:{i:0;a:0:{}s:5:"large";s:1:"x";}}`))
	})
	media, err := src.GetMedia()
	require.NoError(t, err)
	assert.Len(t, media, batchSize)
	assert.Equal(t, "a.png", media[0].MediaDetails.File)
	assert.Empty(t, media[0].MediaDetails.Sizes, "sizes that are not named arrays are skipped")
}

func TestAuthorSlugScanFailure(t *testing.T) {
	src := openWith(t, map[string]any{"permalink_structure": "/%author%/%postname%/"}, func(mock sqlmock.Sqlmock) {
		localdbtest.Garble(mock, "SELECT ID, user_nicename")
	})
	_, err := src.GetPosts()
	assert.Error(t, err)
}
