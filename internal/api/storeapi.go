package api

// The public half of WooCommerce (#74).
//
// Products were read from /wc/v3/products, which is WooCommerce's *admin* API
// and answers 401 without consumer keys, and then from /wp/v2/product, which is
// the post type behind the shop and carries no commerce at all. So a shop whose
// keys nobody had exported as a catalog without prices, and the run told the
// operator to go and generate keys — the step that stops a migration, because
// the person moving a site and the person holding its WooCommerce keys are
// usually two different people.
//
// WooCommerce ships a second API for storefronts. /wc/store/v1/products is what
// a shop's own front end calls, it needs no credentials, and it carries what a
// catalog page needs: prices with their currency and formatting, images,
// categories, attributes, stock and ratings. It has been in core since 6.x.
//
// So the order is: the admin API when keys were given, because it alone sees
// drafts and private products; the Store API otherwise; and the post type last,
// as before, for a shop too old for either. Which one answered is stated, so an
// operator can tell "no keys, and it did not matter" from "no keys, and the
// prices are missing".

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// storeAPIPaths are the two spellings a shop answers on. The unversioned one
// answers identically where it exists, and costs one request to find out on a
// shop where the versioned one does not.
var storeAPIPaths = []string{"/wc/store/v1/products", "/wc/store/products"}

// storeProduct is the Store API's record, reduced to what an export carries.
// Its prices are strings in the smallest currency unit, with the divisor stated
// separately — 1999 with two decimals is 19.99 — because a storefront formats
// them itself.
type storeProduct struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Permalink   string `json:"permalink"`
	Type        string `json:"type"`
	Description string `json:"description"`
	ShortDesc   string `json:"short_description"`
	SKU         string `json:"sku"`
	IsInStock   bool   `json:"is_in_stock"`
	IsOnSale    bool   `json:"on_sale"`
	Rating      string `json:"average_rating"`
	ReviewCount int    `json:"review_count"`
	Prices      struct {
		Price         string `json:"price"`
		RegularPrice  string `json:"regular_price"`
		SalePrice     string `json:"sale_price"`
		CurrencyCode  string `json:"currency_code"`
		CurrencyMinor int    `json:"currency_minor_unit"`
	} `json:"prices"`
	Images []struct {
		ID  int    `json:"id"`
		Src string `json:"src"`
		Alt string `json:"alt"`
	} `json:"images"`
	Categories []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"categories"`
	Tags []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"tags"`
}

// GetStoreProducts reads the catalog from the public storefront API.
//
// It returns false when the shop does not serve one, so the caller falls
// through to the post type rather than treating "no Store API" as "no
// products".
func (c *Client) GetStoreProducts() ([]models.WooCommerceProduct, string, bool) {
	for _, path := range storeAPIPaths {
		products, route, ok := c.walkStoreAPI(path)
		if ok {
			return products, route, true
		}
	}

	return nil, "", false
}

// walkStoreAPI pages through one spelling of the storefront API.
func (c *Client) walkStoreAPI(path string) ([]models.WooCommerceProduct, string, bool) {
	budget, proceed := c.takeBudget(CollectionProducts)
	if !proceed {
		return nil, "", false
	}

	route := c.siteRoot() + "/wp-json" + path

	perPage := storePageSize
	if budget > 0 && budget < perPage {
		perPage = budget
	}

	var all []models.WooCommerceProduct

	for page := 1; ; page++ {
		if page > 1 {
			c.applyRateLimit()
		}

		batch, total, ok := c.storeProductPage(route, page, perPage)
		if !ok {
			// The first page deciding it is what tells a shop without a Store
			// API from one whose second page failed: the first means fall
			// through, the second means keep what was read.
			return all, route, page > 1
		}

		c.recordStated(CollectionProducts, total)
		c.spendBudget(len(batch))
		all = append(all, batch...)

		// A short page is the last one, and a budget already spent means the
		// operator asked for a preview rather than a catalog (#62).
		if len(batch) < perPage || (budget > 0 && len(all) >= budget) {
			break
		}
	}

	return all, route, true
}

