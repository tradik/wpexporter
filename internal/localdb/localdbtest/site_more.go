package localdbtest

import "github.com/DATA-DOG/go-sqlmock"

// expectTaxonomies registers categories, tags and the post type list.
func expectTaxonomies(mock sqlmock.Sqlmock) {
	p := Site.Prefix
	taxColumns := []string{"term_id", "name", "slug", "description", "parent", "count"}
	tax := "FROM " + p + "terms t JOIN " + p + "term_taxonomy tt ON tt.term_id = t.term_id WHERE tt.taxonomy = ?"
	mock.ExpectQuery(tax).WithArgs("category").WillReturnRows(rows(taxColumns,
		[]any{4, "Parent", "parent", "", 0, 0}, []any{5, "Child", "child", "Nested", 4, 1}))
	mock.ExpectQuery(tax).WithArgs("post_tag").WillReturnRows(rows(taxColumns,
		[]any{7, "Go", "go", "", 0, 1}))

	mock.ExpectQuery("SELECT post_type, COUNT(*) FROM " + p + "posts").WillReturnRows(rows([]string{"post_type", "count"},
		[]any{"elementor_library", 2}, []any{"nav_menu_item", 4}, []any{"page", 4}, []any{"portfolio", 1},
		[]any{"post", 2}, []any{"product", 3}, []any{"product_variation", 1}))
}

// expectPeople registers the author and the comments.
func expectPeople(mock sqlmock.Sqlmock) {
	p := Site.Prefix
	mock.ExpectQuery("FROM " + p + "users WHERE ID IN (SELECT DISTINCT post_author").
		WillReturnRows(rows([]string{"ID", "display_name", "user_nicename", "user_url"},
			[]any{1, "Ada Lovelace", "ada", "https://ada.example"}))
	mock.ExpectQuery("FROM " + p + "usermeta WHERE meta_key = 'description'").
		WillReturnRows(rows([]string{"user_id", "meta_value"}, []any{1, "Wrote the first program."}))

	mock.ExpectQuery("FROM " + p + "comments").WillReturnRows(rows(
		[]string{"comment_ID", "comment_post_ID", "comment_parent", "comment_author", "comment_author_url",
			"comment_date", "comment_date_gmt", "comment_content"},
		[]any{100, 1, 0, "Reader", "", "2024-05-08 10:00:00", "2024-05-08 08:00:00", "Nice post\nreally"},
		[]any{101, 1, 100, "Ada Lovelace", "https://ada.example", "2024-05-08 11:00:00", "2024-05-08 09:00:00", "Thanks"}))
}

// expectMenus registers one menu with four items, one of them pointing at an
// unpublished page.
func expectMenus(mock sqlmock.Sqlmock) {
	p := Site.Prefix
	mock.ExpectQuery("WHERE tt.taxonomy = 'nav_menu' ORDER BY t.term_id").
		WillReturnRows(rows([]string{"term_id", "name", "slug", "description"}, []any{50, "Main", "main", ""}))
	mock.ExpectQuery("WHERE p.post_type = 'nav_menu_item'").WillReturnRows(rows(
		[]string{"ID", "post_title", "post_content", "menu_order", "term_id"},
		[]any{60, "", "", 1, 50}, []any{61, "", "", 2, 50}, []any{62, "Docs", "External", 3, 50}, []any{63, "Gone", "", 4, 50}))
	mock.ExpectQuery("'_menu_item_type'").WillReturnRows(rows(metaColumns,
		[]any{60, "_menu_item_type", "post_type"}, []any{60, "_menu_item_object", "page"},
		[]any{60, "_menu_item_object_id", "12"}, []any{60, "_menu_item_menu_item_parent", "0"},
		[]any{60, "_menu_item_classes", `a:2:{i:0;s:4:"menu";i:1;s:0:"";}`},
		[]any{61, "_menu_item_type", "taxonomy"}, []any{61, "_menu_item_object", "category"},
		[]any{61, "_menu_item_object_id", "5"}, []any{61, "_menu_item_menu_item_parent", "60"},
		[]any{62, "_menu_item_type", "custom"}, []any{62, "_menu_item_url", "https://docs.example"},
		[]any{62, "_menu_item_target", "_blank"},
		[]any{63, "_menu_item_type", "post_type"}, []any{63, "_menu_item_object_id", "999"}))
	mock.ExpectQuery("FROM " + p + "options WHERE option_name = ?").WithArgs("theme_mods_twentytwenty").
		WillReturnRows(rows([]string{"option_value"},
			[]any{`a:1:{s:18:"nav_menu_locations";a:2:{s:7:"primary";i:50;s:6:"footer";i:50;}}`}))
}

// The two media items' dates, each used for all four of its date columns.
const (
	photoDate    = "2024-05-01 00:00:00"
	brochureDate = "2024-05-02 00:00:00"
)

// expectMedia registers an image and a PDF.
func expectMedia(mock sqlmock.Sqlmock) {
	p := Site.Prefix
	mock.ExpectQuery("FROM " + p + "posts a LEFT JOIN").WillReturnRows(rows(
		[]string{"ID", "post_author", "post_date", "post_date_gmt", "post_title", "post_excerpt", "post_content",
			"post_name", "post_modified", "post_modified_gmt", "post_parent", "post_mime_type", "guid"},
		[]any{20, 1, photoDate, photoDate, "Photo", "A caption", "A description",
			"photo", photoDate, photoDate, 1, "image/jpeg", "https://example.com/wp-content/uploads/2024/05/photo.jpeg"},
		[]any{21, 1, brochureDate, brochureDate, "Brochure", "", "",
			"brochure", brochureDate, brochureDate, 0, "application/pdf", "https://example.com/?attachment_id=21"}))
	mock.ExpectQuery("'_wp_attached_file'").WillReturnRows(rows(metaColumns,
		[]any{20, "_wp_attached_file", "2024/05/photo.jpeg"}, []any{20, "_wp_attachment_image_alt", "A photo"},
		[]any{20, "_wp_attachment_metadata", AttachmentMetadata}, []any{21, "_wp_attached_file", "2024/05/brochure.pdf"},
		[]any{21, "_wp_attachment_metadata", "not serialized"}))
}

// ExpectNamedPages registers the read a limited run makes for the site
// description: the front page (10) and the posts page (11) alone.
func ExpectNamedPages(mock sqlmock.Sqlmock) {
	p := Site.Prefix
	mock.ExpectQuery("FROM "+p+"posts WHERE post_type = ?").WithArgs("page", 10, 11).WillReturnRows(rows(docColumns,
		doc(10, 0, "home", "Home", "<p>Welcome</p>"), doc(11, 0, "blog", "Blog", "")))
	mock.ExpectQuery("FROM "+p+"postmeta WHERE post_id IN").WithArgs(10, 11, `\_%`).WillReturnRows(rows(metaColumns))
	mock.ExpectQuery("FROM "+p+"term_relationships tr").WithArgs(10, 11).WillReturnRows(
		rows([]string{"object_id", "term_id", "taxonomy", "slug"}))
}
