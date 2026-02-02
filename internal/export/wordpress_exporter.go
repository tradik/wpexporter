package export

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// WordPressExporter exports data in WordPress WXR (WordPress eXtended RSS) format
type WordPressExporter struct {
	config *config.Config
}

// NewWordPressExporter creates a new WordPress exporter
func NewWordPressExporter(cfg *config.Config) *WordPressExporter {
	return &WordPressExporter{
		config: cfg,
	}
}

// WXR XML structures

// WXRExport represents the root RSS element for WXR export
type WXRExport struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	WP      string     `xml:"xmlns:wp,attr"`
	Excerpt string     `xml:"xmlns:excerpt,attr"`
	Content string     `xml:"xmlns:content,attr"`
	DC      string     `xml:"xmlns:dc,attr"`
	Channel WXRChannel `xml:"channel"`
}

// WXRChannel represents the channel element containing site info and items
type WXRChannel struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	PubDate     string        `xml:"pubDate"`
	Language    string        `xml:"language"`
	WXRVersion  string        `xml:"wp:wxr_version"`
	BaseSiteURL string        `xml:"wp:base_site_url"`
	BaseBlogURL string        `xml:"wp:base_blog_url"`
	Generator   string        `xml:"generator"`
	Authors     []WXRAuthor   `xml:"wp:author"`
	Categories  []WXRCategory `xml:"wp:category"`
	Tags        []WXRTag      `xml:"wp:tag"`
	Items       []WXRItem     `xml:"item"`
}

// WXRAuthor represents an author in WXR format
type WXRAuthor struct {
	ID          int    `xml:"wp:author_id"`
	Login       string `xml:"wp:author_login"`
	Email       string `xml:"wp:author_email"`
	DisplayName string `xml:"wp:author_display_name"`
	FirstName   string `xml:"wp:author_first_name"`
	LastName    string `xml:"wp:author_last_name"`
}

// WXRCategory represents a category in WXR format
type WXRCategory struct {
	TermID      int    `xml:"wp:term_id"`
	NiceName    string `xml:"wp:category_nicename"`
	Parent      string `xml:"wp:category_parent"`
	Name        string `xml:"wp:cat_name"`
	Description string `xml:"wp:category_description"`
}

// WXRTag represents a tag in WXR format
type WXRTag struct {
	TermID      int    `xml:"wp:term_id"`
	Slug        string `xml:"wp:tag_slug"`
	Name        string `xml:"wp:tag_name"`
	Description string `xml:"wp:tag_description"`
}

// WXRItem represents a post, page, or attachment in WXR format
type WXRItem struct {
	Title          string            `xml:"title"`
	Link           string            `xml:"link"`
	PubDate        string            `xml:"pubDate"`
	Creator        string            `xml:"dc:creator"`
	GUID           WXRGUID           `xml:"guid"`
	Description    string            `xml:"description"`
	ContentEncoded string            `xml:"content:encoded"`
	ExcerptEncoded string            `xml:"excerpt:encoded"`
	PostID         int               `xml:"wp:post_id"`
	PostDate       string            `xml:"wp:post_date"`
	PostDateGMT    string            `xml:"wp:post_date_gmt"`
	PostModified   string            `xml:"wp:post_modified"`
	CommentStatus  string            `xml:"wp:comment_status"`
	PingStatus     string            `xml:"wp:ping_status"`
	PostName       string            `xml:"wp:post_name"`
	Status         string            `xml:"wp:status"`
	PostParent     int               `xml:"wp:post_parent"`
	MenuOrder      int               `xml:"wp:menu_order"`
	PostType       string            `xml:"wp:post_type"`
	PostPassword   string            `xml:"wp:post_password"`
	IsSticky       int               `xml:"wp:is_sticky"`
	AttachmentURL  string            `xml:"wp:attachment_url,omitempty"`
	Categories     []WXRItemCategory `xml:"category,omitempty"`
	PostMeta       []WXRPostMeta     `xml:"wp:postmeta,omitempty"`
}

// WXRGUID represents the GUID element
type WXRGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// WXRItemCategory represents a category/tag reference in an item
type WXRItemCategory struct {
	Domain   string `xml:"domain,attr"`
	NiceName string `xml:"nicename,attr"`
	Value    string `xml:",cdata"`
}

// WXRPostMeta represents post metadata
type WXRPostMeta struct {
	MetaKey   string `xml:"wp:meta_key"`
	MetaValue string `xml:"wp:meta_value"`
}

// Export exports data in WordPress WXR format
func (e *WordPressExporter) Export(data *models.ExportData) error {
	// Build the WXR structure
	wxr := e.buildWXR(data)

	// Marshal to XML
	output, err := xml.MarshalIndent(wxr, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal WXR XML: %w", err)
	}

	// Add XML declaration
	xmlContent := []byte(xml.Header + string(output))

	// Determine output path
	var outputPath string
	if filepath.Ext(e.config.Output) == ".xml" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "wordpress_export.xml")
	}

	// Write file
	if err := os.WriteFile(outputPath, xmlContent, 0600); err != nil {
		return fmt.Errorf("failed to write WXR file: %w", err)
	}

	fmt.Printf("WordPress WXR export completed: %s\n", outputPath)
	return nil
}

