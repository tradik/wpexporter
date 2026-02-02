package export

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// WeeblyExporter exports data in Weebly-compatible format
// Weebly can import WordPress XML (WXR) format
type WeeblyExporter struct {
	config *config.Config
}

// NewWeeblyExporter creates a new Weebly exporter
func NewWeeblyExporter(cfg *config.Config) *WeeblyExporter {
	return &WeeblyExporter{
		config: cfg,
	}
}

// WeeblyExport represents the root RSS element for Weebly export
type WeeblyExport struct {
	XMLName xml.Name      `xml:"rss"`
	Version string        `xml:"version,attr"`
	WP      string        `xml:"xmlns:wp,attr"`
	Content string        `xml:"xmlns:content,attr"`
	DC      string        `xml:"xmlns:dc,attr"`
	Channel WeeblyChannel `xml:"channel"`
}

// WeeblyChannel represents the channel element
type WeeblyChannel struct {
	Title       string       `xml:"title"`
	Link        string       `xml:"link"`
	Description string       `xml:"description"`
	PubDate     string       `xml:"pubDate"`
	Language    string       `xml:"language"`
	Items       []WeeblyItem `xml:"item"`
}

// WeeblyItem represents a post or page
type WeeblyItem struct {
	Title          string     `xml:"title"`
	Link           string     `xml:"link"`
	PubDate        string     `xml:"pubDate"`
	Creator        string     `xml:"dc:creator"`
	GUID           WeeblyGUID `xml:"guid"`
	ContentEncoded string     `xml:"content:encoded"`
	PostID         int        `xml:"wp:post_id"`
	PostDate       string     `xml:"wp:post_date"`
	PostName       string     `xml:"wp:post_name"`
	Status         string     `xml:"wp:status"`
	PostType       string     `xml:"wp:post_type"`
}

// WeeblyGUID represents the GUID element
type WeeblyGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// WeeblyJSONExport represents JSON export for Weebly
type WeeblyJSONExport struct {
	Version string       `json:"version"`
	Meta    WeeblyMeta   `json:"meta"`
	Posts   []WeeblyPost `json:"posts"`
	Pages   []WeeblyPage `json:"pages"`
}

// WeeblyMeta contains export metadata
type WeeblyMeta struct {
	Exporter   string    `json:"exporter"`
	ExportedAt time.Time `json:"exportedAt"`
	SourceSite string    `json:"sourceSite"`
	SiteName   string    `json:"siteName"`
}

// WeeblyPost represents a blog post for JSON export
type WeeblyPost struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	Slug          string    `json:"slug"`
	Content       string    `json:"content"`
	Excerpt       string    `json:"excerpt,omitempty"`
	Author        string    `json:"author,omitempty"`
	Published     bool      `json:"published"`
	PublishedAt   time.Time `json:"publishedAt,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt"`
	FeaturedImage string    `json:"featuredImage,omitempty"`
	Categories    []string  `json:"categories,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
}

// WeeblyPage represents a page for JSON export
type WeeblyPage struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Content   string    `json:"content"`
	Published bool      `json:"published"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Export exports data in Weebly format (both XML and JSON)
func (e *WeeblyExporter) Export(data *models.ExportData) error {
	baseDir := e.config.Output
	if filepath.Ext(baseDir) == ".xml" || filepath.Ext(baseDir) == ".json" {
		baseDir = filepath.Dir(baseDir)
	}

	// Export XML format (primary)
	if err := e.exportXML(data, baseDir); err != nil {
		return fmt.Errorf("failed to export XML: %w", err)
	}

	// Export JSON format (alternative)
	if err := e.exportJSON(data, baseDir); err != nil {
		return fmt.Errorf("failed to export JSON: %w", err)
	}

	fmt.Printf("Weebly export completed: %s\n", baseDir)
	return nil
}

