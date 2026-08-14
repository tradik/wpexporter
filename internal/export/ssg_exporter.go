package export

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/tradik/wpexporter/pkg/models"
)

// exportSSG writes a drop-in content source for a static site generator.
//
// It differs from the markdown format in four ways, each of which the markdown
// format keeps for backwards compatibility:
//
//   - front matter uses the generator's vocabulary (title/description/category),
//     not WordPress's three spellings of the same string
//   - pages are nested to mirror their URL, so /a/b/ becomes pages/a/b.md
//   - body content is cleaned: entities decoded, alt text filled in, WordPress
//     presentation classes dropped
//   - addresses are root-relative, so the content is not pinned to the old host
func (e *Exporter) exportSSG(data *models.ExportData) error {
	e.buildLookupMaps(data)

	if err := e.exportSSGPosts(data); err != nil {
		return fmt.Errorf("failed to export posts: %w", err)
	}

	if err := e.exportSSGPages(data.Pages); err != nil {
		return fmt.Errorf("failed to export pages: %w", err)
	}

	if err := e.exportSSGCustomTypes(data.CustomTypes); err != nil {
		return fmt.Errorf("failed to export custom post types: %w", err)
	}

	if err := e.exportMetadata(data); err != nil {
		return fmt.Errorf("failed to export metadata: %w", err)
	}

	if err := e.exportComments(data.Comments); err != nil {
		return fmt.Errorf("failed to export comments: %w", err)
	}

	if !e.config.Quiet {
		fmt.Printf("Export completed: %s\n", e.config.Output)
	}

	return nil
}

// exportSSGPosts writes each post under posts/<category>/<slug>.md.
func (e *Exporter) exportSSGPosts(data *models.ExportData) error {
	categoryMap := make(map[int]models.WordPressCategory, len(data.Categories))
	for _, category := range data.Categories {
		categoryMap[category.ID] = category
	}

	hierarchy := e.buildCategoryHierarchy(data.Categories)

	for _, post := range data.Posts {
		// getCategoryPath always yields at least uncategorizedDir, which is what
		// keeps every post at least one directory below posts/ as ssg requires.
		categoryPath := e.getCategoryPath(post, categoryMap, hierarchy)

		dir := filepath.Join(e.config.Output, "posts", filepath.FromSlash(categoryPath))
		if err := e.writeSSGDocument(post, dir, e.generateMarkdownFilename(post), "post"); err != nil {
			return err
		}
	}

	return nil
}

// exportSSGPages writes each page nested to mirror its URL, so a page at
// /baby-water-instructor/cost/ becomes pages/baby-water-instructor/cost.md and
// keeps the site's information architecture visible in the file tree.
func (e *Exporter) exportSSGPages(pages []models.WordPressPost) error {
	for _, page := range pages {
		dir, filename := ssgPageLocation(e.config.Output, page, e.generateMarkdownFilename(page))
		if err := e.writeSSGDocument(page, dir, filename, "page"); err != nil {
			return err
		}
	}

	return nil
}

// exportSSGCustomTypes writes a theme's own content types alongside the pages,
// nested by their published URL exactly as pages are (#28).
//
// They land under pages/ rather than in a directory of their own because that
// is where a generator looks for URL-addressable content, and their SEO-visible
// addresses are the whole point of carrying them over: /services/wms/ stays
// /services/wms/. The document keeps its WordPress type in front matter, so a
// theme can still tell a Service from a Page.
func (e *Exporter) exportSSGCustomTypes(sets []models.CustomTypeSet) error {
	for _, set := range sets {
		for _, entry := range set.Posts {
			dir, filename := ssgPageLocation(e.config.Output, entry, e.generateMarkdownFilename(entry))
			if err := e.writeSSGDocument(entry, dir, filename, set.Slug); err != nil {
				return err
			}
		}
	}

	return nil
}

