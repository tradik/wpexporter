package export

// The catalog, written down (#65).
//
// The products were fetched, counted in `stats.total_products` and written
// nowhere: `-f markdown` and `-f ssg` produced `pages/`, `posts/` and a
// `metadata.json` claiming 282 products that were not on disk. `-f json` kept
// them, so it was the document writers alone — they had never been taught that
// a shop has documents.
//
// For a shop being migrated to a static site this is the whole catalog. Every
// `/produkt/<slug>/` link in the site's own navigation ends at a 404 on the
// built site, and the migration is of a shop without its products.
//
// A product is written as the document it is: at the path its permalink states,
// so the old address still resolves, with the commerce facts in front matter
// where a generator can read them — price, sale price, SKU, stock, categories,
// images — and the long description as the body.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// productType is the single spelling of a product document's `type`, matching
// the REST route the public catalog is served from.
const productType = "product"

// exportProductsMarkdown writes every product under products/<slug>.md.
//
// Flat, like the markdown format's other collections: that format keeps its
// original shape for backward compatibility, and the ssg format is where a
// document mirrors its URL.
func (e *Exporter) exportProductsMarkdown(products []models.WooCommerceProduct) error {
	if len(products) == 0 {
		return nil
	}

	dir := filepath.Join(e.config.Output, "products")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create products directory: %w", err)
	}

	for _, product := range products {
		document := productDocument(product)
		filename := e.generateMarkdownFilename(document)
		content := e.generateMarkdownContent(document, productType)

		if err := os.WriteFile(filepath.Join(dir, filename),
			[]byte(withCommerce(content, product)), 0600); err != nil {
			return fmt.Errorf("failed to write product file %s: %w", filename, err)
		}
	}

	return nil
}

// exportSSGProducts writes every product at the path its permalink states, so
// the address the site's own navigation links to still resolves.
func (e *Exporter) exportSSGProducts(products []models.WooCommerceProduct) error {
	for _, product := range products {
		document := productDocument(product)
		dir, filename := ssgPageLocation(e.config.Output, document, e.generateMarkdownFilename(document))

		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		content := e.generateSSGContent(document, productType)

		if err := os.WriteFile(filepath.Join(dir, filename),
			[]byte(withCommerce(content, product)), 0600); err != nil {
			return fmt.Errorf("failed to write product file %s: %w", filename, err)
		}
	}

	return nil
}

// productDocument maps a product onto the shape both document writers already
// understand, so a product is placed, named, cleaned and converted by exactly
// the code that does it for every other document rather than by a second copy
// of it.
func productDocument(product models.WooCommerceProduct) models.WordPressPost {
	document := models.WordPressPost{
		ID:       product.ID,
		Slug:     product.Slug,
		Link:     product.Permalink,
		Status:   product.Status,
		Date:     product.DateCreated,
		Modified: product.DateModified,
	}

	document.Title.Rendered = product.Name
	document.Content.Rendered = product.Description
	document.Excerpt.Rendered = product.ShortDescription

	return document
}

// withCommerce inserts the facts that make a product a product into a document
// the ordinary writers produced.
//
// The keys are added rather than the front matter rewritten, because a product
// is a document first: everything a generator knows about titles, addresses and
// dates has to keep working, and a second front-matter builder would drift from
// the one it copied.
func withCommerce(document string, product models.WooCommerceProduct) string {
	const marker = "---\n"

	end := strings.Index(document[len(marker):], marker)
	if !strings.HasPrefix(document, marker) || end < 0 {
		// No front matter to extend: the document was written by something
		// other than the generators above, and guessing where to put the keys
		// would corrupt it.
		return document
	}

	var keys strings.Builder
	writeCommerceKeys(&keys, product)

	insert := len(marker) + end

	return document[:insert] + keys.String() + document[insert:]
}

// writeCommerceKeys renders the commerce front matter. Every value is omitted
// when the shop did not set it: a `sale_price: ""` on every product in a shop
// that runs no sales is noise a consumer has to learn to ignore.
func writeCommerceKeys(builder *strings.Builder, product models.WooCommerceProduct) {
	writeYAMLString(builder, "sku", product.SKU)
	writeYAMLString(builder, "price", product.Price)
	writeYAMLString(builder, "regular_price", product.RegularPrice)
	writeYAMLString(builder, "sale_price", product.SalePrice)
	writeYAMLString(builder, "stock_status", product.StockStatus)

	if product.OnSale {
		builder.WriteString("on_sale: true\n")
	}

	if product.Featured {
		builder.WriteString("featured: true\n")
	}

	writeYAMLList(builder, "product_categories", termNames(product.Categories))
	writeYAMLList(builder, "product_tags", tagNames(product.Tags))
	writeYAMLList(builder, "images", imageSources(product.Images))
}

// termNames is the display names of a product's categories, which is what a
// listing on the other side shows.
func termNames(categories []models.ProductCategory) []string {
	names := make([]string, 0, len(categories))
	for _, category := range categories {
		if category.Name != "" {
			names = append(names, category.Name)
		}
	}

	return names
}

// tagNames is the same for tags.
func tagNames(tags []models.ProductTag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag.Name != "" {
			names = append(names, tag.Name)
		}
	}

	return names
}

// imageSources is every picture the product carries, in the order the shop put
// them in — the first is the one a listing shows.
func imageSources(images []models.ProductImage) []string {
	sources := make([]string, 0, len(images))
	for _, image := range images {
		if image.Src != "" {
			sources = append(sources, image.Src)
		}
	}

	return sources
}
