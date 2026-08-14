package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// onePost is the smallest body the posts collection can answer with.
const onePost = `[{"id": 1, "slug": "hello", "link": "https://x.test/hello/"}]`

func newRetryingClient(t *testing.T, retries int, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: retries})
	require.NoError(t, err)

	return client
}

// TestGetPostsRetriesTransientFailures: the failure this exists for. A shared
// host answers 500 under load and the same URL works seconds later; without a
// retry that blip ended an export of thousands of requests (#37).
func TestGetPostsRetriesTransientFailures(t *testing.T) {
	var attempts atomic.Int32

	client := newRetryingClient(t, 3, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Past the first page the collection is exhausted; a stub that always
		// answers with records would walk for ever, as a real site never does.
		if r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte(`[]`))
			return
		}

		if attempts.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write([]byte(onePost))
	})

	posts, err := client.GetPosts()
	require.NoError(t, err)
	require.Len(t, posts, 1)
	assert.Equal(t, int32(3), attempts.Load(), "two refusals then the answer")
}

// TestGetPostsDoesNotRetryAnAnswer: a 404 is the site telling us the route is
// not there. Repeating the request cannot change that, and an export of a large
// site would pay the backoff for every one of them.
func TestGetPostsDoesNotRetryAnAnswer(t *testing.T) {
	var attempts atomic.Int32

	client := newRetryingClient(t, 3, func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetPosts()
	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "asked once, answered once")
}

// TestGetPostsKeepsWhatItFetched: the second half of #37. One unreadable page
// used to discard every record already in hand, which is how a flaky site
// became an unexportable one.
func TestGetPostsKeepsWhatItFetched(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(fullPostsPage()))
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
	})

	posts, err := client.GetPosts()
	require.Error(t, err)
	assert.Len(t, posts, 100, "the first page survives the second one failing")

	var partial *PartialError
	require.True(t, errors.As(err, &partial))
	assert.Equal(t, "posts", partial.Endpoint)
	assert.Equal(t, 2, partial.Page)
	assert.Equal(t, 100, partial.Fetched)

	description, isGap := Gap(err)
	assert.True(t, isGap)
	assert.Contains(t, description, "stopped at page 2 after 100 records")
}

// fullPostsPage builds a page of exactly per_page records, so the walk asks for
// a second one.
func fullPostsPage() string {
	body := "["
	for i := 1; i <= 100; i++ {
		if i > 1 {
			body += ","
		}
		body += fmt.Sprintf(`{"id": %d, "slug": "p%d"}`, i, i)
	}

	return body + "]"
}

// TestGapIgnoresOrdinaryErrors: only a partial read is a gap. An unreachable
// host is a broken export, not an incomplete one, and reporting it as a gap
// would hand back a plausible-looking export of a site nobody read.
func TestGapIgnoresOrdinaryErrors(t *testing.T) {
	_, isGap := Gap(errors.New("no such host"))
	assert.False(t, isGap)

	_, isGap = Gap(nil)
	assert.False(t, isGap)

	cause := errors.New("API returned status 500")
	partial := &PartialError{Endpoint: "pages", Page: 2, Fetched: 100, Err: cause}
	assert.ErrorIs(t, partial, cause, "the cause stays reachable through Unwrap")
}

// TestIsTransientFailure: what deserves another attempt and what does not.
// 501 and 505 are 5xx that mean "never", which is why the rule is a list rather
// than a comparison.
func TestIsTransientFailure(t *testing.T) {
	assert.True(t, isTransientFailure(nil, errors.New("connection reset by peer")),
		"a dropped connection is the case the condition list must not silently drop")
	assert.False(t, isTransientFailure(nil, nil))

	for status, want := range map[int]bool{
		http.StatusOK:                      false,
		http.StatusNotFound:                false,
		http.StatusUnauthorized:            false,
		http.StatusBadRequest:              false,
		http.StatusNotImplemented:          false,
		http.StatusHTTPVersionNotSupported: false,
		http.StatusRequestTimeout:          true,
		http.StatusTooManyRequests:         true,
		http.StatusInternalServerError:     true,
		http.StatusBadGateway:              true,
		http.StatusServiceUnavailable:      true,
		http.StatusGatewayTimeout:          true,
	} {
		assert.Equal(t, want, transientStatus(status), "status %d", status)
	}
}

// transientStatus asks the condition about a status alone.
func transientStatus(status int) bool {
	_, transient := transientStatuses[status]

	return transient
}

// TestRetryAfterDelay: the site's own instruction wins over our backoff, in
// either form the header allows. Anything unreadable falls back to the backoff
// rather than abandoning the retry.
func TestRetryAfterDelay(t *testing.T) {
	assert.Equal(t, 12*time.Second, mustDelay(t, "12"))
	assert.Zero(t, mustDelay(t, ""))
	assert.Zero(t, mustDelay(t, "0"))
	assert.Zero(t, mustDelay(t, "not a number"))
	assert.Zero(t, mustDelay(t, time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)),
		"a date already past is no reason to wait")

	future := mustDelay(t, time.Now().Add(90*time.Second).UTC().Format(http.TimeFormat))
	assert.Greater(t, future, 30*time.Second)
	assert.LessOrEqual(t, future, 90*time.Second)
}

// mustDelay runs retryAfterDelay against a response carrying the header.
func mustDelay(t *testing.T, header string) time.Duration {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if header != "" {
			w.Header().Set("Retry-After", header)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	resp, err := client.httpClient.R().Get(server.URL)
	require.NoError(t, err)

	delay, err := retryAfterDelay(nil, resp)
	require.NoError(t, err)

	return delay
}
