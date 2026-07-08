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

// StrapiExporter exports data in Strapi CMS JSON format
type StrapiExporter struct {
	config *config.Config
}

// NewStrapiExporter creates a new Strapi exporter
func NewStrapiExporter(cfg *config.Config) *StrapiExporter {
	return &StrapiExporter{
		config: cfg,
	}
}

// StrapiExportData represents the complete Strapi export structure
type StrapiExportData struct {
	Version    string           `json:"version"`
	Meta       StrapiMeta       `json:"meta"`
	Articles   []StrapiArticle  `json:"articles"`
	Pages      []StrapiPage     `json:"pages"`
	Categories []StrapiCategory `json:"categories"`
	Tags       []StrapiTag      `json:"tags"`
	Authors    []StrapiAuthor   `json:"authors"`
	Media      []StrapiMedia    `json:"media"`
}

// StrapiMeta contains export metadata
type StrapiMeta struct {
	Exporter   string       `json:"exporter"`
	ExportedAt time.Time    `json:"exportedAt"`
	SourceSite string       `json:"sourceSite"`
	SourceName string       `json:"sourceName"`
	Counts     StrapiCounts `json:"counts"`
}

// StrapiCounts contains content counts
type StrapiCounts struct {
	Articles   int `json:"articles"`
	Pages      int `json:"pages"`
	Categories int `json:"categories"`
	Tags       int `json:"tags"`
	Authors    int `json:"authors"`
	Media      int `json:"media"`
}

// StrapiArticle represents a Strapi article
type StrapiArticle struct {
	ID            int              `json:"id"`
	Title         string           `json:"title"`
	Slug          string           `json:"slug"`
	Content       string           `json:"content"`
	Excerpt       string           `json:"excerpt,omitempty"`
	PublishedAt   *time.Time       `json:"publishedAt,omitempty"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	Status        string           `json:"status"`
	Author        *StrapiRelation  `json:"author,omitempty"`
	Categories    []StrapiRelation `json:"categories,omitempty"`
	Tags          []StrapiRelation `json:"tags,omitempty"`
	FeaturedImage *StrapiRelation  `json:"featuredImage,omitempty"`
	SEO           *StrapiSEO       `json:"seo,omitempty"`
}

// StrapiPage represents a Strapi page
type StrapiPage struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Content     string     `json:"content"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Status      string     `json:"status"`
	SEO         *StrapiSEO `json:"seo,omitempty"`
}

// StrapiCategory represents a Strapi category
type StrapiCategory struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	ParentID    *int   `json:"parentId,omitempty"`
}

// StrapiTag represents a Strapi tag
type StrapiTag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// StrapiAuthor represents a Strapi author/user
type StrapiAuthor struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Email  string `json:"email,omitempty"`
	Bio    string `json:"bio,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

// StrapiMedia represents a Strapi media file
type StrapiMedia struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	AlternativeText string    `json:"alternativeText,omitempty"`
	Caption         string    `json:"caption,omitempty"`
	URL             string    `json:"url"`
	Mime            string    `json:"mime"`
	Width           int       `json:"width,omitempty"`
	Height          int       `json:"height,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// StrapiRelation represents a relation to another entity
type StrapiRelation struct {
	ID int `json:"id"`
}

// StrapiSEO represents SEO metadata
type StrapiSEO struct {
	MetaTitle       string `json:"metaTitle,omitempty"`
	MetaDescription string `json:"metaDescription,omitempty"`
	Keywords        string `json:"keywords,omitempty"`
	CanonicalURL    string `json:"canonicalURL,omitempty"`
	OGTitle         string `json:"ogTitle,omitempty"`
	OGDescription   string `json:"ogDescription,omitempty"`
	OGImage         string `json:"ogImage,omitempty"`
}

// Export exports data in Strapi JSON format
func (e *StrapiExporter) Export(data *models.ExportData) error {
	strapiData := e.buildStrapiData(data)

	jsonData, err := json.MarshalIndent(strapiData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Strapi JSON: %w", err)
	}

	var outputPath string
	if filepath.Ext(e.config.Output) == ".json" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "strapi_export.json")
	}

	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write Strapi export file: %w", err)
	}

	// Also export separate files for each content type
	if err := e.exportSeparateFiles(strapiData); err != nil {
		return fmt.Errorf("failed to write separate Strapi files: %w", err)
	}

	fmt.Printf("Strapi export completed: %s\n", outputPath)
	return nil
}

