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

// recoverPublicProducts reads the catalog a shop publishes without credentials,
// and returns the line the run should print about it.
//
// Two public sources, in the order of how much they carry. WooCommerce's
// storefront API is what a shop's own front end calls: no keys, and it answers
// with prices, images, categories and stock (#74). The post type behind the
// shop is the older fallback, and carries the catalog page without any of the
// commerce (#55). Which one answered is stated, so an operator can tell "no
// keys, and it did not matter" from "no keys, and the prices are missing".
func recoverPublicProducts(client *api.Client) ([]models.WooCommerceProduct, string) {
	if products, route, ok := client.GetStoreProducts(); ok && len(products) > 0 {
		return products, api.StoreProductsNotice(len(products), route)
	}

	products, err := client.GetPublicProducts()
	if len(products) == 0 {
		// Neither public route brought anything. Say which was tried and what
		// it did, rather than concluding on the operator's behalf (#65, #73).
		return nil, api.NoProductsFailure(client.PublicProductRoute(), err)
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
	switch {
	case limited && stated > exported:
		return fmt.Sprintf("%s: %d (limited from %d)", label, exported, stated)

	// A collection the budget never reached. `Products: 0` under a --limit
	// reads as "this shop has no products", which is what sent the reporter of
	// #65 looking for a route bug: the budget was spent on pages before the
	// catalog was asked for, and nothing said so.
	case limited && exported == 0:
		return label + ": 0 (none within --limit)"

	default:
		return fmt.Sprintf("%s: %d", label, exported)
	}
}
