package export

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/media"
	"github.com/tradik/wpexporter/pkg/models"
)

// uncategorizedDir is where a post with no resolvable category lands. It is
// never empty, which is what keeps every post at least one directory below
// posts/ — a requirement of the ssg format.
const uncategorizedDir = "uncategorized"

// Exporter handles data export functionality
type Exporter struct {
	config      *config.Config
	downloader  *media.Downloader
	categoryMap map[int]string    // ID -> Name lookup
	tagMap      map[int]string    // ID -> Name lookup
	categoryIDs termIndex         // ID -> name, slug and parent chain (#45)
	tagIDs      termIndex         // ID -> name and slug (#45)
	userMap     map[int]string    // ID -> Name lookup
	mediaMap    map[int]string    // ID -> media URL lookup (localized when rewriting is active)
	altMap      map[string]string // media URL -> alt text, for filling in missing alt attributes
	pageSlugs   map[int]string    // page ID -> slug, so a child can name its parent (#38)
	// shortcodeLeaks records the unexpanded plugin calls removed from documents,
	// so the export can say what it lost rather than showing it to readers (#47).
	shortcodeLeaks []shortcodeLeak
	// rewriter localizes attachment URLs; nil when the export keeps original URLs.
	rewriter *media.URLRewriter
	// readMore is what this site's own excerpts revealed about the phrase its
	// theme appends to them, so a "continue reading" in a language nobody
	// listed is recognized as chrome all the same.
	readMore readMoreVocabulary
}

// NewExporter creates a new exporter instance
func NewExporter(cfg *config.Config) *Exporter {
	return &Exporter{
		config:     cfg,
		downloader: media.NewDownloader(cfg),
	}
}

// Export exports the data in the specified format
func (e *Exporter) Export(data *models.ExportData) error {
	// Ensure output directory exists
	if err := e.config.EnsureOutputDir(); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Download media files if enabled
	if e.config.DownloadMedia {
		downloaded, err := e.downloader.DownloadMedia(data.Media)
		if err != nil {
			return fmt.Errorf("failed to download media: %w", err)
		}
		data.Stats.MediaDownloaded = downloaded
	}

	e.localizeAddresses(data)

	if err := e.reportAccessibility(data); err != nil {
		return fmt.Errorf("failed to write accessibility report: %w", err)
	}

	// Export based on format
	switch e.config.Format {
	case "json":
		return e.exportJSON(data)
	case "markdown":
		return e.exportMarkdown(data)
	case "shopify":
		return e.exportShopify(data)
	case "magento":
		return e.exportMagento(data)
	case "wordpress":
		return e.exportWordPress(data)
	case "drupal":
		return e.exportDrupal(data)
	case "wix":
		return e.exportWix(data)
	case "squarespace":
		return e.exportSquarespace(data)
	case "webflow":
		return e.exportWebflow(data)
	case "weebly":
		return e.exportWeebly(data)
	case "prestashop":
		return e.exportPrestaShop(data)
	case "ghost":
		return e.exportGhost(data)
	case "strapi":
		return e.exportStrapi(data)
	case "contentful":
		return e.exportContentful(data)
	case "ssg":
		return e.exportSSG(data)
	default:
		return fmt.Errorf("unsupported export format: %s", e.config.Format)
	}
}

// localizeAddresses is the pre-format pass every writer depends on: attachment
// URLs become local paths, same-host addresses take their configured form, and
// each comment adopts the final address of the page it belongs to (#35).
//
// Media and link rewriting is skipped for the formats that need the source
// site's own URLs (shopify, magento, …); comment addressing is not, because a
// comment without its page address cannot be placed at all.
func (e *Exporter) localizeAddresses(data *models.ExportData) {
	// Update media paths in content only for local formats (json, markdown)
	// unless --keep-original-urls is set (for importing markdown to Shopify etc.)
	if e.config.LocalizesURLs() {
		e.updateMediaPaths(data)
		e.updateLinkPaths(data)
	}

	e.resolveCommentAddresses(data)

	// A shortcode the REST API never expanded is plugin source, not content: it
	// is removed from every document and reported with counts (#47).
	if e.config.LocalizesURLs() {
		e.stripShortcodesFromContent(data)
	}
}

// stripShortcodesFromContent cleans every document the export writes, naming
// each one so the report can say where a plugin's output went missing.
func (e *Exporter) stripShortcodesFromContent(data *models.ExportData) {
	clean := func(posts []models.WordPressPost, kind string) {
		for i := range posts {
			e.stripPostShortcodes(&posts[i], kind+" "+posts[i].Slug)
		}
	}

	clean(data.Posts, "post")
	clean(data.Pages, "page")
	for t := range data.CustomTypes {
		clean(data.CustomTypes[t].Posts, data.CustomTypes[t].Slug)
	}

	e.reportShortcodes(data)

	// Reported after the strip, so a page emptied by removing a shortcode is
	// counted as what it is: a page whose body the API never really served
	// (#46).
	e.reportEmptyPages(data)
}

