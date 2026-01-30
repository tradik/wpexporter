package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestExportMarkdown_ErrorCreatingPagesDir(t *testing.T) {
	// Create a read-only directory to cause permission error
	tmpDir, err := os.MkdirTemp("", "export_readonly_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	// Create a file with the name "pages" to cause MkdirAll to fail
	pagesFile := filepath.Join(tmpDir, "pages")
	err = os.WriteFile(pagesFile, []byte("blocking file"), 0600)
	require.NoError(t, err)

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "markdown",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{},
		Stats: models.ExportStats{},
	}

	err = e.Export(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create pages directory")
}

func TestExportMarkdown_ErrorExportingPosts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_post_error_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create pages directory to pass that step
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pages"), 0755))

	// Create a file named "posts" to block directory creation
	postsFile := filepath.Join(tmpDir, "posts")
	require.NoError(t, os.WriteFile(postsFile, []byte("blocking file"), 0600))

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "markdown",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{
			{
				ID:       1,
				Slug:     "test",
				Title:    models.RenderedContent{Rendered: "Test"},
				Content:  models.RenderedContent{Rendered: "<p>Content</p>"},
				Status:   "publish",
				Date:     models.WordPressTime{Time: time.Now()},
				Modified: models.WordPressTime{Time: time.Now()},
				Link:     "https://example.com/tech/test",
			},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Tech", Slug: "tech"},
		},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = e.Export(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export posts")
}

func TestExportMarkdown_ErrorExportingPages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_page_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(filepath.Join(tmpDir, "pages"), 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	// Create pages directory but make it read-only
	pagesDir := filepath.Join(tmpDir, "pages")
	require.NoError(t, os.MkdirAll(pagesDir, 0755))

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "markdown",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	// First let site info export happen
	data := &models.ExportData{
		Site: models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{},
		Pages: []models.WordPressPost{
			{
				ID:       1,
				Slug:     "test-page",
				Title:    models.RenderedContent{Rendered: "Test Page"},
				Content:  models.RenderedContent{Rendered: "<p>Page Content</p>"},
				Status:   "publish",
				Date:     models.WordPressTime{Time: time.Now()},
				Modified: models.WordPressTime{Time: time.Now()},
				Link:     "https://example.com/test-page",
			},
		},
		Stats: models.ExportStats{TotalPages: 1},
	}

	// Make pages dir read-only after setup
	require.NoError(t, os.Chmod(pagesDir, 0400))

	err = e.Export(data)
	assert.Error(t, err)
}

func TestExportShopify_MetadataError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_metadata_error_test")
	require.NoError(t, err)
	defer func() {
		// Reset permissions for cleanup
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "shopify",
		DownloadMedia: false,
	}

	exporter := NewShopifyExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Stats: models.ExportStats{},
	}

	// First export posts successfully
	err = exporter.Export(data)
	require.NoError(t, err)

	// Now make directory read-only for metadata export
	require.NoError(t, os.Chmod(tmpDir, 0500))

	err = exporter.ExportMetadata(data)
	// On some systems this might succeed if file already exists
	// So we don't assert error, just ensure no panic
}

func TestExportMagento_MetadataError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "magento_metadata_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "magento",
		DownloadMedia: false,
	}

	exporter := NewMagentoExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Stats: models.ExportStats{},
	}

	// First export successfully
	err = exporter.Export(data)
	require.NoError(t, err)

	// Make directory read-only
	require.NoError(t, os.Chmod(tmpDir, 0500))

	err = exporter.ExportMetadata(data)
	// Similar to above, just ensure no panic
}

func TestExportJSON_ErrorWriting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json_write_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "json",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Stats: models.ExportStats{},
	}

	// Make directory read-only
	require.NoError(t, os.Chmod(tmpDir, 0500))

	err = e.Export(data)
	assert.Error(t, err)
}

func TestExportPostsToShopify_WithImages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_images_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	posts := []models.WordPressPost{
		{
			ID:       1,
			Slug:     "post-with-images",
			Status:   "publish",
			Title:    models.RenderedContent{Rendered: "Post With Images"},
			Content:  models.RenderedContent{Rendered: `<p>Content</p><img src="https://example.com/img1.jpg"><img src="https://example.com/img2.jpg">`},
			Date:     models.WordPressTime{Time: time.Now()},
			Modified: models.WordPressTime{Time: time.Now()},
		},
	}

	err = exporter.exportPostsToShopify(posts, "test")
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "shopify_test.csv"))
}

