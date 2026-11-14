package localdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/pkg/models"
)

func TestPosts(t *testing.T) {
	src, _ := openSite(t)
	posts, err := src.GetPosts()
	require.NoError(t, err)
	require.Len(t, posts, 2)

	p := posts[0]
	assert.Equal(t, 1, p.ID)
	assert.Equal(t, "post", p.Type)
	assert.Equal(t, "publish", p.Status)
	assert.Equal(t, "Hello world", p.Title.Rendered)
	assert.Equal(t, "<p>Hello [gallery ids=\"20\"]</p>", p.Content.Rendered, "block delimiters removed, shortcode kept")
	assert.Equal(t, "https://example.com/2024/05/hello-world/", p.Link)
	assert.Equal(t, 20, p.FeaturedMedia)
	assert.True(t, p.Sticky)
	assert.Equal(t, "gallery", p.Format)
	assert.Equal(t, []int{5}, p.Categories)
	assert.Equal(t, []int{7}, p.Tags)
	assert.Equal(t, map[string]interface{}{"subtitle": "First"}, p.Meta, "public meta only, first value wins")
	assert.Equal(t, "2024-05-06T07:08:09Z", p.Date.Format("2006-01-02T15:04:05Z"))
	assert.Equal(t, p.Modified.Time, p.ModifiedGMT.Time, "a zeroed GMT date falls back to the local one")

	assert.Equal(t, "standard", posts[1].Format)
	assert.False(t, posts[1].Sticky)
	assert.Equal(t, "<p>Plain line one<br />\nline two</p>\n<p>Second paragraph</p>", posts[1].Content.Rendered)

	again, err := src.GetPosts()
	require.NoError(t, err)
	assert.Equal(t, posts, again, "read once, served from memory")
}

func TestPages(t *testing.T) {
	src, _ := openSite(t)
	pages, err := src.GetPages()
	require.NoError(t, err)
	require.Len(t, pages, 4)

	links := map[string]string{}
	for _, p := range pages {
		links[p.Slug] = p.Link
		assert.Empty(t, p.Format)
	}
	assert.Equal(t, map[string]string{
		"home": "https://example.com/", "blog": "https://example.com/blog/",
		"about": "https://example.com/about/", "team": "https://example.com/about/team/",
	}, links)
	assert.Equal(t, "templates/wide.php", pages[2].Template)
	assert.Empty(t, pages[0].Template, "the default template is not a template")
}

func TestCustomTypes(t *testing.T) {
	src, _ := openSite(t)
	types, err := src.GetPostTypes()
	require.NoError(t, err)

	var slugs []string
	for _, ty := range types {
		slugs = append(slugs, ty.Slug)
		assert.Equal(t, ty.Slug, ty.RestBase)
	}
	assert.Equal(t, []string{"elementor_library", "nav_menu_item", "page", "portfolio", "post"}, slugs,
		"WooCommerce types are not read from the database")
	assert.Equal(t, "Elementor Library", types[0].Name)
	products, err := src.PublishedProducts()
	require.NoError(t, err)
	assert.Equal(t, 3, products)

	entries, err := src.GetCustomPosts("portfolio")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "https://example.com/?p=30&post_type=portfolio", entries[0].Link)

	dt, err := src.documentTypes()
	require.NoError(t, err)
	assert.Equal(t, []string{"post", "page", "portfolio"}, dt, "the bookkeeping rule of internal/api applies")
}

func TestCategoriesAndTags(t *testing.T) {
	src, _ := openSite(t)
	cats, err := src.GetCategories()
	require.NoError(t, err)
	require.Len(t, cats, 2)
	assert.Equal(t, "https://example.com/category/parent/child/", cats[1].Link)
	assert.Equal(t, 4, cats[1].Parent)

	tags, err := src.GetTags()
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "https://example.com/topics/go/", tags[0].Link)

	again, _ := src.GetTags()
	assert.Equal(t, tags, again)
}

func TestUsers(t *testing.T) {
	src, _ := openSite(t)
	users, err := src.GetUsers()
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, models.WordPressUser{ID: 1, Name: "Ada Lovelace", Slug: "ada", URL: "https://ada.example",
		Description: "Wrote the first program.", Link: "https://example.com/author/ada/"}, users[0])
}

func TestComments(t *testing.T) {
	src, _ := openSite(t)
	comments, err := src.GetComments()
	require.NoError(t, err)
	require.Len(t, comments, 2)

	c := comments[0]
	assert.Equal(t, "<p>Nice post<br />\nreally</p>", c.Content)
	assert.Equal(t, "approved", c.Status)
	assert.Equal(t, "https://example.com/2024/05/hello-world/", c.PostURL)
	assert.Equal(t, "https://example.com/2024/05/hello-world/#comment-100", c.Link)
	assert.Equal(t, 100, comments[1].Parent)
}

func TestMenus(t *testing.T) {
	src, _ := openSite(t)
	menus, err := src.GetMenus()
	require.NoError(t, err)
	require.Len(t, menus, 1)

	m := menus[0]
	assert.Equal(t, []string{"footer", "primary"}, m.Locations)
	require.Len(t, m.Items, 3, "the item pointing at an unpublished page is left out")
	assert.Equal(t, models.WordPressMenuItem{ID: 60, Title: "About", URL: "https://example.com/about/", Order: 1,
		Type: "post_type", Object: "page", ObjectID: 12, Classes: []string{"menu"}}, m.Items[0])
	assert.Equal(t, "Child", m.Items[1].Title)
	assert.Equal(t, "https://example.com/category/parent/child/", m.Items[1].URL)
	assert.Equal(t, 60, m.Items[1].Parent)
	assert.Equal(t, "https://docs.example", m.Items[2].URL)
	assert.Equal(t, "_blank", m.Items[2].Target)
	assert.Equal(t, "External", m.Items[2].Description)
}

func TestMedia(t *testing.T) {
	src, _ := openSite(t)
	media, err := src.GetMedia()
	require.NoError(t, err)
	require.Len(t, media, 2)

	img := media[0]
	assert.Equal(t, "image", img.MediaType)
	assert.Equal(t, "https://example.com/wp-content/uploads/2024/05/photo.jpeg", img.SourceURL)
	assert.Equal(t, "A photo", img.AltText)
	assert.Equal(t, "A caption", img.Caption.Rendered)
	assert.Equal(t, 1, img.Post)
	assert.Equal(t, 1200, img.MediaDetails.Width)
	assert.Equal(t, "https://example.com/wp-content/uploads/2024/05/photo-150x150.jpeg",
		img.MediaDetails.Sizes["thumbnail"].SourceURL)
	assert.Equal(t, "https://example.com/?attachment_id=20", img.Link)

	pdf := media[1]
	assert.Equal(t, "file", pdf.MediaType)
	assert.Equal(t, models.MediaDetails{}, pdf.MediaDetails, "undecodable metadata is simply absent")
}
