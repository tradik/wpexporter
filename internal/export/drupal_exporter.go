package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// DrupalExporter exports data in Drupal-compatible JSON format
// Compatible with migrate_source_json and Drupal's migrate module
type DrupalExporter struct {
	config *config.Config
}

// NewDrupalExporter creates a new Drupal exporter
func NewDrupalExporter(cfg *config.Config) *DrupalExporter {
	return &DrupalExporter{
		config: cfg,
	}
}

// DrupalExportData represents the complete Drupal export structure
type DrupalExportData struct {
	Meta  DrupalMeta        `json:"meta"`
	Nodes []DrupalNode      `json:"nodes"`
	Terms []DrupalTerm      `json:"terms"`
	Users []DrupalUser      `json:"users"`
	Media []DrupalMediaItem `json:"media"`
	Menus []DrupalMenu      `json:"menus"`
}

// DrupalMeta contains export metadata
type DrupalMeta struct {
	Exporter   string    `json:"exporter"`
	Version    string    `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	SourceSite string    `json:"source_site"`
	SourceName string    `json:"source_name"`
	TotalNodes int       `json:"total_nodes"`
	TotalTerms int       `json:"total_terms"`
	TotalUsers int       `json:"total_users"`
	TotalMedia int       `json:"total_media"`
}

// DrupalNode represents a Drupal node (article, page, etc.)
type DrupalNode struct {
	NID        int               `json:"nid"`
	UUID       string            `json:"uuid"`
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	Langcode   string            `json:"langcode"`
	Status     int               `json:"status"`
	Created    int64             `json:"created"`
	Changed    int64             `json:"changed"`
	UID        int               `json:"uid"`
	Body       DrupalBody        `json:"body"`
	Path       DrupalPath        `json:"path"`
	Categories []int             `json:"field_categories,omitempty"`
	Tags       []int             `json:"field_tags,omitempty"`
	Image      *DrupalFieldImage `json:"field_image,omitempty"`
	MetaTags   DrupalMetaTags    `json:"metatag,omitempty"`
}

// DrupalBody represents a body field with format
type DrupalBody struct {
	Value   string `json:"value"`
	Summary string `json:"summary"`
	Format  string `json:"format"`
}

// DrupalPath represents URL path alias
type DrupalPath struct {
	Alias    string `json:"alias"`
	Langcode string `json:"langcode"`
}

// DrupalFieldImage represents an image field reference
type DrupalFieldImage struct {
	TargetID int    `json:"target_id"`
	Alt      string `json:"alt"`
	Title    string `json:"title"`
}

// DrupalMetaTags represents SEO meta tags
type DrupalMetaTags struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Keywords    string `json:"keywords,omitempty"`
	Canonical   string `json:"canonical_url,omitempty"`
	OGTitle     string `json:"og_title,omitempty"`
	OGDesc      string `json:"og_description,omitempty"`
	OGImage     string `json:"og_image,omitempty"`
}

// DrupalTerm represents a taxonomy term
type DrupalTerm struct {
	TID         int    `json:"tid"`
	UUID        string `json:"uuid"`
	VID         string `json:"vid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
	Parent      int    `json:"parent"`
	Langcode    string `json:"langcode"`
}

// DrupalUser represents a user account
type DrupalUser struct {
	UID      int    `json:"uid"`
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Mail     string `json:"mail"`
	Status   int    `json:"status"`
	Created  int64  `json:"created"`
	Langcode string `json:"langcode"`
}

// DrupalMediaItem represents a media entity
type DrupalMediaItem struct {
	MID       int        `json:"mid"`
	UUID      string     `json:"uuid"`
	Bundle    string     `json:"bundle"`
	Name      string     `json:"name"`
	Status    int        `json:"status"`
	Created   int64      `json:"created"`
	Changed   int64      `json:"changed"`
	UID       int        `json:"uid"`
	Langcode  string     `json:"langcode"`
	SourceURL string     `json:"field_media_image_url"`
	File      DrupalFile `json:"field_media_file"`
}

// DrupalFile represents a file entity
type DrupalFile struct {
	TargetID int    `json:"target_id"`
	URI      string `json:"uri"`
	Filename string `json:"filename"`
	Filemime string `json:"filemime"`
}

// DrupalMenu represents a menu structure (optional)
type DrupalMenu struct {
	ID    string           `json:"id"`
	Title string           `json:"title"`
	Links []DrupalMenuLink `json:"links"`
}

// DrupalMenuLink represents a menu link
type DrupalMenuLink struct {
	Title    string `json:"title"`
	URI      string `json:"uri"`
	Weight   int    `json:"weight"`
	Expanded bool   `json:"expanded"`
}