// exportJSON exports data as JSON
func (e *Exporter) exportJSON(data *models.ExportData) error {
	// Set export timestamp
	data.ExportedAt = time.Now()

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Determine output file path
	var outputPath string
	if filepath.Ext(e.config.Output) == ".json" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "export.json")
	}

	// Write JSON file
	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	fmt.Printf("Export completed: %s\n", outputPath)
	return nil
}

// exportMarkdown exports data as Markdown files
func (e *Exporter) exportMarkdown(data *models.ExportData) error {
	e.buildLookupMaps(data)

	// Create base directory structure
	pagesDir := filepath.Join(e.config.Output, "pages")

	if err := os.MkdirAll(pagesDir, 0750); err != nil {
		return fmt.Errorf("failed to create pages directory: %w", err)
	}

	// Export site info
	if err := e.exportSiteInfo(data.Site); err != nil {
		return fmt.Errorf("failed to export site info: %w", err)
	}

	// Export posts with category-based folder structure
	if err := e.exportPostsWithCategories(data.Posts, data.Categories, "post"); err != nil {
		return fmt.Errorf("failed to export posts: %w", err)
	}

	// Export pages, nested to mirror their published URL. A flat pages/<slug>.md
	// cannot represent a hierarchical site: two pages in different branches may
	// share a slug, and the second used to overwrite the first without a word
	// (#38).
	if err := e.exportPagesMarkdown(data); err != nil {
		return fmt.Errorf("failed to export pages: %w", err)
	}

	// Export the theme's own content types, one directory per type, so a
	// consumer sees Services as Services rather than as untyped pages (#28).
	if err := e.exportCustomTypesMarkdown(data.CustomTypes); err != nil {
		return fmt.Errorf("failed to export custom post types: %w", err)
	}

	// And the shop's catalog, which was fetched and counted and written nowhere
	// until a reporter looked for it (#65).
	if err := e.exportProductsMarkdown(data.Products); err != nil {
		return fmt.Errorf("failed to export products: %w", err)
	}

	// Export metadata
	if err := e.exportMetadata(data); err != nil {
		return fmt.Errorf("failed to export metadata: %w", err)
	}

	// Reader comments are records, not documents — one comments.json beside
	// metadata.json, addressed by page URL (#35).
	if err := e.exportComments(data.Comments); err != nil {
		return fmt.Errorf("failed to export comments: %w", err)
	}

	fmt.Printf("Export completed: %s\n", e.config.Output)
	return nil
}

// exportCustomTypesMarkdown writes each custom post type under pages/, in a
// directory named after the WordPress type slug: pages/cpt_services/wms.md.
//
// Under pages/ because that is where a consumer looks for URL-addressable
// content — a generator reading this export walks pages/ recursively and would
// never find a top-level cpt_services/ — and in its own directory so the type
// stays visible as itself rather than dissolving into the page list (#28).
func (e *Exporter) exportCustomTypesMarkdown(sets []models.CustomTypeSet) error {
	for _, set := range sets {
		dir := filepath.Join(e.config.Output, "pages", sanitizePathSegment(set.Slug))
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", set.Slug, err)
		}
		if err := e.exportPostsMarkdown(set.Posts, dir, set.Slug); err != nil {
			return err
		}
	}

	return nil
}

// exportSiteInfo exports site information as markdown
func (e *Exporter) exportSiteInfo(site models.SiteInfo) error {
	content := fmt.Sprintf(`# %s

**Description:** %s  
**URL:** %s  
**Home URL:** %s  
**Admin Email:** %s  
**Timezone:** %s  
**Language:** %s  

---

*Exported on %s*
`,
		site.Name,
		site.Description,
		site.URL,
		site.HomeURL,
		site.AdminEmail,
		site.Timezone,
		site.Language,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	filePath := filepath.Join(e.config.Output, "README.md")
	return os.WriteFile(filePath, []byte(content), 0600)
}

// exportPostsWithCategories exports posts organized by category folders
func (e *Exporter) exportPostsWithCategories(posts []models.WordPressPost, categories []models.WordPressCategory, contentType string) error {
	// Create category map for quick lookup
	categoryMap := make(map[int]models.WordPressCategory)
	for _, cat := range categories {
		categoryMap[cat.ID] = cat
	}

	// Create category hierarchy map
	categoryHierarchy := e.buildCategoryHierarchy(categories)

	for _, post := range posts {
		// Determine the category path for this post
		categoryPath := e.getCategoryPath(post, categoryMap, categoryHierarchy)

		// Create the full directory path
		postDir := filepath.Join(e.config.Output, "posts", categoryPath)
		if err := os.MkdirAll(postDir, 0750); err != nil {
			return fmt.Errorf("failed to create category directory %s: %w", postDir, err)
		}

		// Generate filename and content
		filename := e.generateMarkdownFilename(post)
		filePath := filepath.Join(postDir, filename)
		content := e.generateMarkdownContent(post, contentType)

		if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write %s file %s: %w", contentType, filename, err)
		}
	}

	return nil
}

