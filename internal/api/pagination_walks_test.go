package api

// Every paginated walk ends (#37, and the media walk that did not): when the
// site's own page count is reached, when it serves the page before again —
// which is what a server that ignores ?page= does, header or not — and at the
// page ceiling. A normal listing is still read to the end.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// walk is one paginated read, reduced to what it returned.
type walk struct {
	path string
	run  func(*Client) (int, error)
}

func count[T any](items []T, err error) (int, error) { return len(items), err }

// walks are every paginated read of the REST client.
var walks = map[string]walk{
	"posts":    {"/wp-json/wp/v2/posts", func(c *Client) (int, error) { return count(c.GetPosts()) }},
	"pages":    {"/wp-json/wp/v2/pages", func(c *Client) (int, error) { return count(c.GetPages()) }},
	"custom":   {"/wp-json/wp/v2/services", func(c *Client) (int, error) { return count(c.GetCustomPosts("services")) }},
	"media":    {"/wp-json/wp/v2/media", func(c *Client) (int, error) { return count(c.GetMedia()) }},
	"cats":     {"/wp-json/wp/v2/categories", func(c *Client) (int, error) { return count(c.GetCategories()) }},
	"tags":     {"/wp-json/wp/v2/tags", func(c *Client) (int, error) { return count(c.GetTags()) }},
	"users":    {"/wp-json/wp/v2/users", func(c *Client) (int, error) { return count(c.GetUsers()) }},
	"comments": {"/wp-json/wp/v2/comments", func(c *Client) (int, error) { return count(c.GetComments()) }},
	"products": {"/wp-json/wc/v3/products", func(c *Client) (int, error) { return count(c.GetProducts()) }},
	"store": {"/wp-json/wc/store/v1/products", func(c *Client) (int, error) {
		products, _, _ := c.GetStoreProducts()
		return len(products), nil
	}},
	"posts checkpoint": {"/wp-json/wp/v2/posts", func(c *Client) (int, error) {
		return count(c.GetPostsWithCheckpoint(nil, nil))
	}},
	"pages checkpoint": {"/wp-json/wp/v2/pages", func(c *Client) (int, error) {
		return count(c.GetPagesWithCheckpoint(nil, nil))
	}},
	"products checkpoint": {"/wp-json/wc/v3/products", func(c *Client) (int, error) {
		return count(c.GetProductsWithCheckpoint(nil, nil))
	}},
	"media checkpoint": {"/wp-json/wp/v2/media", func(c *Client) (int, error) {
		return count(c.GetMediaWithCheckpoint(nil, nil))
	}},
}

// A site that ignores ?page= — with the header, and without it — ends the
// walk at the second request, with the first page kept once and the repeat
// reported.
func TestWalksEndWhenThePageRepeats(t *testing.T) {
	for _, headers := range []bool{true, false} {
		for name, w := range walks {
			t.Run(fmt.Sprintf("%s headers=%v", name, headers), func(t *testing.T) {
				client, asked := walkSite(t, listing{records: 1000, ignorePage: true, headers: headers})
				n, err := w.run(client)

				assert.Equal(t, 100, n, "the first page, once")
				// The site claims ten pages either way; the repeat is seen on
				// the second.
				assert.Equal(t, 2, asked(w.path))
				if name != "store" {
					var partial *PartialError
					require.True(t, errors.As(err, &partial), "a repeat is a gap, not the end: %v", err)
					assert.ErrorIs(t, err, errPageRepeated)
					assert.Equal(t, 100, partial.Fetched)
				}
			})
		}
	}
}

// The site's own page count ends the walk: a page it serves past the count
// is not asked for.
func TestWalksStopAtTheStatedPageCount(t *testing.T) {
	for name, w := range walks {
		t.Run(name, func(t *testing.T) {
			client, asked := walkSite(t, listing{records: 1000, headers: true, claimPages: 2})
			n, _ := w.run(client)
			assert.Equal(t, 200, n)
			assert.Equal(t, 2, asked(w.path))
		})
	}
}

// A normal listing of 250 records — refused past the end with the headers, or
// an empty page without them — is still read to the end.
func TestWalksReadANormalListingToTheEnd(t *testing.T) {
	for _, l := range []listing{
		{records: 250, headers: true, refuseEnd: true},
		{records: 250},
		{records: 300}, // full last page, then an empty one
	} {
		for name, w := range walks {
			t.Run(fmt.Sprintf("%s %+v", name, l), func(t *testing.T) {
				client, _ := walkSite(t, l)
				n, err := w.run(client)
				require.NoError(t, err)
				assert.Equal(t, l.records, n)
			})
		}
	}
}

// The ceiling holds for a site that never repeats and never stops: the walk
// ends with what it read and says so.
func TestPagerCeiling(t *testing.T) {
	var p pager
	last, err := p.done(maxContentPages-1, 0, 1)
	assert.False(t, last)
	assert.NoError(t, err)

	last, err = p.done(maxContentPages, 0, 7)
	assert.True(t, last)
	var partial *PartialError
	require.True(t, errors.As(err, &partial))
	assert.Equal(t, 7, partial.Fetched)
	assert.Contains(t, err.Error(), "did not end within")
}
