package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// WebflowExporter exports data in Webflow-compatible CSV format
// Webflow CMS uses CSV files for bulk imports
type WebflowExporter struct {
	config *config.Config
}

// NewWebflowExporter creates a new Webflow exporter
func NewWebflowExporter(cfg *config.Config) *WebflowExporter {
	return &WebflowExporter{
		config: cfg,
	}
}

// WebflowItem represents a Webflow CMS item
type WebflowItem struct {
	Name            string   `json:"name"`
	Slug            string   `json:"slug"`
	Content         string   `json:"content,omitempty"`
	Excerpt         string   `json:"excerpt,omitempty"`
	FeaturedImage   string   `json:"featuredImage,omitempty"`
	Author          string   `json:"author,omitempty"`
	PublishedOn     string   `json:"publishedOn,omitempty"`
	Categories      []string `json:"categories,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	MetaTitle       string   `json:"metaTitle,omitempty"`
	MetaDescription string   `json:"metaDescription,omitempty"`
	Draft           bool     `json:"draft"`
}

// Export exports data in Webflow CSV format
func (e *WebflowExporter) Export(data *models.ExportData) error {
	baseDir := e.config.Output
	if filepath.Ext(baseDir) == ".csv" || filepath.Ext(baseDir) == ".json" {
		baseDir = filepath.Dir(baseDir)
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

	// Export posts as CSV
	if len(data.Posts) > 0 {
		if err := e.exportPostsCSV(data.Posts, baseDir, tagMap, categoryMap, userMap, mediaMap); err != nil {
			return fmt.Errorf("failed to export posts: %w", err)
		}
	}

	// Export pages as CSV
	if len(data.Pages) > 0 {
		if err := e.exportPagesCSV(data.Pages, baseDir); err != nil {
			return fmt.Errorf("failed to export pages: %w", err)
		}
	}

	// Export categories as CSV (for Webflow CMS collections)
	if len(data.Categories) > 0 {
		if err := e.exportCategoriesCSV(data.Categories, baseDir); err != nil {
			return fmt.Errorf("failed to export categories: %w", err)
		}
	}

	// Export authors as CSV
	if len(data.Users) > 0 {
		if err := e.exportAuthorsCSV(data.Users, baseDir); err != nil {
			return fmt.Errorf("failed to export authors: %w", err)
		}
	}

	// Export JSON version for reference
	if err := e.exportJSON(data, baseDir, tagMap, categoryMap, userMap, mediaMap); err != nil {
		return fmt.Errorf("failed to export JSON: %w", err)
	}

	fmt.Printf("Webflow export completed: %s\n", baseDir)
	return nil
}

// exportPostsCSV exports posts as Webflow-compatible CSV
func (e *WebflowExporter) exportPostsCSV(
	posts []models.WordPressPost,
	baseDir string,
	tagMap, categoryMap, userMap map[int]string,
	mediaMap map[int]string,
) error {
	outputPath := filepath.Join(baseDir, "webflow_posts.csv")
	file, err := os.Create(outputPath) // #nosec G304 -- outputPath is safely constructed from config.Output
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Name", "Slug", "Post Body", "Post Summary", "Main Image",
		"Author", "Published On", "Categories", "Tags",
		"SEO Title", "SEO Description", "Draft",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write posts
	for _, post := range posts {
		// Get author name
		author := userMap[post.Author]
		if author == "" {
			author = "Admin"
		}

		// Get featured image URL
		featuredImage := ""
		if url, ok := mediaMap[post.FeaturedMedia]; ok {
			featuredImage = url
		}

		// Get tags
		var tags []string
		for _, tagID := range post.Tags {
			if name, ok := tagMap[tagID]; ok {
				tags = append(tags, name)
			}
		}

		// Get categories
		var categories []string
		for _, catID := range post.Categories {
			if name, ok := categoryMap[catID]; ok {
				categories = append(categories, name)
			}
		}

		// Format published date
		publishedOn := ""
		if post.Status == statusPublish {
			publishedOn = post.Date.Format("2006-01-02T15:04:05Z")
		}

		// Determine draft status
		draft := "false"
		if post.Status != statusPublish {
			draft = "true"
		}

		row := []string{
			post.Title.Rendered,
			post.Slug,
			post.Content.Rendered,
			stripHTMLTags(post.Excerpt.Rendered),
			featuredImage,
			author,
			publishedOn,
			strings.Join(categories, ", "),
			strings.Join(tags, ", "),
			post.SEO.Title,
			post.SEO.MetaDescription,
			draft,
		}

		if err := writer.Write(csvSafeRow(row)); err != nil {
			return err
		}
	}

	return nil
}

// exportPagesCSV exports pages as Webflow-compatible CSV
func (e *WebflowExporter) exportPagesCSV(pages []models.WordPressPost, baseDir string) error {
	outputPath := filepath.Join(baseDir, "webflow_pages.csv")
	file, err := os.Create(outputPath) // #nosec G304 -- outputPath is safely constructed from config.Output
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Name", "Slug", "Page Content",
		"SEO Title", "SEO Description", "Draft",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write pages
	for _, page := range pages {
		draft := "false"
		if page.Status != statusPublish {
			draft = "true"
		}

		row := []string{
			page.Title.Rendered,
			page.Slug,
			page.Content.Rendered,
			page.SEO.Title,
			page.SEO.MetaDescription,
			draft,
		}

		if err := writer.Write(csvSafeRow(row)); err != nil {
			return err
		}
	}

	return nil
}

// exportCategoriesCSV exports categories as Webflow-compatible CSV
func (e *WebflowExporter) exportCategoriesCSV(categories []models.WordPressCategory, baseDir string) error {
	outputPath := filepath.Join(baseDir, "webflow_categories.csv")
	file, err := os.Create(outputPath) // #nosec G304 -- outputPath is safely constructed from config.Output
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Name", "Slug", "Description"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write categories
	for _, cat := range categories {
		row := []string{cat.Name, cat.Slug, cat.Description}
		if err := writer.Write(csvSafeRow(row)); err != nil {
			return err
		}
	}

	return nil
}

// exportAuthorsCSV exports authors as Webflow-compatible CSV
func (e *WebflowExporter) exportAuthorsCSV(users []models.WordPressUser, baseDir string) error {
	outputPath := filepath.Join(baseDir, "webflow_authors.csv")
	file, err := os.Create(outputPath) // #nosec G304 -- outputPath is safely constructed from config.Output
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Name", "Slug", "Bio", "Avatar"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write authors
	for _, user := range users {
		avatar := ""
		if url, ok := user.AvatarURLs["96"]; ok {
			avatar = url
		}

		row := []string{user.Name, user.Slug, user.Description, avatar}
		if err := writer.Write(csvSafeRow(row)); err != nil {
			return err
		}
	}

	return nil
}

// exportJSON exports data as JSON for reference and alternative import
func (e *WebflowExporter) exportJSON(
	data *models.ExportData,
	baseDir string,
	tagMap, categoryMap, userMap map[int]string,
	mediaMap map[int]string,
) error {
	items := make([]WebflowItem, 0, len(data.Posts)+len(data.Pages))

	// Convert posts
	for _, post := range data.Posts {
		item := WebflowItem{
			Name:            post.Title.Rendered,
			Slug:            post.Slug,
			Content:         post.Content.Rendered,
			Excerpt:         stripHTMLTags(post.Excerpt.Rendered),
			Author:          userMap[post.Author],
			MetaTitle:       post.SEO.Title,
			MetaDescription: post.SEO.MetaDescription,
			Draft:           post.Status != statusPublish,
		}

		if post.Status == statusPublish {
			item.PublishedOn = post.Date.Format(time.RFC3339)
		}

		if url, ok := mediaMap[post.FeaturedMedia]; ok {
			item.FeaturedImage = url
		}

		for _, tagID := range post.Tags {
			if name, ok := tagMap[tagID]; ok {
				item.Tags = append(item.Tags, name)
			}
		}

		for _, catID := range post.Categories {
			if name, ok := categoryMap[catID]; ok {
				item.Categories = append(item.Categories, name)
			}
		}

		items = append(items, item)
	}

	// Convert pages
	for _, page := range data.Pages {
		item := WebflowItem{
			Name:            page.Title.Rendered,
			Slug:            page.Slug,
			Content:         page.Content.Rendered,
			MetaTitle:       page.SEO.Title,
			MetaDescription: page.SEO.MetaDescription,
			Draft:           page.Status != statusPublish,
		}

		if page.Status == statusPublish {
			item.PublishedOn = page.Date.Format(time.RFC3339)
		}

		items = append(items, item)
	}

	jsonData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	outputPath := filepath.Join(baseDir, "webflow_export.json")
	return os.WriteFile(outputPath, jsonData, 0600)
}