// exportPostsMarkdown exports posts/pages as markdown files
// exportPagesMarkdown writes every page under the path its URL states, so
// /zerowisko/znaczenie/ becomes pages/zerowisko/znaczenie.md and stops sharing
// a file with the unrelated top-level /znaczenie/ (#38).
//
// The placement — and therefore the collision report — is the one the SSG
// format already used; the two formats disagreeing about where a page lives was
// the whole defect. Pages written are counted so the summary can state them
// against pages fetched rather than assume the two match.
func (e *Exporter) exportPagesMarkdown(data *models.ExportData) error {
	placement := newPagePlacement()

	for _, page := range data.Pages {
		dir, filename := ssgPageLocation(e.config.Output, page, e.generateMarkdownFilename(page))
		filename = placement.claim(dir, filename, page.ID)

		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create page directory %s: %w", dir, err)
		}

		content := e.generateMarkdownContent(page, "page")
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write page file %s: %w", filename, err)
		}

		placement.recordWrite()
	}

	data.Stats.PagesWritten = placement.written
	e.reportCollisions(placement)
	e.notePostLoopPages(data)

	return nil
}

// reportCollisions states every renamed document. Two pages competing for one
// file is not a detail to leave in the tree for someone to notice later: it
// means an address on the live site has no document of its own here.
func (e *Exporter) reportCollisions(placement *pagePlacement) {
	if e.config.Quiet {
		return
	}

	for _, collision := range placement.report() {
		fmt.Printf("Warning: %s\n", collision)
	}
}

func (e *Exporter) exportPostsMarkdown(posts []models.WordPressPost, dir, contentType string) error {
	for _, post := range posts {
		filename := e.generateMarkdownFilename(post)
		filePath := filepath.Join(dir, filename)

		content := e.generateMarkdownContent(post, contentType)

		if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write %s file %s: %w", contentType, filename, err)
		}
	}

	return nil
}

// generateMarkdownFilename generates a filename for a markdown file
func (e *Exporter) generateMarkdownFilename(post models.WordPressPost) string {
	// Use only slug for filename (no date)
	slug := post.Slug

	if slug == "" {
		slug = fmt.Sprintf("post-%d", post.ID)
	}

	// Sanitize slug
	slug = strings.ReplaceAll(slug, "/", "-")
	slug = strings.ReplaceAll(slug, "\\", "-")
	slug = strings.ReplaceAll(slug, ":", "-")

	return fmt.Sprintf("%s.md", slug)
}

// buildCategoryHierarchy creates a map of category ID to its parent path
func (e *Exporter) buildCategoryHierarchy(categories []models.WordPressCategory) map[int][]string {
	hierarchy := make(map[int][]string)
	categoryMap := make(map[int]models.WordPressCategory)

	// Create category lookup map
	for _, cat := range categories {
		categoryMap[cat.ID] = cat
	}

	// Build hierarchy paths
	var buildPath func(int) []string
	buildPath = func(catID int) []string {
		if path, exists := hierarchy[catID]; exists {
			return path
		}

		cat, exists := categoryMap[catID]
		if !exists {
			return []string{}
		}

		var path []string
		if cat.Parent > 0 {
			parentPath := buildPath(cat.Parent)
			path = append(parentPath, e.sanitizeDirectoryName(cat.Slug))
		} else {
			path = []string{e.sanitizeDirectoryName(cat.Slug)}
		}

		hierarchy[catID] = path
		return path
	}

	// Build paths for all categories
	for _, cat := range categories {
		buildPath(cat.ID)
	}

	return hierarchy
}

// getCategoryPath determines the directory path for a post based on its categories
func (e *Exporter) getCategoryPath(post models.WordPressPost, categoryMap map[int]models.WordPressCategory, hierarchy map[int][]string) string {
	// First, try to extract categories from the post link
	if linkCategories := e.extractCategoriesFromLink(post.Link); linkCategories != "" {
		return linkCategories
	}

	if len(post.Categories) == 0 {
		return uncategorizedDir
	}

	// Use the first category for the primary path
	primaryCategoryID := post.Categories[0]

	if path, exists := hierarchy[primaryCategoryID]; exists && len(path) > 0 {
		categoryPath := filepath.Join(path...)

		// Skip generic "posts" category and use uncategorized instead
		if categoryPath == "posts" {
			return uncategorizedDir
		}

		return categoryPath
	}

	// Fallback to category slug if hierarchy lookup fails
	if cat, exists := categoryMap[primaryCategoryID]; exists {
		slug := e.sanitizeDirectoryName(cat.Slug)

		// Skip generic "posts" category
		if slug == "posts" {
			return uncategorizedDir
		}

		return slug
	}

	return uncategorizedDir
}

