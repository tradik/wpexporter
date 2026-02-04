package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// WixExporter exports data in Wix-compatible JSON format
type WixExporter struct {
	config *config.Config
}

// NewWixExporter creates a new Wix exporter
func NewWixExporter(cfg *config.Config) *WixExporter {
	return &WixExporter{
		config: cfg,
	}
}

// WixExportData represents the complete Wix export structure
type WixExportData struct {
	Version    string        `json:"version"`
	Meta       WixMeta       `json:"meta"`
	Site       WixSite       `json:"site"`
	Posts      []WixPost     `json:"posts"`
	Pages      []WixPage     `json:"pages"`
	Media      []WixMedia    `json:"media"`
	Tags       []WixTag      `json:"tags"`
	Categories []WixCategory `json:"categories"`
}

// WixMeta contains export metadata
type WixMeta struct {
	Exporter   string    `json:"exporter"`
	ExportedAt time.Time `json:"exportedAt"`
	SourceURL  string    `json:"sourceUrl"`
}

// WixSite contains site information
type WixSite struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Language    string `json:"language"`
}

// WixPost represents a Wix blog post
type WixPost struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Content         string    `json:"content"`
	Excerpt         string    `json:"excerpt,omitempty"`
	CoverImage      string    `json:"coverImage,omitempty"`
	Featured        bool      `json:"featured"`
	Published       bool      `json:"published"`
	PublishedDate   time.Time `json:"publishedDate,omitempty"`
	LastUpdatedDate time.Time `json:"lastUpdatedDate"`
	Author          string    `json:"author,omitempty"`
	Tags            []string  `json:"tags,omitempty"`
	Categories      []string  `json:"categories,omitempty"`
	SEOTitle        string    `json:"seoTitle,omitempty"`
	SEODescription  string    `json:"seoDescription,omitempty"`
}

// WixPage represents a Wix page
type WixPage struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Content         string    `json:"content"`
	Published       bool      `json:"published"`
	LastUpdatedDate time.Time `json:"lastUpdatedDate"`
	SEOTitle        string    `json:"seoTitle,omitempty"`
	SEODescription  string    `json:"seoDescription,omitempty"`
}

// WixMedia represents a Wix media item
type WixMedia struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	MimeType string `json:"mimeType"`
	AltText  string `json:"altText,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// WixTag represents a Wix tag
type WixTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// WixCategory represents a Wix category
type WixCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parentId,omitempty"`
}

// Export exports data in Wix JSON format
func (e *WixExporter) Export(data *models.ExportData) error {
	wixData := e.buildWixData(data)

	jsonData, err := json.MarshalIndent(wixData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Wix JSON: %w", err)
	}

	var outputPath string
	if filepath.Ext(e.config.Output) == ".json" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "wix_export.json")
	}

	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write Wix export file: %w", err)
	}

	fmt.Printf("Wix export completed: %s\n", outputPath)
	return nil
}

// buildWixData constructs the complete Wix export structure
func (e *WixExporter) buildWixData(data *models.ExportData) WixExportData {
	wixData := WixExportData{
		Version: "1.4.0",
		Meta: WixMeta{
			Exporter:   "wpexporter",
			ExportedAt: time.Now(),
			SourceURL:  data.Site.URL,
		},
		Site: WixSite{
			Title:       data.Site.Name,
			Description: data.Site.Description,
			Language:    data.Site.Language,
		},
		Posts:      make([]WixPost, 0, len(data.Posts)),
		Pages:      make([]WixPage, 0, len(data.Pages)),
		Media:      make([]WixMedia, 0, len(data.Media)),
		Tags:       make([]WixTag, 0, len(data.Tags)),
		Categories: make([]WixCategory, 0, len(data.Categories)),
	}

	// Build lookup maps
	tagMap := make(map[int]string)
	for _, tag := range data.Tags {
		wixTag := e.convertTag(tag)
		wixData.Tags = append(wixData.Tags, wixTag)
		tagMap[tag.ID] = tag.Name
	}

	categoryMap := make(map[int]string)
	for _, cat := range data.Categories {
		wixCat := e.convertCategory(cat)
		wixData.Categories = append(wixData.Categories, wixCat)
		categoryMap[cat.ID] = cat.Name
	}

	userMap := make(map[int]string)
	for _, user := range data.Users {
		userMap[user.ID] = user.Name
	}

	mediaMap := make(map[int]string)
	for _, media := range data.Media {
		wixMedia := e.convertMedia(media)
		wixData.Media = append(wixData.Media, wixMedia)
		mediaMap[media.ID] = media.SourceURL
	}

	// Convert posts
	for _, post := range data.Posts {
		wixPost := e.convertPost(post, tagMap, categoryMap, userMap, mediaMap)
		wixData.Posts = append(wixData.Posts, wixPost)
	}

	// Convert pages
	for _, page := range data.Pages {
		wixPage := e.convertPage(page)
		wixData.Pages = append(wixData.Pages, wixPage)
	}

	return wixData
}

