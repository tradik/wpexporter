package api

// Exporting less than everything (#60). The point is not to hand back fewer
// records — it is not to fetch them: a preview of five pages that downloads
// five hundred is slow for whoever asked, unkind to the source host and
// expensive for whoever pays for the bandwidth.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// countingSite serves `records` posts and pages, 100 to a page, and counts the
// requests it was asked for.
func countingSite(t *testing.T, records int, cfg *config.Config) (*Client, *atomic.Int32) {
	t.Helper()

	var requests atomic.Int32

	server := newCountingServer(t, records, &requests)

	cfg.URL = server
	cfg.Timeout = 5

	client, err := NewClient(cfg)
	require.NoError(t, err)

	return client, &requests
}

func newCountingServer(t *testing.T, records int, requests *atomic.Int32) string {
	t.Helper()

	handler := func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-WP-Total", strconv.Itoa(records))

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if page == 0 {
			page = 1
		}

		start := (page - 1) * perPage
		if start >= records {
			_, _ = w.Write([]byte(`[]`))

			return
		}

		end := start + perPage
		if end > records {
			end = records
		}

		items := make([]string, 0, end-start)
		for id := start + 1; id <= end; id++ {
			items = append(items, fmt.Sprintf(`{"id":%d,"slug":"doc-%d"}`, id, id))
		}

		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	}

	return startServer(t, handler)
}

// TestLimitStopsTheWalkRatherThanTruncating: the whole point. Five documents
// from a five-hundred-document site must cost one request, not five.
func TestLimitStopsTheWalkRatherThanTruncating(t *testing.T) {
	client, requests := countingSite(t, 500, &config.Config{Limit: 5})

	posts, err := client.GetPosts()
	require.NoError(t, err)

	assert.Len(t, posts, 5)
	assert.Equal(t, int32(1), requests.Load(), "one request, not five hundred records fetched")
	assert.Equal(t, 1, posts[0].ID, "newest first is the REST default, and the first five are the five")
}

// TestLimitAsksForOnlyWhatItNeeds: there is no sense fetching a hundred records
// to keep five, and the source host feels the difference.
func TestLimitAsksForOnlyWhatItNeeds(t *testing.T) {
	var asked string

	server := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("per_page")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1},{"id":2},{"id":3}]`))
	})

	client, err := NewClient(&config.Config{URL: server, Timeout: 5, Limit: 3})
	require.NoError(t, err)

	_, err = client.GetPosts()
	require.NoError(t, err)
	assert.Equal(t, "3", asked)
}

// TestTotalLimitIsSharedAcrossCollections: "at most N documents in total" means
// the pages see what the posts left.
func TestTotalLimitIsSharedAcrossCollections(t *testing.T) {
	client, _ := countingSite(t, 500, &config.Config{Limit: 7})

	posts, err := client.GetPosts()
	require.NoError(t, err)
	require.Len(t, posts, 7)

	pages, err := client.GetPages()
	require.NoError(t, err)
	assert.Empty(t, pages, "the budget was spent by the posts")
}

// TestPerTypeLimitGivesEachKindItsOwn: a preview usually wants a few of each,
// not a few of the first kind it met.
func TestPerTypeLimitGivesEachKindItsOwn(t *testing.T) {
	client, _ := countingSite(t, 500, &config.Config{LimitPerType: 4})

	posts, err := client.GetPosts()
	require.NoError(t, err)
	assert.Len(t, posts, 4)

	pages, err := client.GetPages()
	require.NoError(t, err)
	assert.Len(t, pages, 4, "each kind has its own budget")
}

// TestBothLimitsTakeTheSmaller: the two caps compose rather than fight.
func TestBothLimitsTakeTheSmaller(t *testing.T) {
	client, _ := countingSite(t, 500, &config.Config{Limit: 5, LimitPerType: 3})

	posts, err := client.GetPosts()
	require.NoError(t, err)
	assert.Len(t, posts, 3, "the per-type cap is the smaller one here")

	pages, err := client.GetPages()
	require.NoError(t, err)
	assert.Len(t, pages, 2, "and the total leaves only two")
}

// TestNoLimitFetchesEverything: the default is unchanged, which is what makes
// every existing export the same as it was.
func TestNoLimitFetchesEverything(t *testing.T) {
	client, _ := countingSite(t, 250, &config.Config{})

	posts, err := client.GetPosts()
	require.NoError(t, err)
	assert.Len(t, posts, 250)
	assert.False(t, client.Limited())
}

// TestStatedTotalIsRemembered: a truncated export has to be able to say what it
// truncated, or it is the silent cap this was written to avoid.
func TestStatedTotalIsRemembered(t *testing.T) {
	client, _ := countingSite(t, 75, &config.Config{Limit: 5})

	posts, err := client.GetPosts()
	require.NoError(t, err)
	require.Len(t, posts, 5)

	assert.Equal(t, 75, client.StatedTotal("posts"))
	assert.Zero(t, client.StatedTotal("pages"), "a collection never walked stated nothing")
	assert.True(t, client.Limited())
}

// TestLimitArithmetic covers the budget on its own, including the edges a flag
// can be given.
func TestLimitArithmetic(t *testing.T) {
	unlimited := newLimits(0, 0, nil)
	assert.Zero(t, unlimited.budget(CollectionPosts))
	assert.False(t, unlimited.exhausted())
	assert.False(t, unlimited.active())

	total := newLimits(10, 0, nil)
	assert.Equal(t, 10, total.budget(CollectionPosts))
	total.spend(10)
	assert.True(t, total.exhausted())
	assert.Zero(t, total.budget(CollectionPosts))

	// Spending more than the budget must not wrap into a negative one.
	over := newLimits(3, 0, nil)
	over.spend(9)
	assert.True(t, over.exhausted())

	perType := newLimits(0, 4, nil)
	assert.Equal(t, 4, perType.budget(CollectionPosts))
	perType.spend(4)
	assert.Equal(t, 4, perType.budget(CollectionPosts), "a per-type budget is not shared")

	negative := newLimits(-1, -5, map[string]int{"posts": -3})
	assert.Zero(t, negative.budget(CollectionPosts))
	assert.False(t, negative.active(), "a negative flag value is not a budget")

	// A kind named on its own beats the default for that kind, and only that
	// kind: five of everything, ten media (#62).
	shaped := newLimits(0, 5, map[string]int{CollectionMedia: 10})
	assert.Equal(t, 5, shaped.budget(CollectionPosts))
	assert.Equal(t, 10, shaped.budget(CollectionMedia))
	assert.Equal(t, 5, shaped.budget("services"), "a custom type takes the default")

	// Named kinds with no default leave everything else unbounded.
	namedOnly := newLimits(0, 0, map[string]int{CollectionPosts: 3})
	assert.Equal(t, 3, namedOnly.budget(CollectionPosts))
	assert.Zero(t, namedOnly.budget(CollectionPages))
	assert.True(t, namedOnly.active())
}

// startServer runs a stub WordPress and returns its base URL.
func startServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server.URL
}
