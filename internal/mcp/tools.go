package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/pkg/models"
)

// GetTools returns all available MCP tools
func GetTools() []Tool {
	return []Tool{
		{
			Name:        "list_formats",
			Description: "List all available export formats for WordPress content",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "get_site_info",
			Description: "Get information about a WordPress site (name, description, URL, timezone)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        schemaTypeString,
						Description: "WordPress site URL (e.g., https://example.com)",
					},
					"authUser": {
						Type:        schemaTypeString,
						Description: "Username for Basic Auth (optional)",
					},
					"authPass": {
						Type:        schemaTypeString,
						Description: "Password for Basic Auth (optional)",
					},
					"authToken": {
						Type:        schemaTypeString,
						Description: "Bearer token for authentication (optional)",
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "list_posts",
			Description: "List WordPress posts from a site with optional filtering",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        schemaTypeString,
						Description: "WordPress site URL",
					},
					"authUser": {
						Type:        schemaTypeString,
						Description: "Username for Basic Auth (optional)",
					},
					"authPass": {
						Type:        schemaTypeString,
						Description: "Password for Basic Auth (optional)",
					},
					"authToken": {
						Type:        schemaTypeString,
						Description: "Bearer token for authentication (optional)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of posts to return (default: 10)",
						Default:     10,
					},
					"pathFilter": {
						Type:        schemaTypeString,
						Description: "Filter posts by URL path pattern (e.g., /blog/)",
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "list_pages",
			Description: "List WordPress pages from a site",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        schemaTypeString,
						Description: "WordPress site URL",
					},
					"authUser": {
						Type:        schemaTypeString,
						Description: "Username for Basic Auth (optional)",
					},
					"authPass": {
						Type:        schemaTypeString,
						Description: "Password for Basic Auth (optional)",
					},
					"authToken": {
						Type:        schemaTypeString,
						Description: "Bearer token for authentication (optional)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of pages to return (default: 10)",
						Default:     10,
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "export_site",
			Description: "Export WordPress site content to specified format",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        schemaTypeString,
						Description: "WordPress site URL",
					},
					"format": {
						Type:        schemaTypeString,
						Description: "Export format",
						Enum: []string{
							"json", "markdown", "ssg", "shopify", "magento", "wordpress",
							"drupal", "wix", "squarespace", "webflow", "weebly",
							"prestashop", "ghost", "strapi", "contentful",
						},
						Default: "json",
					},
					"output": {
						Type:        schemaTypeString,
						Description: "Output directory path (optional, defaults to export/{domain}.{date})",
					},
					"authUser": {
						Type:        schemaTypeString,
						Description: "Username for Basic Auth (optional)",
					},
					"authPass": {
						Type:        schemaTypeString,
						Description: "Password for Basic Auth (optional)",
					},
					"authToken": {
						Type:        schemaTypeString,
						Description: "Bearer token for authentication (optional)",
					},
					"downloadMedia": {
						Type:        "boolean",
						Description: "Download media files (default: true)",
						Default:     true,
					},
					"noPosts": {
						Type:        "boolean",
						Description: "Skip exporting posts",
						Default:     false,
					},
					"noPages": {
						Type:        "boolean",
						Description: "Skip exporting pages",
						Default:     false,
					},
					"noProducts": {
						Type:        "boolean",
						Description: "Skip exporting WooCommerce products",
						Default:     false,
					},
					"noComments": {
						Type:        "boolean",
						Description: "Skip exporting reader comments",
						Default:     false,
					},
					"pathFilter": {
						Type:        schemaTypeString,
						Description: "Filter content by URL path pattern",
					},
					"assistedCrawl": {
						Type:        "boolean",
						Description: "Crawl URLs for SEO metadata",
						Default:     false,
					},
					"flatHtml": {
						Type:        "boolean",
						Description: "Convert HTML to Markdown",
						Default:     false,
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "get_post",
			Description: "Get a specific WordPress post by ID",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        schemaTypeString,
						Description: "WordPress site URL",
					},
					"postId": {
						Type:        "integer",
						Description: "Post ID to retrieve",
					},
					"authUser": {
						Type:        schemaTypeString,
						Description: "Username for Basic Auth (optional)",
					},
					"authPass": {
						Type:        schemaTypeString,
						Description: "Password for Basic Auth (optional)",
					},
					"authToken": {
						Type:        schemaTypeString,
						Description: "Bearer token for authentication (optional)",
					},
				},
				Required: []string{"url", "postId"},
			},
		},
		{
			Name:        "list_categories",
			Description: "List WordPress categories from a site",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        schemaTypeString,
						Description: "WordPress site URL",
					},
					"authUser": {
						Type:        schemaTypeString,
						Description: "Username for Basic Auth (optional)",
					},
					"authPass": {
						Type:        schemaTypeString,
						Description: "Password for Basic Auth (optional)",
					},
					"authToken": {
						Type:        schemaTypeString,
						Description: "Bearer token for authentication (optional)",
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "list_media",
			Description: "List WordPress media files from a site",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        schemaTypeString,
						Description: "WordPress site URL",
					},
					"authUser": {
						Type:        schemaTypeString,
						Description: "Username for Basic Auth (optional)",
					},
					"authPass": {
						Type:        schemaTypeString,
						Description: "Password for Basic Auth (optional)",
					},
					"authToken": {
						Type:        schemaTypeString,
						Description: "Bearer token for authentication (optional)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of media items to return (default: 20)",
						Default:     20,
					},
				},
				Required: []string{"url"},
			},
		},
	}
}

