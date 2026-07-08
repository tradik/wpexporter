// Package export provides functionality for exporting WordPress content to various formats.
package export

import (
	"encoding/csv"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// safeHref escapes a URL for safe inclusion in an HTML href attribute and rejects
// non-http(s)/mailto schemes (e.g. javascript:) to prevent HTML/JS injection when
// the generated Body (HTML) is rendered in a browser (FE-001).
func safeHref(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "#"
	}
	switch strings.ToLower(u.Scheme) {
	case "", "http", "https", "mailto":
		return html.EscapeString(u.String())
	default:
		return "#"
	}
}

// ShopifyExporter handles export to Shopify-compatible CSV format.
type ShopifyExporter struct {
	config      *config.Config
	categoryMap map[int]models.WordPressCategory
	tagMap      map[int]models.WordPressTag
	userMap     map[int]models.WordPressUser
	mediaMap    map[int]models.WordPressMedia
}

// ShopifyProduct represents a single Shopify product row.
type ShopifyProduct struct {
	Handle                     string
	Title                      string
	BodyHTML                   string
	Vendor                     string
	Type                       string
	Tags                       string
	Published                  string
	Option1Name                string
	Option1Value               string
	Option2Name                string
	Option2Value               string
	Option3Name                string
	Option3Value               string
	VariantSKU                 string
	VariantGrams               string
	VariantInventoryTracker    string
	VariantInventoryQty        string
	VariantInventoryPolicy     string
	VariantFulfillmentService  string
	VariantPrice               string
	VariantCompareAtPrice      string
	VariantRequiresShipping    string
	VariantTaxable             string
	VariantBarcode             string
	ImageSrc                   string
	ImagePosition              string
	ImageAltText               string
	GiftCard                   string
	SEOTitle                   string
	SEODescription             string
	GoogleShoppingCategory     string
	GoogleShoppingGender       string
	GoogleShoppingAgeGroup     string
	GoogleShoppingMPN          string
	GoogleShoppingCondition    string
	GoogleShoppingCustomLabel0 string
	GoogleShoppingCustomLabel1 string
	GoogleShoppingCustomLabel2 string
	GoogleShoppingCustomLabel3 string
	GoogleShoppingCustomLabel4 string
	VariantImage               string
	VariantWeightUnit          string
	VariantTaxCode             string
	CostPerItem                string
	Status                     string
}

// NewShopifyExporter creates a new Shopify exporter instance.
func NewShopifyExporter(cfg *config.Config) *ShopifyExporter {
	return &ShopifyExporter{
		config:      cfg,
		categoryMap: make(map[int]models.WordPressCategory),
		tagMap:      make(map[int]models.WordPressTag),
		userMap:     make(map[int]models.WordPressUser),
		mediaMap:    make(map[int]models.WordPressMedia),
	}
}

// Export exports WordPress data to Shopify-compatible CSV format.
func (s *ShopifyExporter) Export(data *models.ExportData) error {
	// Build lookup maps for categories, tags, users, and media
	s.buildLookupMaps(data)

	// Ensure output directory exists
	if err := s.config.EnsureOutputDir(); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Export posts as products (content/blog posts)
	if len(data.Posts) > 0 {
		if err := s.exportPostsToShopify(data.Posts, "posts"); err != nil {
			return fmt.Errorf("failed to export posts: %w", err)
		}
	}

	// Export pages as products (optional, separate file)
	if len(data.Pages) > 0 {
		if err := s.exportPostsToShopify(data.Pages, "pages"); err != nil {
			return fmt.Errorf("failed to export pages: %w", err)
		}
	}

	// Export WooCommerce products if available
	if len(data.Products) > 0 {
		if err := s.exportWooProductsToShopify(data.Products, "woo_products"); err != nil {
			return fmt.Errorf("failed to export WooCommerce products: %w", err)
		}
	}

	// Export all content combined into a single products CSV
	allContent := append(data.Posts, data.Pages...)
	if err := s.exportPostsToShopify(allContent, "products"); err != nil {
		return fmt.Errorf("failed to export combined products: %w", err)
	}

	fmt.Printf("Shopify export completed: %s\n", s.config.Output)
	return nil
}

