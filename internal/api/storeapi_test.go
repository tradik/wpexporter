package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// storeRecord is one product as the storefront API sends it: prices as integers
// in the smallest currency unit, with the divisor stated separately.
func storeRecord(id int) string {
	return fmt.Sprintf(`{"id":%d,"name":"Panel %d","slug":"panel-%d",`+
		`"permalink":"https://sklep.test/produkt/panel-%d/","type":"simple",`+
		`"description":"<p>Opis</p>","short_description":"Krotki","sku":"PD-%d",`+
		`"is_in_stock":true,"on_sale":true,"average_rating":"4.5","review_count":7,`+
		`"prices":{"price":"18900","regular_price":"21900","sale_price":"18900",`+
		`"currency_code":"PLN","currency_minor_unit":2},`+
		`"images":[{"id":9,"src":"https://sklep.test/panel.jpg","alt":"Panel"}],`+
		`"categories":[{"id":5,"name":"Podłogi","slug":"podlogi"}],`+
		`"tags":[{"id":3,"name":"Promocja","slug":"promocja"}]}`, id, id, id, id, id)
}

// storeServer answers the storefront API at one spelling and 404s the rest.
func storeServer(t *testing.T, path string, count int, asked *[]string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked = append(*asked, r.URL.Path)

		if !strings.HasSuffix(r.URL.Path, path) {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte(`[]`))

			return
		}

		w.Header().Set("X-WP-Total", fmt.Sprint(count))

		records := make([]string, 0, count)
		for i := 1; i <= count; i++ {
			records = append(records, storeRecord(i))
		}

		_, _ = w.Write([]byte("[" + strings.Join(records, ",") + "]"))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client
}

// TestStoreAPICarriesThePrices: the whole point of #74. /wc/v3 is the admin API
// and answers 401 without keys; /wp/v2/product is the post type and carries no
// commerce. The storefront API is public and carries what a catalog needs, so a
// migration no longer stops at "go and generate consumer keys" — the step that
// usually needs a different person entirely.
func TestStoreAPICarriesThePrices(t *testing.T) {
	var asked []string
	client := storeServer(t, "/wc/store/v1/products", 5, &asked)

	products, route, ok := client.GetStoreProducts()
	require.True(t, ok)
	require.Len(t, products, 5)
	assert.Contains(t, route, "/wc/store/v1/products")

	first := products[0]
	assert.Equal(t, "Panel 1", first.Name)
	assert.Equal(t, "189.00", first.Price, "minor units become the decimal the export carries")
	assert.Equal(t, "219.00", first.RegularPrice)
	assert.True(t, first.OnSale)
	assert.Equal(t, "instock", first.StockStatus)
	assert.Equal(t, "PD-1", first.SKU)
	require.Len(t, first.Images, 1)
	require.Len(t, first.Categories, 1)
	assert.Equal(t, "Podłogi", first.Categories[0].Name)
	assert.Equal(t, "Promocja", first.Tags[0].Name)
}

// TestStoreAPIUnversionedSpelling: /wc/store/products answers identically where
// it exists, and a shop serving only that one would otherwise fall through to
// the catalog-without-prices.
func TestStoreAPIUnversionedSpelling(t *testing.T) {
	var asked []string
	client := storeServer(t, "/wc/store/products", 2, &asked)

	products, route, ok := client.GetStoreProducts()
	require.True(t, ok)
	assert.Len(t, products, 2)
	assert.Contains(t, route, "/wc/store/products")
}

// TestNoStoreAPIFallsThrough: a shop too old for the storefront API must fall
// through to the post type rather than have "no Store API" reported as "no
// products" — which is the shape of failure this project keeps meeting.
func TestNoStoreAPIFallsThrough(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	products, _, ok := client.GetStoreProducts()
	assert.False(t, ok)
	assert.Empty(t, products)
}

// TestStoreAmount: a price is never invented. An empty field stays empty,
// because a "0" imports as a free product — worse than a missing one.
func TestStoreAmount(t *testing.T) {
	assert.Equal(t, "19.99", storeAmount("1999", 2))
	assert.Equal(t, "5.000", storeAmount("5000", 3))
	assert.Equal(t, "1899", storeAmount("1899", 0))
	assert.Equal(t, "19.99", storeAmount("19.99", 2), "a shop already sending decimals is believed")
	assert.Empty(t, storeAmount("", 2))
	assert.Empty(t, storeAmount("   ", 2))
	assert.Equal(t, "0.09", storeAmount("9", 2), "a small amount keeps its leading zero")
}

// TestStoreNoticeSaysWhatItCarries: an operator has to be able to tell "no
// keys, and it did not matter" from "no keys, and the prices are missing".
func TestStoreNoticeSaysWhatItCarries(t *testing.T) {
	notice := StoreProductsNotice(5, "https://sklep.test/wp-json/wc/store/v1/products")

	assert.Contains(t, notice, "Products: 5")
	assert.Contains(t, notice, "prices")
	assert.Contains(t, notice, "Drafts and private products are not on a storefront")
}

// TestStoreAPIPagesAndStopsAtTheBudget: a preview asks for what it needs and
// stops, rather than fetching a catalog and discarding it (#60, #62).
func TestStoreAPIPagesAndStopsAtTheBudget(t *testing.T) {
	var pages []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/wc/store/v1/products") {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		pages = append(pages, r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-WP-Total", "40")
		_, _ = w.Write([]byte("[" + storeRecord(1) + "," + storeRecord(2) + "]"))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0, Limit: 2})
	require.NoError(t, err)

	products, _, ok := client.GetStoreProducts()
	require.True(t, ok)
	assert.Len(t, products, 2)
	assert.Equal(t, []string{"1"}, pages, "one request, and the budget is spent")
	assert.Equal(t, 40, client.StatedTotal(CollectionProducts),
		"what the shop says it holds, so the truncation can be reported")
}

// TestStockStatusBothWays: WooCommerce's own spelling of a flag the storefront
// sends as a boolean.
func TestStockStatusBothWays(t *testing.T) {
	assert.Equal(t, "instock", stockStatus(true))
	assert.Equal(t, "outofstock", stockStatus(false))
}

// TestAPageOfHTMLIsNotACatalog: a wall answering 200 must not be read as a
// storefront with one unparseable product — it falls through, and the fallback
// below it reports what happened (#73).
func TestAPageOfHTMLIsNotACatalog(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(cloudflareBlockPage))
	})

	products, _, ok := client.GetStoreProducts()
	assert.False(t, ok)
	assert.Empty(t, products)
}
