package exportcli

// A catalog for a shop that handed out no API keys (#55).
//
// /wc/v3/products needs WooCommerce consumer keys and answers 401 without them.
// The same products are usually public on /wp/v2/product — and `product` is
// excluded from the custom-type walk because it has its own exporter, so such a
// shop had no path at all: `Products: 0`, printed exactly as a shop with no
// products would print it.
//
// The fallback reads what the site publishes anyway. It is a catalog, not a
// shop: the commerce fields live where the public route cannot see them, and
// the export says so rather than handing over products that would import at no
// price.

import (
	"fmt"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/pkg/models"
)

// recoverPublicProducts reads the products the ordinary WordPress route serves,
// and returns the line the run should print about them.
func recoverPublicProducts(client *api.Client) ([]models.WooCommerceProduct, string) {
	products, err := client.GetPublicProducts()
	if err != nil && len(products) == 0 {
		// The public route is not there either — some shops keep `product` out
		// of REST entirely. Nothing to recover, and the reason is worth stating.
		return nil, api.NoProductsNotice()
	}

	if len(products) == 0 {
		return nil, api.NoProductsNotice()
	}

	return products, api.PartialProductsNotice(len(products))
}

// countLine renders one collection's line in the summary, saying so when the
// number is a deliberate truncation rather than the whole of what the site has.
//
// A truncated export that cannot say what it truncated is the failure mode of
// every silent cap — the same shape as the gaps in #37 and the shortfalls in
// #43, and worth the same sentence (#60).
func countLine(label string, exported, stated int, limited bool) string {
	if limited && stated > exported {
		return fmt.Sprintf("%s: %d (limited from %d)", label, exported, stated)
	}

	return fmt.Sprintf("%s: %d", label, exported)
}
