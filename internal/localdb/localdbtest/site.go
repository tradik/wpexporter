package localdbtest

import (
	"sort"

	"github.com/DATA-DOG/go-sqlmock"
)

// Site describes the canned site's settings, for assertions.
var Site = struct {
	Home, Prefix string
}{Home: "https://example.com", Prefix: "wp_"}

// AttachmentMetadata is the serialized metadata of the canned image.
const AttachmentMetadata = `a:4:{s:5:"width";i:1200;s:6:"height";i:800;s:4:"file";s:18:"2024/05/photo.jpeg";` +
	`s:5:"sizes";a:1:{s:9:"thumbnail";a:4:{s:4:"file";s:18:"photo-150x150.jpeg";s:5:"width";i:150;` +
	`s:6:"height";i:150;s:9:"mime-type";s:10:"image/jpeg";}}}`

// docColumns are the columns of a document query.
var docColumns = []string{"ID", "post_author", "post_date", "post_date_gmt", "post_content", "post_title",
	"post_excerpt", "comment_status", "ping_status", "post_name", "post_modified", "post_modified_gmt",
	"post_parent", "guid"}

// doc is one document row.
func doc(id, parent int, slug, title, content string) []any {
	return []any{id, 1, "2024-05-06 07:08:09", "2024-05-06 05:08:09", content, title, "",
		"open", "open", slug, "2024-05-07 07:08:09", "0000-00-00 00:00:00", parent, "https://example.com/?p=x"}
}

// metaColumns are the columns of a meta query.
var metaColumns = []string{"post_id", "meta_key", "meta_value"}

// ExpectSite registers every query the local source sends for the canned site:
// two posts, four pages, one portfolio entry, one author, two comments, one
// menu, two media items.
func ExpectSite(mock sqlmock.Sqlmock) {
	ExpectSiteWith(mock, nil)
}

// Options are the canned site's options.
var Options = map[string]any{
	"blogname": "Local Site", "blogdescription": "Read from the database",
	"siteurl": "https://example.com", "home": "https://example.com/",
	"timezone_string": "", "gmt_offset": "2", "date_format": "F j, Y",
	"time_format": "H:i", "start_of_week": "1", "WPLANG": "",
	"show_on_front": "page", "page_on_front": "10", "page_for_posts": "11",
	"permalink_structure": "/%year%/%monthnum%/%postname%/", "category_base": "",
	"tag_base": "topics", "sticky_posts": "a:1:{i:0;i:1;}", "stylesheet": "twentytwenty",
	"upload_path": "", "upload_url_path": nil,
}

// ExpectSiteWith is ExpectSite with some options replaced.
func ExpectSiteWith(mock sqlmock.Sqlmock, options map[string]any) {
	p := Site.Prefix
	var values [][]any
	for _, name := range sortedKeys(Options) {
		v := Options[name]
		if o, ok := options[name]; ok {
			v = o
		}
		values = append(values, []any{name, v})
	}
	mock.ExpectQuery("FROM " + p + "options WHERE option_name IN").WillReturnRows(rows([]string{"option_name", "option_value"}, values...))
	mock.ExpectQuery("SELECT ID, user_nicename FROM " + p + "users WHERE ID IN").
		WillReturnRows(rows([]string{"ID", "user_nicename"}, []any{1, "ada"}))

	docs := "FROM " + p + "posts WHERE post_type = ?"
	mock.ExpectQuery(docs).WithArgs("post", 0, 500).WillReturnRows(rows(docColumns,
		doc(1, 0, "hello-world", "Hello world", "<!-- wp:paragraph -->\n<p>Hello [gallery ids=\"20\"]</p>\n<!-- /wp:paragraph -->"),
		doc(2, 0, "second", "Second", "Plain line one\nline two\n\nSecond paragraph")))
	mock.ExpectQuery(docs).WithArgs("page", 0, 500).WillReturnRows(rows(docColumns,
		doc(10, 0, "home", "Home", "<p>Welcome</p>"), doc(11, 0, "blog", "Blog", ""),
		doc(12, 0, "about", "About", "<p>About us</p>"), doc(13, 12, "team", "Team", "<p>The team</p>")))
	mock.ExpectQuery(docs).WithArgs("portfolio", 0, 500).WillReturnRows(rows(docColumns,
		doc(30, 0, "bridge", "Bridge", "<p>A bridge</p>")))

	meta := "FROM " + p + "postmeta WHERE post_id IN"
	mock.ExpectQuery(meta).WithArgs(1, 2, `\_%`).WillReturnRows(rows(metaColumns,
		[]any{1, "_thumbnail_id", "20"}, []any{1, "subtitle", "First"}, []any{1, "subtitle", "Ignored"}))
	mock.ExpectQuery(meta).WithArgs(10, 11, 12, 13, `\_%`).WillReturnRows(rows(metaColumns,
		[]any{12, "_wp_page_template", "templates/wide.php"}, []any{10, "_wp_page_template", "default"}))
	mock.ExpectQuery(meta).WithArgs(30, `\_%`).WillReturnRows(rows(metaColumns))

	terms := "FROM " + p + "term_relationships tr"
	termColumns := []string{"object_id", "term_id", "taxonomy", "slug"}
	mock.ExpectQuery(terms).WithArgs(1, 2).WillReturnRows(rows(termColumns,
		[]any{1, 5, "category", "child"}, []any{1, 7, "post_tag", "go"}, []any{1, 9, "post_format", "post-format-gallery"}))
	mock.ExpectQuery(terms).WithArgs(10, 11, 12, 13).WillReturnRows(rows(termColumns))
	mock.ExpectQuery(terms).WithArgs(30).WillReturnRows(rows(termColumns))

	expectTaxonomies(mock)
	expectPeople(mock)
	expectMenus(mock)
	expectMedia(mock)
}

// sortedKeys returns m's keys in order, so the fixture is deterministic.
func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