// RegisterAllTools registers all tool handlers with the server
func RegisterAllTools(server *Server) {
	tools := GetTools()
	handlers := map[string]ToolHandler{
		"list_formats":    handleListFormats,
		"get_site_info":   handleGetSiteInfo,
		"list_posts":      handleListPosts,
		"list_pages":      handleListPages,
		"export_site":     handleExportSite,
		"get_post":        handleGetPost,
		"list_categories": handleListCategories,
		"list_media":      handleListMedia,
	}

	for _, tool := range tools {
		if handler, exists := handlers[tool.Name]; exists {
			server.RegisterTool(tool, handler)
		}
	}
}

// Helper to create config from args
func configFromArgs(args map[string]interface{}) *config.Config {
	cfg := config.DefaultConfig()
	// Use shorter timeout and fewer retries for MCP to provide faster feedback
	cfg.Timeout = 10
	cfg.Retries = 1

	if url, ok := args["url"].(string); ok {
		cfg.URL = url
	}
	if authUser, ok := args["authUser"].(string); ok {
		cfg.AuthUser = authUser
	}
	if authPass, ok := args["authPass"].(string); ok {
		cfg.AuthPass = authPass
	}
	if authToken, ok := args["authToken"].(string); ok {
		cfg.AuthToken = authToken
	}
	if format, ok := args["format"].(string); ok {
		cfg.Format = format
	}
	if output, ok := args["output"].(string); ok {
		cfg.Output = output
	}
	if downloadMedia, ok := args["downloadMedia"].(bool); ok {
		cfg.DownloadMedia = downloadMedia
	}
	if noPosts, ok := args["noPosts"].(bool); ok {
		cfg.NoPosts = noPosts
	}
	if noPages, ok := args["noPages"].(bool); ok {
		cfg.NoPages = noPages
	}
	if noProducts, ok := args["noProducts"].(bool); ok {
		cfg.NoProducts = noProducts
	}
	if noComments, ok := args["noComments"].(bool); ok {
		cfg.NoComments = noComments
	}
	if pathFilter, ok := args["pathFilter"].(string); ok {
		cfg.PathFilter = pathFilter
	}
	if assistedCrawl, ok := args["assistedCrawl"].(bool); ok {
		cfg.AssistedCrawl = assistedCrawl
	}
	if flatHtml, ok := args["flatHtml"].(bool); ok {
		cfg.FlatHTML = flatHtml
	}

	return cfg
}

// handleListFormats returns available export formats
func handleListFormats(_ map[string]interface{}) (*CallToolResult, error) {
	formats := []map[string]string{
		{"name": "json", "description": "Standard JSON format with all WordPress data"},
		{"name": "markdown", "description": "Markdown files with YAML frontmatter"},
		{"name": "shopify", "description": "Shopify-compatible CSV for products and blog posts"},
		{"name": "magento", "description": "Magento 2 import format"},
		{"name": "wordpress", "description": "WordPress WXR (WordPress eXtended RSS) format"},
		{"name": "drupal", "description": "Drupal migration format"},
		{"name": "wix", "description": "Wix-compatible format"},
		{"name": "squarespace", "description": "Squarespace import format"},
		{"name": "webflow", "description": "Webflow CMS collection format"},
		{"name": "weebly", "description": "Weebly import format"},
		{"name": "prestashop", "description": "PrestaShop import format for e-commerce"},
		{"name": "ghost", "description": "Ghost CMS JSON import format"},
		{"name": "strapi", "description": "Strapi headless CMS format"},
		{"name": "contentful", "description": "Contentful CMS import format"},
		{"name": "ssg", "description": "Static site generator content source (URL-mirroring paths, single-spelled front matter, cleaned HTML)"},
	}

	data, _ := json.MarshalIndent(formats, "", "  ")
	return &CallToolResult{
		Content: []Content{TextContent(string(data))},
	}, nil
}

