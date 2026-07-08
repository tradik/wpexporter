// Package export provides functionality for exporting WordPress content to various formats.
package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// MagentoExporter handles export to Magento-compatible CSV format.
type MagentoExporter struct {
	config      *config.Config
	categoryMap map[int]models.WordPressCategory
	tagMap      map[int]models.WordPressTag
	userMap     map[int]models.WordPressUser
	mediaMap    map[int]models.WordPressMedia
}

// MagentoProduct represents a single Magento product row.
type MagentoProduct struct {
	SKU                    string
	StoreViewCode          string
	AttributeSetCode       string
	ProductType            string
	Categories             string
	ProductWebsites        string
	Name                   string
	Description            string
	ShortDescription       string
	Weight                 string
	ProductOnline          string
	TaxClassName           string
	Visibility             string
	Price                  string
	SpecialPrice           string
	SpecialPriceFromDate   string
	SpecialPriceToDate     string
	URLKey                 string
	MetaTitle              string
	MetaKeywords           string
	MetaDescription        string
	BaseImage              string
	BaseImageLabel         string
	SmallImage             string
	SmallImageLabel        string
	ThumbnailImage         string
	ThumbnailImageLabel    string
	AdditionalImages       string
	AdditionalImageLabels  string
	Qty                    string
	OutOfStockQty          string
	UseConfigMinQty        string
	IsQtyDecimal           string
	AllowBackorders        string
	UseConfigBackorders    string
	MinCartQty             string
	UseConfigMinSaleQty    string
	MaxCartQty             string
	UseConfigMaxSaleQty    string
	IsInStock              string
	NotifyOnStockBelow     string
	UseConfigNotifyStock   string
	ManageStock            string
	UseConfigManageStock   string
	UseConfigQtyIncrements string
	QtyIncrements          string
	UseConfigEnableIncr    string
	EnableQtyIncrements    string
	IsDecimalDivided       string
	WebsiteID              string
	RelatedSKUs            string
	RelatedPosition        string
	CrosssellSKUs          string
	CrosssellPosition      string
	UpsellSKUs             string
	UpsellPosition         string
	AdditionalAttributes   string
}

// NewMagentoExporter creates a new Magento exporter instance.
func NewMagentoExporter(cfg *config.Config) *MagentoExporter {
	return &MagentoExporter{
		config:      cfg,
		categoryMap: make(map[int]models.WordPressCategory),
		tagMap:      make(map[int]models.WordPressTag),
		userMap:     make(map[int]models.WordPressUser),
		mediaMap:    make(map[int]models.WordPressMedia),
	}
}

// Export exports WordPress data to Magento-compatible CSV format.
func (m *MagentoExporter) Export(data *models.ExportData) error {
	// Build lookup maps for categories, tags, users, and media
	m.buildLookupMaps(data)

	// Ensure output directory exists
	if err := m.config.EnsureOutputDir(); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Export posts as products (content/blog posts)
	if len(data.Posts) > 0 {
		if err := m.exportPostsToMagento(data.Posts, "posts"); err != nil {
			return fmt.Errorf("failed to export posts: %w", err)
		}
	}

	// Export pages as products (optional, separate file)
	if len(data.Pages) > 0 {
		if err := m.exportPostsToMagento(data.Pages, "pages"); err != nil {
			return fmt.Errorf("failed to export pages: %w", err)
		}
	}

	// Export WooCommerce products if available
	if len(data.Products) > 0 {
		if err := m.exportWooProductsToMagento(data.Products, "woo_products"); err != nil {
			return fmt.Errorf("failed to export WooCommerce products: %w", err)
		}
	}

	// Export all content combined into a single products CSV
	allContent := append(data.Posts, data.Pages...)
	if err := m.exportPostsToMagento(allContent, "products"); err != nil {
		return fmt.Errorf("failed to export combined products: %w", err)
	}

	fmt.Printf("Magento export completed: %s\n", m.config.Output)
	return nil
}