// sanitizeDirectoryName sanitizes a string for use as a directory name
func (e *Exporter) sanitizeDirectoryName(name string) string {
	// Replace invalid characters with hyphens
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	sanitized := name

	for _, char := range invalid {
		sanitized = strings.ReplaceAll(sanitized, char, "-")
	}

	// Remove multiple consecutive hyphens
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "category"
	}

	return sanitized
}

// extractCategoriesFromLink extracts category path from WordPress permalink structure
func (e *Exporter) extractCategoriesFromLink(link string) string {
	if link == "" {
		return ""
	}

	// Parse the URL to get the path
	parsedURL, err := url.Parse(link)
	if err != nil {
		return ""
	}

	path := strings.Trim(parsedURL.Path, "/")
	if path == "" {
		return ""
	}

	// Split the path into segments
	segments := strings.Split(path, "/")

	// Common WordPress permalink structures:
	// 1. /%category%/%postname%/
	// 2. /%category%/%subcategory%/%postname%/
	// 3. /%year%/%monthnum%/%day%/%postname%/
	// 4. /%postname%/ (no categories)

	// If there's only one segment, it's likely just the post slug
	if len(segments) <= 1 {
		return ""
	}

	// Check if the last segment looks like a post slug (no file extension, reasonable length)
	_ = segments[len(segments)-1] // Last segment is the post slug

	// Skip if it looks like a date-based permalink (YYYY/MM/DD structure)
	if len(segments) >= 3 {
		// Check if first three segments are numeric (year/month/day)
		if e.isNumeric(segments[0]) && e.isNumeric(segments[1]) && e.isNumeric(segments[2]) {
			return ""
		}
	}

	// Extract category segments (all but the last one, which should be the post slug)
	categorySegments := segments[:len(segments)-1]

	// Filter out common non-category segments
	var validCategories []string
	for _, segment := range categorySegments {
		// Skip numeric segments (likely dates)
		if e.isNumeric(segment) {
			continue
		}

		// Skip common WordPress segments that aren't categories (but keep 'news' as it's often a valid category)
		if segment == "blog" || segment == "posts" || segment == "archives" {
			continue
		}

		// Sanitize and add valid category segments
		sanitized := e.sanitizeDirectoryName(segment)
		if sanitized != "" && sanitized != "category" {
			validCategories = append(validCategories, sanitized)
		}
	}

	// Return the category path
	if len(validCategories) > 0 {
		return filepath.Join(validCategories...)
	}

	return ""
}

