package api

// A shop that handed out no API keys (#55). /wc/v3 answers 401, the same
// products are public on /wp/v2/product, and `product` is excluded from the
// custom-type walk — so the export had no path at all and printed `Products: 0`
// exactly as a shop with no products would.

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// publicProductsBody is what the ordinary WordPress route serves: the catalog
// page, and none of the commerce.
const publicProductsBody = `[
  {"id":31,"slug":"blue-mug","link":"https://x.test/product/blue-mug/","status":"publish",
   "title":{"rendered":"Blue mug"},"content":{"rendered":"<p>Stoneware.</p>"},
   "excerpt":{"rendered":"A mug."}},
  {"id":32,"slug":"red-mug","link":"https://x.test/product/red-mug/","status":"publish",
   "title":{"rendered":"Red mug"},"content":{"rendered":"<p>Also stoneware.</p>"}}
]`

// TestWooCommerceRefusalIsToldApartFromAbsence: 401 means the shop exists and
// will not talk to us; 404 means there is no WooCommerce. The remedies differ,
// so the answers must too.
func TestWooCommerceRefusalIsToldApartFromAbsence(t *testing.T) {
	refusing := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	products, err := refusing.GetProducts()
	assert.Empty(t, products)
	require.ErrorIs(t, err, ErrProductsNeedKeys)

	absent := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	products, err = absent.GetProducts()
	assert.Empty(t, products)
	assert.NoError(t, err, "no WooCommerce is a fact about the site, not an error")
}

// TestPublicProductsCarryTheCatalogue: what the fallback recovers, and what it
// leaves empty. A price of "0" would import as a free product, which is worse
// than an absent one.
func TestPublicProductsCarryTheCatalogue(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if !strings.HasSuffix(r.URL.Path, "/wp/v2/product") || r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte(`[]`))

			return
		}

		_, _ = w.Write([]byte(publicProductsBody))
	})

	products, err := client.GetPublicProducts()
	require.NoError(t, err)
	require.Len(t, products, 2)

	first := products[0]
	assert.Equal(t, 31, first.ID)
	assert.Equal(t, "Blue mug", first.Name)
	assert.Equal(t, "blue-mug", first.Slug)
	assert.Equal(t, "https://x.test/product/blue-mug/", first.Permalink)
	assert.Contains(t, first.Description, "Stoneware")
	assert.Equal(t, "publish", first.Status)

	assert.Empty(t, first.Price, "the public route does not publish a price, so none is invented")
	assert.Empty(t, first.RegularPrice)
	assert.Empty(t, first.SKU)
	assert.Empty(t, first.StockStatus)
}

// TestPublicProductsOnASiteThatPublishesNone: some shops keep `product` out of
// REST entirely, and there is nothing to recover.
func TestPublicProductsOnASiteThatPublishesNone(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	products, err := client.GetPublicProducts()
	require.NoError(t, err)
	assert.Empty(t, products)
}

// TestPublicProductsKeepWhatTheyRead: the walk's own partial-read contract
// applies here too — a collection that breaks off mid-way still yields what it
// served.
func TestPublicProductsKeepWhatTheyRead(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(publicProductsBody))

			return
		}

		w.WriteHeader(http.StatusInternalServerError)
	})

	products, err := client.GetPublicProducts()
	assert.Len(t, products, 2, "what was read is returned even when the walk broke off")

	var partial *PartialError
	assert.True(t, errors.As(err, &partial) || err == nil)
}

// TestProductNoticesSayWhichFactTheyMean: "0" and "could not read them" printed
// identically before, which is what sent the reporter investigating.
func TestProductNoticesSayWhichFactTheyMean(t *testing.T) {
	partial := PartialProductsNotice(5)
	assert.Contains(t, partial, "Products: 5 from /wp/v2/product")
	assert.Contains(t, partial, "401")
	assert.Contains(t, partial, "no price, SKU, stock or variations")
	assert.Contains(t, partial, "--auth-user", "the report names the remedy")

	none := NoProductsNotice()
	assert.Contains(t, none, "Products: 0")
	assert.Contains(t, none, "refused the request")
}