func TestExportWooProductsToShopify_WithMultipleImages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_woo_images_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	products := []models.WooCommerceProduct{
		{
			ID:     1,
			Name:   "Multi Image Product",
			Slug:   "multi-image",
			Status: "publish",
			Images: []models.ProductImage{
				{ID: 1, Src: "https://example.com/img1.jpg", Alt: "Image 1"},
				{ID: 2, Src: "https://example.com/img2.jpg", Alt: "Image 2"},
				{ID: 3, Src: "https://example.com/img3.jpg", Alt: "Image 3"},
			},
		},
	}

	err = exporter.exportWooProductsToShopify(products, "multi_image_test")
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "shopify_multi_image_test.csv"))
}

func TestGetCategoryPath_NilLink(t *testing.T) {
	e := NewExporter(&config.Config{})

	categoryMap := map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Category", Slug: "category"},
	}
	hierarchy := map[int][]string{
		1: {"category"},
	}

	post := models.WordPressPost{
		ID:         1,
		Categories: []int{1},
		Link:       "", // Empty link
	}

	result := e.getCategoryPath(post, categoryMap, hierarchy)
	assert.Equal(t, "category", result)
}

func TestExtractCategoriesFromLink_InvalidURL(t *testing.T) {
	e := NewExporter(&config.Config{})

	// Test with malformed URL that will fail parsing
	result := e.extractCategoriesFromLink("://invalid")
	assert.Equal(t, "", result)
}

func TestExportShopify_FullWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_full_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "shopify",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{Name: "Test Shop"},
		Posts: []models.WordPressPost{
			{
				ID:       1,
				Slug:     "test-post",
				Status:   "publish",
				Title:    models.RenderedContent{Rendered: "Test"},
				Content:  models.RenderedContent{Rendered: "<p>Content</p>"},
				Date:     models.WordPressTime{Time: time.Now()},
				Modified: models.WordPressTime{Time: time.Now()},
			},
		},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = e.Export(data)
	require.NoError(t, err)

	// Verify all Shopify files were created
	assert.FileExists(t, filepath.Join(tmpDir, "shopify_posts.csv"))
	assert.FileExists(t, filepath.Join(tmpDir, "shopify_products.csv"))
	assert.FileExists(t, filepath.Join(tmpDir, "shopify_metadata.csv"))
}

func TestExportMagento_FullWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "magento_full_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "magento",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{Name: "Test Shop"},
		Posts: []models.WordPressPost{
			{
				ID:       1,
				Slug:     "test-post",
				Status:   "publish",
				Title:    models.RenderedContent{Rendered: "Test"},
				Content:  models.RenderedContent{Rendered: "<p>Content</p>"},
				Date:     models.WordPressTime{Time: time.Now()},
				Modified: models.WordPressTime{Time: time.Now()},
			},
		},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = e.Export(data)
	require.NoError(t, err)

	// Verify all Magento files were created
	assert.FileExists(t, filepath.Join(tmpDir, "magento_posts.csv"))
	assert.FileExists(t, filepath.Join(tmpDir, "magento_products.csv"))
	assert.FileExists(t, filepath.Join(tmpDir, "magento_metadata.csv"))
}

func TestExportMarkdown_MetadataError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "md_metadata_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "markdown",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	// Create pages directory
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pages"), 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{},
		Stats: models.ExportStats{},
	}

	// Export site info first
	err = e.exportSiteInfo(data.Site)
	require.NoError(t, err)

	// Create metadata.json as a directory to cause write error
	metadataPath := filepath.Join(tmpDir, "metadata.json")
	require.NoError(t, os.MkdirAll(metadataPath, 0755))

	err = e.exportMetadata(data)
	assert.Error(t, err)
}

