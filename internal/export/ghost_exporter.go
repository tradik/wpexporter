package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/rx"
	"github.com/tradik/wpexporter/pkg/models"
)

// GhostExporter exports data in Ghost CMS JSON format
type GhostExporter struct {
	config *config.Config
}

// NewGhostExporter creates a new Ghost exporter
func NewGhostExporter(cfg *config.Config) *GhostExporter {
	return &GhostExporter{
		config: cfg,
	}
}

// GhostExportData represents the complete Ghost export structure
type GhostExportData struct {
	DB []GhostDB `json:"db"`
}

// GhostDB represents a Ghost database export
type GhostDB struct {
	Meta GhostMeta `json:"meta"`
	Data GhostData `json:"data"`
}

// GhostMeta contains export metadata
type GhostMeta struct {
	ExportedOn int64  `json:"exported_on"`
	Version    string `json:"version"`
}

// GhostData contains all Ghost content
type GhostData struct {
	Posts        []GhostPost       `json:"posts"`
	PostsAuthors []GhostPostAuthor `json:"posts_authors,omitempty"`
	PostsTags    []GhostPostTag    `json:"posts_tags,omitempty"`
	Tags         []GhostTag        `json:"tags"`
	Users        []GhostUser       `json:"users"`
}

// GhostPost represents a Ghost post
type GhostPost struct {
	ID              string `json:"id"`
	UUID            string `json:"uuid"`
	Title           string `json:"title"`
	Slug            string `json:"slug"`
	HTML            string `json:"html"`
	Plaintext       string `json:"plaintext,omitempty"`
	FeatureImage    string `json:"feature_image,omitempty"`
	Featured        int    `json:"featured"`
	Status          string `json:"status"`
	Visibility      string `json:"visibility"`
	MetaTitle       string `json:"meta_title,omitempty"`
	MetaDescription string `json:"meta_description,omitempty"`
	AuthorID        string `json:"author_id"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	PublishedAt     int64  `json:"published_at,omitempty"`
	CustomExcerpt   string `json:"custom_excerpt,omitempty"`
	Type            string `json:"type"`
}

// GhostTag represents a Ghost tag
type GhostTag struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// GhostUser represents a Ghost user/author
type GhostUser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// GhostPostAuthor represents the post-author relationship
type GhostPostAuthor struct {
	PostID    string `json:"post_id"`
	AuthorID  string `json:"author_id"`
	SortOrder int    `json:"sort_order"`
}

// GhostPostTag represents the post-tag relationship
type GhostPostTag struct {
	PostID    string `json:"post_id"`
	TagID     string `json:"tag_id"`
	SortOrder int    `json:"sort_order"`
}

// Export exports data in Ghost JSON format
func (e *GhostExporter) Export(data *models.ExportData) error {
	ghostData := e.buildGhostData(data)

	jsonData, err := json.MarshalIndent(ghostData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Ghost JSON: %w", err)
	}

	var outputPath string
	if filepath.Ext(e.config.Output) == ".json" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "ghost_export.json")
	}

	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write Ghost export file: %w", err)
	}

	fmt.Printf("Ghost export completed: %s\n", outputPath)
	return nil
}

// buildGhostData constructs the complete Ghost export structure
func (e *GhostExporter) buildGhostData(data *models.ExportData) GhostExportData {
	ghostDB := GhostDB{
		Meta: GhostMeta{
			ExportedOn: time.Now().UnixMilli(),
			Version:    "5.0.0",
		},
		Data: GhostData{
			Posts:        make([]GhostPost, 0),
			PostsAuthors: make([]GhostPostAuthor, 0),
			PostsTags:    make([]GhostPostTag, 0),
			Tags:         make([]GhostTag, 0),
			Users:        make([]GhostUser, 0),
		},
	}

	// Build tag map and convert tags
	tagMap := make(map[int]string)
	for _, tag := range data.Tags {
		ghostTag := e.convertTag(tag)
		ghostDB.Data.Tags = append(ghostDB.Data.Tags, ghostTag)
		tagMap[tag.ID] = ghostTag.ID
	}

	// Convert categories as tags (Ghost doesn't have categories)
	for _, cat := range data.Categories {
		ghostTag := e.convertCategoryToTag(cat)
		ghostDB.Data.Tags = append(ghostDB.Data.Tags, ghostTag)
		tagMap[cat.ID+100000] = ghostTag.ID // Offset to avoid ID collision
	}

	// Build user map and convert users
	userMap := make(map[int]string)
	for _, user := range data.Users {
		ghostUser := e.convertUser(user)
		ghostDB.Data.Users = append(ghostDB.Data.Users, ghostUser)
		userMap[user.ID] = ghostUser.ID
	}

	// Default author if no users
	if len(ghostDB.Data.Users) == 0 {
		defaultUser := GhostUser{
			ID:        "1",
			Name:      "Admin",
			Slug:      "admin",
			Email:     "admin@example.com",
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		}
		ghostDB.Data.Users = append(ghostDB.Data.Users, defaultUser)
		userMap[0] = "1"
	}

	// Convert posts
	for _, post := range data.Posts {
		ghostPost := e.convertPost(post, userMap, "post")
		ghostDB.Data.Posts = append(ghostDB.Data.Posts, ghostPost)

		// Add post-author relationship
		authorID := userMap[post.Author]
		if authorID == "" {
			authorID = ghostDB.Data.Users[0].ID
		}
		ghostDB.Data.PostsAuthors = append(ghostDB.Data.PostsAuthors, GhostPostAuthor{
			PostID:    ghostPost.ID,
			AuthorID:  authorID,
			SortOrder: 0,
		})

		// Add post-tag relationships for tags
		for i, tagID := range post.Tags {
			if tid, ok := tagMap[tagID]; ok {
				ghostDB.Data.PostsTags = append(ghostDB.Data.PostsTags, GhostPostTag{
					PostID:    ghostPost.ID,
					TagID:     tid,
					SortOrder: i,
				})
			}
		}

		// Add post-tag relationships for categories
		for i, catID := range post.Categories {
			if tid, ok := tagMap[catID+100000]; ok {
				ghostDB.Data.PostsTags = append(ghostDB.Data.PostsTags, GhostPostTag{
					PostID:    ghostPost.ID,
					TagID:     tid,
					SortOrder: len(post.Tags) + i,
				})
			}
		}
	}

	// Convert pages
	for _, page := range data.Pages {
		ghostPost := e.convertPost(page, userMap, "page")
		ghostDB.Data.Posts = append(ghostDB.Data.Posts, ghostPost)

		authorID := userMap[page.Author]
		if authorID == "" {
			authorID = ghostDB.Data.Users[0].ID
		}
		ghostDB.Data.PostsAuthors = append(ghostDB.Data.PostsAuthors, GhostPostAuthor{
			PostID:    ghostPost.ID,
			AuthorID:  authorID,
			SortOrder: 0,
		})
	}

	return GhostExportData{DB: []GhostDB{ghostDB}}
}

// convertPost converts a WordPress post to a Ghost post
func (e *GhostExporter) convertPost(post models.WordPressPost, userMap map[int]string, postType string) GhostPost {
	status := "draft"
	if post.Status == statusPublish {
		status = "published"
	}

	featured := 0
	if post.Sticky {
		featured = 1
	}

	var publishedAt int64
	if status == "published" {
		publishedAt = post.Date.UnixMilli()
	}

	// Feature image would need to be resolved from media map
	// For now, we leave it empty - can be populated if media map is passed
	featureImage := ""

	return GhostPost{
		ID:              fmt.Sprintf("%d", post.ID),
		UUID:            generateGhostUUID(post.ID),
		Title:           post.Title.Rendered,
		Slug:            post.Slug,
		HTML:            post.Content.Rendered,
		Plaintext:       stripHTMLTags(post.Content.Rendered),
		FeatureImage:    featureImage,
		Featured:        featured,
		Status:          status,
		Visibility:      "public",
		MetaTitle:       post.SEO.Title,
		MetaDescription: post.SEO.MetaDescription,
		AuthorID:        userMap[post.Author],
		CreatedAt:       post.Date.UnixMilli(),
		UpdatedAt:       post.Modified.UnixMilli(),
		PublishedAt:     publishedAt,
		CustomExcerpt:   stripHTMLTags(post.Excerpt.Rendered),
		Type:            postType,
	}
}

// convertTag converts a WordPress tag to a Ghost tag
func (e *GhostExporter) convertTag(tag models.WordPressTag) GhostTag {
	return GhostTag{
		ID:          fmt.Sprintf("tag-%d", tag.ID),
		Name:        tag.Name,
		Slug:        tag.Slug,
		Description: tag.Description,
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}
}

// convertCategoryToTag converts a WordPress category to a Ghost tag
func (e *GhostExporter) convertCategoryToTag(cat models.WordPressCategory) GhostTag {
	return GhostTag{
		ID:          fmt.Sprintf("cat-%d", cat.ID),
		Name:        cat.Name,
		Slug:        cat.Slug,
		Description: cat.Description,
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}
}

// convertUser converts a WordPress user to a Ghost user
func (e *GhostExporter) convertUser(user models.WordPressUser) GhostUser {
	email := fmt.Sprintf("%s@example.com", user.Slug)
	return GhostUser{
		ID:        fmt.Sprintf("%d", user.ID),
		Name:      user.Name,
		Slug:      user.Slug,
		Email:     email,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
}

// generateGhostUUID generates a UUID-like string for Ghost
func generateGhostUUID(id int) string {
	return fmt.Sprintf("%08x-0000-0000-0000-%012d", id, id)
}

// stripHTMLTags removes HTML tags from a string
func stripHTMLTags(html string) string {
	re := rx.Get(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")
	text = strings.TrimSpace(text)
	return text
}