// buildWXR constructs the complete WXR structure
func (e *WordPressExporter) buildWXR(data *models.ExportData) WXRExport {
	wxr := WXRExport{
		Version: "2.0",
		WP:      "http://wordpress.org/export/1.2/",
		Excerpt: "http://wordpress.org/export/1.2/excerpt/",
		Content: "http://purl.org/rss/1.0/modules/content/",
		DC:      "http://purl.org/dc/elements/1.1/",
		Channel: WXRChannel{
			Title:       data.Site.Name,
			Link:        data.Site.URL,
			Description: data.Site.Description,
			PubDate:     time.Now().Format(time.RFC1123Z),
			Language:    data.Site.Language,
			WXRVersion:  "1.2",
			BaseSiteURL: data.Site.URL,
			BaseBlogURL: data.Site.HomeURL,
			Generator:   "wpexporter/1.3.5",
		},
	}

	// Add authors
	wxr.Channel.Authors = e.convertAuthors(data.Users)

	// Add categories
	wxr.Channel.Categories = e.convertCategories(data.Categories)

	// Add tags
	wxr.Channel.Tags = e.convertTags(data.Tags)

	// Add items (posts, pages, media)
	wxr.Channel.Items = e.convertItems(data)

	return wxr
}

// convertAuthors converts WordPress users to WXR authors
func (e *WordPressExporter) convertAuthors(users []models.WordPressUser) []WXRAuthor {
	authors := make([]WXRAuthor, 0, len(users))
	for _, user := range users {
		author := WXRAuthor{
			ID:          user.ID,
			Login:       user.Slug,
			Email:       "",
			DisplayName: user.Name,
			FirstName:   "",
			LastName:    "",
		}
		authors = append(authors, author)
	}
	return authors
}

// convertCategories converts WordPress categories to WXR categories
func (e *WordPressExporter) convertCategories(categories []models.WordPressCategory) []WXRCategory {
	// Build parent slug map for categories
	slugMap := make(map[int]string)
	for _, cat := range categories {
		slugMap[cat.ID] = cat.Slug
	}

	wxrCategories := make([]WXRCategory, 0, len(categories))
	for _, cat := range categories {
		parentSlug := ""
		if cat.Parent > 0 {
			parentSlug = slugMap[cat.Parent]
		}

		wxrCat := WXRCategory{
			TermID:      cat.ID,
			NiceName:    cat.Slug,
			Parent:      parentSlug,
			Name:        cat.Name,
			Description: cat.Description,
		}
		wxrCategories = append(wxrCategories, wxrCat)
	}
	return wxrCategories
}

// convertTags converts WordPress tags to WXR tags
func (e *WordPressExporter) convertTags(tags []models.WordPressTag) []WXRTag {
	wxrTags := make([]WXRTag, 0, len(tags))
	for _, tag := range tags {
		wxrTag := WXRTag{
			TermID:      tag.ID,
			Slug:        tag.Slug,
			Name:        tag.Name,
			Description: tag.Description,
		}
		wxrTags = append(wxrTags, wxrTag)
	}
	return wxrTags
}

// convertItems converts all content types to WXR items
func (e *WordPressExporter) convertItems(data *models.ExportData) []WXRItem {
	items := make([]WXRItem, 0)

	// Build category map for lookups
	categoryMap := make(map[int]models.WordPressCategory)
	for _, cat := range data.Categories {
		categoryMap[cat.ID] = cat
	}

	// Build tag map for lookups
	tagMap := make(map[int]models.WordPressTag)
	for _, tag := range data.Tags {
		tagMap[tag.ID] = tag
	}

	// Build user map for lookups
	userMap := make(map[int]models.WordPressUser)
	for _, user := range data.Users {
		userMap[user.ID] = user
	}

	// Convert posts
	for _, post := range data.Posts {
		item := e.convertPostToItem(post, "post", categoryMap, tagMap, userMap)
		items = append(items, item)
	}

	// Convert pages
	for _, page := range data.Pages {
		item := e.convertPostToItem(page, "page", categoryMap, tagMap, userMap)
		items = append(items, item)
	}

	// Convert media/attachments
	for _, media := range data.Media {
		item := e.convertMediaToItem(media, userMap)
		items = append(items, item)
	}

	return items
}