// Export exports data in Drupal-compatible JSON format
func (e *DrupalExporter) Export(data *models.ExportData) error {
	// Build Drupal export structure
	drupalData := e.buildDrupalData(data)

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(drupalData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Drupal JSON: %w", err)
	}

	// Determine output path
	var outputPath string
	if filepath.Ext(e.config.Output) == ".json" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "drupal_export.json")
	}

	// Write main export file
	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write Drupal export file: %w", err)
	}

	// Also export separate files for easier migration
	if err := e.exportSeparateFiles(drupalData); err != nil {
		return fmt.Errorf("failed to write separate Drupal files: %w", err)
	}

	fmt.Printf("Drupal export completed: %s\n", outputPath)
	return nil
}

// buildDrupalData constructs the complete Drupal export structure
func (e *DrupalExporter) buildDrupalData(data *models.ExportData) DrupalExportData {
	drupal := DrupalExportData{
		Meta: DrupalMeta{
			Exporter:   "wpexporter",
			Version:    "1.4.0",
			ExportedAt: time.Now(),
			SourceSite: data.Site.URL,
			SourceName: data.Site.Name,
		},
	}

	// Convert users first (needed for reference)
	drupal.Users = e.convertUsers(data.Users)
	drupal.Meta.TotalUsers = len(drupal.Users)

	// Convert taxonomy terms (categories and tags)
	drupal.Terms = e.convertTerms(data.Categories, data.Tags)
	drupal.Meta.TotalTerms = len(drupal.Terms)

	// Convert media
	drupal.Media = e.convertMedia(data.Media)
	drupal.Meta.TotalMedia = len(drupal.Media)

	// Convert nodes (posts and pages)
	drupal.Nodes = e.convertNodes(data.Posts, data.Pages, data.Media)
	drupal.Meta.TotalNodes = len(drupal.Nodes)

	return drupal
}

// convertUsers converts WordPress users to Drupal users
func (e *DrupalExporter) convertUsers(users []models.WordPressUser) []DrupalUser {
	drupalUsers := make([]DrupalUser, 0, len(users))
	for _, user := range users {
		drupalUser := DrupalUser{
			UID:      user.ID,
			UUID:     generateUUID(user.ID, "user"),
			Name:     user.Slug,
			Mail:     "",
			Status:   1,
			Created:  time.Now().Unix(),
			Langcode: "en",
		}
		drupalUsers = append(drupalUsers, drupalUser)
	}
	return drupalUsers
}

// convertTerms converts WordPress categories and tags to Drupal taxonomy terms
func (e *DrupalExporter) convertTerms(categories []models.WordPressCategory, tags []models.WordPressTag) []DrupalTerm {
	terms := make([]DrupalTerm, 0, len(categories)+len(tags))

	// Convert categories
	for _, cat := range categories {
		term := DrupalTerm{
			TID:         cat.ID,
			UUID:        generateUUID(cat.ID, "category"),
			VID:         "categories",
			Name:        cat.Name,
			Description: cat.Description,
			Weight:      0,
			Parent:      cat.Parent,
			Langcode:    "en",
		}
		terms = append(terms, term)
	}

	// Convert tags with offset to avoid ID collision
	tagOffset := 100000
	for _, tag := range tags {
		term := DrupalTerm{
			TID:         tag.ID + tagOffset,
			UUID:        generateUUID(tag.ID, "tag"),
			VID:         "tags",
			Name:        tag.Name,
			Description: tag.Description,
			Weight:      0,
			Parent:      0,
			Langcode:    "en",
		}
		terms = append(terms, term)
	}

	return terms
}

// convertMedia converts WordPress media to Drupal media entities
func (e *DrupalExporter) convertMedia(media []models.WordPressMedia) []DrupalMediaItem {
	drupalMedia := make([]DrupalMediaItem, 0, len(media))

	for _, m := range media {
		// Determine bundle based on mime type
		bundle := "image"
		if strings.HasPrefix(m.MimeType, "video/") {
			bundle = "video"
		} else if strings.HasPrefix(m.MimeType, "audio/") {
			bundle = "audio"
		} else if !strings.HasPrefix(m.MimeType, "image/") {
			bundle = "file"
		}

		drupalItem := DrupalMediaItem{
			MID:       m.ID,
			UUID:      generateUUID(m.ID, "media"),
			Bundle:    bundle,
			Name:      m.Title.Rendered,
			Status:    statusToInt(m.Status),
			Created:   m.Date.Unix(),
			Changed:   m.Modified.Unix(),
			UID:       m.Author,
			Langcode:  "en",
			SourceURL: m.SourceURL,
			File: DrupalFile{
				TargetID: m.ID,
				URI:      convertToPublicURI(m.SourceURL),
				Filename: extractFilename(m.SourceURL),
				Filemime: m.MimeType,
			},
		}
		drupalMedia = append(drupalMedia, drupalItem)
	}

	return drupalMedia
}