// convertPost converts a WordPress post to a Wix post
func (e *WixExporter) convertPost(post models.WordPressPost, tagMap, categoryMap, userMap map[int]string, mediaMap map[int]string) WixPost {
	wixPost := WixPost{
		ID:              fmt.Sprintf("%d", post.ID),
		Title:           post.Title.Rendered,
		Slug:            post.Slug,
		Content:         post.Content.Rendered,
		Excerpt:         stripHTMLTags(post.Excerpt.Rendered),
		Featured:        post.Sticky,
		Published:       post.Status == "publish",
		LastUpdatedDate: post.Modified.Time,
		SEOTitle:        post.SEO.Title,
		SEODescription:  post.SEO.MetaDescription,
	}

	if post.Status == "publish" {
		wixPost.PublishedDate = post.Date.Time
	}

	// Set author
	if author, ok := userMap[post.Author]; ok {
		wixPost.Author = author
	}

	// Set cover image
	if url, ok := mediaMap[post.FeaturedMedia]; ok {
		wixPost.CoverImage = url
	}

	// Set tags
	for _, tagID := range post.Tags {
		if name, ok := tagMap[tagID]; ok {
			wixPost.Tags = append(wixPost.Tags, name)
		}
	}

	// Set categories
	for _, catID := range post.Categories {
		if name, ok := categoryMap[catID]; ok {
			wixPost.Categories = append(wixPost.Categories, name)
		}
	}

	return wixPost
}

// convertPage converts a WordPress page to a Wix page
func (e *WixExporter) convertPage(page models.WordPressPost) WixPage {
	return WixPage{
		ID:              fmt.Sprintf("%d", page.ID),
		Title:           page.Title.Rendered,
		Slug:            page.Slug,
		Content:         page.Content.Rendered,
		Published:       page.Status == "publish",
		LastUpdatedDate: page.Modified.Time,
		SEOTitle:        page.SEO.Title,
		SEODescription:  page.SEO.MetaDescription,
	}
}

// convertTag converts a WordPress tag to a Wix tag
func (e *WixExporter) convertTag(tag models.WordPressTag) WixTag {
	return WixTag{
		ID:   fmt.Sprintf("%d", tag.ID),
		Name: tag.Name,
		Slug: tag.Slug,
	}
}

// convertCategory converts a WordPress category to a Wix category
func (e *WixExporter) convertCategory(cat models.WordPressCategory) WixCategory {
	wixCat := WixCategory{
		ID:          fmt.Sprintf("%d", cat.ID),
		Name:        cat.Name,
		Slug:        cat.Slug,
		Description: cat.Description,
	}
	if cat.Parent > 0 {
		wixCat.ParentID = fmt.Sprintf("%d", cat.Parent)
	}
	return wixCat
}

// convertMedia converts a WordPress media item to a Wix media
func (e *WixExporter) convertMedia(media models.WordPressMedia) WixMedia {
	width := 0
	height := 0
	if w, ok := media.MediaDetails.Width.(float64); ok {
		width = int(w)
	} else if w, ok := media.MediaDetails.Width.(int); ok {
		width = w
	}
	if h, ok := media.MediaDetails.Height.(float64); ok {
		height = int(h)
	} else if h, ok := media.MediaDetails.Height.(int); ok {
		height = h
	}

	return WixMedia{
		ID:       fmt.Sprintf("%d", media.ID),
		Title:    media.Title.Rendered,
		URL:      media.SourceURL,
		MimeType: media.MimeType,
		AltText:  media.AltText,
		Width:    width,
		Height:   height,
	}
}