// storePageSize is what one request asks for. The Store API caps per_page at
// 100 like the rest of WordPress.
const storePageSize = 100

// storeProductPage reads one page, reporting false for anything that is not a
// page of products.
func (c *Client) storeProductPage(route string, page, perPage int) ([]models.WooCommerceProduct, int, bool) {
	resp, err := c.httpClient.R().Get(fmt.Sprintf("%s?page=%d&per_page=%d", route, page, perPage))
	if err != nil || resp.StatusCode() != http.StatusOK {
		return nil, 0, false
	}

	var records []storeProduct
	if err := json.Unmarshal(resp.Body(), &records); err != nil || len(records) == 0 {
		return nil, 0, false
	}

	products := make([]models.WooCommerceProduct, 0, len(records))
	for i := range records {
		products = append(products, records[i].product())
	}

	return products, collectionTotal(resp.Header().Get(totalHeader)), true
}

// product maps a storefront record onto the export's own.
func (r storeProduct) product() models.WooCommerceProduct {
	product := models.WooCommerceProduct{
		ID:               r.ID,
		Name:             r.Name,
		Slug:             r.Slug,
		Permalink:        r.Permalink,
		Type:             r.Type,
		Status:           "publish", // the storefront serves nothing else
		Description:      r.Description,
		ShortDescription: r.ShortDesc,
		SKU:              r.SKU,
		Price:            storeAmount(r.Prices.Price, r.Prices.CurrencyMinor),
		RegularPrice:     storeAmount(r.Prices.RegularPrice, r.Prices.CurrencyMinor),
		SalePrice:        storeAmount(r.Prices.SalePrice, r.Prices.CurrencyMinor),
		OnSale:           r.IsOnSale,
		AverageRating:    r.Rating,
		RatingCount:      r.ReviewCount,
		StockStatus:      stockStatus(r.IsInStock),
	}

	for _, image := range r.Images {
		product.Images = append(product.Images,
			models.ProductImage{ID: image.ID, Src: image.Src, Alt: image.Alt})
	}

	for _, category := range r.Categories {
		product.Categories = append(product.Categories,
			models.ProductCategory{ID: category.ID, Name: category.Name, Slug: category.Slug})
	}

	for _, tag := range r.Tags {
		product.Tags = append(product.Tags, models.ProductTag{ID: tag.ID, Name: tag.Name, Slug: tag.Slug})
	}

	return product
}

// storeAmount turns the Store API's minor-unit integer into the decimal string
// the rest of the export carries: 1999 with two decimals is "19.99".
//
// A price is never invented. An empty field stays empty, because a "0" would
// import as a free product — which is worse than a missing one, and is the rule
// the /wp/v2 fallback already follows (#55).
func storeAmount(amount string, minorUnit int) string {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return ""
	}

	if minorUnit <= 0 {
		return amount
	}

	value, err := strconv.Atoi(amount)
	if err != nil {
		// Some shops already send a decimal string. It is the answer either way.
		return amount
	}

	divisor := 1
	for i := 0; i < minorUnit; i++ {
		divisor *= 10
	}

	return fmt.Sprintf("%d.%0*d", value/divisor, minorUnit, value%divisor)
}

// stockStatus is WooCommerce's own spelling of the flag the storefront sends as
// a boolean.
func stockStatus(inStock bool) string {
	if inStock {
		return "instock"
	}

	return "outofstock"
}

// StoreProductsNotice says the catalog came from the storefront API, and what
// that means for what it carries.
func StoreProductsNotice(count int, route string) string {
	return fmt.Sprintf("Products: %d from %s — WooCommerce's public storefront API, so these "+
		"carry prices, images, categories and stock without consumer keys. Drafts and private "+
		"products are not on a storefront; --auth-user/--auth-pass with WooCommerce keys reads "+
		"those from /wc/v3.", count, route)
}