// exportWooProductsToMagento exports WooCommerce products to a Magento CSV file.
func (m *MagentoExporter) exportWooProductsToMagento(products []models.WooCommerceProduct, filename string) error {
	outputPath := filepath.Clean(filepath.Join(m.config.Output, fmt.Sprintf("magento_%s.csv", filename)))

	file, err := os.Create(outputPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(m.getCSVHeaders()); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, product := range products {
		magentoProduct := m.convertWooProductToMagentoProduct(product)
		if err := writer.Write(csvSafeRow(m.productToCSVRow(magentoProduct))); err != nil {
			return fmt.Errorf("failed to write product row: %w", err)
		}
	}

	return nil
}

// convertWooProductToMagentoProduct converts a WooCommerce product to a Magento product.
func (m *MagentoExporter) convertWooProductToMagentoProduct(product models.WooCommerceProduct) MagentoProduct {
	// Use the WooCommerce SKU or generate one
	sku := product.SKU
	if sku == "" {
		sku = m.generateSKU(product.Slug, product.ID)
	}

	// Get categories from WooCommerce product categories
	var categoryPaths []string
	for _, cat := range product.Categories {
		categoryPaths = append(categoryPaths, fmt.Sprintf("Default Category/%s", cat.Name))
	}
	categories := strings.Join(categoryPaths, ",")
	if categories == "" {
		categories = "Default Category"
	}

	// Get keywords from WooCommerce product tags
	var keywords []string
	for _, tag := range product.Tags {
		keywords = append(keywords, tag.Name)
	}
	metaKeywords := strings.Join(keywords, ",")

	// Get first image
	var baseImage, imageLabel string
	var additionalImages []string
	for i, img := range product.Images {
		filename := m.extractFilename(img.Src)
		if i == 0 {
			baseImage = filename
			imageLabel = img.Alt
			if imageLabel == "" {
				imageLabel = img.Name
			}
		} else {
			additionalImages = append(additionalImages, filename)
		}
	}

	// Determine product status
	productOnline := "0"
	if product.Status == statusPublish {
		productOnline = "1"
	}

	// Get inventory quantity
	qty := "0"
	if product.StockQuantity != nil {
		switch v := product.StockQuantity.(type) {
		case float64:
			qty = fmt.Sprintf("%.0f", v)
		case int:
			qty = fmt.Sprintf("%d", v)
		}
	}

	// Determine if in stock
	isInStock := "1"
	if product.StockStatus == "outofstock" {
		isInStock = "0"
	}

	// Map product type
	productType := "simple"
	if product.Type == "variable" {
		productType = "configurable"
	} else if product.Type == "grouped" {
		productType = "grouped"
	} else if product.Type == "external" {
		productType = "simple"
	} else if product.Virtual {
		productType = "virtual"
	} else if product.Downloadable {
		productType = "downloadable"
	}

	// Determine visibility
	visibility := "Catalog, Search"
	switch product.CatalogVisibility {
	case "hidden":
		visibility = "Not Visible Individually"
	case "catalog":
		visibility = "Catalog"
	case "search":
		visibility = "Search"
	}

	// Manage stock setting
	manageStock := "0"
	if product.ManageStock {
		manageStock = "1"
	}

	return MagentoProduct{
		SKU:                    sku,
		StoreViewCode:          "",
		AttributeSetCode:       "Default",
		ProductType:            productType,
		Categories:             categories,
		ProductWebsites:        "base",
		Name:                   product.Name,
		Description:            m.cleanHTMLForMagento(product.Description),
		ShortDescription:       m.cleanHTMLForMagento(product.ShortDescription),
		Weight:                 product.Weight,
		ProductOnline:          productOnline,
		TaxClassName:           "Taxable Goods",
		Visibility:             visibility,
		Price:                  product.RegularPrice,
		SpecialPrice:           product.SalePrice,
		SpecialPriceFromDate:   "",
		SpecialPriceToDate:     "",
		URLKey:                 m.generateURLKey(product.Slug, product.ID),
		MetaTitle:              m.truncateString(product.Name, 70),
		MetaKeywords:           metaKeywords,
		MetaDescription:        m.truncateString(product.ShortDescription, 160),
		BaseImage:              baseImage,
		BaseImageLabel:         imageLabel,
		SmallImage:             baseImage,
		SmallImageLabel:        imageLabel,
		ThumbnailImage:         baseImage,
		ThumbnailImageLabel:    imageLabel,
		AdditionalImages:       strings.Join(additionalImages, ","),
		AdditionalImageLabels:  "",
		Qty:                    qty,
		OutOfStockQty:          "0",
		UseConfigMinQty:        "1",
		IsQtyDecimal:           "0",
		AllowBackorders:        boolToMagento(product.BackordersAllowed),
		UseConfigBackorders:    "0",
		MinCartQty:             "1",
		UseConfigMinSaleQty:    "1",
		MaxCartQty:             "0",
		UseConfigMaxSaleQty:    "1",
		IsInStock:              isInStock,
		NotifyOnStockBelow:     "1",
		UseConfigNotifyStock:   "1",
		ManageStock:            manageStock,
		UseConfigManageStock:   "0",
		UseConfigQtyIncrements: "1",
		QtyIncrements:          "1",
		UseConfigEnableIncr:    "1",
		EnableQtyIncrements:    "0",
		IsDecimalDivided:       "0",
		WebsiteID:              "1",
		RelatedSKUs:            "",
		RelatedPosition:        "",
		CrosssellSKUs:          "",
		CrosssellPosition:      "",
		UpsellSKUs:             "",
		UpsellPosition:         "",
		AdditionalAttributes:   "",
	}
}

// boolToMagento converts a boolean to Magento "1"/"0" string.
func boolToMagento(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// buildLookupMaps creates lookup maps for efficient access to metadata.
func (m *MagentoExporter) buildLookupMaps(data *models.ExportData) {
	// Build category map
	for _, cat := range data.Categories {
		m.categoryMap[cat.ID] = cat
	}

	// Build tag map
	for _, tag := range data.Tags {
		m.tagMap[tag.ID] = tag
	}

	// Build user map
	for _, user := range data.Users {
		m.userMap[user.ID] = user
	}

	// Build media map
	for _, media := range data.Media {
		m.mediaMap[media.ID] = media
	}
}

// exportPostsToMagento exports posts/pages to a Magento CSV file.
func (m *MagentoExporter) exportPostsToMagento(posts []models.WordPressPost, filename string) error {
	// Determine output file path
	outputPath := filepath.Clean(filepath.Join(m.config.Output, fmt.Sprintf("magento_%s.csv", filename)))

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
	if err := writer.Write(m.getCSVHeaders()); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write product rows
	for _, post := range posts {
		product := m.convertPostToMagentoProduct(post)
		if err := writer.Write(csvSafeRow(m.productToCSVRow(product))); err != nil {
			return fmt.Errorf("failed to write product row: %w", err)
		}
	}

	return nil
}

// getCSVHeaders returns the Magento CSV header row.
func (m *MagentoExporter) getCSVHeaders() []string {
	return []string{
		"sku",
		"store_view_code",
		"attribute_set_code",
		"product_type",
		"categories",
		"product_websites",
		"name",
		"description",
		"short_description",
		"weight",
		"product_online",
		"tax_class_name",
		"visibility",
		"price",
		"special_price",
		"special_price_from_date",
		"special_price_to_date",
		"url_key",
		"meta_title",
		"meta_keywords",
		"meta_description",
		"base_image",
		"base_image_label",
		"small_image",
		"small_image_label",
		"thumbnail_image",
		"thumbnail_image_label",
		"additional_images",
		"additional_image_labels",
		"qty",
		"out_of_stock_qty",
		"use_config_min_qty",
		"is_qty_decimal",
		"allow_backorders",
		"use_config_backorders",
		"min_cart_qty",
		"use_config_min_sale_qty",
		"max_cart_qty",
		"use_config_max_sale_qty",
		"is_in_stock",
		"notify_on_stock_below",
		"use_config_notify_stock_qty",
		"manage_stock",
		"use_config_manage_stock",
		"use_config_qty_increments",
		"qty_increments",
		"use_config_enable_qty_inc",
		"enable_qty_increments",
		"is_decimal_divided",
		"website_id",
		"related_skus",
		"related_position",
		"crosssell_skus",
		"crosssell_position",
		"upsell_skus",
		"upsell_position",
		"additional_attributes",
	}
}

// convertPostToMagentoProduct converts a WordPress post to a Magento product.
func (m *MagentoExporter) convertPostToMagentoProduct(post models.WordPressPost) MagentoProduct {
	// Generate SKU from slug
	sku := m.generateSKU(post.Slug, post.ID)

	// Get categories as Magento format path
	categories := m.getCategoriesPath(post.Categories)

	// Clean content for Magento
	description := m.cleanHTMLForMagento(post.Content.Rendered)
	shortDescription := m.cleanHTMLForMagento(post.Excerpt.Rendered)

	// Get featured image
	baseImage, imageLabel := m.getFeaturedImage(post.FeaturedMedia)

	// Get additional images
	additionalImages := m.getAdditionalImages(post.Content.Rendered, baseImage)

	// Generate URL key
	urlKey := m.generateURLKey(post.Slug, post.ID)

	// Generate meta fields
	metaTitle := m.truncateString(post.Title.Rendered, 70)
	metaDescription := m.truncateString(m.stripHTML(post.Excerpt.Rendered), 160)
	metaKeywords := m.getMetaKeywords(post.Tags)

	// Determine product status
	productOnline := m.isOnline(post.Status)

	return MagentoProduct{
		SKU:                    sku,
		StoreViewCode:          "",
		AttributeSetCode:       "Default",
		ProductType:            "simple",
		Categories:             categories,
		ProductWebsites:        "base",
		Name:                   post.Title.Rendered,
		Description:            description,
		ShortDescription:       shortDescription,
		Weight:                 "0",
		ProductOnline:          productOnline,
		TaxClassName:           "Taxable Goods",
		Visibility:             "Catalog, Search",
		Price:                  "0.00",
		SpecialPrice:           "",
		SpecialPriceFromDate:   "",
		SpecialPriceToDate:     "",
		URLKey:                 urlKey,
		MetaTitle:              metaTitle,
		MetaKeywords:           metaKeywords,
		MetaDescription:        metaDescription,
		BaseImage:              baseImage,
		BaseImageLabel:         imageLabel,
		SmallImage:             baseImage,
		SmallImageLabel:        imageLabel,
		ThumbnailImage:         baseImage,
		ThumbnailImageLabel:    imageLabel,
		AdditionalImages:       additionalImages,
		AdditionalImageLabels:  "",
		Qty:                    "0",
		OutOfStockQty:          "0",
		UseConfigMinQty:        "1",
		IsQtyDecimal:           "0",
		AllowBackorders:        "0",
		UseConfigBackorders:    "1",
		MinCartQty:             "1",
		UseConfigMinSaleQty:    "1",
		MaxCartQty:             "0",
		UseConfigMaxSaleQty:    "1",
		IsInStock:              "1",
		NotifyOnStockBelow:     "1",
		UseConfigNotifyStock:   "1",
		ManageStock:            "0",
		UseConfigManageStock:   "1",
		UseConfigQtyIncrements: "1",
		QtyIncrements:          "1",
		UseConfigEnableIncr:    "1",
		EnableQtyIncrements:    "0",
		IsDecimalDivided:       "0",
		WebsiteID:              "1",
		RelatedSKUs:            "",
		RelatedPosition:        "",
		CrosssellSKUs:          "",
		CrosssellPosition:      "",
		UpsellSKUs:             "",
		UpsellPosition:         "",
		AdditionalAttributes:   "",
	}
}

// productToCSVRow converts a MagentoProduct to a CSV row.
func (m *MagentoExporter) productToCSVRow(p MagentoProduct) []string {
	return []string{
		p.SKU,
		p.StoreViewCode,
		p.AttributeSetCode,
		p.ProductType,
		p.Categories,
		p.ProductWebsites,
		p.Name,
		p.Description,
		p.ShortDescription,
		p.Weight,
		p.ProductOnline,
		p.TaxClassName,
		p.Visibility,
		p.Price,
		p.SpecialPrice,
		p.SpecialPriceFromDate,
		p.SpecialPriceToDate,
		p.URLKey,
		p.MetaTitle,
		p.MetaKeywords,
		p.MetaDescription,
		p.BaseImage,
		p.BaseImageLabel,
		p.SmallImage,
		p.SmallImageLabel,
		p.ThumbnailImage,
		p.ThumbnailImageLabel,
		p.AdditionalImages,
		p.AdditionalImageLabels,
		p.Qty,
		p.OutOfStockQty,
		p.UseConfigMinQty,
		p.IsQtyDecimal,
		p.AllowBackorders,
		p.UseConfigBackorders,
		p.MinCartQty,
		p.UseConfigMinSaleQty,
		p.MaxCartQty,
		p.UseConfigMaxSaleQty,
		p.IsInStock,
		p.NotifyOnStockBelow,
		p.UseConfigNotifyStock,
		p.ManageStock,
		p.UseConfigManageStock,
		p.UseConfigQtyIncrements,
		p.QtyIncrements,
		p.UseConfigEnableIncr,
		p.EnableQtyIncrements,
		p.IsDecimalDivided,
		p.WebsiteID,
		p.RelatedSKUs,
		p.RelatedPosition,
		p.CrosssellSKUs,
		p.CrosssellPosition,
		p.UpsellSKUs,
		p.UpsellPosition,
		p.AdditionalAttributes,
	}
}

// generateSKU creates a unique SKU from the slug.
func (m *MagentoExporter) generateSKU(slug string, id int) string {
	if slug == "" {
		return fmt.Sprintf("WP-%d", id)
	}

	// Sanitize SKU: uppercase, alphanumeric and hyphens/underscores only
	sku := strings.ToUpper(slug)
	reg := regexp.MustCompile(`[^A-Z0-9_-]`)
	sku = reg.ReplaceAllString(sku, "-")

	// Remove consecutive hyphens
	for strings.Contains(sku, "--") {
		sku = strings.ReplaceAll(sku, "--", "-")
	}

	// Trim hyphens from start and end
	sku = strings.Trim(sku, "-")

	if sku == "" {
		return fmt.Sprintf("WP-%d", id)
	}

	return sku
}

// generateURLKey creates a URL-friendly key from the slug.
func (m *MagentoExporter) generateURLKey(slug string, id int) string {
	if slug == "" {
		return fmt.Sprintf("product-%d", id)
	}

	// Sanitize URL key: lowercase, alphanumeric and hyphens only
	urlKey := strings.ToLower(slug)
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	urlKey = reg.ReplaceAllString(urlKey, "-")

	// Remove consecutive hyphens
	for strings.Contains(urlKey, "--") {
		urlKey = strings.ReplaceAll(urlKey, "--", "-")
	}

	// Trim hyphens from start and end
	urlKey = strings.Trim(urlKey, "-")

	if urlKey == "" {
		return fmt.Sprintf("product-%d", id)
	}

	return urlKey
}

// getCategoriesPath returns the Magento category path format.
func (m *MagentoExporter) getCategoriesPath(categoryIDs []int) string {
	if len(categoryIDs) == 0 {
		return "Default Category"
	}

	var paths []string
	for _, catID := range categoryIDs {
		if cat, exists := m.categoryMap[catID]; exists {
			// Build category path: Default Category/Category Name
			path := fmt.Sprintf("Default Category/%s", cat.Name)
			paths = append(paths, path)
		}
	}

	if len(paths) == 0 {
		return "Default Category"
	}

	return strings.Join(paths, ",")
}

// getMetaKeywords returns comma-separated keywords from tags.
func (m *MagentoExporter) getMetaKeywords(tagIDs []int) string {
	if len(tagIDs) == 0 {
		return ""
	}

	var keywords []string
	for _, tagID := range tagIDs {
		if tag, exists := m.tagMap[tagID]; exists {
			keywords = append(keywords, tag.Name)
		}
	}

	return strings.Join(keywords, ",")
}

// isOnline returns "1" or "0" based on post status.
func (m *MagentoExporter) isOnline(status string) string {
	if status == statusPublish {
		return "1"
	}
	return "0"
}

// getFeaturedImage returns the featured image path and label.
func (m *MagentoExporter) getFeaturedImage(mediaID int) (string, string) {
	if media, exists := m.mediaMap[mediaID]; exists {
		label := media.AltText
		if label == "" {
			label = media.Title.Rendered
		}
		// For Magento, we just use the filename from the URL
		imagePath := m.extractFilename(media.SourceURL)
		return imagePath, label
	}
	return "", ""
}

// getAdditionalImages extracts additional image filenames from content.
func (m *MagentoExporter) getAdditionalImages(html, excludeImage string) string {
	if html == "" {
		return ""
	}

	// Find all image sources in the content
	imgPattern := regexp.MustCompile(`<img[^>]+src\s*=\s*["']([^"']+)["']`)
	matches := imgPattern.FindAllStringSubmatch(html, -1)

	var images []string
	seen := make(map[string]bool)
	excludeFilename := m.extractFilename(excludeImage)

	for _, match := range matches {
		if len(match) > 1 {
			filename := m.extractFilename(match[1])
			if filename != "" && !seen[filename] && filename != excludeFilename {
				images = append(images, filename)
				seen[filename] = true
			}
		}
	}

	return strings.Join(images, ",")
}

// extractFilename extracts the filename from a URL.
func (m *MagentoExporter) extractFilename(url string) string {
	if url == "" {
		return ""
	}

	// Find the last segment of the URL path
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		// Remove query parameters
		if idx := strings.Index(filename, "?"); idx != -1 {
			filename = filename[:idx]
		}
		return filename
	}
	return ""
}

// cleanHTMLForMagento cleans HTML content for Magento compatibility.
func (m *MagentoExporter) cleanHTMLForMagento(html string) string {
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

// stripHTML removes all HTML tags from a string.
func (m *MagentoExporter) stripHTML(html string) string {
	if html == "" {
		return ""
	}

	tagPattern := regexp.MustCompile(`<[^>]+>`)
	text := tagPattern.ReplaceAllString(html, "")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	return strings.TrimSpace(text)
}

// truncateString truncates a string to a maximum length.
func (m *MagentoExporter) truncateString(str string, maxLen int) string {
	// Clean HTML tags first
	str = m.stripHTML(str)

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

// ExportMetadata exports site metadata to a separate CSV file.
func (m *MagentoExporter) ExportMetadata(data *models.ExportData) error {
	outputPath := filepath.Clean(filepath.Join(m.config.Output, "magento_metadata.csv"))

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
		{"Export Format", "Magento 2"},
		{"Exported At", time.Now().Format(time.RFC3339)},
	}

	for _, row := range metadata {
		if err := writer.Write(csvSafeRow(row)); err != nil {
			return fmt.Errorf("failed to write metadata row: %w", err)
		}
	}

	return nil
}
