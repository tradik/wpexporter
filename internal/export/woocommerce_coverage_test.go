package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestExport_MagentoWithWooProducts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_magento_woo_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "magento",
		DownloadMedia: false,
	}

	exporter := NewMagentoExporter(cfg)

	stockQty := 10.0
	products := []models.WooCommerceProduct{
		{
			ID:                1,
			Name:              "Test WooCommerce Product",
			Slug:              "test-woo-product",
			Status:            "publish",
			Type:              "simple",
			Description:       "<p>Product description</p>",
			ShortDescription:  "Short desc",
			SKU:               "WOO-SKU-001",
			Price:             "19.99",
			RegularPrice:      "24.99",
			SalePrice:         "19.99",
			ManageStock:       true,
			StockQuantity:     stockQty,
			StockStatus:       "instock",
			BackordersAllowed: true,
			CatalogVisibility: "visible",
			Weight:            "1.5",
			Categories: []models.ProductCategory{
				{ID: 1, Name: "Electronics", Slug: "electronics"},
			},
			Tags: []models.ProductTag{
				{ID: 1, Name: "Sale", Slug: "sale"},
			},
			Images: []models.ProductImage{
				{ID: 1, Src: "https://example.com/image1.jpg", Alt: "Image 1", Name: "image1"},
				{ID: 2, Src: "https://example.com/image2.jpg", Alt: "", Name: "image2"},
			},
		},
	}

	data := &models.ExportData{
		Products: products,
		Stats:    models.ExportStats{},
	}

	err = exporter.Export(data)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "magento_woo_products.csv"))
}

func TestExport_ShopifyWithWooProducts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_shopify_woo_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "shopify",
		DownloadMedia: false,
	}

	exporter := NewShopifyExporter(cfg)

	stockQty := 5
	products := []models.WooCommerceProduct{
		{
			ID:               1,
			Name:             "WooCommerce Test",
			Slug:             "woocommerce-test",
			Status:           "publish",
			Type:             "simple",
			Description:      "<p>Description here</p>",
			ShortDescription: "Short",
			SKU:              "SHOPIFY-001",
			Price:            "29.99",
			RegularPrice:     "39.99",
			StockQuantity:    stockQty,
			TaxStatus:        "taxable",
			ShippingRequired: true,
			Weight:           "2.5",
			Categories: []models.ProductCategory{
				{ID: 1, Name: "Clothing", Slug: "clothing"},
			},
			Tags: []models.ProductTag{
				{ID: 1, Name: "New", Slug: "new"},
			},
			Images: []models.ProductImage{
				{ID: 1, Src: "https://example.com/product.jpg", Alt: "Product Image", Name: "product"},
				{ID: 2, Src: "https://example.com/product2.jpg", Alt: "", Name: "product2"},
			},
		},
	}

	data := &models.ExportData{
		Products: products,
		Stats:    models.ExportStats{},
	}

	err = exporter.Export(data)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "shopify_woo_products.csv"))
}

func TestBoolToMagento(t *testing.T) {
	assert.Equal(t, "1", boolToMagento(true))
	assert.Equal(t, "0", boolToMagento(false))
}

func TestBoolToShopify(t *testing.T) {
	assert.Equal(t, "TRUE", boolToShopify(true))
	assert.Equal(t, "FALSE", boolToShopify(false))
}

func TestConvertWooProductToMagentoProduct_AllProductTypes(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	testCases := []struct {
		name         string
		productType  string
		expectedType string
	}{
		{"simple", "simple", "simple"},
		{"variable", "variable", "configurable"},
		{"grouped", "grouped", "grouped"},
		{"external", "external", "simple"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			product := models.WooCommerceProduct{
				ID:   1,
				Type: tc.productType,
				Slug: "test",
			}

			result := exporter.convertWooProductToMagentoProduct(product)
			assert.Equal(t, tc.expectedType, result.ProductType)
		})
	}
}

func TestConvertWooProductToMagentoProduct_VirtualAndDownloadable(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	// Virtual product
	virtualProduct := models.WooCommerceProduct{
		ID:      1,
		Slug:    "virtual",
		Virtual: true,
	}
	result := exporter.convertWooProductToMagentoProduct(virtualProduct)
	assert.Equal(t, "virtual", result.ProductType)

	// Downloadable product
	downloadableProduct := models.WooCommerceProduct{
		ID:           2,
		Slug:         "downloadable",
		Downloadable: true,
	}
	result = exporter.convertWooProductToMagentoProduct(downloadableProduct)
	assert.Equal(t, "downloadable", result.ProductType)
}

func TestConvertWooProductToMagentoProduct_CatalogVisibility(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	testCases := []struct {
		visibility         string
		expectedVisibility string
	}{
		{"hidden", "Not Visible Individually"},
		{"catalog", "Catalog"},
		{"search", "Search"},
		{"visible", "Catalog, Search"},
	}

	for _, tc := range testCases {
		t.Run(tc.visibility, func(t *testing.T) {
			product := models.WooCommerceProduct{
				ID:                1,
				Slug:              "test",
				CatalogVisibility: tc.visibility,
			}

			result := exporter.convertWooProductToMagentoProduct(product)
			assert.Equal(t, tc.expectedVisibility, result.Visibility)
		})
	}
}

func TestConvertWooProductToMagentoProduct_StockStatus(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	// Out of stock
	outOfStock := models.WooCommerceProduct{
		ID:          1,
		Slug:        "out-of-stock",
		StockStatus: "outofstock",
	}
	result := exporter.convertWooProductToMagentoProduct(outOfStock)
	assert.Equal(t, "0", result.IsInStock)

	// In stock
	inStock := models.WooCommerceProduct{
		ID:          2,
		Slug:        "in-stock",
		StockStatus: "instock",
	}
	result = exporter.convertWooProductToMagentoProduct(inStock)
	assert.Equal(t, "1", result.IsInStock)
}