// isNumeric checks if a string contains only digits
func (e *Exporter) isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// generateMarkdownContent generates markdown content for a post
func (e *Exporter) generateMarkdownContent(post models.WordPressPost, contentType string) string {
	var builder strings.Builder

	// Front matter
	builder.WriteString("---\n")
	builder.WriteString(fmt.Sprintf("id: %d\n", post.ID))
	builder.WriteString(fmt.Sprintf("title: \"%s\"\n", e.escapeYAML(plainText(post.Title.Rendered))))
	builder.WriteString(fmt.Sprintf("slug: \"%s\"\n", post.Slug))
	builder.WriteString(fmt.Sprintf("date: %s\n", post.Date.Format("2006-01-02T15:04:05Z07:00")))
	builder.WriteString(fmt.Sprintf("modified: %s\n", post.Modified.Format("2006-01-02T15:04:05Z07:00")))
	builder.WriteString(fmt.Sprintf("status: \"%s\"\n", post.Status))
	builder.WriteString(fmt.Sprintf("type: \"%s\"\n", contentType))
	builder.WriteString(fmt.Sprintf("link: \"%s\"\n", post.Link))

	// A pinned post says so. WordPress lets an editor put a post at the top of
	// the blog, and a listing sorted by date alone drops it wherever its date
	// falls — sixth, on the site that reported this. Omitted when false, so it
	// appears only where the editor asked for it (#51).
	if post.Sticky {
		builder.WriteString("sticky: true\n")
	}

	// A page whose body is a listing element says so, so a target points its
	// own archive at this address instead of migrating a page over it (#41).
	e.writePostLoopFrontMatter(&builder, post, contentType)

	// A child page states its parent, so a consumer can rebuild the tree the
	// URL implies without re-deriving it from paths (#38). The slug travels
	// with the ID because an ID means nothing after a migration.
	if post.Parent > 0 {
		if !e.config.NoIDs {
			builder.WriteString(fmt.Sprintf("parent: %d\n", post.Parent))
		}
		if slug, ok := e.pageSlugs[post.Parent]; ok && slug != "" {
			builder.WriteString(fmt.Sprintf("parent_slug: \"%s\"\n", e.escapeYAML(slug)))
		}
	}

	if post.Author > 0 {
		if name, ok := e.userMap[post.Author]; ok && name != "" {
			builder.WriteString(fmt.Sprintf("author: \"%s\"\n", e.escapeYAML(plainText(name))))
		}
		if !e.config.NoIDs {
			builder.WriteString(fmt.Sprintf("author_id: %d\n", post.Author))
		}
	}

	if post.FeaturedMedia > 0 {
		if url, ok := e.mediaMap[post.FeaturedMedia]; ok && url != "" {
			builder.WriteString(fmt.Sprintf("featured_image: \"%s\"\n", e.escapeYAML(url)))
		}
		if !e.config.NoIDs {
			builder.WriteString(fmt.Sprintf("featured_image_id: %d\n", post.FeaturedMedia))
		}
	}

	if len(post.Categories) > 0 {
		// Collect category names
		var categoryNames []string
		for _, catID := range post.Categories {
			if name, ok := e.categoryMap[catID]; ok && name != "" {
				categoryNames = append(categoryNames, name)
			}
		}
		// Output names only if we have any
		if len(categoryNames) > 0 {
			builder.WriteString("categories:\n")
			for _, name := range categoryNames {
				builder.WriteString(fmt.Sprintf("  - \"%s\"\n", e.escapeYAML(plainText(name))))
			}
		}

		// The addresses those names are published under. A target that makes a
		// slug out of a display name gets it wrong wherever WordPress did not,
		// and every archive it publishes then 404s (#45).
		e.writeTermAddresses(&builder, e.categoryIDs.identities(post.Categories), "category")

		// Output IDs unless --no-ids
		if !e.config.NoIDs {
			builder.WriteString("category_ids:\n")
			for _, catID := range post.Categories {
				builder.WriteString(fmt.Sprintf("  - %d\n", catID))
			}
		}
	}

	if len(post.Tags) > 0 {
		// Collect tag names
		var tagNames []string
		for _, tagID := range post.Tags {
			if name, ok := e.tagMap[tagID]; ok && name != "" {
				tagNames = append(tagNames, name)
			}
		}
		// Output names only if we have any
		if len(tagNames) > 0 {
			builder.WriteString("tags:\n")
			for _, name := range tagNames {
				builder.WriteString(fmt.Sprintf("  - \"%s\"\n", e.escapeYAML(plainText(name))))
			}
		}

		e.writeTermAddresses(&builder, e.tagIDs.identities(post.Tags), "tag")
		// Output IDs unless --no-ids
		if !e.config.NoIDs {
			builder.WriteString("tag_ids:\n")
			for _, tagID := range post.Tags {
				builder.WriteString(fmt.Sprintf("  - %d\n", tagID))
			}
		}
	}

	// SEO fields (if crawled via --assisted-crawl)
	e.writeSEOFrontMatter(&builder, post.SEO)

	// Excerpt in frontmatter, reduced to plain text: the theme's "Continue
	// reading" anchor is navigation rather than content and otherwise ends up in
	// <meta name="description">. Kept even with --ssg-sections so consumers that
	// read the frontmatter key still get it.
	excerptText := plainTextExcerpt(post.Excerpt.Rendered, e.readMore)
	if excerptText != "" {
		builder.WriteString(fmt.Sprintf("excerpt: \"%s\"\n", e.escapeYAML(excerptText)))
	}

	builder.WriteString("---\n\n")

	body := e.convertHTMLToMarkdown(post.Content.Rendered)

	if e.config.SSGSections {
		// ssg fills page.Excerpt/page.Content from these section markers, not from
		// the frontmatter key; the frontmatter title becomes the page heading, so a
		// body `# Title` would render a second H1 (issue #20).
		if excerptText != "" {
			builder.WriteString("## Excerpt\n\n")
			builder.WriteString(excerptText)
			builder.WriteString("\n\n")
		}
		builder.WriteString("## Content\n\n")
		builder.WriteString(body)
	} else {
		// Title as a leading H1 for plain-Markdown consumers; entities decoded so
		// the heading renders text, not `&#8211;` (issue #23).
		builder.WriteString(fmt.Sprintf("# %s\n\n", plainText(post.Title.Rendered)))
		builder.WriteString(body)
	}

	return builder.String()
}

// exportMetadata exports categories, tags, users, and media as JSON
func (e *Exporter) exportMetadata(data *models.ExportData) error {
	jsonData, err := exportMetadataJSON(data)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	filePath := filepath.Join(e.config.Output, "metadata.json")
	return os.WriteFile(filePath, jsonData, 0600)
}

