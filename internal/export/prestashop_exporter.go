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

// PrestaShopExporter exports data in PrestaShop-compatible CSV format
type PrestaShopExporter struct {
	config *config.Config
}

// NewPrestaShopExporter creates a new PrestaShop exporter
func NewPrestaShopExporter(cfg *config.Config) *PrestaShopExporter {
	return &PrestaShopExporter{
		config: cfg,
	}
}

// PrestaShopProduct represents a PrestaShop product
type PrestaShopProduct struct {
	ID               int    `json:"id"`
	Active           int    `json:"active"`
	Name             string `json:"name"`
	Categories       string `json:"categories"`
	Price            string `json:"price"`
	TaxRulesGroup    int    `json:"taxRulesGroup"`
	WholesalePrice   string `json:"wholesalePrice"`
	OnSale           int    `json:"onSale"`
	Quantity         int    `json:"quantity"`
	MinimalQuantity  int    `json:"minimalQuantity"`
	Reference        string `json:"reference"`
	Supplier         string `json:"supplier"`
	Manufacturer     string `json:"manufacturer"`
	EAN13            string `json:"ean13"`
	UPC              string `json:"upc"`
	Width            string `json:"width"`
	Height           string `json:"height"`
	Depth            string `json:"depth"`
	Weight           string `json:"weight"`
	ShippingTime     string `json:"shippingTime"`
	Description      string `json:"description"`
	DescriptionShort string `json:"descriptionShort"`
	Tags             string `json:"tags"`
	MetaTitle        string `json:"metaTitle"`
	MetaKeywords     string `json:"metaKeywords"`
	MetaDescription  string `json:"metaDescription"`
	LinkRewrite      string `json:"linkRewrite"`
	ImageURLs        string `json:"imageUrls"`
}

// Export exports data in PrestaShop CSV format
func (e *PrestaShopExporter) Export(data *models.ExportData) error {
	baseDir := e.config.Output
	if filepath.Ext(baseDir) == ".csv" || filepath.Ext(baseDir) == ".json" {
		baseDir = filepath.Dir(baseDir)
	}

	// Build lookup maps
	categoryMap := make(map[int]string)
	for _, cat := range data.Categories {
		categoryMap[cat.ID] = cat.Name
	}

	tagMap := make(map[int]string)
	for _, tag := range data.Tags {
		tagMap[tag.ID] = tag.Name
	}

	userMap := make(map[int]string)
	for _, user := range data.Users {
		userMap[user.ID] = user.Name
	}

	mediaMap := make(map[int]string)
	for _, media := range data.Media {
		mediaMap[media.ID] = media.SourceURL
	}

	// Export products (from posts)
	if len(data.Posts) > 0 {
		if err := e.exportProductsCSV(data.Posts, baseDir, "prestashop_posts.csv", categoryMap, tagMap, mediaMap); err != nil {
			return fmt.Errorf("failed to export posts: %w", err)
		}
	}

	// Export products (from pages)
	if len(data.Pages) > 0 {
		if err := e.exportProductsCSV(data.Pages, baseDir, "prestashop_pages.csv", categoryMap, tagMap, mediaMap); err != nil {
			return fmt.Errorf("failed to export pages: %w", err)
		}
	}

	// Export categories
	if len(data.Categories) > 0 {
		if err := e.exportCategoriesCSV(data.Categories, baseDir); err != nil {
			return fmt.Errorf("failed to export categories: %w", err)
		}
	}

	// Export combined products
	allPosts := append(data.Posts, data.Pages...)
	if len(allPosts) > 0 {
		if err := e.exportProductsCSV(allPosts, baseDir, "prestashop_products.csv", categoryMap, tagMap, mediaMap); err != nil {
			return fmt.Errorf("failed to export combined products: %w", err)
		}
	}

	// Export JSON for reference
	if err := e.exportJSON(data, baseDir, categoryMap, tagMap, mediaMap); err != nil {
		return fmt.Errorf("failed to export JSON: %w", err)
	}

	// Export metadata
	if err := e.exportMetadata(data, baseDir); err != nil {
		return fmt.Errorf("failed to export metadata: %w", err)
	}

	fmt.Printf("PrestaShop export completed: %s\n", baseDir)
	return nil
}