func TestConvertWooProductToMagentoProduct_EmptySKU(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	product := models.WooCommerceProduct{
		ID:   123,
		Slug: "test-product",
		SKU:  "", // Empty SKU
	}

	result := exporter.convertWooProductToMagentoProduct(product)
	assert.Equal(t, "TEST-PRODUCT", result.SKU)
}

func TestConvertWooProductToMagentoProduct_DraftStatus(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	product := models.WooCommerceProduct{
		ID:     1,
		Slug:   "draft-product",
		Status: "draft",
	}

	result := exporter.convertWooProductToMagentoProduct(product)
	assert.Equal(t, "0", result.ProductOnline)
}

func TestConvertWooProductToShopifyProduct_NoCategories(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	product := models.WooCommerceProduct{
		ID:         1,
		Name:       "Test Product",
		Slug:       "test-product",
		Status:     "publish",
		Categories: []models.ProductCategory{},
	}

	result := exporter.convertWooProductToShopifyProduct(product)
	assert.Equal(t, "Product", result.Type)
}

func TestConvertWooProductToShopifyProduct_DraftStatus(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	product := models.WooCommerceProduct{
		ID:     1,
		Name:   "Draft Product",
		Slug:   "draft-product",
		Status: "draft",
	}

	result := exporter.convertWooProductToShopifyProduct(product)
	assert.Equal(t, "FALSE", result.Published)
	assert.Equal(t, "draft", result.Status)
}

func TestConvertWooProductToShopifyProduct_WithWeight(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	product := models.WooCommerceProduct{
		ID:     1,
		Name:   "Heavy Product",
		Slug:   "heavy-product",
		Weight: "5",
	}

	result := exporter.convertWooProductToShopifyProduct(product)
	assert.Equal(t, "5000", result.VariantGrams)
}

func TestConvertWooProductToShopifyProduct_ImageWithoutAlt(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	product := models.WooCommerceProduct{
		ID:   1,
		Name: "Product with Image",
		Slug: "product-with-image",
		Images: []models.ProductImage{
			{ID: 1, Src: "https://example.com/img.jpg", Alt: "", Name: "Image Name"},
		},
	}

	result := exporter.convertWooProductToShopifyProduct(product)
	assert.Equal(t, "Image Name", result.ImageAltText)
}

func TestMagentoExporter_GenerateSKU_EmptySlug(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	result := exporter.generateSKU("", 456)
	assert.Equal(t, "WP-456", result)
}

func TestMagentoExporter_GenerateSKU_SpecialChars(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	result := exporter.generateSKU("test--product--name", 1)
	assert.Equal(t, "TEST-PRODUCT-NAME", result)
}

func TestMagentoExporter_GenerateURLKey_EmptySlug(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	result := exporter.generateURLKey("", 789)
	assert.Equal(t, "product-789", result)
}

func TestMagentoExporter_ExtractFilename_WithQueryParams(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	result := exporter.extractFilename("https://example.com/images/photo.jpg?v=123")
	assert.Equal(t, "photo.jpg", result)
}

func TestMagentoExporter_ExtractFilename_Empty(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	result := exporter.extractFilename("")
	assert.Equal(t, "", result)
}

func TestShopifyExporter_GenerateHandle_EmptyResult(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	// Slug that becomes empty after sanitization
	result := exporter.generateHandle("---", 123)
	assert.Equal(t, "product-123", result)
}

func TestMagentoExporter_GenerateSKU_BecomesEmpty(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	// Slug that becomes empty after sanitization
	result := exporter.generateSKU("---", 123)
	assert.Equal(t, "WP-123", result)
}

func TestMagentoExporter_GenerateURLKey_BecomesEmpty(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	// Slug that becomes empty after sanitization
	result := exporter.generateURLKey("---", 123)
	assert.Equal(t, "product-123", result)
}

func TestMagentoExporter_StockQuantityInt(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	stockQty := 15
	product := models.WooCommerceProduct{
		ID:            1,
		Slug:          "test",
		StockQuantity: stockQty,
	}

	result := exporter.convertWooProductToMagentoProduct(product)
	assert.Equal(t, "15", result.Qty)
}

func TestShopifyExporter_StockQuantityInt(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	stockQty := 25
	product := models.WooCommerceProduct{
		ID:            1,
		Slug:          "test",
		StockQuantity: stockQty,
	}

	result := exporter.convertWooProductToShopifyProduct(product)
	assert.Equal(t, "25", result.VariantInventoryQty)
}

func TestMagentoExporter_ConvertWooProduct_ImageWithEmptyAlt(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	product := models.WooCommerceProduct{
		ID:   1,
		Slug: "test",
		Images: []models.ProductImage{
			{ID: 1, Src: "https://example.com/img.jpg", Alt: "", Name: "Product Image"},
		},
	}

	result := exporter.convertWooProductToMagentoProduct(product)
	assert.Equal(t, "Product Image", result.BaseImageLabel)
}

func TestMagentoExporter_ConvertWooProduct_NoCategories(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	product := models.WooCommerceProduct{
		ID:         1,
		Slug:       "test",
		Categories: []models.ProductCategory{},
	}

	result := exporter.convertWooProductToMagentoProduct(product)
	assert.Equal(t, "Default Category", result.Categories)
}