// ssgPageLocation resolves the directory and filename that mirror a document's
// published URL. Shared by pages and custom post types, which are addressed the
// same way.
func ssgPageLocation(output string, post models.WordPressPost, defaultName string) (dir, filename string) {
	nested := pageURLPath(post)

	dir = filepath.Join(output, "pages")
	filename = defaultName

	if len(nested) > 0 {
		dir = filepath.Join(dir, filepath.FromSlash(path.Join(nested[:len(nested)-1]...)))
		filename = nested[len(nested)-1] + ".md"
	}

	return dir, filename
}

// pageURLPath splits a page's link into its path segments, falling back to the
// slug when the link is missing or carries no path.
func pageURLPath(page models.WordPressPost) []string {
	parsed, err := url.Parse(page.Link)
	if err != nil {
		return nonEmptySegments(page.Slug)
	}

	segments := nonEmptySegments(parsed.Path)
	if len(segments) == 0 {
		return nonEmptySegments(page.Slug)
	}

	return segments
}

// nonEmptySegments splits a path and drops empty and traversal segments, so a
// crafted link can never write outside the export directory.
func nonEmptySegments(rawPath string) []string {
	var segments []string

	for _, segment := range strings.Split(rawPath, "/") {
		segment = strings.TrimSpace(segment)
		if segment == "" || segment == "." || segment == ".." {
			continue
		}

		segments = append(segments, sanitizePathSegment(segment))
	}

	return segments
}

// sanitizePathSegment strips separators and drive characters from one segment.
func sanitizePathSegment(segment string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-")

	return replacer.Replace(segment)
}

// writeSSGDocument renders and writes one document.
func (e *Exporter) writeSSGDocument(post models.WordPressPost, dir, filename, contentType string) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	filePath := filepath.Join(dir, filename)
	content := e.generateSSGContent(post, contentType)

	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write %s file %s: %w", contentType, filename, err)
	}

	return nil
}

// generateSSGContent renders one document in the generator's front-matter
// contract.
//
// Every key here is single-spelled: `title` rather than title/seo_title/og_title,
// `description` rather than meta_description/og_description, `category` rather
// than categories/category_ids. A generator reads one name per concept.
func (e *Exporter) generateSSGContent(post models.WordPressPost, contentType string) string {
	var builder strings.Builder

	builder.WriteString("---\n")
	e.writeSSGFrontMatter(&builder, post, contentType)
	builder.WriteString("---\n\n")

	builder.WriteString(e.convertHTMLToMarkdown(e.cleanContent(post.Content.Rendered)))

	return builder.String()
}

// writeSSGFrontMatter writes the front-matter body.
func (e *Exporter) writeSSGFrontMatter(builder *strings.Builder, post models.WordPressPost, contentType string) {
	writeYAMLString(builder, "title", e.escapeYAML(ssgTitle(post)))
	writeYAMLString(builder, "slug", e.escapeYAML(post.Slug))
	writeYAMLString(builder, "status", e.escapeYAML(post.Status))
	writeYAMLString(builder, "type", e.escapeYAML(contentType))
	writeYAMLString(builder, "date", post.Date.Format(time.RFC3339))
	writeYAMLString(builder, "modified", post.Modified.Format(time.RFC3339))
	writeYAMLString(builder, "link", e.escapeYAML(post.Link))
	writeYAMLString(builder, "author", e.escapeYAML(e.userMap[post.Author]))
	writeYAMLString(builder, "category", e.escapeYAML(e.primaryCategory(post)))
	writeYAMLString(builder, "description", e.escapeYAML(ssgDescription(post)))
	writeYAMLString(builder, "excerpt", e.escapeYAML(plainTextExcerpt(post.Excerpt.Rendered)))

	if post.FeaturedMedia > 0 {
		writeYAMLString(builder, "featured_image", e.escapeYAML(e.mediaMap[post.FeaturedMedia]))
	}

	// Everything else the page declared. A generator ignores keys it does not
	// recognize, but it cannot recover one the export dropped — and plugins put
	// real information in tags nobody anticipated.
	e.writeMetaMap(builder, post.SEO.Meta)
	e.writeJSONLD(builder, post.SEO.JSONLD)
}

