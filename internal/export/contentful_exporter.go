package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// ContentfulExporter exports data in Contentful CMS JSON format
type ContentfulExporter struct {
	config *config.Config
}

// NewContentfulExporter creates a new Contentful exporter
func NewContentfulExporter(cfg *config.Config) *ContentfulExporter {
	return &ContentfulExporter{
		config: cfg,
	}
}

// ContentfulExportData represents the complete Contentful export structure
type ContentfulExportData struct {
	ContentTypes []ContentfulContentType `json:"contentTypes"`
	Entries      []ContentfulEntry       `json:"entries"`
	Assets       []ContentfulAsset       `json:"assets"`
	Locales      []ContentfulLocale      `json:"locales"`
}

// ContentfulContentType represents a content type definition
type ContentfulContentType struct {
	Sys          ContentfulSys     `json:"sys"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	DisplayField string            `json:"displayField"`
	Fields       []ContentfulField `json:"fields"`
}

// ContentfulField represents a field definition
type ContentfulField struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Type      string                `json:"type"`
	Required  bool                  `json:"required"`
	Localized bool                  `json:"localized"`
	LinkType  string                `json:"linkType,omitempty"`
	Items     *ContentfulFieldItems `json:"items,omitempty"`
}

// ContentfulFieldItems represents array field items
type ContentfulFieldItems struct {
	Type     string `json:"type"`
	LinkType string `json:"linkType,omitempty"`
}

// ContentfulEntry represents a content entry
type ContentfulEntry struct {
	Sys    ContentfulSys                     `json:"sys"`
	Fields map[string]map[string]interface{} `json:"fields"`
}

// ContentfulAsset represents a media asset
type ContentfulAsset struct {
	Sys    ContentfulSys                     `json:"sys"`
	Fields map[string]map[string]interface{} `json:"fields"`
}

// ContentfulSys represents system metadata
type ContentfulSys struct {
	Space       *ContentfulLink `json:"space,omitempty"`
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	ContentType *ContentfulLink `json:"contentType,omitempty"`
	CreatedAt   string          `json:"createdAt,omitempty"`
	UpdatedAt   string          `json:"updatedAt,omitempty"`
	PublishedAt string          `json:"publishedAt,omitempty"`
	Version     int             `json:"version,omitempty"`
}

// ContentfulLink represents a link to another entity
type ContentfulLink struct {
	Sys ContentfulLinkSys `json:"sys"`
}

// ContentfulLinkSys represents link system metadata
type ContentfulLinkSys struct {
	Type     string `json:"type"`
	LinkType string `json:"linkType"`
	ID       string `json:"id"`
}

// ContentfulLocale represents a locale configuration
type ContentfulLocale struct {
	Name         string `json:"name"`
	Code         string `json:"code"`
	FallbackCode string `json:"fallbackCode,omitempty"`
	Default      bool   `json:"default"`
}

// Export exports data in Contentful JSON format
func (e *ContentfulExporter) Export(data *models.ExportData) error {
	contentfulData := e.buildContentfulData(data)

	jsonData, err := json.MarshalIndent(contentfulData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Contentful JSON: %w", err)
	}

	var outputPath string
	if filepath.Ext(e.config.Output) == ".json" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "contentful_export.json")
	}

	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write Contentful export file: %w", err)
	}

	fmt.Printf("Contentful export completed: %s\n", outputPath)
	return nil
}

// buildContentfulData constructs the complete Contentful export structure
func (e *ContentfulExporter) buildContentfulData(data *models.ExportData) ContentfulExportData {
	contentfulData := ContentfulExportData{
		ContentTypes: e.buildContentTypes(),
		Entries:      make([]ContentfulEntry, 0),
		Assets:       make([]ContentfulAsset, 0),
		Locales: []ContentfulLocale{
			{
				Name:    "English (United States)",
				Code:    "en-US",
				Default: true,
			},
		},
	}

	locale := "en-US"

	// Convert categories
	for _, cat := range data.Categories {
		entry := e.convertCategoryToEntry(cat, locale)
		contentfulData.Entries = append(contentfulData.Entries, entry)
	}

	// Convert tags
	for _, tag := range data.Tags {
		entry := e.convertTagToEntry(tag, locale)
		contentfulData.Entries = append(contentfulData.Entries, entry)
	}

	// Convert authors
	for _, user := range data.Users {
		entry := e.convertAuthorToEntry(user, locale)
		contentfulData.Entries = append(contentfulData.Entries, entry)
	}

	// Convert media to assets
	for _, media := range data.Media {
		asset := e.convertMediaToAsset(media, locale)
		contentfulData.Assets = append(contentfulData.Assets, asset)
	}

	// Convert posts as articles
	for _, post := range data.Posts {
		entry := e.convertPostToEntry(post, "blogPost", locale)
		contentfulData.Entries = append(contentfulData.Entries, entry)
	}

	// Convert pages
	for _, page := range data.Pages {
		entry := e.convertPostToEntry(page, "page", locale)
		contentfulData.Entries = append(contentfulData.Entries, entry)
	}

	return contentfulData
}

// buildContentTypes creates the content type definitions
func (e *ContentfulExporter) buildContentTypes() []ContentfulContentType {
	return []ContentfulContentType{
		{
			Sys:          ContentfulSys{ID: "blogPost", Type: "ContentType"},
			Name:         "Blog Post",
			Description:  "Blog post imported from WordPress",
			DisplayField: "title",
			Fields: []ContentfulField{
				{ID: "title", Name: "Title", Type: "Symbol", Required: true, Localized: true},
				{ID: "slug", Name: "Slug", Type: "Symbol", Required: true, Localized: false},
				{ID: "content", Name: "Content", Type: "Text", Required: false, Localized: true},
				{ID: "excerpt", Name: "Excerpt", Type: "Text", Required: false, Localized: true},
				{ID: "featuredImage", Name: "Featured Image", Type: "Link", LinkType: "Asset", Required: false, Localized: false},
				{ID: "author", Name: "Author", Type: "Link", LinkType: "Entry", Required: false, Localized: false},
				{ID: "categories", Name: "Categories", Type: "Array", Items: &ContentfulFieldItems{Type: "Link", LinkType: "Entry"}},
				{ID: "tags", Name: "Tags", Type: "Array", Items: &ContentfulFieldItems{Type: "Link", LinkType: "Entry"}},
				{ID: "publishedDate", Name: "Published Date", Type: "Date", Required: false, Localized: false},
				{ID: "seoTitle", Name: "SEO Title", Type: "Symbol", Required: false, Localized: true},
				{ID: "seoDescription", Name: "SEO Description", Type: "Text", Required: false, Localized: true},
			},
		},
		{
			Sys:          ContentfulSys{ID: "page", Type: "ContentType"},
			Name:         "Page",
			Description:  "Page imported from WordPress",
			DisplayField: "title",
			Fields: []ContentfulField{
				{ID: "title", Name: "Title", Type: "Symbol", Required: true, Localized: true},
				{ID: "slug", Name: "Slug", Type: "Symbol", Required: true, Localized: false},
				{ID: "content", Name: "Content", Type: "Text", Required: false, Localized: true},
				{ID: "seoTitle", Name: "SEO Title", Type: "Symbol", Required: false, Localized: true},
				{ID: "seoDescription", Name: "SEO Description", Type: "Text", Required: false, Localized: true},
			},
		},
		{
			Sys:          ContentfulSys{ID: "category", Type: "ContentType"},
			Name:         "Category",
			Description:  "Category imported from WordPress",
			DisplayField: "name",
			Fields: []ContentfulField{
				{ID: "name", Name: "Name", Type: "Symbol", Required: true, Localized: true},
				{ID: "slug", Name: "Slug", Type: "Symbol", Required: true, Localized: false},
				{ID: "description", Name: "Description", Type: "Text", Required: false, Localized: true},
				{ID: "parent", Name: "Parent Category", Type: "Link", LinkType: "Entry", Required: false, Localized: false},
			},
		},
		{
			Sys:          ContentfulSys{ID: "tag", Type: "ContentType"},
			Name:         "Tag",
			Description:  "Tag imported from WordPress",
			DisplayField: "name",
			Fields: []ContentfulField{
				{ID: "name", Name: "Name", Type: "Symbol", Required: true, Localized: true},
				{ID: "slug", Name: "Slug", Type: "Symbol", Required: true, Localized: false},
			},
		},
		{
			Sys:          ContentfulSys{ID: "author", Type: "ContentType"},
			Name:         "Author",
			Description:  "Author imported from WordPress",
			DisplayField: "name",
			Fields: []ContentfulField{
				{ID: "name", Name: "Name", Type: "Symbol", Required: true, Localized: false},
				{ID: "slug", Name: "Slug", Type: "Symbol", Required: true, Localized: false},
				{ID: "bio", Name: "Bio", Type: "Text", Required: false, Localized: true},
				{ID: "avatar", Name: "Avatar", Type: "Link", LinkType: "Asset", Required: false, Localized: false},
			},
		},
	}
}

// convertPostToEntry converts a WordPress post to a Contentful entry
func (e *ContentfulExporter) convertPostToEntry(post models.WordPressPost, contentType, locale string) ContentfulEntry {
	entry := ContentfulEntry{
		Sys: ContentfulSys{
			ID:          fmt.Sprintf("%s-%d", contentType, post.ID),
			Type:        "Entry",
			ContentType: &ContentfulLink{Sys: ContentfulLinkSys{Type: "Link", LinkType: "ContentType", ID: contentType}},
			CreatedAt:   post.Date.Format(time.RFC3339),
			UpdatedAt:   post.Modified.Format(time.RFC3339),
		},
		Fields: make(map[string]map[string]interface{}),
	}

	if post.Status == statusPublish {
		entry.Sys.PublishedAt = post.Date.Format(time.RFC3339)
	}

	// Set fields with locale
	entry.Fields["title"] = map[string]interface{}{locale: post.Title.Rendered}
	entry.Fields["slug"] = map[string]interface{}{locale: post.Slug}
	entry.Fields["content"] = map[string]interface{}{locale: post.Content.Rendered}

	if post.Excerpt.Rendered != "" {
		entry.Fields["excerpt"] = map[string]interface{}{locale: stripHTMLTags(post.Excerpt.Rendered)}
	}

	// Add featured image link
	if post.FeaturedMedia > 0 {
		entry.Fields["featuredImage"] = map[string]interface{}{
			locale: ContentfulLink{
				Sys: ContentfulLinkSys{Type: "Link", LinkType: "Asset", ID: fmt.Sprintf("asset-%d", post.FeaturedMedia)},
			},
		}
	}

	// Add author link
	if post.Author > 0 {
		entry.Fields["author"] = map[string]interface{}{
			locale: ContentfulLink{
				Sys: ContentfulLinkSys{Type: "Link", LinkType: "Entry", ID: fmt.Sprintf("author-%d", post.Author)},
			},
		}
	}

	// Add category links
	if len(post.Categories) > 0 {
		catLinks := make([]ContentfulLink, 0, len(post.Categories))
		for _, catID := range post.Categories {
			catLinks = append(catLinks, ContentfulLink{
				Sys: ContentfulLinkSys{Type: "Link", LinkType: "Entry", ID: fmt.Sprintf("category-%d", catID)},
			})
		}
		entry.Fields["categories"] = map[string]interface{}{locale: catLinks}
	}

	// Add tag links
	if len(post.Tags) > 0 {
		tagLinks := make([]ContentfulLink, 0, len(post.Tags))
		for _, tagID := range post.Tags {
			tagLinks = append(tagLinks, ContentfulLink{
				Sys: ContentfulLinkSys{Type: "Link", LinkType: "Entry", ID: fmt.Sprintf("tag-%d", tagID)},
			})
		}
		entry.Fields["tags"] = map[string]interface{}{locale: tagLinks}
	}

	// Add published date
	entry.Fields["publishedDate"] = map[string]interface{}{locale: post.Date.Format(time.RFC3339)}

	// Add SEO fields
	if post.SEO.Title != "" {
		entry.Fields["seoTitle"] = map[string]interface{}{locale: post.SEO.Title}
	}
	if post.SEO.MetaDescription != "" {
		entry.Fields["seoDescription"] = map[string]interface{}{locale: post.SEO.MetaDescription}
	}

	return entry
}

// convertCategoryToEntry converts a WordPress category to a Contentful entry
func (e *ContentfulExporter) convertCategoryToEntry(cat models.WordPressCategory, locale string) ContentfulEntry {
	entry := ContentfulEntry{
		Sys: ContentfulSys{
			ID:          fmt.Sprintf("category-%d", cat.ID),
			Type:        "Entry",
			ContentType: &ContentfulLink{Sys: ContentfulLinkSys{Type: "Link", LinkType: "ContentType", ID: "category"}},
		},
		Fields: make(map[string]map[string]interface{}),
	}

	entry.Fields["name"] = map[string]interface{}{locale: cat.Name}
	entry.Fields["slug"] = map[string]interface{}{locale: cat.Slug}
	if cat.Description != "" {
		entry.Fields["description"] = map[string]interface{}{locale: cat.Description}
	}
	if cat.Parent > 0 {
		entry.Fields["parent"] = map[string]interface{}{
			locale: ContentfulLink{
				Sys: ContentfulLinkSys{Type: "Link", LinkType: "Entry", ID: fmt.Sprintf("category-%d", cat.Parent)},
			},
		}
	}

	return entry
}

// convertTagToEntry converts a WordPress tag to a Contentful entry
func (e *ContentfulExporter) convertTagToEntry(tag models.WordPressTag, locale string) ContentfulEntry {
	entry := ContentfulEntry{
		Sys: ContentfulSys{
			ID:          fmt.Sprintf("tag-%d", tag.ID),
			Type:        "Entry",
			ContentType: &ContentfulLink{Sys: ContentfulLinkSys{Type: "Link", LinkType: "ContentType", ID: "tag"}},
		},
		Fields: make(map[string]map[string]interface{}),
	}

	entry.Fields["name"] = map[string]interface{}{locale: tag.Name}
	entry.Fields["slug"] = map[string]interface{}{locale: tag.Slug}

	return entry
}

// convertAuthorToEntry converts a WordPress user to a Contentful author entry
func (e *ContentfulExporter) convertAuthorToEntry(user models.WordPressUser, locale string) ContentfulEntry {
	entry := ContentfulEntry{
		Sys: ContentfulSys{
			ID:          fmt.Sprintf("author-%d", user.ID),
			Type:        "Entry",
			ContentType: &ContentfulLink{Sys: ContentfulLinkSys{Type: "Link", LinkType: "ContentType", ID: "author"}},
		},
		Fields: make(map[string]map[string]interface{}),
	}

	entry.Fields["name"] = map[string]interface{}{locale: user.Name}
	entry.Fields["slug"] = map[string]interface{}{locale: user.Slug}
	if user.Description != "" {
		entry.Fields["bio"] = map[string]interface{}{locale: user.Description}
	}

	return entry
}

// convertMediaToAsset converts a WordPress media item to a Contentful asset
func (e *ContentfulExporter) convertMediaToAsset(media models.WordPressMedia, locale string) ContentfulAsset {
	asset := ContentfulAsset{
		Sys: ContentfulSys{
			ID:        fmt.Sprintf("asset-%d", media.ID),
			Type:      "Asset",
			CreatedAt: media.Date.Format(time.RFC3339),
			UpdatedAt: media.Modified.Format(time.RFC3339),
		},
		Fields: make(map[string]map[string]interface{}),
	}

	asset.Fields["title"] = map[string]interface{}{locale: media.Title.Rendered}
	if media.AltText != "" {
		asset.Fields["description"] = map[string]interface{}{locale: media.AltText}
	}

	// File field
	fileName := media.Slug
	if idx := strings.LastIndex(media.SourceURL, "/"); idx != -1 {
		fileName = media.SourceURL[idx+1:]
	}

	asset.Fields["file"] = map[string]interface{}{
		locale: map[string]interface{}{
			"url":         media.SourceURL,
			"fileName":    fileName,
			"contentType": media.MimeType,
		},
	}

	return asset
}