// convertNodes converts WordPress posts and pages to Drupal nodes
func (e *DrupalExporter) convertNodes(posts, pages []models.WordPressPost, media []models.WordPressMedia) []DrupalNode {
	nodes := make([]DrupalNode, 0, len(posts)+len(pages))

	// Build media map for featured image lookup
	mediaMap := make(map[int]models.WordPressMedia)
	for _, m := range media {
		mediaMap[m.ID] = m
	}

	// Convert posts as "article" type
	for _, post := range posts {
		node := e.convertPostToNode(post, "article", mediaMap)
		nodes = append(nodes, node)
	}

	// Convert pages as "page" type with ID offset
	pageOffset := 1000000
	for _, page := range pages {
		node := e.convertPostToNode(page, "page", mediaMap)
		node.NID = page.ID + pageOffset
		nodes = append(nodes, node)
	}

	return nodes
}

// convertPostToNode converts a WordPress post/page to a Drupal node
func (e *DrupalExporter) convertPostToNode(post models.WordPressPost, nodeType string, mediaMap map[int]models.WordPressMedia) DrupalNode {
	node := DrupalNode{
		NID:      post.ID,
		UUID:     generateUUID(post.ID, nodeType),
		Type:     nodeType,
		Title:    post.Title.Rendered,
		Langcode: "en",
		Status:   statusToInt(post.Status),
		Created:  post.Date.Unix(),
		Changed:  post.Modified.Unix(),
		UID:      post.Author,
		Body: DrupalBody{
			Value:   post.Content.Rendered,
			Summary: stripTags(post.Excerpt.Rendered),
			Format:  "full_html",
		},
		Path: DrupalPath{
			Alias:    "/" + post.Slug,
			Langcode: "en",
		},
	}

	// Add categories
	if len(post.Categories) > 0 {
		node.Categories = post.Categories
	}

	// Add tags with offset
	if len(post.Tags) > 0 {
		tagOffset := 100000
		offsetTags := make([]int, len(post.Tags))
		for i, tagID := range post.Tags {
			offsetTags[i] = tagID + tagOffset
		}
		node.Tags = offsetTags
	}

	// Add featured image
	if post.FeaturedMedia > 0 {
		if media, ok := mediaMap[post.FeaturedMedia]; ok {
			node.Image = &DrupalFieldImage{
				TargetID: post.FeaturedMedia,
				Alt:      media.AltText,
				Title:    media.Title.Rendered,
			}
		}
	}

	// Add SEO meta tags
	if post.SEO.Title != "" || post.SEO.MetaDescription != "" {
		node.MetaTags = DrupalMetaTags{
			Title:       post.SEO.Title,
			Description: post.SEO.MetaDescription,
			Keywords:    post.SEO.MetaKeywords,
			Canonical:   post.SEO.CanonicalURL,
			OGTitle:     post.SEO.OGTitle,
			OGDesc:      post.SEO.OGDescription,
			OGImage:     post.SEO.OGImage,
		}
	}

	return node
}

// exportSeparateFiles exports individual JSON files for each content type
func (e *DrupalExporter) exportSeparateFiles(data DrupalExportData) error {
	baseDir := e.config.Output
	if filepath.Ext(baseDir) == ".json" {
		baseDir = filepath.Dir(baseDir)
	}

	// Export nodes
	nodesJSON, err := json.MarshalIndent(data.Nodes, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "drupal_nodes.json"), nodesJSON, 0600); err != nil {
		return err
	}

	// Export terms
	termsJSON, err := json.MarshalIndent(data.Terms, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "drupal_terms.json"), termsJSON, 0600); err != nil {
		return err
	}

	// Export users
	usersJSON, err := json.MarshalIndent(data.Users, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "drupal_users.json"), usersJSON, 0600); err != nil {
		return err
	}

	// Export media
	mediaJSON, err := json.MarshalIndent(data.Media, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "drupal_media.json"), mediaJSON, 0600); err != nil {
		return err
	}

	return nil
}

// Helper functions

// generateUUID generates a deterministic UUID-like string for Drupal
func generateUUID(id int, prefix string) string {
	// Generate a deterministic UUID based on ID and prefix
	// Format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	hash := fmt.Sprintf("%08x", id)
	prefixHash := fmt.Sprintf("%04x", len(prefix))
	return fmt.Sprintf("%s-%s-0000-0000-%012d", hash, prefixHash, id)
}

// statusToInt converts WordPress status string to Drupal status int
func statusToInt(status string) int {
	if status == statusPublish {
		return 1
	}
	return 0
}

// stripTags removes HTML tags from a string
func stripTags(html string) string {
	// Simple HTML tag stripper
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")
	text = strings.TrimSpace(text)
	return text
}

// convertToPublicURI converts a URL to Drupal public:// URI
func convertToPublicURI(urlStr string) string {
	// Extract filename and create public:// URI
	filename := extractFilename(urlStr)
	// Try to preserve directory structure from wp-content/uploads
	if idx := strings.Index(urlStr, "/wp-content/uploads/"); idx != -1 {
		path := urlStr[idx+len("/wp-content/uploads/"):]
		return "public://" + path
	}
	return "public://" + filename
}

// extractFilename extracts filename from URL
func extractFilename(urlStr string) string {
	parts := strings.Split(urlStr, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "file"
}