// updateMediaPaths localizes attachment URLs across all exported content.
//
// It runs before the format switch, so the rewriter it builds is also what
// exportMarkdown later uses for featured_image.
func (e *Exporter) updateMediaPaths(data *models.ExportData) {
	if !e.config.DownloadMedia {
		return
	}

	// Built once and reused for every field: indexing the attachment list is
	// O(media) and would otherwise be repeated for each post.
	e.rewriter = e.downloader.NewURLRewriter(data.Media)

	// Fetch what the library never listed, before anything is rewritten, so the
	// pass below resolves those references too (#30).
	e.salvageUnlistedMedia(data)

	e.localizeMarketing(data.Marketing)

	for i := range data.Posts {
		e.localizePostMedia(&data.Posts[i])
	}

	for i := range data.Pages {
		e.localizePostMedia(&data.Pages[i])
	}

	// Custom post types carry the same images as everything else; skipping them
	// would leave a Services entry pointing at the old host (#28).
	for t := range data.CustomTypes {
		for i := range data.CustomTypes[t].Posts {
			e.localizePostMedia(&data.CustomTypes[t].Posts[i])
		}
	}
}

// localizePostMedia localizes every attachment reference a post carries: body
// content, excerpt, and the og:image scraped from the live page.
//
// canonical_url, link and hreflangs are deliberately left absolute — they are
// addresses of the source site rather than assets, and a consumer needs them to
// derive the target URL. og:image is scraped rather than read from the media
// library, so one pointing at a CDN or a third-party host resolves to nothing in
// the index and correctly stays absolute.
func (e *Exporter) localizePostMedia(post *models.WordPressPost) {
	post.Content.Rendered = e.rewriter.Rewrite(post.Content.Rendered)
	post.Excerpt.Rendered = e.rewriter.Rewrite(post.Excerpt.Rendered)
	e.localizeSEOMedia(&post.SEO)
}

// localizeSEOMedia rewrites every attachment reference the crawled metadata
// carries. og:image was the only one handled before, so twitter:image, the meta
// map (msapplication-TileImage and the like) and the JSON-LD blocks kept pointing
// at the source host even for files that had been downloaded (#30).
//
// The rewriter replaces only what resolves to an exported attachment, so the page
// addresses that also appear in JSON-LD pass through untouched.
func (e *Exporter) localizeSEOMedia(seo *models.SEOData) {
	seo.OGImage = e.rewriter.Rewrite(seo.OGImage)
	seo.TwitterImage = e.rewriter.Rewrite(seo.TwitterImage)

	for key, value := range seo.Meta {
		seo.Meta[key] = e.rewriter.Rewrite(value)
	}

	for i, block := range seo.JSONLD {
		seo.JSONLD[i] = e.rewriter.Rewrite(block)
	}
}

// localizeMarketing rewrites the site-wide brand assets. They sit in the head of
// every page, so leaving them absolute makes the whole migrated site depend on the
// source host serving its favicon (#30).
func (e *Exporter) localizeMarketing(marketing *models.SiteMarketing) {
	if marketing == nil {
		return
	}

	marketing.OGImage = e.rewriter.Rewrite(marketing.OGImage)
	marketing.Favicon = e.rewriter.Rewrite(marketing.Favicon)
	marketing.AppleTouchIcon = e.rewriter.Rewrite(marketing.AppleTouchIcon)
	marketing.Logo = e.rewriter.Rewrite(marketing.Logo)
}

// salvageUnlistedMedia fetches the same-host assets that content and metadata
// reference but /wp/v2/media never listed — page-builder renditions, deleted
// attachments, brand assets — and registers them so the rewrite pass below
// resolves them like any other attachment (#30).
func (e *Exporter) salvageUnlistedMedia(data *models.ExportData) {
	collector := e.rewriter.NewAssetCollector()

	scanPost := func(post *models.WordPressPost) {
		collector.Scan(post.Content.Rendered)
		collector.Scan(post.Excerpt.Rendered)
		collector.Scan(post.SEO.OGImage)
		collector.Scan(post.SEO.TwitterImage)
		for _, value := range post.SEO.Meta {
			collector.Scan(value)
		}
		for _, block := range post.SEO.JSONLD {
			collector.Scan(block)
		}
	}

	for i := range data.Posts {
		scanPost(&data.Posts[i])
	}
	for i := range data.Pages {
		scanPost(&data.Pages[i])
	}
	for t := range data.CustomTypes {
		for i := range data.CustomTypes[t].Posts {
			scanPost(&data.CustomTypes[t].Posts[i])
		}
	}

	if data.Marketing != nil {
		collector.Scan(data.Marketing.OGImage)
		collector.Scan(data.Marketing.Favicon)
		collector.Scan(data.Marketing.AppleTouchIcon)
		collector.Scan(data.Marketing.Logo)
	}

	targets := collector.Assets()
	if len(targets) == 0 {
		return
	}

	if !e.config.Quiet {
		fmt.Printf("Salvaging %d media files referenced but not in the media library...\n", len(targets))
	}

	saved := e.downloader.SalvageAssets(e.rewriter, targets)

	if !e.config.Quiet {
		fmt.Printf("Salvaged %d/%d files\n", saved, len(targets))
	}

	data.Stats.MediaDownloaded += saved
}