// convertPostToItem converts a WordPress post/page to a WXR item
func (e *WordPressExporter) convertPostToItem(
	post models.WordPressPost,
	postType string,
	categoryMap map[int]models.WordPressCategory,
	tagMap map[int]models.WordPressTag,
	userMap map[int]models.WordPressUser,
) WXRItem {
	// Get author name
	authorName := ""
	if user, ok := userMap[post.Author]; ok {
		authorName = user.Slug
	}

	item := WXRItem{
		Title:          post.Title.Rendered,
		Link:           post.Link,
		PubDate:        post.Date.Format(time.RFC1123Z),
		Creator:        authorName,
		GUID:           WXRGUID{IsPermaLink: "false", Value: post.Link},
		Description:    "",
		ContentEncoded: wrapCDATA(post.Content.Rendered),
		ExcerptEncoded: wrapCDATA(post.Excerpt.Rendered),
		PostID:         post.ID,
		PostDate:       post.Date.Format("2006-01-02 15:04:05"),
		PostDateGMT:    post.DateGMT.Format("2006-01-02 15:04:05"),
		PostModified:   post.Modified.Format("2006-01-02 15:04:05"),
		CommentStatus:  post.CommentStatus,
		PingStatus:     post.PingStatus,
		PostName:       post.Slug,
		Status:         post.Status,
		PostParent:     0,
		MenuOrder:      0,
		PostType:       postType,
		PostPassword:   "",
		IsSticky:       boolToInt(post.Sticky),
	}

	// Add categories
	for _, catID := range post.Categories {
		if cat, ok := categoryMap[catID]; ok {
			item.Categories = append(item.Categories, WXRItemCategory{
				Domain:   "category",
				NiceName: cat.Slug,
				Value:    cat.Name,
			})
		}
	}

	// Add tags
	for _, tagID := range post.Tags {
		if tag, ok := tagMap[tagID]; ok {
			item.Categories = append(item.Categories, WXRItemCategory{
				Domain:   "post_tag",
				NiceName: tag.Slug,
				Value:    tag.Name,
			})
		}
	}

	// Add featured image as post meta
	if post.FeaturedMedia > 0 {
		item.PostMeta = append(item.PostMeta, WXRPostMeta{
			MetaKey:   "_thumbnail_id",
			MetaValue: fmt.Sprintf("%d", post.FeaturedMedia),
		})
	}

	// Add SEO meta if available
	if post.SEO.Title != "" {
		item.PostMeta = append(item.PostMeta, WXRPostMeta{
			MetaKey:   "_yoast_wpseo_title",
			MetaValue: post.SEO.Title,
		})
	}
	if post.SEO.MetaDescription != "" {
		item.PostMeta = append(item.PostMeta, WXRPostMeta{
			MetaKey:   "_yoast_wpseo_metadesc",
			MetaValue: post.SEO.MetaDescription,
		})
	}

	return item
}

// convertMediaToItem converts a WordPress media item to a WXR item
func (e *WordPressExporter) convertMediaToItem(
	media models.WordPressMedia,
	userMap map[int]models.WordPressUser,
) WXRItem {
	// Get author name
	authorName := ""
	if user, ok := userMap[media.Author]; ok {
		authorName = user.Slug
	}

	item := WXRItem{
		Title:          media.Title.Rendered,
		Link:           media.Link,
		PubDate:        media.Date.Format(time.RFC1123Z),
		Creator:        authorName,
		GUID:           WXRGUID{IsPermaLink: "false", Value: media.SourceURL},
		Description:    media.Description.Rendered,
		ContentEncoded: wrapCDATA(media.Caption.Rendered),
		ExcerptEncoded: wrapCDATA(media.Caption.Rendered),
		PostID:         media.ID,
		PostDate:       media.Date.Format("2006-01-02 15:04:05"),
		PostDateGMT:    media.DateGMT.Format("2006-01-02 15:04:05"),
		PostModified:   media.Modified.Format("2006-01-02 15:04:05"),
		CommentStatus:  "open",
		PingStatus:     "closed",
		PostName:       media.Slug,
		Status:         media.Status,
		PostParent:     media.Post,
		MenuOrder:      0,
		PostType:       "attachment",
		PostPassword:   "",
		IsSticky:       0,
		AttachmentURL:  media.SourceURL,
	}

	// Add attachment metadata
	item.PostMeta = append(item.PostMeta, WXRPostMeta{
		MetaKey:   "_wp_attached_file",
		MetaValue: extractFilePath(media.SourceURL),
	})

	return item
}

// wrapCDATA wraps content in CDATA if it contains HTML
func wrapCDATA(content string) string {
	if strings.Contains(content, "<") || strings.Contains(content, ">") ||
		strings.Contains(content, "&") {
		return "<![CDATA[" + content + "]]>"
	}
	return content
}

// boolToInt converts a boolean to int (0 or 1)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// extractFilePath extracts the file path from a full URL
func extractFilePath(urlStr string) string {
	// Extract path after /wp-content/uploads/
	if idx := strings.Index(urlStr, "/wp-content/uploads/"); idx != -1 {
		return urlStr[idx+len("/wp-content/uploads/"):]
	}
	// Fallback: return filename
	parts := strings.Split(urlStr, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return urlStr
}
