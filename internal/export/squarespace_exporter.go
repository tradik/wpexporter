package export

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// SquarespaceExporter exports data in Squarespace-compatible XML format
// Squarespace uses a modified WordPress XML (WXR) format for imports
type SquarespaceExporter struct {
	config *config.Config
}

// NewSquarespaceExporter creates a new Squarespace exporter
func NewSquarespaceExporter(cfg *config.Config) *SquarespaceExporter {
	return &SquarespaceExporter{
		config: cfg,
	}
}

// SquarespaceExport represents the root RSS element for Squarespace export
type SquarespaceExport struct {
	XMLName xml.Name           `xml:"rss"`
	Version string             `xml:"version,attr"`
	WP      string             `xml:"xmlns:wp,attr"`
	Excerpt string             `xml:"xmlns:excerpt,attr"`
	Content string             `xml:"xmlns:content,attr"`
	DC      string             `xml:"xmlns:dc,attr"`
	Channel SquarespaceChannel `xml:"channel"`
}

// SquarespaceChannel represents the channel element
type SquarespaceChannel struct {
	Title       string                `xml:"title"`
	Link        string                `xml:"link"`
	Description string                `xml:"description"`
	PubDate     string                `xml:"pubDate"`
	Language    string                `xml:"language"`
	WXRVersion  string                `xml:"wp:wxr_version"`
	BaseSiteURL string                `xml:"wp:base_site_url"`
	BaseBlogURL string                `xml:"wp:base_blog_url"`
	Categories  []SquarespaceCategory `xml:"wp:category"`
	Tags        []SquarespaceTag      `xml:"wp:tag"`
	Items       []SquarespaceItem     `xml:"item"`
}

// SquarespaceCategory represents a category
type SquarespaceCategory struct {
	TermID   int    `xml:"wp:term_id"`
	NiceName string `xml:"wp:category_nicename"`
	Parent   string `xml:"wp:category_parent"`
	Name     string `xml:"wp:cat_name"`
}

// SquarespaceTag represents a tag
type SquarespaceTag struct {
	TermID int    `xml:"wp:term_id"`
	Slug   string `xml:"wp:tag_slug"`
	Name   string `xml:"wp:tag_name"`
}

// SquarespaceItem represents a post, page, or attachment
type SquarespaceItem struct {
	Title          string                    `xml:"title"`
	Link           string                    `xml:"link"`
	PubDate        string                    `xml:"pubDate"`
	Creator        string                    `xml:"dc:creator"`
	GUID           SquarespaceGUID           `xml:"guid"`
	Description    string                    `xml:"description"`
	ContentEncoded string                    `xml:"content:encoded"`
	ExcerptEncoded string                    `xml:"excerpt:encoded"`
	PostID         int                       `xml:"wp:post_id"`
	PostDate       string                    `xml:"wp:post_date"`
	PostName       string                    `xml:"wp:post_name"`
	Status         string                    `xml:"wp:status"`
	PostType       string                    `xml:"wp:post_type"`
	AttachmentURL  string                    `xml:"wp:attachment_url,omitempty"`
	Categories     []SquarespaceItemCategory `xml:"category,omitempty"`
}

// SquarespaceGUID represents the GUID element
type SquarespaceGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// SquarespaceItemCategory represents a category/tag reference in an item
type SquarespaceItemCategory struct {
	Domain   string `xml:"domain,attr"`
	NiceName string `xml:"nicename,attr"`
	Value    string `xml:",cdata"`
}

// Export exports data in Squarespace XML format
func (e *SquarespaceExporter) Export(data *models.ExportData) error {
	sqData := e.buildSquarespaceData(data)

	output, err := xml.MarshalIndent(sqData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Squarespace XML: %w", err)
	}

	xmlContent := []byte(xml.Header + string(output))

	var outputPath string
	if filepath.Ext(e.config.Output) == ".xml" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "squarespace_export.xml")
	}

	if err := os.WriteFile(outputPath, xmlContent, 0600); err != nil {
		return fmt.Errorf("failed to write Squarespace file: %w", err)
	}

	fmt.Printf("Squarespace export completed: %s\n", outputPath)
	return nil
}