// handleGetSiteInfo retrieves site information
func handleGetSiteInfo(args map[string]interface{}) (*CallToolResult, error) {
	cfg := configFromArgs(args)

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	siteInfo, err := client.GetSiteInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get site info: %w", err)
	}

	data, _ := json.MarshalIndent(siteInfo, "", "  ")
	return &CallToolResult{
		Content: []Content{TextContent(string(data))},
	}, nil
}

// handleListPosts returns posts from the site
func handleListPosts(args map[string]interface{}) (*CallToolResult, error) {
	cfg := configFromArgs(args)

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	posts, err := client.GetPosts()
	if err != nil {
		return nil, fmt.Errorf("failed to get posts: %w", err)
	}

	// Apply path filter if specified
	if pathFilter, ok := args["pathFilter"].(string); ok && pathFilter != "" {
		filtered := make([]models.WordPressPost, 0)
		for _, post := range posts {
			if strings.Contains(post.Link, pathFilter) {
				filtered = append(filtered, post)
			}
		}
		posts = filtered
	}

	// Limit results
	if len(posts) > limit {
		posts = posts[:limit]
	}

	// Create summary
	summary := make([]map[string]interface{}, len(posts))
	for i, post := range posts {
		summary[i] = map[string]interface{}{
			"id":       post.ID,
			"title":    post.Title.Rendered,
			"slug":     post.Slug,
			"link":     post.Link,
			"date":     post.Date,
			"status":   post.Status,
			"author":   post.Author,
			"modified": post.Modified,
		}
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	return &CallToolResult{
		Content: []Content{TextContent(fmt.Sprintf("Found %d posts (showing %d):\n%s", len(posts), len(summary), string(data)))},
	}, nil
}

// handleListPages returns pages from the site
func handleListPages(args map[string]interface{}) (*CallToolResult, error) {
	cfg := configFromArgs(args)

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	pages, err := client.GetPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get pages: %w", err)
	}

	// Limit results
	if len(pages) > limit {
		pages = pages[:limit]
	}

	// Create summary
	summary := make([]map[string]interface{}, len(pages))
	for i, page := range pages {
		summary[i] = map[string]interface{}{
			"id":       page.ID,
			"title":    page.Title.Rendered,
			"slug":     page.Slug,
			"link":     page.Link,
			"date":     page.Date,
			"status":   page.Status,
			"modified": page.Modified,
		}
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	return &CallToolResult{
		Content: []Content{TextContent(fmt.Sprintf("Found %d pages (showing %d):\n%s", len(pages), len(summary), string(data)))},
	}, nil
}

// collectExportData reads everything the export writes: the site itself, the
// collections the caller did not switch off, and the counts the metadata block
// states.
//
// Posts and pages are the export, so failing to read either fails the run. The
// rest of a WordPress site is optional by installation — products need
// WooCommerce, media a download pass, comments an enabled REST route — and a
// refusal there leaves that collection empty instead of ending the export. An
// MCP client has no console to read a warning from; an empty collection and a
// zero count are the report.
func collectExportData(client *api.Client, cfg *config.Config) (*models.ExportData, error) {
	siteInfo, err := client.GetSiteInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get site info: %w", err)
	}

	var posts []models.WordPressPost
	if !cfg.NoPosts {
		if posts, err = client.GetPosts(); err != nil {
			return nil, fmt.Errorf("failed to get posts: %w", err)
		}
	}

	var pages []models.WordPressPost
	if !cfg.NoPages {
		if pages, err = client.GetPages(); err != nil {
			return nil, fmt.Errorf("failed to get pages: %w", err)
		}
	}

	var products []models.WooCommerceProduct
	if !cfg.NoProducts {
		products, _ = client.GetProducts()
	}

	var media []models.WordPressMedia
	if cfg.DownloadMedia {
		media, _ = client.GetMedia()
	}

	// Reader comments ship like every other collection (#35). An export driven
	// over MCP would otherwise drop them exactly as the CLI used to.
	var comments []models.WordPressComment
	if !cfg.NoComments {
		comments, _ = client.GetComments()
	}

	categories, _ := client.GetCategories()
	tags, _ := client.GetTags()
	users, _ := client.GetUsers()

	return &models.ExportData{
		Site:       *siteInfo,
		Posts:      posts,
		Pages:      pages,
		Products:   products,
		Media:      media,
		Categories: categories,
		Tags:       tags,
		Users:      users,
		Comments:   comments,
		Stats: models.ExportStats{
			TotalPosts:      len(posts),
			TotalPages:      len(pages),
			TotalProducts:   len(products),
			TotalMedia:      len(media),
			TotalCategories: len(categories),
			TotalTags:       len(tags),
			TotalUsers:      len(users),
			TotalComments:   len(comments),
		},
	}, nil
}