// localizeMediaURL localizes a single attachment URL, leaving it untouched when
// the export is configured to keep original URLs.
func (e *Exporter) localizeMediaURL(rawURL string) string {
	if e.rewriter == nil {
		return rawURL
	}

	return e.rewriter.Rewrite(rawURL)
}

// updateLinkPaths converts same-host address fields to root-relative paths when
// --link-style root is set.
//
// These are addresses of the source site rather than assets, so the default
// keeps them absolute: a consumer needs the original URL to derive the target
// one. A site rebuilt at the same paths wants the root-relative form instead,
// because that preserves each URL (and its search ranking) on the new host
// without pinning the content to the old one.
func (e *Exporter) updateLinkPaths(data *models.ExportData) {
	if e.config.EffectiveLinkStyle() != "root" {
		return
	}

	for i := range data.Posts {
		e.rootRelativizeAddresses(&data.Posts[i], "")
	}

	for i := range data.Pages {
		e.rootRelativizeAddresses(&data.Pages[i], "")
	}

	// A custom type's entries are addressed like pages, and their SEO-visible
	// URLs are exactly what the migration has to preserve (#28). The type slug
	// is carried in so an entry WordPress never gave a pretty permalink can be
	// given one (#78).
	for t := range data.CustomTypes {
		for i := range data.CustomTypes[t].Posts {
			e.rootRelativizeAddresses(&data.CustomTypes[t].Posts[i], data.CustomTypes[t].Slug)
		}
	}

	// Menu links must match the exported permalinks, or the rebuilt navigation
	// points at the old host while the content it links to does not.
	for i := range data.Menus {
		for j := range data.Menus[i].Items {
			data.Menus[i].Items[j].URL = e.rootRelativeURL(data.Menus[i].Items[j].URL)
		}
	}
}

// rootRelativizeAddresses converts every address field a post carries.
//
// link is the document's own published address, so a permalink that carries no
// path is replaced by the one the export files the document at (#78). Canonical
// and hreflang keep their query: those name a document on the source site,
// where the query is genuinely the address.
func (e *Exporter) rootRelativizeAddresses(post *models.WordPressPost, typeSlug string) {
	post.Link = e.rootRelativeURL(post.Link)
	if synthesized, ok := synthesizePath(post.Link, post.Slug, typeSlug); ok {
		post.Link = synthesized
	}

	post.SEO.CanonicalURL = e.rootRelativeURL(post.SEO.CanonicalURL)

	for i := range post.SEO.Hreflangs {
		post.SEO.Hreflangs[i].Href = e.rootRelativeURL(post.SEO.Hreflangs[i].Href)
	}
}

// synthesizePath builds the address of a document whose permalink carries none.
//
// A post type registered without a rewrite rule keeps WordPress's query-string
// form, /?modula-gallery=1289, whose path is "/" and whose whole meaning is in
// the query. Every consumer that routes on paths reads such a document as the
// site root, so two of them overwrite the exported front page (#78). There is
// no SEO-visible address to preserve in that case — the export has to give the
// document one, and it gives it the address it files it at: /<type>/<slug>/ for
// a custom post type, /<slug>/ for a page or post.
//
// It reports false, leaving the caller's address untouched, when the permalink
// already carries a path, when it addresses a foreign host, when it has no
// query to have hidden the address in — the front page's own "/" is a real
// address, not a missing one — or when there is no slug to build from.
func synthesizePath(link, slug, typeSlug string) (string, bool) {
	// A foreign host is not ours to rewrite, whatever shape its address has.
	if parsed, err := url.Parse(link); err != nil || parsed.Host != "" {
		return "", false
	}

	if !models.QueryOnlyAddress(link) {
		return "", false
	}

	segments := append(nonEmptySegments(typeSlug), nonEmptySegments(slug)...)
	if len(segments) == 0 {
		return "", false
	}

	return "/" + strings.Join(segments, "/") + "/", true
}

