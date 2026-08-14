package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/cache"
	"github.com/tradik/wpexporter/internal/config"
)

// commentsBody is a public (view-context) response: no `status` field, a
// threaded reply, and the avatar renditions WordPress offers.
const commentsBody = `[
	{"id": 11, "post": 7, "parent": 0, "author_name": "Jan",
	 "author_url": "https://jan.test", "date": "2024-03-01T10:00:00",
	 "date_gmt": "2024-03-01T09:00:00", "content": {"rendered": "<p>Dobry tekst</p>"},
	 "link": "https://x.test/blog/wms/#comment-11", "type": "comment",
	 "author_avatar_urls": {"24": "https://g.test/24", "96": "https://g.test/96", "48": "https://g.test/48"}},
	{"id": 12, "post": 7, "parent": 11, "author_name": "Ewa",
	 "date": "2024-03-02T10:00:00", "date_gmt": "2024-03-02T09:00:00",
	 "content": {"rendered": "<p>Zgadzam się</p>"},
	 "link": "https://x.test/blog/wms/#comment-12", "type": "comment", "status": "approved"}
]`

func newCommentsClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client
}

// TestGetComments: a public read carries author, thread and body, and the
// missing edit-context status resolves to "approved" — which is all the public
// collection ever lists.
func TestGetComments(t *testing.T) {
	client := newCommentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "asc", r.URL.Query().Get("order"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(commentsBody))
	})

	comments, err := client.GetComments()
	require.NoError(t, err)
	require.Len(t, comments, 2)

	assert.Equal(t, 11, comments[0].ID)
	assert.Equal(t, 7, comments[0].Post)
	assert.Equal(t, "Jan", comments[0].Author)
	assert.Equal(t, "https://jan.test", comments[0].AuthorURL)
	assert.Equal(t, "<p>Dobry tekst</p>", comments[0].Content)
	assert.Equal(t, "approved", comments[0].Status, "a view-context read implies approved")
	assert.Equal(t, "https://g.test/96", comments[0].AuthorAvatar, "largest rendition wins")
	assert.Equal(t, 2024, comments[0].Date.Year())

	assert.Equal(t, 11, comments[1].Parent, "the reply keeps its parent")
}

// TestGetCommentsPaginates: a full page is followed by the next one, and a
// short page ends the walk without a wasted request.
func TestGetCommentsPaginates(t *testing.T) {
	var requested []string

	client := newCommentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(fullCommentsPage(commentsPerPage, 1)))
			return
		}
		_, _ = w.Write([]byte(fullCommentsPage(2, 1000)))
	})

	comments, err := client.GetComments()
	require.NoError(t, err)
	assert.Len(t, comments, commentsPerPage+2)
	assert.Equal(t, []string{"1", "2"}, requested)
}

// TestGetCommentsSortsByID: threads only replay in creation order, so a reply
// must never precede its parent however the API ordered the response.
func TestGetCommentsSortsByID(t *testing.T) {
	client := newCommentsClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id": 20, "post": 1, "parent": 19}, {"id": 19, "post": 1}]`))
	})

	comments, err := client.GetComments()
	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, 19, comments[0].ID)
	assert.Equal(t, 20, comments[1].ID)
}

// TestGetCommentsPastLastPage: WordPress answers 400 beyond the last page,
// which ends the walk rather than failing the export.
func TestGetCommentsPastLastPage(t *testing.T) {
	client := newCommentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fullCommentsPage(commentsPerPage, 1)))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	})

	comments, err := client.GetComments()
	require.NoError(t, err)
	assert.Len(t, comments, commentsPerPage)
}

// TestGetCommentsNotAccessible: a disabled or gated route is a normal outcome
// reported as ErrCommentsNotAccessible, so the caller can warn and carry on.
func TestGetCommentsNotAccessible(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		client := newCommentsClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		})

		_, err := client.GetComments()
		assert.True(t, errors.Is(err, ErrCommentsNotAccessible), "status %d", status)
	}
}

// TestGetCommentsDisabled: a site with commenting switched off answers 403
// rest_comment_disabled. That is not a permission problem, and the caller must
// be able to tell it apart — no credential opens a route to comments that do
// not exist (magnavalor.eu answers exactly this).
func TestGetCommentsDisabled(t *testing.T) {
	client := newCommentsClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"rest_comment_disabled",` +
			`"message":"Comments are disabled.","data":{"status":403}}`))
	})

	_, err := client.GetComments()
	assert.True(t, errors.Is(err, ErrCommentsDisabled), "got %v", err)
	assert.False(t, errors.Is(err, ErrCommentsNotAccessible),
		"a disabled comment system is not a gated one")
}

// TestGetCommentsServerError: anything else is a real failure and says so.
func TestGetCommentsServerError(t *testing.T) {
	client := newCommentsClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.GetComments()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// TestGetCommentsMalformed: a body that is not the documented array is an
// error, not an empty comment set.
func TestGetCommentsMalformed(t *testing.T) {
	client := newCommentsClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"rest_no"}`))
	})

	_, err := client.GetComments()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse comments response")
}

// TestGetCommentsTransportError: an unreachable host is reported with the page
// that failed.
func TestGetCommentsTransportError(t *testing.T) {
	client, err := NewClient(&config.Config{URL: "http://127.0.0.1:1", Timeout: 1, Retries: 0})
	require.NoError(t, err)

	_, err = client.GetComments()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "comments page 1")
}

// TestGetCommentsUsesCache: the second read is served from disk, so a resumed
// or repeated export does not re-walk a site's whole comment history.
func TestGetCommentsUsesCache(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(commentsBody))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	fileCache, err := cache.NewFileCache(t.TempDir(), time.Hour, server.URL)
	require.NoError(t, err)
	client.SetCache(fileCache)

	first, err := client.GetComments()
	require.NoError(t, err)
	afterFirst := hits

	second, err := client.GetComments()
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Equal(t, afterFirst, hits, "the cached read must not touch the API")
}

// TestLargestAvatarSkipsJunk: an empty URL and a non-numeric key are ignored
// rather than chosen.
func TestLargestAvatarSkipsJunk(t *testing.T) {
	assert.Equal(t, "https://g.test/48", largestAvatar(map[string]string{
		"24": "https://g.test/24", "48": "https://g.test/48", "96": "", "full": "https://g.test/full",
	}))
	assert.Empty(t, largestAvatar(nil))
}

// fullCommentsPage builds n comments starting at the given id.
func fullCommentsPage(n, firstID int) string {
	items := make([]string, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, fmt.Sprintf(
			`{"id": %d, "post": 1, "author_name": "A", "content": {"rendered": "x"}}`, firstID+i))
	}

	return "[" + strings.Join(items, ",") + "]"
}
