package localdb

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
)

// failure is one statement made to fail, and the getter that must report it.
type failure struct {
	fragment string
	args     []driver.Value
	getter   string
}

var failures = []failure{
	{"FROM wp_options WHERE option_name IN", nil, "SiteInfo"},
	{"FROM wp_posts WHERE post_type = ?", []driver.Value{"post", 0, 500}, "Posts"},
	// GetSiteInfo reads the pages first (for the front page), so it is the
	// one that meets the failure; a later GetPages simply reads again.
	{"FROM wp_posts WHERE post_type = ?", []driver.Value{"page", 0, 500}, "SiteInfo"},
	{"FROM wp_posts WHERE post_type = ?", []driver.Value{"portfolio", 0, 500}, "Comments"},
	{"FROM wp_postmeta WHERE post_id IN", []driver.Value{1, 2, `\_%`}, "Posts"},
	{"FROM wp_term_relationships tr", []driver.Value{1, 2}, "Posts"},
	{"WHERE tt.taxonomy = ?", []driver.Value{"category"}, "Categories"},
	{"WHERE tt.taxonomy = ?", []driver.Value{"post_tag"}, "Tags"},
	{"SELECT post_type, COUNT(*)", nil, "Types"},
	{"WHERE ID IN (SELECT DISTINCT post_author", nil, "Users"},
	{"FROM wp_usermeta", nil, "Users"},
	{"FROM wp_comments", nil, "Comments"},
	{"WHERE tt.taxonomy = 'nav_menu' ORDER BY t.term_id", nil, "Menus"},
	{"WHERE p.post_type = 'nav_menu_item'", nil, "Menus"},
	{"'_menu_item_type'", nil, "Menus"},
	{"WHERE option_name = ?", nil, "Menus"},
	{"FROM wp_posts a LEFT JOIN", nil, "Media"},
	{"'_wp_attached_file'", nil, "Media"},
}

// getters calls every collection, in an order that lets a failure upstream be
// seen by each consumer.
func getters(src *Source) map[string]error {
	errs := map[string]error{}
	_, errs["SiteInfo"] = src.GetSiteInfo()
	_, errs["Posts"] = src.GetPosts()
	_, errs["Pages"] = src.GetPages()
	_, errs["Categories"] = src.GetCategories()
	_, errs["Tags"] = src.GetTags()
	_, errs["Types"] = src.GetPostTypes()
	_, errs["Users"] = src.GetUsers()
	_, errs["Comments"] = src.GetComments()
	_, errs["Menus"] = src.GetMenus()
	_, errs["Media"] = src.GetMedia()
	_, errs["Uploads"] = src.Uploads()
	_, errs["Home"] = src.Home()
	return errs
}

// Every statement can fail, or return rows of the wrong shape; each failure
// reaches the getter that needs it, and no error ever carries the password.
func TestEveryQueryFailureIsReportedWithoutThePassword(t *testing.T) {
	for _, f := range failures {
		for _, mode := range []string{"error", "garbled"} {
			t.Run(f.getter+"/"+mode+"/"+f.fragment, func(t *testing.T) {
				db, mock, _ := localdbtest.New(t)
				localdbtest.ExpectSnapshot(mock)
				if mode == "error" {
					localdbtest.Fail(mock, f.fragment, f.args...)
				} else {
					localdbtest.Garble(mock, f.fragment, f.args...)
				}
				localdbtest.ExpectSite(mock)

				s := settings()
				s.DBPassword = localdbtest.Password
				src, err := New(context.Background(), db, s, Options{})
				require.NoError(t, err)
				defer func() { _ = src.Close() }()

				errs := getters(src)
				if mode == "garbled" && f.fragment == "WHERE option_name = ?" {
					// One text column scans whatever it holds; theme mods that
					// do not decode simply assign no locations.
					assert.NoError(t, errs[f.getter])
				} else {
					assert.Error(t, errs[f.getter])
				}
				for name, err := range errs {
					if err != nil {
						assert.NotContains(t, err.Error(), localdbtest.Password, name)
					}
				}
			})
		}
	}
}

// With %category% and %author% in the structure, those are read too, and
// their failures reach the posts.
func TestPostLinksWithCategoryAndAuthor(t *testing.T) {
	structure := map[string]any{"permalink_structure": "/%category%/%author%/%post_id%-%postname%"}

	db, mock, _ := localdbtest.New(t)
	localdbtest.ExpectSnapshot(mock)
	localdbtest.ExpectSiteWith(mock, structure)
	src, err := New(context.Background(), db, settings(), Options{})
	require.NoError(t, err)
	defer func() { _ = src.Close() }()

	posts, err := src.GetPosts()
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/parent/child/ada/1-hello-world", posts[0].Link)
	assert.Equal(t, "https://example.com/uncategorized/ada/2-second", posts[1].Link)

	for _, fragment := range []string{"WHERE tt.taxonomy = ?", "SELECT ID, user_nicename"} {
		db, mock, _ := localdbtest.New(t)
		localdbtest.ExpectSnapshot(mock)
		localdbtest.Fail(mock, fragment)
		localdbtest.ExpectSiteWith(mock, structure)
		src, err := New(context.Background(), db, settings(), Options{})
		require.NoError(t, err)
		_, err = src.GetPosts()
		assert.Error(t, err, fragment)
		_ = src.Close()
	}
}