// exportWooProductsToShopify exports WooCommerce products to a Shopify CSV file.
func (s *ShopifyExporter) exportWooProductsToShopify(products []models.WooCommerceProduct, filename string) error {
	outputPath := filepath.Clean(filepath.Join(s.config.Output, fmt.Sprintf("shopify_%s.csv", filename)))

	file, err := os.Create(outputPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(s.getCSVHeaders()); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, product := range products {
		shopifyProduct := s.convertWooProductToShopifyProduct(product)
		if err := writer.Write(csvSafeRow(s.productToCSVRow(shopifyProduct))); err != nil {
			return fmt.Errorf("failed to write product row: %w", err)
		}

		// Write additional image rows
		for i, img := range product.Images {
			if i == 0 {
				continue // Skip first image as it's already the main image
			}
			imageRow := s.createImageRow(shopifyProduct.Handle, img.Src, i+1)
			if err := writer.Write(csvSafeRow(imageRow)); err != nil {
				return fmt.Errorf("failed to write image row: %w", err)
			}
		}
	}

	return nil
}

// convertWooProductToShopifyProduct converts a WooCommerce product to a Shopify product.
func (s *ShopifyExporter) convertWooProductToShopifyProduct(product models.WooCommerceProduct) ShopifyProduct {
	handle := s.generateHandle(product.Slug, product.ID)

	// Get tags from WooCommerce product tags
	var tagNames []string
	for _, tag := range product.Tags {
		tagNames = append(tagNames, tag.Name)
	}
	tags := strings.Join(tagNames, ", ")

	// Get product type from first category
	productType := "Product"
	if len(product.Categories) > 0 {
		productType = product.Categories[0].Name
	}

	// Get first image
	var imageURL, imageAlt string
	if len(product.Images) > 0 {
		imageURL = product.Images[0].Src
		imageAlt = product.Images[0].Alt
		if imageAlt == "" {
			imageAlt = product.Images[0].Name
		}
	}

	// Determine publish status
	published := "FALSE"
	status := "draft"
	if product.Status == statusPublish {
		published = "TRUE"
		status = "active"
	}

	// Get inventory quantity
	inventoryQty := "0"
	if product.StockQuantity != nil {
		switch v := product.StockQuantity.(type) {
		case float64:
			inventoryQty = fmt.Sprintf("%.0f", v)
		case int:
			inventoryQty = fmt.Sprintf("%d", v)
		}
	}

	// Calculate weight in grams
	weightGrams := "0"
	if product.Weight != "" {
		// Assuming weight is in kg, convert to grams
		weightGrams = product.Weight + "000"
	}

	return ShopifyProduct{
		Handle:                     handle,
		Title:                      product.Name,
		BodyHTML:                   s.cleanHTMLForShopify(product.Description),
		Vendor:                     "WordPress",
		Type:                       productType,
		Tags:                       tags,
		Published:                  published,
		Option1Name:                "Title",
		Option1Value:               "Default Title",
		Option2Name:                "",
		Option2Value:               "",
		Option3Name:                "",
		Option3Value:               "",
		VariantSKU:                 product.SKU,
		VariantGrams:               weightGrams,
		VariantInventoryTracker:    "shopify",
		VariantInventoryQty:        inventoryQty,
		VariantInventoryPolicy:     "deny",
		VariantFulfillmentService:  "manual",
		VariantPrice:               product.Price,
		VariantCompareAtPrice:      product.RegularPrice,
		VariantRequiresShipping:    boolToShopify(product.ShippingRequired),
		VariantTaxable:             boolToShopify(product.TaxStatus == "taxable"),
		VariantBarcode:             "",
		ImageSrc:                   imageURL,
		ImagePosition:              "1",
		ImageAltText:               imageAlt,
		GiftCard:                   "FALSE",
		SEOTitle:                   s.truncateString(product.Name, 70),
		SEODescription:             s.truncateString(product.ShortDescription, 320),
		GoogleShoppingCategory:     "",
		GoogleShoppingGender:       "",
		GoogleShoppingAgeGroup:     "",
		GoogleShoppingMPN:          "",
		GoogleShoppingCondition:    "new",
		GoogleShoppingCustomLabel0: "",
		GoogleShoppingCustomLabel1: "",
		GoogleShoppingCustomLabel2: "",
		GoogleShoppingCustomLabel3: "",
		GoogleShoppingCustomLabel4: "",
		VariantImage:               "",
		VariantWeightUnit:          "g",
		VariantTaxCode:             "",
		CostPerItem:                "",
		Status:                     status,
	}
}

// boolToShopify converts a boolean to Shopify "TRUE"/"FALSE" string.
func boolToShopify(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

// buildLookupMaps creates lookup maps for efficient access to metadata.
func (s *ShopifyExporter) buildLookupMaps(data *models.ExportData) {
	// Build category map
	for _, cat := range data.Categories {
		s.categoryMap[cat.ID] = cat
	}

	// Build tag map
	for _, tag := range data.Tags {
		s.tagMap[tag.ID] = tag
	}

	// Build user map
	for _, user := range data.Users {
		s.userMap[user.ID] = user
	}

	// Build media map
	for _, media := range data.Media {
		s.mediaMap[media.ID] = media
	}
}

// exportPostsToShopify exports posts/pages to a Shopify CSV file.
func (s *ShopifyExporter) exportPostsToShopify(posts []models.WordPressPost, filename string) error {
	// Determine output file path
	outputPath := filepath.Clean(filepath.Join(s.config.Output, fmt.Sprintf("shopify_%s.csv", filename)))

	// Create CSV file
	file, err := os.Create(outputPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header row
	if err := writer.Write(s.getCSVHeaders()); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write product rows
	for _, post := range posts {
		product := s.convertPostToShopifyProduct(post)
		if err := writer.Write(csvSafeRow(s.productToCSVRow(product))); err != nil {
			return fmt.Errorf("failed to write product row: %w", err)
		}

		// Write additional image rows if there are more images in content
		additionalImages := s.extractImagesFromContent(post.Content.Rendered)
		for i, imgURL := range additionalImages {
			if i == 0 {
				continue // Skip first image as it's already the main image
			}
			imageRow := s.createImageRow(product.Handle, imgURL, i+1)
			if err := writer.Write(csvSafeRow(imageRow)); err != nil {
				return fmt.Errorf("failed to write image row: %w", err)
			}
		}
	}

	return nil
}

// getCSVHeaders returns the Shopify CSV header row.
func (s *ShopifyExporter) getCSVHeaders() []string {
	return []string{
		"Handle",
		"Title",
		"Body (HTML)",
		"Vendor",
		"Type",
		"Tags",
		"Published",
		"Option1 Name",
		"Option1 Value",
		"Option2 Name",
		"Option2 Value",
		"Option3 Name",
		"Option3 Value",
		"Variant SKU",
		"Variant Grams",
		"Variant Inventory Tracker",
		"Variant Inventory Qty",
		"Variant Inventory Policy",
		"Variant Fulfillment Service",
		"Variant Price",
		"Variant Compare At Price",
		"Variant Requires Shipping",
		"Variant Taxable",
		"Variant Barcode",
		"Image Src",
		"Image Position",
		"Image Alt Text",
		"Gift Card",
		"SEO Title",
		"SEO Description",
		"Google Shopping / Google Product Category",
		"Google Shopping / Gender",
		"Google Shopping / Age Group",
		"Google Shopping / MPN",
		"Google Shopping / Condition",
		"Google Shopping / Custom Product",
		"Google Shopping / Custom Label 0",
		"Google Shopping / Custom Label 1",
		"Google Shopping / Custom Label 2",
		"Google Shopping / Custom Label 3",
		"Google Shopping / Custom Label 4",
		"Variant Image",
		"Variant Weight Unit",
		"Variant Tax Code",
		"Cost per item",
		"Status",
	}
}

// convertPostToShopifyProduct converts a WordPress post to a Shopify product.
func (s *ShopifyExporter) convertPostToShopifyProduct(post models.WordPressPost) ShopifyProduct {
	// Generate handle from slug
	handle := s.generateHandle(post.Slug, post.ID)

	// Get vendor from author
	vendor := s.getVendorFromAuthor(post.Author)

	// Get product type from first category
	productType := s.getProductType(post.Categories)

	// Get tags as comma-separated string
	tags := s.getTagsString(post.Tags)

	// Get category names for metadata
	categoryNames := s.getCategoryNames(post.Categories)

	// Determine publish status
	published := s.isPublished(post.Status)

	// Get featured image
	imageURL, imageAlt := s.getFeaturedImage(post.FeaturedMedia)

	// Build body HTML with metadata header and content
	metadataHTML := s.generateMetadataHTML(post, vendor, categoryNames, tags)
	contentHTML := s.cleanHTMLForShopify(post.Content.Rendered)
	bodyHTML := metadataHTML + contentHTML

	// Generate SEO fields
	seoTitle := s.truncateString(post.Title.Rendered, 70)
	seoDescription := s.generateSEODescription(post.Excerpt.Rendered, 320)

	// Determine product status
	status := s.getStatus(post.Status)

	return ShopifyProduct{
		Handle:                     handle,
		Title:                      post.Title.Rendered,
		BodyHTML:                   bodyHTML,
		Vendor:                     vendor,
		Type:                       productType,
		Tags:                       tags,
		Published:                  published,
		Option1Name:                "Title",
		Option1Value:               "Default Title",
		Option2Name:                "",
		Option2Value:               "",
		Option3Name:                "",
		Option3Value:               "",
		VariantSKU:                 fmt.Sprintf("WP-%d", post.ID),
		VariantGrams:               "0",
		VariantInventoryTracker:    "",
		VariantInventoryQty:        "0",
		VariantInventoryPolicy:     "deny",
		VariantFulfillmentService:  "manual",
		VariantPrice:               "0.00",
		VariantCompareAtPrice:      "",
		VariantRequiresShipping:    "FALSE",
		VariantTaxable:             "FALSE",
		VariantBarcode:             "",
		ImageSrc:                   imageURL,
		ImagePosition:              "1",
		ImageAltText:               imageAlt,
		GiftCard:                   "FALSE",
		SEOTitle:                   seoTitle,
		SEODescription:             seoDescription,
		GoogleShoppingCategory:     "",
		GoogleShoppingGender:       "",
		GoogleShoppingAgeGroup:     "",
		GoogleShoppingMPN:          "",
		GoogleShoppingCondition:    "new",
		GoogleShoppingCustomLabel0: "",
		GoogleShoppingCustomLabel1: "",
		GoogleShoppingCustomLabel2: "",
		GoogleShoppingCustomLabel3: "",
		GoogleShoppingCustomLabel4: "",
		VariantImage:               "",
		VariantWeightUnit:          "kg",
		VariantTaxCode:             "",
		CostPerItem:                "",
		Status:                     status,
	}
}

// productToCSVRow converts a ShopifyProduct to a CSV row.
func (s *ShopifyExporter) productToCSVRow(p ShopifyProduct) []string {
	return []string{
		p.Handle,
		p.Title,
		p.BodyHTML,
		p.Vendor,
		p.Type,
		p.Tags,
		p.Published,
		p.Option1Name,
		p.Option1Value,
		p.Option2Name,
		p.Option2Value,
		p.Option3Name,
		p.Option3Value,
		p.VariantSKU,
		p.VariantGrams,
		p.VariantInventoryTracker,
		p.VariantInventoryQty,
		p.VariantInventoryPolicy,
		p.VariantFulfillmentService,
		p.VariantPrice,
		p.VariantCompareAtPrice,
		p.VariantRequiresShipping,
		p.VariantTaxable,
		p.VariantBarcode,
		p.ImageSrc,
		p.ImagePosition,
		p.ImageAltText,
		p.GiftCard,
		p.SEOTitle,
		p.SEODescription,
		p.GoogleShoppingCategory,
		p.GoogleShoppingGender,
		p.GoogleShoppingAgeGroup,
		p.GoogleShoppingMPN,
		p.GoogleShoppingCondition,
		"", // Google Shopping Custom Product
		p.GoogleShoppingCustomLabel0,
		p.GoogleShoppingCustomLabel1,
		p.GoogleShoppingCustomLabel2,
		p.GoogleShoppingCustomLabel3,
		p.GoogleShoppingCustomLabel4,
		p.VariantImage,
		p.VariantWeightUnit,
		p.VariantTaxCode,
		p.CostPerItem,
		p.Status,
	}
}

// createImageRow creates a CSV row for an additional product image.
func (s *ShopifyExporter) createImageRow(handle, imageURL string, position int) []string {
	row := make([]string, len(s.getCSVHeaders()))
	row[0] = handle                       // Handle
	row[24] = imageURL                    // Image Src
	row[25] = fmt.Sprintf("%d", position) // Image Position
	return row
}

// generateHandle creates a URL-friendly handle from the slug.
func (s *ShopifyExporter) generateHandle(slug string, id int) string {
	if slug == "" {
		return fmt.Sprintf("product-%d", id)
	}

	// Sanitize handle: lowercase, alphanumeric and hyphens only
	handle := strings.ToLower(slug)
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	handle = reg.ReplaceAllString(handle, "-")

	// Remove consecutive hyphens
	for strings.Contains(handle, "--") {
		handle = strings.ReplaceAll(handle, "--", "-")
	}

	// Trim hyphens from start and end
	handle = strings.Trim(handle, "-")

	if handle == "" {
		return fmt.Sprintf("product-%d", id)
	}

	return handle
}

// getVendorFromAuthor retrieves the vendor name from the author ID.
func (s *ShopifyExporter) getVendorFromAuthor(authorID int) string {
	if user, exists := s.userMap[authorID]; exists {
		if user.Name != "" {
			return user.Name
		}
		return user.Slug
	}
	return "WordPress"
}

// getProductType returns the product type from categories.
func (s *ShopifyExporter) getProductType(categoryIDs []int) string {
	if len(categoryIDs) == 0 {
		return "Content"
	}

	// Use the first category as the product type
	if cat, exists := s.categoryMap[categoryIDs[0]]; exists {
		return cat.Name
	}

	return "Content"
}

// getTagsString returns a comma-separated string of tag names.
func (s *ShopifyExporter) getTagsString(tagIDs []int) string {
	if len(tagIDs) == 0 {
		return ""
	}

	var tags []string
	for _, tagID := range tagIDs {
		if tag, exists := s.tagMap[tagID]; exists {
			tags = append(tags, tag.Name)
		}
	}

	return strings.Join(tags, ", ")
}

// getCategoryNames returns a comma-separated string of category names.
func (s *ShopifyExporter) getCategoryNames(categoryIDs []int) string {
	if len(categoryIDs) == 0 {
		return ""
	}

	var categories []string
	for _, catID := range categoryIDs {
		if cat, exists := s.categoryMap[catID]; exists {
			categories = append(categories, cat.Name)
		}
	}

	return strings.Join(categories, ", ")
}

// generateMetadataHTML generates an HTML metadata section for the post content.
// Fields match the Markdown frontmatter for consistency across export formats.
func (s *ShopifyExporter) generateMetadataHTML(post models.WordPressPost, author, categories, tags string) string {
	var builder strings.Builder

	builder.WriteString("<div class=\"post-metadata\" style=\"margin-bottom: 20px; padding: 15px; ")
	builder.WriteString("background-color: #f9f9f9; border-left: 4px solid #0073aa;\">\n")

	// ID
	builder.WriteString(fmt.Sprintf("<p><strong>ID:</strong> %d</p>\n", post.ID))

	// Slug
	if post.Slug != "" {
		builder.WriteString(fmt.Sprintf("<p><strong>Slug:</strong> %s</p>\n", html.EscapeString(post.Slug)))
	}

	// Date
	if !post.Date.IsZero() {
		builder.WriteString(fmt.Sprintf("<p><strong>Date:</strong> %s</p>\n", post.Date.Format("2006-01-02T15:04:05Z07:00")))
	}

	// Modified date
	if !post.Modified.IsZero() {
		builder.WriteString(fmt.Sprintf("<p><strong>Modified:</strong> %s</p>\n", post.Modified.Format("2006-01-02T15:04:05Z07:00")))
	}

	// Status
	if post.Status != "" {
		builder.WriteString(fmt.Sprintf("<p><strong>Status:</strong> %s</p>\n", html.EscapeString(post.Status)))
	}

	// Type
	if post.Type != "" {
		builder.WriteString(fmt.Sprintf("<p><strong>Type:</strong> %s</p>\n", html.EscapeString(post.Type)))
	}

	// Link
	if post.Link != "" {
		builder.WriteString(fmt.Sprintf("<p><strong>Link:</strong> <a href=\"%s\">%s</a></p>\n",
			safeHref(post.Link), html.EscapeString(post.Link)))
	}

	// Author
	if author != "" && author != "WordPress" {
		builder.WriteString(fmt.Sprintf("<p><strong>Author:</strong> %s</p>\n", html.EscapeString(author)))
	}

	// Featured Media
	if post.FeaturedMedia > 0 {
		builder.WriteString(fmt.Sprintf("<p><strong>Featured Media:</strong> %d</p>\n", post.FeaturedMedia))
	}

	// Categories
	if categories != "" {
		builder.WriteString(fmt.Sprintf("<p><strong>Categories:</strong> %s</p>\n", html.EscapeString(categories)))
	}

	// Tags
	if tags != "" {
		builder.WriteString(fmt.Sprintf("<p><strong>Tags:</strong> %s</p>\n", html.EscapeString(tags)))
	}

	// Language
	if post.SEO.Lang != "" {
		builder.WriteString(fmt.Sprintf("<p><strong>Lang:</strong> %s</p>\n", html.EscapeString(post.SEO.Lang)))
	}

	// Hreflangs
	if len(post.SEO.Hreflangs) > 0 {
		builder.WriteString("<p><strong>Hreflangs:</strong></p>\n<ul>\n")
		for _, h := range post.SEO.Hreflangs {
			builder.WriteString(fmt.Sprintf("<li>%s: <a href=\"%s\">%s</a></li>\n",
				html.EscapeString(h.Lang), safeHref(h.Href), html.EscapeString(h.Href)))
		}
		builder.WriteString("</ul>\n")
	}

	builder.WriteString("</div>\n\n")

	return builder.String()
}

// isPublished returns "TRUE" or "FALSE" based on post status.
func (s *ShopifyExporter) isPublished(status string) string {
	if status == statusPublish {
		return "TRUE"
	}
	return "FALSE"
}

// getStatus returns the Shopify status from WordPress status.
func (s *ShopifyExporter) getStatus(wpStatus string) string {
	switch wpStatus {
	case statusPublish:
		return "active"
	case "draft":
		return "draft"
	case "pending":
		return "draft"
	case "private":
		return "draft"
	default:
		return "draft"
	}
}

// getFeaturedImage returns the featured image URL and alt text.
func (s *ShopifyExporter) getFeaturedImage(mediaID int) (string, string) {
	if media, exists := s.mediaMap[mediaID]; exists {
		altText := media.AltText
		if altText == "" {
			altText = media.Title.Rendered
		}
		return media.SourceURL, altText
	}
	return "", ""
}

// cleanHTMLForShopify cleans HTML content for Shopify compatibility.
func (s *ShopifyExporter) cleanHTMLForShopify(html string) string {
	if html == "" {
		return ""
	}

	// Remove WordPress-specific shortcodes
	shortcodePattern := regexp.MustCompile(`\[/?[^\]]+\]`)
	cleaned := shortcodePattern.ReplaceAllString(html, "")

	// Remove WordPress block comments
	blockCommentPattern := regexp.MustCompile(`<!--\s*/?wp:[^>]+-->`)
	cleaned = blockCommentPattern.ReplaceAllString(cleaned, "")

	// Remove empty paragraphs
	emptyParagraphPattern := regexp.MustCompile(`<p>\s*</p>`)
	cleaned = emptyParagraphPattern.ReplaceAllString(cleaned, "")

	// Remove multiple newlines
	multipleNewlinePattern := regexp.MustCompile(`\n{3,}`)
	cleaned = multipleNewlinePattern.ReplaceAllString(cleaned, "\n\n")

	// Trim whitespace
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// extractImagesFromContent extracts image URLs from HTML content.
func (s *ShopifyExporter) extractImagesFromContent(html string) []string {
	if html == "" {
		return nil
	}

	// Find all image sources in the content
	imgPattern := regexp.MustCompile(`<img[^>]+src\s*=\s*["']([^"']+)["']`)
	matches := imgPattern.FindAllStringSubmatch(html, -1)

	var images []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			images = append(images, match[1])
			seen[match[1]] = true
		}
	}

	return images
}

// truncateString truncates a string to a maximum length.
func (s *ShopifyExporter) truncateString(str string, maxLen int) string {
	// Clean HTML tags first
	tagPattern := regexp.MustCompile(`<[^>]+>`)
	str = tagPattern.ReplaceAllString(str, "")

	// Decode common HTML entities
	str = strings.ReplaceAll(str, "&amp;", "&")
	str = strings.ReplaceAll(str, "&lt;", "<")
	str = strings.ReplaceAll(str, "&gt;", ">")
	str = strings.ReplaceAll(str, "&quot;", "\"")
	str = strings.ReplaceAll(str, "&#39;", "'")
	str = strings.ReplaceAll(str, "&nbsp;", " ")

	// Trim whitespace
	str = strings.TrimSpace(str)

	if len(str) <= maxLen {
		return str
	}

	// Find the last space before maxLen
	truncated := str[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		truncated = truncated[:lastSpace]
	}

	return strings.TrimSpace(truncated) + "..."
}

// generateSEODescription generates an SEO description from excerpt.
func (s *ShopifyExporter) generateSEODescription(excerpt string, maxLen int) string {
	if excerpt == "" {
		return ""
	}

	return s.truncateString(excerpt, maxLen)
}

// ExportMetadata exports site metadata to a separate CSV file.
func (s *ShopifyExporter) ExportMetadata(data *models.ExportData) error {
	outputPath := filepath.Clean(filepath.Join(s.config.Output, "shopify_metadata.csv"))

	file, err := os.Create(outputPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to create metadata CSV file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write metadata information
	if err := writer.Write([]string{"Field", "Value"}); err != nil {
		return fmt.Errorf("failed to write metadata header: %w", err)
	}

	metadata := [][]string{
		{"Site Name", data.Site.Name},
		{"Site Description", data.Site.Description},
		{"Site URL", data.Site.URL},
		{"Home URL", data.Site.HomeURL},
		{"Language", data.Site.Language},
		{"Timezone", data.Site.Timezone},
		{"Total Posts", fmt.Sprintf("%d", data.Stats.TotalPosts)},
		{"Total Pages", fmt.Sprintf("%d", data.Stats.TotalPages)},
		{"Total Media", fmt.Sprintf("%d", data.Stats.TotalMedia)},
		{"Total Categories", fmt.Sprintf("%d", data.Stats.TotalCategories)},
		{"Total Tags", fmt.Sprintf("%d", data.Stats.TotalTags)},
		{"Exported At", time.Now().Format(time.RFC3339)},
	}

	for _, row := range metadata {
		if err := writer.Write(csvSafeRow(row)); err != nil {
			return fmt.Errorf("failed to write metadata row: %w", err)
		}
	}

	return nil
}