// exportProductsCSV exports WordPress posts as PrestaShop products CSV
func (e *PrestaShopExporter) exportProductsCSV(
	posts []models.WordPressPost,
	baseDir, filename string,
	categoryMap, tagMap map[int]string,
	mediaMap map[int]string,
) error {
	outputPath := filepath.Join(baseDir, filename)
	file, err := os.Create(outputPath) // #nosec G304 -- outputPath is safely constructed from config.Output
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';' // PrestaShop uses semicolon as delimiter
	defer writer.Flush()

	// Write header (PrestaShop product import format)
	header := []string{
		"ID", "Active", "Name", "Categories", "Price", "Tax rules ID",
		"Wholesale price", "On sale", "Quantity", "Minimal quantity",
		"Reference", "Supplier", "Manufacturer", "EAN13", "UPC",
		"Width", "Height", "Depth", "Weight", "Delivery time of in-stock products",
		"Description", "Short description", "Tags",
		"Meta title", "Meta keywords", "Meta description", "URL rewrite",
		"Image URLs",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write products
	for _, post := range posts {
		product := e.convertPostToProduct(post, categoryMap, tagMap, mediaMap)

		row := []string{
			fmt.Sprintf("%d", product.ID),
			fmt.Sprintf("%d", product.Active),
			product.Name,
			product.Categories,
			product.Price,
			fmt.Sprintf("%d", product.TaxRulesGroup),
			product.WholesalePrice,
			fmt.Sprintf("%d", product.OnSale),
			fmt.Sprintf("%d", product.Quantity),
			fmt.Sprintf("%d", product.MinimalQuantity),
			product.Reference,
			product.Supplier,
			product.Manufacturer,
			product.EAN13,
			product.UPC,
			product.Width,
			product.Height,
			product.Depth,
			product.Weight,
			product.ShippingTime,
			product.Description,
			product.DescriptionShort,
			product.Tags,
			product.MetaTitle,
			product.MetaKeywords,
			product.MetaDescription,
			product.LinkRewrite,
			product.ImageURLs,
		}

		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// exportCategoriesCSV exports categories in PrestaShop format
func (e *PrestaShopExporter) exportCategoriesCSV(categories []models.WordPressCategory, baseDir string) error {
	outputPath := filepath.Join(baseDir, "prestashop_categories.csv")
	file, err := os.Create(outputPath) // #nosec G304 -- outputPath is safely constructed from config.Output
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	// Write header
	header := []string{
		"ID", "Active", "Name", "Parent category", "Root category",
		"Description", "Meta title", "Meta keywords", "Meta description",
		"URL rewritten", "Image URL",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write categories
	for _, cat := range categories {
		parentID := ""
		if cat.Parent > 0 {
			parentID = fmt.Sprintf("%d", cat.Parent)
		}

		rootCategory := "0"
		if cat.Parent == 0 {
			rootCategory = "1"
		}

		row := []string{
			fmt.Sprintf("%d", cat.ID),
			"1", // Active
			cat.Name,
			parentID,
			rootCategory,
			cat.Description,
			cat.Name, // Meta title
			"",       // Meta keywords
			cat.Description,
			cat.Slug,
			"", // Image URL
		}

		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// exportJSON exports data as JSON for reference
func (e *PrestaShopExporter) exportJSON(
	data *models.ExportData,
	baseDir string,
	categoryMap, tagMap map[int]string,
	mediaMap map[int]string,
) error {
	products := make([]PrestaShopProduct, 0, len(data.Posts)+len(data.Pages))

	for _, post := range data.Posts {
		product := e.convertPostToProduct(post, categoryMap, tagMap, mediaMap)
		products = append(products, product)
	}

	for _, page := range data.Pages {
		product := e.convertPostToProduct(page, categoryMap, tagMap, mediaMap)
		product.ID = page.ID + 1000000 // Offset to avoid collision
		products = append(products, product)
	}

	jsonData, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return err
	}

	outputPath := filepath.Join(baseDir, "prestashop_export.json")
	return os.WriteFile(outputPath, jsonData, 0600)
}

// exportMetadata exports export metadata
func (e *PrestaShopExporter) exportMetadata(data *models.ExportData, baseDir string) error {
	outputPath := filepath.Join(baseDir, "prestashop_metadata.csv")
	file, err := os.Create(outputPath) // #nosec G304 -- outputPath is safely constructed from config.Output
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"Field", "Value"}); err != nil {
		return err
	}

	// Write metadata
	rows := [][]string{
		{"Exporter", "wpexporter v1.3.6"},
		{"Export Date", time.Now().Format(time.RFC3339)},
		{"Source Site", data.Site.URL},
		{"Site Name", data.Site.Name},
		{"Total Products", fmt.Sprintf("%d", len(data.Posts)+len(data.Pages))},
		{"Total Posts", fmt.Sprintf("%d", len(data.Posts))},
		{"Total Pages", fmt.Sprintf("%d", len(data.Pages))},
		{"Total Categories", fmt.Sprintf("%d", len(data.Categories))},
		{"Total Tags", fmt.Sprintf("%d", len(data.Tags))},
		{"Total Media", fmt.Sprintf("%d", len(data.Media))},
	}

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// convertPostToProduct converts a WordPress post to a PrestaShop product
func (e *PrestaShopExporter) convertPostToProduct(
	post models.WordPressPost,
	categoryMap, tagMap map[int]string,
	mediaMap map[int]string,
) PrestaShopProduct {
	// Build categories string
	var categories []string
	for _, catID := range post.Categories {
		if name, ok := categoryMap[catID]; ok {
			categories = append(categories, name)
		}
	}
	if len(categories) == 0 {
		categories = []string{"Default"}
	}

	// Build tags string
	var tags []string
	for _, tagID := range post.Tags {
		if name, ok := tagMap[tagID]; ok {
			tags = append(tags, name)
		}
	}

	// Get image URL
	imageURL := ""
	if url, ok := mediaMap[post.FeaturedMedia]; ok {
		imageURL = url
	}

	// Determine active status
	active := 0
	if post.Status == "publish" {
		active = 1
	}

	// Generate reference (SKU)
	reference := fmt.Sprintf("WP-%d", post.ID)

	return PrestaShopProduct{
		ID:               post.ID,
		Active:           active,
		Name:             post.Title.Rendered,
		Categories:       strings.Join(categories, ","),
		Price:            "0.00",
		TaxRulesGroup:    0,
		WholesalePrice:   "0.00",
		OnSale:           0,
		Quantity:         999,
		MinimalQuantity:  1,
		Reference:        reference,
		Supplier:         "",
		Manufacturer:     "",
		EAN13:            "",
		UPC:              "",
		Width:            "0",
		Height:           "0",
		Depth:            "0",
		Weight:           "0",
		ShippingTime:     "",
		Description:      post.Content.Rendered,
		DescriptionShort: stripHTMLTags(post.Excerpt.Rendered),
		Tags:             strings.Join(tags, ","),
		MetaTitle:        post.SEO.Title,
		MetaKeywords:     post.SEO.MetaKeywords,
		MetaDescription:  post.SEO.MetaDescription,
		LinkRewrite:      post.Slug,
		ImageURLs:        imageURL,
	}
}
