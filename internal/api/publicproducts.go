package api

// A catalog without API keys (#55).
//
// Products are fetched from /wc/v3/products, which needs WooCommerce consumer
// keys. A shop that has not handed any out answers 401 there — and the same
// products are usually public on the ordinary WordPress route:
//
//	GET /wp-json/wc/v3/products?per_page=1   → 401
//	GET /wp-json/wp/v2/product?per_page=100  → 200, 5 items
//
// `product` is excluded from the custom-type walk because it has its own
// exporter, so a shop without keys had no path at all: zero products, and a
// summary that said "0" as though the shop were empty.
//
// What the public route carries is the catalog *page* — title, slug, address,
// description, featured image, categories and tags. What it does not carry is
// the commerce: price, SKU, stock, variations, attributes and dimensions live in
// post meta and WooCommerce's own tables, and are not in that payload. So this
// is a fallback and says so: it stops a migrated catalog being a wall of
// 404s, and it is not a substitute for the keys.

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/tradik/wpexporter/pkg/models"
)

// ErrProductsNeedKeys reports that the WooCommerce API refused the request for
// want of consumer keys. It is a fact about the shop's configuration rather
// than a failure of the export, and the caller answers it by reading the
// products the site publishes anyway.
var ErrProductsNeedKeys = errors.New("the WooCommerce API needs consumer keys")

// publicProductBase is the post type WooCommerce registers its products under.
const publicProductBase = "product"

// GetPublicProducts reads the products the site publishes on the ordinary
// WordPress route, for a shop whose WooCommerce API refused the request.
//
// The records are deliberately partial, and every field the public route cannot
// answer for is left empty rather than filled with a plausible zero: a price of
// "0" would import as a free product, which is worse than an absent one.
func (c *Client) GetPublicProducts() ([]models.WooCommerceProduct, error) {
	posts, err := c.getAllContent(publicProductBase)
	if err != nil && len(posts) == 0 {
		return nil, err
	}

	products := make([]models.WooCommerceProduct, 0, len(posts))
	for i := range posts {
		products = append(products, publicProduct(posts[i]))
	}

	return products, err
}

// publicProduct maps one published product page onto the product record.
func publicProduct(post models.WordPressPost) models.WooCommerceProduct {
	return models.WooCommerceProduct{
		ID:               post.ID,
		Name:             post.Title.Rendered,
		Slug:             post.Slug,
		Permalink:        post.Link,
		DateCreated:      post.Date,
		DateModified:     post.Modified,
		Status:           post.Status,
		Type:             "simple", // the route says nothing about variations
		Description:      post.Content.Rendered,
		ShortDescription: post.Excerpt.Rendered,
	}
}

// PartialProductsNotice explains what such an export does and does not carry.
// It is one sentence because it has to survive being read in a hurry, and it
// names the remedy rather than only the symptom.
func PartialProductsNotice(count int) string {
	return fmt.Sprintf(
		"Products: %d from /wp/v2/product — the WooCommerce API refused the request (401: no "+
			"consumer keys), so these carry title, address, description, image and terms, and no "+
			"price, SKU, stock or variations. Pass --auth-user/--auth-pass with WooCommerce keys "+
			"for the full catalog.", count)
}

// NoProductsNotice tells "there are none" apart from "they could not be read",
// which the summary used to print identically as `Products: 0`.
//
// It names the route and what the route answered. The first version said the
// public route "published none either", which is a claim rather than an
// observation: on a shop whose products were simply never reached, that line
// sent the operator hunting for a route bug that was not there, and cost them a
// day (#65). A report may say what happened; it may not say what it concluded.
func NoProductsNotice(route string, status int) string {
	return noProductsBecause(route, status, nil)
}

// noProductsBecause is the same line with the walk's own failure to hand.
//
// "published none" is a claim about the shop, and it may only be made when the
// route actually answered with an empty collection. A route that was refused, or
// that served a page of HTML because a wall stood in front of it, published
// nothing of the kind — it was never read, and saying otherwise sent the
// reporter of #73 to check an endpoint that had been returning five products
// all along.
func noProductsBecause(route string, status int, err error) string {
	if route == "" {
		route = "/wp/v2/" + publicProductBase
	}

	return fmt.Sprintf("Products: 0 — the WooCommerce API refused the request "+
		"(401: no consumer keys) and %s %s.", route, routeOutcome(status, err))
}

// routeOutcome is what the fallback route did, in the words of what happened.
func routeOutcome(status int, err error) string {
	switch {
	case errors.Is(err, ErrNotJSON):
		return "answered with a page rather than a REST document, so it was never read — " +
			"try --user-agent with a browser's string if a wall is in front of this site"
	case status > 0 && status != 200:
		return fmt.Sprintf("answered %d", status)
	case err != nil:
		return fmt.Sprintf("could not be read (%v)", err)
	default:
		return "published none"
	}
}

// PublicProductRoute is the address the fallback reads, so a report can name it
// rather than leave the operator to guess which spelling was tried (#65).
func (c *Client) PublicProductRoute() string {
	return c.baseURL + "/" + publicProductBase
}

// RefusalStatus digs the HTTP status out of a walk's failure, or 0 when the
// failure was not a status at all.
func RefusalStatus(err error) int {
	if err == nil {
		return 0
	}

	var partial *PartialError
	if !errors.As(err, &partial) || partial.Err == nil {
		return 0
	}

	match := statusInMessageRe.FindStringSubmatch(partial.Err.Error())
	if match == nil {
		return 0
	}

	status, convErr := strconv.Atoi(match[1])
	if convErr != nil {
		return 0
	}

	return status
}

// statusInMessageRe reads the number back out of "API returned status 404".
var statusInMessageRe = regexp.MustCompile(`status (\d{3})`)

// NoProductsFailure is the line for a fallback that brought nothing, told from
// the walk's own error rather than from a status alone (#73).
func NoProductsFailure(route string, err error) string {
	return noProductsBecause(route, RefusalStatus(err), err)
}