// buildStrapiData constructs the complete Strapi export structure
func (e *StrapiExporter) buildStrapiData(data *models.ExportData) StrapiExportData {
	strapiData := StrapiExportData{
		Version: "1.4.0",
		Meta: StrapiMeta{
			Exporter:   "wpexporter",
			ExportedAt: time.Now(),
			SourceSite: data.Site.URL,
			SourceName: data.Site.Name,
		},
		Articles:   make([]StrapiArticle, 0, len(data.Posts)),
		Pages:      make([]StrapiPage, 0, len(data.Pages)),
		Categories: make([]StrapiCategory, 0, len(data.Categories)),
		Tags:       make([]StrapiTag, 0, len(data.Tags)),
		Authors:    make([]StrapiAuthor, 0, len(data.Users)),
		Media:      make([]StrapiMedia, 0, len(data.Media)),
	}

	// Convert categories
	for _, cat := range data.Categories {
		strapiData.Categories = append(strapiData.Categories, e.convertCategory(cat))
	}

	// Convert tags
	for _, tag := range data.Tags {
		strapiData.Tags = append(strapiData.Tags, e.convertTag(tag))
	}

	// Convert authors
	for _, user := range data.Users {
		strapiData.Authors = append(strapiData.Authors, e.convertAuthor(user))
	}

	// Convert media
	for _, media := range data.Media {
		strapiData.Media = append(strapiData.Media, e.convertMedia(media))
	}

	// Convert posts as articles
	for _, post := range data.Posts {
		strapiData.Articles = append(strapiData.Articles, e.convertArticle(post))
	}

	// Convert pages
	for _, page := range data.Pages {
		strapiData.Pages = append(strapiData.Pages, e.convertPage(page))
	}

	// Update counts
	strapiData.Meta.Counts = StrapiCounts{
		Articles:   len(strapiData.Articles),
		Pages:      len(strapiData.Pages),
		Categories: len(strapiData.Categories),
		Tags:       len(strapiData.Tags),
		Authors:    len(strapiData.Authors),
		Media:      len(strapiData.Media),
	}

	return strapiData
}

// convertArticle converts a WordPress post to a Strapi article
func (e *StrapiExporter) convertArticle(post models.WordPressPost) StrapiArticle {
	status := "draft"
	var publishedAt *time.Time
	if post.Status == statusPublish {
		status = "published"
		t := post.Date.Time
		publishedAt = &t
	}

	article := StrapiArticle{
		ID:          post.ID,
		Title:       post.Title.Rendered,
		Slug:        post.Slug,
		Content:     post.Content.Rendered,
		Excerpt:     stripHTMLTags(post.Excerpt.Rendered),
		PublishedAt: publishedAt,
		CreatedAt:   post.Date.Time,
		UpdatedAt:   post.Modified.Time,
		Status:      status,
	}

	// Add author relation
	if post.Author > 0 {
		article.Author = &StrapiRelation{ID: post.Author}
	}

	// Add category relations
	for _, catID := range post.Categories {
		article.Categories = append(article.Categories, StrapiRelation{ID: catID})
	}

	// Add tag relations
	for _, tagID := range post.Tags {
		article.Tags = append(article.Tags, StrapiRelation{ID: tagID})
	}

	// Add featured image relation
	if post.FeaturedMedia > 0 {
		article.FeaturedImage = &StrapiRelation{ID: post.FeaturedMedia}
	}

	// Add SEO data
	if post.SEO.Title != "" || post.SEO.MetaDescription != "" {
		article.SEO = &StrapiSEO{
			MetaTitle:       post.SEO.Title,
			MetaDescription: post.SEO.MetaDescription,
			Keywords:        post.SEO.MetaKeywords,
			CanonicalURL:    post.SEO.CanonicalURL,
			OGTitle:         post.SEO.OGTitle,
			OGDescription:   post.SEO.OGDescription,
			OGImage:         post.SEO.OGImage,
		}
	}

	return article
}