// handleExportSite performs a full site export
func handleExportSite(args map[string]interface{}) (*CallToolResult, error) {
	cfg := configFromArgs(args)

	// Generate output path if not specified
	if cfg.Output == "" {
		if err := cfg.GenerateDefaultOutput(); err != nil {
			return nil, fmt.Errorf("failed to generate output path: %w", err)
		}
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(cfg.Output, 0750); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	exporter := export.NewExporter(cfg)

	// Collect data
	startTime := time.Now()

	exportData, err := collectExportData(client, cfg)
	if err != nil {
		return nil, err
	}

	// Export
	if err := exporter.Export(exportData); err != nil {
		return nil, fmt.Errorf("export failed: %w", err)
	}

	duration := time.Since(startTime)

	// Get absolute path
	absOutput, _ := filepath.Abs(cfg.Output)

	stats := exportData.Stats
	result := map[string]interface{}{
		"status":   "success",
		"output":   absOutput,
		"format":   cfg.Format,
		"duration": duration.String(),
		"stats": map[string]int{
			"posts":      stats.TotalPosts,
			"pages":      stats.TotalPages,
			"products":   stats.TotalProducts,
			"media":      stats.TotalMedia,
			"categories": stats.TotalCategories,
			"tags":       stats.TotalTags,
			"users":      stats.TotalUsers,
			"comments":   stats.TotalComments,
		},
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &CallToolResult{
		Content: []Content{TextContent(string(data))},
	}, nil
}

// handleGetPost retrieves a specific post
func handleGetPost(args map[string]interface{}) (*CallToolResult, error) {
	cfg := configFromArgs(args)

	postID := 0
	if id, ok := args["postId"].(float64); ok {
		postID = int(id)
	}
	if postID == 0 {
		return nil, fmt.Errorf("postId is required")
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	post, err := client.GetPostByID(postID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post %d: %w", postID, err)
	}

	data, _ := json.MarshalIndent(post, "", "  ")
	return &CallToolResult{
		Content: []Content{TextContent(string(data))},
	}, nil
}

// handleListCategories returns categories from the site
func handleListCategories(args map[string]interface{}) (*CallToolResult, error) {
	cfg := configFromArgs(args)

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	categories, err := client.GetCategories()
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	// Create summary
	summary := make([]map[string]interface{}, len(categories))
	for i, cat := range categories {
		summary[i] = map[string]interface{}{
			"id":          cat.ID,
			"name":        cat.Name,
			"slug":        cat.Slug,
			"description": cat.Description,
			"parent":      cat.Parent,
			"count":       cat.Count,
		}
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	return &CallToolResult{
		Content: []Content{TextContent(fmt.Sprintf("Found %d categories:\n%s", len(categories), string(data)))},
	}, nil
}

// handleListMedia returns media items from the site
func handleListMedia(args map[string]interface{}) (*CallToolResult, error) {
	cfg := configFromArgs(args)

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	media, err := client.GetMedia()
	if err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}

	// Limit results
	if len(media) > limit {
		media = media[:limit]
	}

	// Create summary
	summary := make([]map[string]interface{}, len(media))
	for i, m := range media {
		summary[i] = map[string]interface{}{
			"id":        m.ID,
			"title":     m.Title.Rendered,
			"mimeType":  m.MimeType,
			"sourceUrl": m.SourceURL,
			"date":      m.Date,
		}
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	return &CallToolResult{
		Content: []Content{TextContent(fmt.Sprintf("Found %d media items (showing %d):\n%s", len(media), len(summary), string(data)))},
	}, nil
}