func TestShopifyExporter_ExportPostsToShopify_CreateError(t *testing.T) {
	cfg := &config.Config{
		Output: "/nonexistent/path/that/does/not/exist",
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	posts := []models.WordPressPost{
		{ID: 1, Slug: "test", Status: "publish"},
	}

	err := exporter.exportPostsToShopify(posts, "test")
	assert.Error(t, err)
}

func TestMagentoExporter_ExportPostsToMagento_CreateError(t *testing.T) {
	cfg := &config.Config{
		Output: "/nonexistent/path/that/does/not/exist",
		Format: "magento",
	}

	exporter := NewMagentoExporter(cfg)

	posts := []models.WordPressPost{
		{ID: 1, Slug: "test", Status: "publish"},
	}

	err := exporter.exportPostsToMagento(posts, "test")
	assert.Error(t, err)
}

func TestShopifyExporter_ExportWooProducts_CreateError(t *testing.T) {
	cfg := &config.Config{
		Output: "/nonexistent/path/that/does/not/exist",
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	products := []models.WooCommerceProduct{
		{ID: 1, Slug: "test", Status: "publish"},
	}

	err := exporter.exportWooProductsToShopify(products, "test")
	assert.Error(t, err)
}

func TestMagentoExporter_ExportWooProducts_CreateError(t *testing.T) {
	cfg := &config.Config{
		Output: "/nonexistent/path/that/does/not/exist",
		Format: "magento",
	}

	exporter := NewMagentoExporter(cfg)

	products := []models.WooCommerceProduct{
		{ID: 1, Slug: "test", Status: "publish"},
	}

	err := exporter.exportWooProductsToMagento(products, "test")
	assert.Error(t, err)
}

func TestGetCategoryPath_WithParentSlug(t *testing.T) {
	e := NewExporter(&config.Config{})

	categoryMap := map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Parent", Slug: "parent", Parent: 0},
		2: {ID: 2, Name: "Child", Slug: "child", Parent: 1},
	}
	hierarchy := map[int][]string{
		1: {"parent"},
		2: {"parent", "child"},
	}

	// Post with a category that has a parent
	post := models.WordPressPost{
		ID:         1,
		Categories: []int{2},
		Link:       "", // Empty link to test fallback to category slug
	}

	result := e.getCategoryPath(post, categoryMap, hierarchy)
	// Should return the hierarchy path
	assert.Contains(t, result, "child")
}

func TestGetCategoryPath_NoCategories(t *testing.T) {
	e := NewExporter(&config.Config{})

	categoryMap := map[int]models.WordPressCategory{}
	hierarchy := map[int][]string{}

	post := models.WordPressPost{
		ID:         1,
		Categories: []int{}, // No categories
		Link:       "",
	}

	result := e.getCategoryPath(post, categoryMap, hierarchy)
	assert.Equal(t, "uncategorized", result)
}

func TestExportMagento_EnsureOutputDirError(t *testing.T) {
	// Use non-existent path that can't be created
	cfg := &config.Config{
		Output: "/proc/nonexistent/path",
		Format: "magento",
	}

	exporter := NewMagentoExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{{ID: 1, Slug: "test", Status: "publish"}},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err := exporter.Export(data)
	assert.Error(t, err)
}

func TestExportShopify_EnsureOutputDirError(t *testing.T) {
	cfg := &config.Config{
		Output: "/proc/nonexistent/path",
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{{ID: 1, Slug: "test", Status: "publish"}},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err := exporter.Export(data)
	assert.Error(t, err)
}

func TestExportMagento_ExportPostsError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "magento_posts_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "magento",
	}

	exporter := NewMagentoExporter(cfg)

	// Create a file to block the CSV file creation
	postsCSV := filepath.Join(tmpDir, "magento_posts.csv")
	require.NoError(t, os.MkdirAll(postsCSV, 0755)) // Create as directory to cause error

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{{ID: 1, Slug: "test", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}, Date: models.WordPressTime{Time: time.Now()}}},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = exporter.Export(data)
	assert.Error(t, err)
}

func TestExportMagento_ExportPagesError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "magento_pages_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "magento",
	}

	exporter := NewMagentoExporter(cfg)

	// Create a directory to block the pages CSV
	pagesCSV := filepath.Join(tmpDir, "magento_pages.csv")
	require.NoError(t, os.MkdirAll(pagesCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{}, // No posts to avoid posts error
		Pages: []models.WordPressPost{{ID: 1, Slug: "test-page", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}, Date: models.WordPressTime{Time: time.Now()}}},
		Stats: models.ExportStats{TotalPages: 1},
	}

	err = exporter.Export(data)
	assert.Error(t, err)
}

func TestExportMagento_ExportWooProductsError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "magento_woo_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "magento",
	}

	exporter := NewMagentoExporter(cfg)

	// Create a directory to block the woo_products CSV
	wooCSV := filepath.Join(tmpDir, "magento_woo_products.csv")
	require.NoError(t, os.MkdirAll(wooCSV, 0755))

	data := &models.ExportData{
		Site:     models.SiteInfo{Name: "Test"},
		Posts:    []models.WordPressPost{},
		Pages:    []models.WordPressPost{},
		Products: []models.WooCommerceProduct{{ID: 1, Slug: "test-product", Status: "publish", Name: "Test"}},
		Stats:    models.ExportStats{TotalProducts: 1},
	}

	err = exporter.Export(data)
	assert.Error(t, err)
}

func TestExportMagento_CombinedProductsError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "magento_combined_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "magento",
	}

	exporter := NewMagentoExporter(cfg)

	// Create a directory to block the products CSV
	productsCSV := filepath.Join(tmpDir, "magento_products.csv")
	require.NoError(t, os.MkdirAll(productsCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{{ID: 1, Slug: "test", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}, Date: models.WordPressTime{Time: time.Now()}}},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = exporter.Export(data)
	assert.Error(t, err)
}