// convertPage converts a WordPress page to a Strapi page
func (e *StrapiExporter) convertPage(page models.WordPressPost) StrapiPage {
	status := "draft"
	var publishedAt *time.Time
	if page.Status == statusPublish {
		status = "published"
		t := page.Date.Time
		publishedAt = &t
	}

	strapiPage := StrapiPage{
		ID:          page.ID,
		Title:       page.Title.Rendered,
		Slug:        page.Slug,
		Content:     page.Content.Rendered,
		PublishedAt: publishedAt,
		CreatedAt:   page.Date.Time,
		UpdatedAt:   page.Modified.Time,
		Status:      status,
	}

	// Add SEO data
	if page.SEO.Title != "" || page.SEO.MetaDescription != "" {
		strapiPage.SEO = &StrapiSEO{
			MetaTitle:       page.SEO.Title,
			MetaDescription: page.SEO.MetaDescription,
			Keywords:        page.SEO.MetaKeywords,
			CanonicalURL:    page.SEO.CanonicalURL,
		}
	}

	return strapiPage
}

// convertCategory converts a WordPress category to a Strapi category
func (e *StrapiExporter) convertCategory(cat models.WordPressCategory) StrapiCategory {
	strapiCat := StrapiCategory{
		ID:          cat.ID,
		Name:        cat.Name,
		Slug:        cat.Slug,
		Description: cat.Description,
	}
	if cat.Parent > 0 {
		strapiCat.ParentID = &cat.Parent
	}
	return strapiCat
}

// convertTag converts a WordPress tag to a Strapi tag
func (e *StrapiExporter) convertTag(tag models.WordPressTag) StrapiTag {
	return StrapiTag{
		ID:   tag.ID,
		Name: tag.Name,
		Slug: tag.Slug,
	}
}

// convertAuthor converts a WordPress user to a Strapi author
func (e *StrapiExporter) convertAuthor(user models.WordPressUser) StrapiAuthor {
	avatar := ""
	if url, ok := user.AvatarURLs["96"]; ok {
		avatar = url
	}
	return StrapiAuthor{
		ID:     user.ID,
		Name:   user.Name,
		Slug:   user.Slug,
		Email:  fmt.Sprintf("%s@example.com", user.Slug),
		Bio:    user.Description,
		Avatar: avatar,
	}
}

// convertMedia converts a WordPress media item to a Strapi media
func (e *StrapiExporter) convertMedia(media models.WordPressMedia) StrapiMedia {
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

	return StrapiMedia{
		ID:              media.ID,
		Name:            media.Title.Rendered,
		AlternativeText: media.AltText,
		Caption:         stripHTMLTags(media.Caption.Rendered),
		URL:             media.SourceURL,
		Mime:            media.MimeType,
		Width:           width,
		Height:          height,
		CreatedAt:       media.Date.Time,
		UpdatedAt:       media.Modified.Time,
	}
}

// exportSeparateFiles exports individual JSON files for each content type
func (e *StrapiExporter) exportSeparateFiles(data StrapiExportData) error {
	baseDir := e.config.Output
	if filepath.Ext(baseDir) == ".json" {
		baseDir = filepath.Dir(baseDir)
	}

	// Export articles
	articlesJSON, err := json.MarshalIndent(data.Articles, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "strapi_articles.json"), articlesJSON, 0600); err != nil {
		return err
	}

	// Export pages
	pagesJSON, err := json.MarshalIndent(data.Pages, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "strapi_pages.json"), pagesJSON, 0600); err != nil {
		return err
	}

	// Export categories
	categoriesJSON, err := json.MarshalIndent(data.Categories, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "strapi_categories.json"), categoriesJSON, 0600); err != nil {
		return err
	}

	// Export tags
	tagsJSON, err := json.MarshalIndent(data.Tags, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "strapi_tags.json"), tagsJSON, 0600); err != nil {
		return err
	}

	// Export authors
	authorsJSON, err := json.MarshalIndent(data.Authors, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "strapi_authors.json"), authorsJSON, 0600); err != nil {
		return err
	}

	// Export media
	mediaJSON, err := json.MarshalIndent(data.Media, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "strapi_media.json"), mediaJSON, 0600); err != nil {
		return err
	}

	return nil
}