// writeYAMLString writes one quoted key, skipping it when the value is empty so
// the generator sees an absent key rather than an empty string.
func writeYAMLString(builder *strings.Builder, key, value string) {
	if value == "" {
		return
	}

	fmt.Fprintf(builder, "%s: %q\n", key, value)
}

// ssgTitle picks the best available title, preferring the SEO title the site
// actually rendered.
func ssgTitle(post models.WordPressPost) string {
	if title := plainText(post.SEO.Title); title != "" {
		return title
	}

	return plainText(post.Title.Rendered)
}

// ssgDescription collapses WordPress's three spellings of a description into the
// one key a generator reads, preferring the most deliberate source.
func ssgDescription(post models.WordPressPost) string {
	for _, candidate := range []string{post.SEO.MetaDescription, post.SEO.OGDescription} {
		if description := plainText(candidate); description != "" {
			return description
		}
	}

	return plainTextExcerpt(post.Excerpt.Rendered)
}

// primaryCategory returns the post's first named category.
func (e *Exporter) primaryCategory(post models.WordPressPost) string {
	for _, categoryID := range post.Categories {
		if name, ok := e.categoryMap[categoryID]; ok && name != "" {
			return name
		}
	}

	return ""
}

// cleanContent applies the content transforms an SSG source needs: entities
// decoded to UTF-8, alt text filled in from the media library, WordPress
// presentation attributes dropped.
func (e *Exporter) cleanContent(content string) string {
	return decodeTypographicEntities(cleanImages(content, e.altMap))
}

// buildLookupMaps populates the ID-to-name maps the writers read, plus the alt
// text index keyed on every URL form the content may carry.
func (e *Exporter) buildLookupMaps(data *models.ExportData) {
	e.categoryMap = make(map[int]string, len(data.Categories))
	for _, category := range data.Categories {
		e.categoryMap[category.ID] = category.Name
	}

	e.tagMap = make(map[int]string, len(data.Tags))
	for _, tag := range data.Tags {
		e.tagMap[tag.ID] = tag.Name
	}

	e.userMap = make(map[int]string, len(data.Users))
	for _, user := range data.Users {
		e.userMap[user.ID] = user.Name
	}

	e.mediaMap = make(map[int]string, len(data.Media))
	e.altMap = make(map[string]string, len(data.Media)*2)

	for _, media := range data.Media {
		// Localized here so featured_image matches the body content, which the
		// rewriter has already been over.
		localized := e.localizeMediaURL(media.SourceURL)
		e.mediaMap[media.ID] = localized

		if media.AltText == "" {
			continue
		}

		// Indexed under both forms: the content may carry either, depending on
		// whether media rewriting ran.
		e.altMap[localized] = media.AltText
		e.altMap[media.SourceURL] = media.AltText
	}
}

// exportMetadataJSON marshals the metadata document shared by the markdown and
// ssg formats.
func exportMetadataJSON(data *models.ExportData) ([]byte, error) {
	metadata := map[string]interface{}{
		"site":        data.Site,
		"categories":  data.Categories,
		"tags":        data.Tags,
		"users":       data.Users,
		"media":       data.Media,
		"stats":       data.Stats,
		"exported_at": time.Now(),
	}

	// Tracking identifiers are a property of the site, so they live alongside the
	// other site-wide records rather than being repeated on every post.
	if data.Analytics != nil {
		metadata["analytics"] = data.Analytics
	}

	// Site-level marketing wiring (verification tokens, social profiles and
	// defaults, favicon/logo/theme color) so a migration can configure the target
	// instead of re-entering it by hand (#24).
	if data.Marketing != nil && !data.Marketing.IsEmpty() {
		metadata["marketing"] = data.Marketing
	}

	if len(data.Menus) > 0 {
		metadata["menus"] = data.Menus
	}

	// Which content types the site publishes beyond posts and pages, and what
	// each holds. A consumer reading only metadata.json still learns that a
	// Services section exists (#28).
	if len(data.CustomTypes) > 0 {
		metadata["custom_types"] = data.CustomTypes
	}

	return json.MarshalIndent(metadata, "", "  ")
}