// buildSquarespaceData constructs the complete Squarespace export structure
func (e *SquarespaceExporter) buildSquarespaceData(data *models.ExportData) SquarespaceExport {
	sqExport := SquarespaceExport{
		Version: "2.0",
		WP:      "http://wordpress.org/export/1.2/",
		Excerpt: "http://wordpress.org/export/1.2/excerpt/",
		Content: "http://purl.org/rss/1.0/modules/content/",
		DC:      "http://purl.org/dc/elements/1.1/",
		Channel: SquarespaceChannel{
			Title:       data.Site.Name,
			Link:        data.Site.URL,
			Description: data.Site.Description,
			PubDate:     time.Now().Format(time.RFC1123Z),
			Language:    data.Site.Language,
			WXRVersion:  "1.2",
			BaseSiteURL: data.Site.URL,
			BaseBlogURL: data.Site.HomeURL,
		},
	}

	// Build category map
	categoryMap := make(map[int]string)
	for _, cat := range data.Categories {
		categoryMap[cat.ID] = cat.Slug
	}

	// Convert categories
	for _, cat := range data.Categories {
		parentSlug := ""
		if cat.Parent > 0 {
			parentSlug = categoryMap[cat.Parent]
		}
		sqExport.Channel.Categories = append(sqExport.Channel.Categories, SquarespaceCategory{
			TermID:   cat.ID,
			NiceName: cat.Slug,
			Parent:   parentSlug,
			Name:     cat.Name,
		})
	}

	// Convert tags
	for _, tag := range data.Tags {
		sqExport.Channel.Tags = append(sqExport.Channel.Tags, SquarespaceTag{
			TermID: tag.ID,
			Slug:   tag.Slug,
			Name:   tag.Name,
		})
	}

	// Build tag map
	tagMap := make(map[int]models.WordPressTag)
	for _, tag := range data.Tags {
		tagMap[tag.ID] = tag
	}

	// Build category name map
	catNameMap := make(map[int]models.WordPressCategory)
	for _, cat := range data.Categories {
		catNameMap[cat.ID] = cat
	}

	// Build user map
	userMap := make(map[int]string)
	for _, user := range data.Users {
		userMap[user.ID] = user.Slug
	}

	// Convert posts
	for _, post := range data.Posts {
		item := e.convertPostToItem(post, "post", catNameMap, tagMap, userMap)
		sqExport.Channel.Items = append(sqExport.Channel.Items, item)
	}

	// Convert pages
	for _, page := range data.Pages {
		item := e.convertPostToItem(page, "page", catNameMap, tagMap, userMap)
		sqExport.Channel.Items = append(sqExport.Channel.Items, item)
	}

	// Convert media
	for _, media := range data.Media {
		item := e.convertMediaToItem(media, userMap)
		sqExport.Channel.Items = append(sqExport.Channel.Items, item)
	}

	return sqExport
}

// convertPostToItem converts a WordPress post to a Squarespace item
func (e *SquarespaceExporter) convertPostToItem(
	post models.WordPressPost,
	postType string,
	categoryMap map[int]models.WordPressCategory,
	tagMap map[int]models.WordPressTag,
	userMap map[int]string,
) SquarespaceItem {
	creator := userMap[post.Author]
	if creator == "" {
		creator = "admin"
	}

	item := SquarespaceItem{
		Title:          post.Title.Rendered,
		Link:           post.Link,
		PubDate:        post.Date.Format(time.RFC1123Z),
		Creator:        creator,
		GUID:           SquarespaceGUID{IsPermaLink: "false", Value: post.Link},
		Description:    "",
		ContentEncoded: wrapCDATA(post.Content.Rendered),
		ExcerptEncoded: wrapCDATA(post.Excerpt.Rendered),
		PostID:         post.ID,
		PostDate:       post.Date.Format("2006-01-02 15:04:05"),
		PostName:       post.Slug,
		Status:         post.Status,
		PostType:       postType,
	}

	// Add categories
	for _, catID := range post.Categories {
		if cat, ok := categoryMap[catID]; ok {
			item.Categories = append(item.Categories, SquarespaceItemCategory{
				Domain:   "category",
				NiceName: cat.Slug,
				Value:    cat.Name,
			})
		}
	}

	// Add tags
	for _, tagID := range post.Tags {
		if tag, ok := tagMap[tagID]; ok {
			item.Categories = append(item.Categories, SquarespaceItemCategory{
				Domain:   "post_tag",
				NiceName: tag.Slug,
				Value:    tag.Name,
			})
		}
	}

	return item
}

// convertMediaToItem converts a WordPress media item to a Squarespace item
func (e *SquarespaceExporter) convertMediaToItem(media models.WordPressMedia, userMap map[int]string) SquarespaceItem {
	creator := userMap[media.Author]
	if creator == "" {
		creator = "admin"
	}

	return SquarespaceItem{
		Title:          media.Title.Rendered,
		Link:           media.Link,
		PubDate:        media.Date.Format(time.RFC1123Z),
		Creator:        creator,
		GUID:           SquarespaceGUID{IsPermaLink: "false", Value: media.SourceURL},
		Description:    media.Description.Rendered,
		ContentEncoded: wrapCDATA(media.Caption.Rendered),
		ExcerptEncoded: wrapCDATA(media.Caption.Rendered),
		PostID:         media.ID,
		PostDate:       media.Date.Format("2006-01-02 15:04:05"),
		PostName:       media.Slug,
		Status:         media.Status,
		PostType:       "attachment",
		AttachmentURL:  media.SourceURL,
	}
}
