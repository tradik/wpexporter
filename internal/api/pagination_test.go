package api

// A collection that refuses the page size (#43). `--custom-types mec-events`
// against a site with 56 events brought none of them: WordPress answers 400
// both past the last page and for a per_page it will not accept, and the walk
// read the second as the first — zero records, no error, nothing in the report.

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

// invalidParamBody is what a site with a lower per_page cap answers.
const invalidParamBody = `{"code":"rest_invalid_param","message":"Invalid parameter(s): per_page",` +
	`"data":{"status":400,"params":{"per_page":"per_page must be between 1 and 25."}}}`

// beyondEndBody is the ordinary end of a collection.
const beyondEndBody = `{"code":"rest_post_invalid_page_number","message":"The page number requested is larger than the number of pages available.","data":{"status":400}}`

// cappedCollection serves `records` items, but only to a request whose page
// size it accepts — exactly the shape of the site in the issue.
func cappedCollection(t *testing.T, cap, records int) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))

		if perPage > cap {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(invalidParamBody))

			return
		}

		start := (page - 1) * perPage
		if start >= records {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(beyondEndBody))

			return
		}

		end := start + perPage
		if end > records {
			end = records
		}

		items := make([]string, 0, end-start)
		for id := start + 1; id <= end; id++ {
			items = append(items, fmt.Sprintf(`{"id":%d,"slug":"event-%d"}`, id, id))
		}

		w.Header().Set("X-WP-Total", strconv.Itoa(records))
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client
}

// TestCollectionSurvivesAPageSizeCap: the bug. A site that caps per_page below
// the REST maximum used to export as an empty type.
func TestCollectionSurvivesAPageSizeCap(t *testing.T) {
	client := cappedCollection(t, 25, 56)

	posts, err := client.GetCustomPosts("mec-events")
	require.NoError(t, err)
	assert.Len(t, posts, 56, "every record the site says it has")
}

// TestCollectionReportsARefusalItCannotAdaptTo: a 400 that is neither the end
// of the collection nor a page size is a gap, not an empty type.
func TestCollectionReportsARefusalItCannotAdaptTo(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"rest_forbidden_context","message":"Sorry."}`))
	})

	posts, err := client.GetCustomPosts("mec-events")
	require.Error(t, err)
	assert.Empty(t, posts)
	assert.Contains(t, err.Error(), "rest_forbidden_context",
		"the site's own name for the refusal, so the operator can look it up")
}

// TestCollectionReportsAShortfall: the site states the size of a collection in
// a header. A walk that ends with fewer records than that has missed something,
// and saying so is the difference between a wrong export and one that knows it.
func TestCollectionReportsAShortfall(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-WP-Total", "56")

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`[{"id":1,"slug":"one"}]`))

			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(beyondEndBody))
	})

	posts, err := client.GetPosts()
	require.Error(t, err)
	assert.Len(t, posts, 1, "what was read is still returned")

	description, isGap := Gap(err)
	assert.True(t, isGap)
	assert.Contains(t, description, "the site lists 56 records here")
}

// TestCollectionIsSilentWhenItMatches: an export that read everything the site
// claims says nothing, so the shortfall line means something when it appears.
func TestCollectionIsSilentWhenItMatches(t *testing.T) {
	client := cappedCollection(t, 100, 3)

	posts, err := client.GetPosts()
	require.NoError(t, err)
	assert.Len(t, posts, 3)
}

// TestPageSizeIsNotShrunkMidWalk: page numbers are relative to the page size,
// so shrinking it after records are in hand would re-read some and skip others.
// Such a refusal is reported instead.
func TestPageSizeIsNotShrunkMidWalk(t *testing.T) {
	var requests atomic.Int32

	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if requests.Add(1) == 1 {
			items := make([]string, 0, 100)
			for id := 1; id <= 100; id++ {
				items = append(items, fmt.Sprintf(`{"id":%d}`, id))
			}
			_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))

			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(invalidParamBody))
	})

	posts, err := client.GetPosts()
	require.Error(t, err)
	assert.Len(t, posts, 100, "the first page is kept rather than re-read at another size")
	assert.Contains(t, err.Error(), "rest_invalid_param")
}

// TestClassifyRefusal covers the decision itself, including the case where no
// smaller size is left to try.
func TestClassifyRefusal(t *testing.T) {
	assert.Equal(t, 50, classifyRefusal([]byte(invalidParamBody), 100, 0).retryWith)
	assert.Equal(t, 0, classifyRefusal([]byte(invalidParamBody), 1, 0).retryWith,
		"1 is the smallest page there is")
	assert.False(t, classifyRefusal([]byte(invalidParamBody), 1, 0).done,
		"a size it cannot shrink further is a refusal, not the end of the collection")

	assert.True(t, classifyRefusal([]byte(beyondEndBody), 100, 0).done)
	assert.True(t, classifyRefusal([]byte("not json at all"), 100, 0).done,
		"a body that says nothing is read as the end, which is what it used to mean")
	assert.Equal(t, "rest_forbidden", classifyRefusal([]byte(`{"code":"rest_forbidden"}`), 100, 0).code)
}

// TestCollectionTotal reads the header, and refuses to invent a number from
// one that is missing or malformed.
func TestCollectionTotal(t *testing.T) {
	assert.Equal(t, 56, collectionTotal(" 56 "))
	assert.Equal(t, 0, collectionTotal(""))
	assert.Equal(t, 0, collectionTotal("many"))
	assert.Equal(t, 0, collectionTotal("-3"))
}

// TestNextPageSize walks the ladder and stops at the bottom.
func TestNextPageSize(t *testing.T) {
	size, ok := nextPageSize(100)
	assert.True(t, ok)
	assert.Equal(t, 50, size)

	_, ok = nextPageSize(1)
	assert.False(t, ok)

	_, ok = nextPageSize(37)
	assert.False(t, ok, "a size that is not on the ladder has no successor")
}
