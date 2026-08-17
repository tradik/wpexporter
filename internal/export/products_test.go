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

// shopProducts is the shape a WooCommerce catalog arrives in.
func shopProducts() []models.WooCommerceProduct {
	return []models.WooCommerceProduct{
		{
			ID:           41,
			Name:         "Panel dębowy Rustic",
			Slug:         "panel-debowy-rustic",
			Permalink:    "https://sklep.test/produkt/panel-debowy-rustic/",
			Status:       "publish",
			Description:  "<p>Deska trójwarstwowa, olejowana.</p>",
			SKU:          "PD-RUS-01",
			Price:        "189.00",
			RegularPrice: "219.00",
			SalePrice:    "189.00",
			OnSale:       true,
			StockStatus:  "instock",
			Categories:   []models.ProductCategory{{ID: 5, Name: "Podłogi", Slug: "podlogi"}},
			Images:       []models.ProductImage{{ID: 9, Src: "https://sklep.test/wp-content/panel.jpg"}},
		},
		{
			ID:        42,
			Name:      "Listwa przypodłogowa",
			Slug:      "listwa-przypodlogowa",
			Permalink: "https://sklep.test/produkt/listwa-przypodlogowa/",
			Status:    "publish",
			Price:     "29.00",
		},
	}
}

// exporterInto builds an exporter writing to a temporary directory.
func exporterInto(t *testing.T, format string) (*Exporter, string) {
	t.Helper()

	dir := t.TempDir()

	return NewExporter(&config.Config{Output: dir, Format: format, Quiet: true}), dir
}

// TestMarkdownWritesTheCatalog: the failure. 282 products were fetched, counted
// in stats.total_products and written nowhere — `ls out/` showed pages/ and
// posts/ and no catalog at all, on a shop whose migration is the catalog.
func TestMarkdownWritesTheCatalog(t *testing.T) {
	exporter, dir := exporterInto(t, "markdown")
	require.NoError(t, exporter.exportProductsMarkdown(shopProducts()))

	body, err := os.ReadFile(filepath.Join(dir, "products", "panel-debowy-rustic.md"))
	require.NoError(t, err)

	document := string(body)
	assert.Contains(t, document, `title: "Panel dębowy Rustic"`)
	assert.Contains(t, document, `type: "product"`)
	assert.Contains(t, document, `sku: "PD-RUS-01"`)
	assert.Contains(t, document, `price: "189.00"`)
	assert.Contains(t, document, `regular_price: "219.00"`)
	assert.Contains(t, document, "on_sale: true")
	assert.Contains(t, document, `stock_status: "instock"`)
	assert.Contains(t, document, "Deska trójwarstwowa")

	second, err := os.ReadFile(filepath.Join(dir, "products", "listwa-przypodlogowa.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(second), "on_sale",
		"a shop that runs no sales gets no key saying so")
	assert.NotContains(t, string(second), "sku:")
}

// TestSSGProductsFollowTheirURL: the reporter's actual cost. Every
// /produkt/<slug>/ link in the shop's own navigation ends at a 404 on the built
// site unless the document sits where the address says.
func TestSSGProductsFollowTheirURL(t *testing.T) {
	exporter, dir := exporterInto(t, "ssg")
	require.NoError(t, exporter.exportSSGProducts(shopProducts()))

	body, err := os.ReadFile(filepath.Join(dir, "pages", "produkt", "panel-debowy-rustic.md"))
	require.NoError(t, err)

	document := string(body)
	assert.Contains(t, document, `type: "product"`)
	assert.Contains(t, document, `link: "https://sklep.test/produkt/panel-debowy-rustic/"`)
	assert.Contains(t, document, `price: "189.00"`)
	assert.Contains(t, document, "product_categories:")
	assert.Contains(t, document, `- "Podłogi"`)
	assert.Contains(t, document, "images:")
}

// TestCommerceKeysJoinTheFrontMatter: a product is a document first. The keys
// are inserted into the front matter the ordinary writers produced, so
// everything a generator knows about titles, addresses and dates keeps working
// and no second front-matter builder exists to drift from this one.
func TestCommerceKeysJoinTheFrontMatter(t *testing.T) {
	document := withCommerce("---\ntitle: \"X\"\n---\n\nBody\n", shopProducts()[0])

	assert.Equal(t, "---\ntitle: \"X\"\n", document[:len("---\ntitle: \"X\"\n")],
		"the original keys keep their place")
	assert.Contains(t, document, `sku: "PD-RUS-01"`)
	assert.Contains(t, document, "\n---\n\nBody\n", "and the body is untouched")
}

// TestCommerceKeysLeaveUnknownDocumentsAlone: guessing where to put front
// matter in a document that has none would corrupt it.
func TestCommerceKeysLeaveUnknownDocumentsAlone(t *testing.T) {
	assert.Equal(t, "no front matter here", withCommerce("no front matter here", shopProducts()[0]))
	assert.Equal(t, "---\nunterminated\n", withCommerce("---\nunterminated\n", shopProducts()[0]))
}

// TestNoProductsWritesNoDirectory: a site without a shop gets no empty
// products/ directory suggesting it lost one.
func TestNoProductsWritesNoDirectory(t *testing.T) {
	exporter, dir := exporterInto(t, "markdown")
	require.NoError(t, exporter.exportProductsMarkdown(nil))

	_, err := os.Stat(filepath.Join(dir, "products"))
	assert.True(t, os.IsNotExist(err))
}

// TestProductTagsAndFeaturedTravel: the keys a listing on the other side sorts
// and groups by. Featured is written only where the shop set it, so a catalog
// of ordinary products carries no key claiming they are ordinary.
func TestProductTagsAndFeaturedTravel(t *testing.T) {
	product := models.WooCommerceProduct{
		ID:        7,
		Name:      "Zestaw pielęgnacyjny",
		Slug:      "zestaw",
		Permalink: "https://sklep.test/produkt/zestaw/",
		Featured:  true,
		Tags: []models.ProductTag{
			{ID: 1, Name: "Promocja"},
			{ID: 2, Name: ""},
		},
	}

	document := withCommerce("---\ntitle: \"Z\"\n---\n\nBody\n", product)

	assert.Contains(t, document, "featured: true")
	assert.Contains(t, document, "product_tags:")
	assert.Contains(t, document, `- "Promocja"`)
	assert.NotContains(t, document, `- ""`, "a tag with no name is not a tag")
}

// TestProductWritersRefuseAnUnwritableOutput: an output directory that cannot
// be created is reported rather than silently producing an export with no
// catalog in it — which is the exact failure #65 was.
func TestProductWritersRefuseAnUnwritableOutput(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocked, []byte("x"), 0600))

	exporter := NewExporter(&config.Config{Output: blocked, Format: "markdown", Quiet: true})
	assert.Error(t, exporter.exportProductsMarkdown(shopProducts()))
	assert.Error(t, exporter.exportSSGProducts(shopProducts()))
}
