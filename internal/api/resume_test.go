package api

// The fetchers behind --resume, which had no tests.
//
// They are the other half of internal/checkpoint: the state file is only
// trustworthy if the code that writes it records exactly what it fetched, and
// only useful if the code that reads it starts where the last run stopped. A
// mistake here is not a wrong file — it is a resumed export that skips a page
// it never actually read, and reports success.

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

	"github.com/tradik/wpexporter/internal/checkpoint"
	"github.com/tradik/wpexporter/internal/config"
)

// pagedSite serves `records` items of one collection, 100 to a page, and
// records which pages were asked for.
func pagedSite(t *testing.T, collection string, records int) (*Client, *[]int) {
	t.Helper()

	var asked []int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if !strings.HasSuffix(r.URL.Path, "/"+collection) {
			_, _ = w.Write([]byte(`[]`))

			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		asked = append(asked, page)

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
			items = append(items, fmt.Sprintf(`{"id":%d,"slug":"item-%d"}`, id, id))
		}

		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client, &asked
}

// freshState is a checkpoint at the beginning of an export.
func freshState(t *testing.T) *checkpoint.State {
	t.Helper()

	state, err := checkpoint.NewManager(t.TempDir(), true).Load("https://x.test")
	require.NoError(t, err)

	return state
}

// TestResumeRecordsWhatItFetched: the state written during a run has to
// describe the run, or the next one cannot trust it.
func TestResumeRecordsWhatItFetched(t *testing.T) {
	client, _ := pagedSite(t, "posts", 150)
	state := freshState(t)

	var saves atomic.Int32
	posts, err := client.GetPostsWithCheckpoint(state, func() error {
		saves.Add(1)

		return nil
	})
	require.NoError(t, err)

	assert.Len(t, posts, 150)
	assert.Len(t, state.PostIDs, 150, "every record fetched is recorded")
	assert.True(t, state.IsPostsCompleted())
	assert.Positive(t, saves.Load(), "the checkpoint is written as the walk goes, not only at the end")
}

// TestResumeStartsWhereItStopped: a run that died on page 2 asks for page 2,
// not page 1 — that is the whole feature.
func TestResumeStartsWhereItStopped(t *testing.T) {
	client, asked := pagedSite(t, "posts", 250)

	state := freshState(t)
	state.SetPostsPage(3)

	posts, err := client.GetPostsWithCheckpoint(state, nil)
	require.NoError(t, err)

	assert.Len(t, posts, 50, "the third page and what follows it")
	assert.NotContains(t, *asked, 1, "a page the previous run already read is not read again")
	assert.Contains(t, *asked, 3)
}

// TestResumeSkipsAFinishedCollection: a collection marked complete costs no
// request at all.
func TestResumeSkipsAFinishedCollection(t *testing.T) {
	client, asked := pagedSite(t, "posts", 150)

	state := freshState(t)
	state.SetPostsCompleted()

	posts, err := client.GetPostsWithCheckpoint(state, nil)
	require.NoError(t, err)

	assert.Empty(t, posts)
	assert.Empty(t, *asked, "nothing was asked of the site")
}

// TestResumeWithoutAState: the fetchers are also the plain path when --resume
// is off, and must not require a checkpoint to work.
func TestResumeWithoutAState(t *testing.T) {
	client, _ := pagedSite(t, "posts", 20)

	posts, err := client.GetPostsWithCheckpoint(nil, nil)
	require.NoError(t, err)
	assert.Len(t, posts, 20)
}

// TestResumeRecordsTheFailure: an export that dies leaves behind the reason, so
// the operator resuming it knows what happened rather than guessing.
func TestResumeRecordsTheFailure(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	state := freshState(t)

	_, err := client.GetPostsWithCheckpoint(state, nil)
	require.Error(t, err)
	assert.Contains(t, state.LastError, "500")
	assert.False(t, state.IsPostsCompleted(), "a collection that failed is not finished")
}

// TestResumeStopsWhenTheCheckpointCannotBeSaved: continuing to fetch after the
// state stopped being writable would build an export nothing could resume.
func TestResumeStopsWhenTheCheckpointCannotBeSaved(t *testing.T) {
	client, _ := pagedSite(t, "posts", 150)
	state := freshState(t)

	_, err := client.GetPostsWithCheckpoint(state, func() error {
		return fmt.Errorf("disk full")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save checkpoint")
}

// TestEveryCollectionResumes: pages, products and media carry the same
// contract as posts, and each has its own copy of it.
func TestEveryCollectionResumes(t *testing.T) {
	t.Run("pages", func(t *testing.T) {
		client, _ := pagedSite(t, "pages", 30)
		state := freshState(t)

		pages, err := client.GetPagesWithCheckpoint(state, nil)
		require.NoError(t, err)
		assert.Len(t, pages, 30)
		assert.True(t, state.IsPagesCompleted())
		assert.Len(t, state.PageIDs, 30)
	})

	t.Run("media", func(t *testing.T) {
		client, _ := pagedSite(t, "media", 12)
		state := freshState(t)

		media, err := client.GetMediaWithCheckpoint(state, nil)
		require.NoError(t, err)
		assert.Len(t, media, 12)
		assert.True(t, state.IsMediaCompleted())
		assert.Len(t, state.MediaIDs, 12)
	})

	t.Run("finished collections are skipped", func(t *testing.T) {
		client, asked := pagedSite(t, "pages", 30)

		state := freshState(t)
		state.SetPagesCompleted()
		state.SetMediaCompleted()
		state.SetProductsCompleted()

		pages, err := client.GetPagesWithCheckpoint(state, nil)
		require.NoError(t, err)
		assert.Empty(t, pages)

		media, err := client.GetMediaWithCheckpoint(state, nil)
		require.NoError(t, err)
		assert.Empty(t, media)

		products, err := client.GetProductsWithCheckpoint(state, nil)
		require.NoError(t, err)
		assert.Empty(t, products)

		assert.Empty(t, *asked)
	})
}

// TestProductsResumeToleratesNoWooCommerce: a site without the plugin answers
// 404 on the products route, which is not a failed export.
func TestProductsResumeToleratesNoWooCommerce(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	products, err := client.GetProductsWithCheckpoint(freshState(t), nil)

	assert.Empty(t, products)
	assert.NoError(t, err, "no WooCommerce is a fact about the site, not an error")
}
