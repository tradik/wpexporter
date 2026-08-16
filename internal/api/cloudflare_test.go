package api

// Naming the wall (#58), and the media walk that must not end a run (#57).

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockPage is what a firewall rule serves.
const blockPage = `<!DOCTYPE html><html><head><title>Attention Required! | Cloudflare</title></head>
<body><h1>Sorry, you have been blocked</h1><p>Error code: 1020</p></body></html>`

// challengePage is the interstitial a browser is expected to solve.
const challengePage = `<!DOCTYPE html><html><head><title>Just a moment...</title></head>
<body><div class="cf-browser-verification">Checking your browser before accessing</div></body></html>`

// TestCloudflareBlockIsNamed: a 403 on a route a browser opens fine is bot
// protection, not a broken REST API — and the report has to say which.
func TestCloudflareBlockIsNamed(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("cf-ray", "8f2c1a2b3c4d5e6f-WAW")
		w.Header().Set("server", "cloudflare")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(blockPage))
	})

	_, err := client.GetPosts()
	require.Error(t, err)

	description, isGap := Gap(err)
	require.True(t, isGap, "a wall is a gap, not the end of the export")
	assert.Contains(t, description, "Cloudflare refused the request")
	assert.Contains(t, description, "error 1020")
	assert.Contains(t, description, "--user-agent", "the report names the remedy that applies")
	assert.Contains(t, description, "--rate-limit")
}

// TestAnOrdinaryRefusalIsNotBlamedOnCloudflare: a plain 403 from WordPress must
// not be dressed up as a firewall, or the advice sends the operator to the
// wrong place.
func TestAnOrdinaryRefusalIsNotBlamedOnCloudflare(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"rest_forbidden"}`))
	})

	_, err := client.GetPosts()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 403")
	assert.NotContains(t, err.Error(), "Cloudflare")
}

// TestChallengeIsNotRetried: an identical request cannot pass an interstitial,
// so three backoff waits are three delays for nothing.
func TestChallengeIsNotRetried(t *testing.T) {
	var attempts int

	client := newRetryingClient(t, 3, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("cf-ray", "8f2c1a2b3c4d5e6f-WAW")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(challengePage))
	})

	_, err := client.GetPosts()
	require.Error(t, err)
	assert.Equal(t, 1, attempts, "asked once; a wall is not weather")
	assert.Contains(t, err.Error(), "browser challenge")
}

// TestOrdinary503IsStillRetried: the challenge check must not cost a struggling
// server its retries.
func TestOrdinary503IsStillRetried(t *testing.T) {
	var attempts int

	client := newRetryingClient(t, 2, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")

		if attempts <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		if r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte(`[]`))

			return
		}

		_, _ = w.Write([]byte(onePost))
	})

	posts, err := client.GetPosts()
	require.NoError(t, err)
	assert.Len(t, posts, 1)
	assert.GreaterOrEqual(t, attempts, 3, "two refusals were retried, then the walk went on")
}

// TestInspectRefusalReadsTheMarks: each of the three signatures on its own, and
// a response carrying none of them.
func TestInspectRefusalReadsTheMarks(t *testing.T) {
	assert.Empty(t, InspectRefusal(nil).Advice())

	byHeader := Refusal{ByCloudflare: true}
	assert.Contains(t, byHeader.Advice(), "Cloudflare refused")

	withCode := Refusal{ByCloudflare: true, Code: "1015"}
	assert.Contains(t, withCode.Advice(), "error 1015")

	challenge := Refusal{ByCloudflare: true, Challenge: true}
	assert.Contains(t, challenge.Advice(), "browser challenge")

	assert.Empty(t, Refusal{}.Advice(), "nothing is claimed about a response with no marks")
}

// TestMediaWalkKeepsWhatItRead: the failure from #57. A single 500 on page 22
// of the media listing ended the whole run and discarded 1251 posts and 89
// pages already in hand.
func TestMediaWalkKeepsWhatItRead(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page >= 3 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		items := make([]string, 0, 100)
		for id := (page-1)*100 + 1; id <= page*100; id++ {
			items = append(items, `{"id":`+strconv.Itoa(id)+`,"source_url":"https://x.test/a.jpg"}`)
		}

		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	})

	media, err := client.GetMedia()
	require.Error(t, err)
	assert.Len(t, media, 200, "two pages read before the wall are two pages kept")

	description, isGap := Gap(err)
	assert.True(t, isGap)
	assert.Contains(t, description, "media: stopped at page 3 after 200 records")
}

// TestEveryCollectionKeepsWhatItRead: categories, tags and users carry the same
// contract, so one failure cannot cost an export the rest of its run.
func TestEveryCollectionKeepsWhatItRead(t *testing.T) {
	for _, collection := range []string{"categories", "tags", "users"} {
		t.Run(collection, func(t *testing.T) {
			client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
			})

			var err error
			switch collection {
			case "categories":
				_, err = client.GetCategories()
			case "tags":
				_, err = client.GetTags()
			case "users":
				_, err = client.GetUsers()
			}

			require.Error(t, err)
			description, isGap := Gap(err)
			require.True(t, isGap, "%s must report a gap rather than a bare failure", collection)
			assert.Contains(t, description, collection)
		})
	}
}