// exportXML exports data in WordPress XML format for Weebly import
func (e *WeeblyExporter) exportXML(data *models.ExportData, baseDir string) error {
	weeblyExport := WeeblyExport{
		Version: "2.0",
		WP:      "http://wordpress.org/export/1.2/",
		Content: "http://purl.org/rss/1.0/modules/content/",
		DC:      "http://purl.org/dc/elements/1.1/",
		Channel: WeeblyChannel{
			Title:       data.Site.Name,
			Link:        data.Site.URL,
			Description: data.Site.Description,
			PubDate:     time.Now().Format(time.RFC1123Z),
			Language:    data.Site.Language,
		},
	}

	// Build user map
	userMap := make(map[int]string)
	for _, user := range data.Users {
		userMap[user.ID] = user.Slug
	}

	// Convert posts
	for _, post := range data.Posts {
		item := e.convertPostToItem(post, "post", userMap)
		weeblyExport.Channel.Items = append(weeblyExport.Channel.Items, item)
	}

	// Convert pages
	for _, page := range data.Pages {
		item := e.convertPostToItem(page, "page", userMap)
		weeblyExport.Channel.Items = append(weeblyExport.Channel.Items, item)
	}

	output, err := xml.MarshalIndent(weeblyExport, "", "  ")
	if err != nil {
		return err
	}

	xmlContent := []byte(xml.Header + string(output))
	outputPath := filepath.Join(baseDir, "weebly_export.xml")
	return os.WriteFile(outputPath, xmlContent, 0600)
}

// exportJSON exports data in JSON format for alternative import
func (e *WeeblyExporter) exportJSON(data *models.ExportData, baseDir string) error {
	jsonExport := WeeblyJSONExport{
		Version: "1.3.5",
		Meta: WeeblyMeta{
			Exporter:   "wpexporter",
			ExportedAt: time.Now(),
			SourceSite: data.Site.URL,
			SiteName:   data.Site.Name,
		},
		Posts: make([]WeeblyPost, 0, len(data.Posts)),
		Pages: make([]WeeblyPage, 0, len(data.Pages)),
	}

	// Build lookup maps
	tagMap := make(map[int]string)
	for _, tag := range data.Tags {
		tagMap[tag.ID] = tag.Name
	}

	categoryMap := make(map[int]string)
	for _, cat := range data.Categories {
		categoryMap[cat.ID] = cat.Name
	}

	userMap := make(map[int]string)
	for _, user := range data.Users {
		userMap[user.ID] = user.Name
	}

	mediaMap := make(map[int]string)
	for _, media := range data.Media {
		mediaMap[media.ID] = media.SourceURL
	}

	// Convert posts
	for _, post := range data.Posts {
		weeblyPost := WeeblyPost{
			ID:        post.ID,
			Title:     post.Title.Rendered,
			Slug:      post.Slug,
			Content:   post.Content.Rendered,
			Excerpt:   stripHTMLTags(post.Excerpt.Rendered),
			Author:    userMap[post.Author],
			Published: post.Status == "publish",
			UpdatedAt: post.Modified.Time,
		}

		if post.Status == "publish" {
			weeblyPost.PublishedAt = post.Date.Time
		}

		if url, ok := mediaMap[post.FeaturedMedia]; ok {
			weeblyPost.FeaturedImage = url
		}

		for _, tagID := range post.Tags {
			if name, ok := tagMap[tagID]; ok {
				weeblyPost.Tags = append(weeblyPost.Tags, name)
			}
		}

		for _, catID := range post.Categories {
			if name, ok := categoryMap[catID]; ok {
				weeblyPost.Categories = append(weeblyPost.Categories, name)
			}
		}

		jsonExport.Posts = append(jsonExport.Posts, weeblyPost)
	}

	// Convert pages
	for _, page := range data.Pages {
		weeblyPage := WeeblyPage{
			ID:        page.ID,
			Title:     page.Title.Rendered,
			Slug:      page.Slug,
			Content:   page.Content.Rendered,
			Published: page.Status == "publish",
			UpdatedAt: page.Modified.Time,
		}
		jsonExport.Pages = append(jsonExport.Pages, weeblyPage)
	}

	jsonData, err := json.MarshalIndent(jsonExport, "", "  ")
	if err != nil {
		return err
	}

	outputPath := filepath.Join(baseDir, "weebly_export.json")
	return os.WriteFile(outputPath, jsonData, 0600)
}

// convertPostToItem converts a WordPress post to a Weebly XML item
func (e *WeeblyExporter) convertPostToItem(post models.WordPressPost, postType string, userMap map[int]string) WeeblyItem {
	creator := userMap[post.Author]
	if creator == "" {
		creator = "admin"
	}

	return WeeblyItem{
		Title:          post.Title.Rendered,
		Link:           post.Link,
		PubDate:        post.Date.Format(time.RFC1123Z),
		Creator:        creator,
		GUID:           WeeblyGUID{IsPermaLink: "false", Value: post.Link},
		ContentEncoded: wrapCDATA(post.Content.Rendered),
		PostID:         post.ID,
		PostDate:       post.Date.Format("2006-01-02 15:04:05"),
		PostName:       post.Slug,
		Status:         post.Status,
		PostType:       postType,
	}
}