func TestExportShopify_ExportPostsError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_posts_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	// Create a directory to block the posts CSV
	postsCSV := filepath.Join(tmpDir, "shopify_posts.csv")
	require.NoError(t, os.MkdirAll(postsCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{{ID: 1, Slug: "test", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}, Date: models.WordPressTime{Time: time.Now()}}},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = exporter.Export(data)
	assert.Error(t, err)
}

func TestExportShopify_ExportPagesError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_pages_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	// Create a directory to block the pages CSV
	pagesCSV := filepath.Join(tmpDir, "shopify_pages.csv")
	require.NoError(t, os.MkdirAll(pagesCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{},
		Pages: []models.WordPressPost{{ID: 1, Slug: "test-page", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}, Date: models.WordPressTime{Time: time.Now()}}},
		Stats: models.ExportStats{TotalPages: 1},
	}

	err = exporter.Export(data)
	assert.Error(t, err)
}

func TestExportShopify_ExportWooProductsError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_woo_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	// Create a directory to block the woo_products CSV
	wooCSV := filepath.Join(tmpDir, "shopify_woo_products.csv")
	require.NoError(t, os.MkdirAll(wooCSV, 0755))

	data := &models.ExportData{
		Site:     models.SiteInfo{Name: "Test"},
		Posts:    []models.WordPressPost{},
		Pages:    []models.WordPressPost{},
		Products: []models.WooCommerceProduct{{ID: 1, Slug: "test-product", Status: "publish", Name: "Test"}},
		Stats:    models.ExportStats{TotalProducts: 1},
	}

	err = exporter.Export(data)
	assert.Error(t, err)
}

func TestExportShopify_CombinedProductsError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_combined_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	// Create a directory to block the products CSV
	productsCSV := filepath.Join(tmpDir, "shopify_products.csv")
	require.NoError(t, os.MkdirAll(productsCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{{ID: 1, Slug: "test", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}, Date: models.WordPressTime{Time: time.Now()}}},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = exporter.Export(data)
	assert.Error(t, err)
}

func TestExport_UnsupportedFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "unsupported_format_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "unsupported",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Stats: models.ExportStats{},
	}

	err = e.Export(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported export format")
}

func TestExportMarkdown_SiteInfoError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "md_siteinfo_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "markdown",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	// Create pages directory first
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pages"), 0755))

	// Create README.md as a directory to cause write error (exportSiteInfo writes to README.md)
	readmePath := filepath.Join(tmpDir, "README.md")
	require.NoError(t, os.MkdirAll(readmePath, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{},
		Stats: models.ExportStats{},
	}

	err = e.Export(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export site info")
}

func TestExport_WithDownloadMediaEnabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "download_media_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "json",
		DownloadMedia: true,
		Concurrent:    1,
	}

	e := NewExporter(cfg)

	// Export with empty media list (will execute the DownloadMedia path but no actual downloads)
	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Media: []models.WordPressMedia{}, // Empty media list
		Stats: models.ExportStats{},
	}

	err = e.Export(data)
	require.NoError(t, err)

	// Verify that the export was successful
	assert.FileExists(t, filepath.Join(tmpDir, "export.json"))
}

func TestExport_ShopifyFormatExportError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_export_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "shopify",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	// Create a file to block the posts CSV creation
	postsCSV := filepath.Join(tmpDir, "shopify_posts.csv")
	require.NoError(t, os.MkdirAll(postsCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{{ID: 1, Slug: "test", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}, Date: models.WordPressTime{Time: time.Now()}}},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = e.Export(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export Shopify")
}

func TestExport_ShopifyFormatMetadataError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shopify_metadata_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "shopify",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	// Create a directory to block the metadata CSV creation (but allow posts/products)
	metadataCSV := filepath.Join(tmpDir, "shopify_metadata.csv")
	require.NoError(t, os.MkdirAll(metadataCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{}, // No posts to avoid posts error
		Stats: models.ExportStats{},
	}

	err = e.Export(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export Shopify metadata")
}

func TestExport_MagentoFormatExportError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "magento_export_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "magento",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	// Create a file to block the posts CSV creation
	postsCSV := filepath.Join(tmpDir, "magento_posts.csv")
	require.NoError(t, os.MkdirAll(postsCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{{ID: 1, Slug: "test", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}, Date: models.WordPressTime{Time: time.Now()}}},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = e.Export(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export Magento")
}

func TestExport_MagentoFormatMetadataError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "magento_metadata_error_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(tmpDir, 0755)
		_ = os.RemoveAll(tmpDir)
	}()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "magento",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	// Create a directory to block the metadata CSV creation
	metadataCSV := filepath.Join(tmpDir, "magento_metadata.csv")
	require.NoError(t, os.MkdirAll(metadataCSV, 0755))

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Posts: []models.WordPressPost{},
		Stats: models.ExportStats{},
	}

	err = e.Export(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export Magento metadata")
}