// rootRelativeURL strips scheme and host from a same-host address, preserving
// query and fragment. A URL on a foreign host is returned unchanged — an
// external canonical or hreflang alternate must keep pointing where it points.
func (e *Exporter) rootRelativeURL(rawURL string) string {
	if rawURL == "" || !e.config.IsSameHost(rawURL) {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Path == "" {
		return rawURL
	}

	relative := parsed.Path
	if parsed.RawQuery != "" {
		relative += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		relative += "#" + parsed.Fragment
	}

	return relative
}

// escapeYAML escapes special characters for YAML
func (e *Exporter) escapeYAML(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// convertHTMLToMarkdown converts a post's rendered HTML to Markdown. The
// implementation lives in htmlToMarkdown (markdown.go); it is attribute-aware and
// strips any leftover tags so Gutenberg block markup does not survive half-converted
// (issue #21).
func (e *Exporter) convertHTMLToMarkdown(html string) string {
	return htmlToMarkdownKeeping(html, e.preserveRules())
}

// preserveRules are what this export keeps as HTML.
//
// --preserve-classes and --preserve-ids name elements explicitly; they applied
// only to --flat-html and --basic-html until #67. On top of them,
// --preserve-styling decides how much a conversion holds on to when it has
// nowhere to put a class: a heading whose classes mean something (auto),
// nothing (none), or every element that carries one (all). What counts as
// meaningless is extended per site with --boilerplate-classes.
func (e *Exporter) preserveRules() preserveRules {
	if e.config == nil {
		return preserveRules{}
	}

	return preserveRules{
		classes:        e.config.PreserveClasses,
		ids:            e.config.PreserveIDs,
		styledHeadings: e.config.PreserveStyling != StylingNone,
		styledAnything: e.config.PreserveStyling == StylingAll,
		ignored:        compileClassPatterns(e.config.BoilerplateClasses),
	}
}

// exportShopify exports data as Shopify-compatible CSV
func (e *Exporter) exportShopify(data *models.ExportData) error {
	shopifyExporter := NewShopifyExporter(e.config)

	// Export products to CSV
	if err := shopifyExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Shopify products: %w", err)
	}

	// Export metadata
	if err := shopifyExporter.ExportMetadata(data); err != nil {
		return fmt.Errorf("failed to export Shopify metadata: %w", err)
	}

	return nil
}

// exportMagento exports data as Magento-compatible CSV
func (e *Exporter) exportMagento(data *models.ExportData) error {
	magentoExporter := NewMagentoExporter(e.config)

	// Export products to CSV
	if err := magentoExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Magento products: %w", err)
	}

	// Export metadata
	if err := magentoExporter.ExportMetadata(data); err != nil {
		return fmt.Errorf("failed to export Magento metadata: %w", err)
	}

	return nil
}

// exportWordPress exports data as WordPress WXR (WordPress eXtended RSS) XML
func (e *Exporter) exportWordPress(data *models.ExportData) error {
	wpExporter := NewWordPressExporter(e.config)

	if err := wpExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export WordPress WXR: %w", err)
	}

	return nil
}

// exportDrupal exports data as Drupal-compatible JSON
func (e *Exporter) exportDrupal(data *models.ExportData) error {
	drupalExporter := NewDrupalExporter(e.config)

	if err := drupalExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Drupal JSON: %w", err)
	}

	return nil
}

// exportWix exports data as Wix-compatible JSON
func (e *Exporter) exportWix(data *models.ExportData) error {
	wixExporter := NewWixExporter(e.config)

	if err := wixExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Wix JSON: %w", err)
	}

	return nil
}

// exportSquarespace exports data as Squarespace-compatible XML
func (e *Exporter) exportSquarespace(data *models.ExportData) error {
	sqExporter := NewSquarespaceExporter(e.config)

	if err := sqExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Squarespace XML: %w", err)
	}

	return nil
}

// exportWebflow exports data as Webflow-compatible CSV
func (e *Exporter) exportWebflow(data *models.ExportData) error {
	webflowExporter := NewWebflowExporter(e.config)

	if err := webflowExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Webflow CSV: %w", err)
	}

	return nil
}

// exportWeebly exports data as Weebly-compatible XML/JSON
func (e *Exporter) exportWeebly(data *models.ExportData) error {
	weeblyExporter := NewWeeblyExporter(e.config)

	if err := weeblyExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Weebly: %w", err)
	}

	return nil
}

// exportPrestaShop exports data as PrestaShop-compatible CSV
func (e *Exporter) exportPrestaShop(data *models.ExportData) error {
	psExporter := NewPrestaShopExporter(e.config)

	if err := psExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export PrestaShop CSV: %w", err)
	}

	return nil
}

// exportGhost exports data as Ghost-compatible JSON
func (e *Exporter) exportGhost(data *models.ExportData) error {
	ghostExporter := NewGhostExporter(e.config)

	if err := ghostExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Ghost JSON: %w", err)
	}

	return nil
}

// exportStrapi exports data as Strapi-compatible JSON
func (e *Exporter) exportStrapi(data *models.ExportData) error {
	strapiExporter := NewStrapiExporter(e.config)

	if err := strapiExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Strapi JSON: %w", err)
	}

	return nil
}

// exportContentful exports data as Contentful-compatible JSON
func (e *Exporter) exportContentful(data *models.ExportData) error {
	contentfulExporter := NewContentfulExporter(e.config)

	if err := contentfulExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Contentful JSON: %w", err)
	}

	return nil
}
