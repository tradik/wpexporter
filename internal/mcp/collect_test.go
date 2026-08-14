package mcp

// collectExportData is the MCP server's half of the export: the CLI has its own
// fetch loop, so anything the two disagree about is content an agent silently
// loses (#35).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
)

// commentsPage is one approved comment, as a public /wp/v2/comments read serves
// it: no edit-context status field.
const commentsPage = `[
	{"id": 11, "post": 7, "parent": 0, "author_name": "Jan",
	 "date": "2024-03-01T10:00:00", "date_gmt": "2024-03-01T09:00:00",
	 "content": {"rendered": "<p>Dobry tekst</p>"},
	 "link": "https://x.test/blog/wms/#comment-11", "type": "comment"}
]`

// wpSite answers the routes an export reads. Collections are empty apart from
// comments, which is the collection under test; every path visited is recorded
// so a test can assert what was *not* requested.
type wpSite struct {
	mu     sync.Mutex
	visits map[string]int
}

func newWPSite(t *testing.T) (*api.Client, *wpSite) {
	t.Helper()

	site := &wpSite{visits: map[string]int{}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.mu.Lock()
		site.visits[r.URL.Path]++
		page := r.URL.Query().Get("page")
		site.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/wp/v2/comments") && page != "2":
			_, _ = w.Write([]byte(commentsPage))
		case strings.HasSuffix(r.URL.Path, "/wp/v2/settings"):
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"rest_forbidden"}`))
		case r.URL.Path == "/wp-json" || r.URL.Path == "/wp-json/":
			_, _ = w.Write([]byte(`{"name":"Test","description":"A site","url":"https://x.test"}`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)

	client, err := api.NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client, site
}

func (s *wpSite) visited(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.visits[path]
}

// TestCollectExportDataCarriesComments: the tool that an assistant calls must
// export what the CLI exports, counts included — an MCP client has no console
// to read a warning from, so the count is the whole report.
func TestCollectExportDataCarriesComments(t *testing.T) {
	client, _ := newWPSite(t)

	data, err := collectExportData(client, config.DefaultConfig())
	require.NoError(t, err)

	require.Len(t, data.Comments, 1)
	assert.Equal(t, 11, data.Comments[0].ID)
	assert.Equal(t, "Jan", data.Comments[0].Author)
	assert.Equal(t, 1, data.Stats.TotalComments)
	assert.Equal(t, "Test", data.Site.Name)
}

// TestCollectExportDataHonoursNoComments: the switch has to stop the fetch, not
// just drop the result — a site with a million comments should not be read at
// all when the caller said no.
func TestCollectExportDataHonoursNoComments(t *testing.T) {
	client, site := newWPSite(t)

	cfg := config.DefaultConfig()
	cfg.NoComments = true

	data, err := collectExportData(client, cfg)
	require.NoError(t, err)

	assert.Empty(t, data.Comments)
	assert.Zero(t, data.Stats.TotalComments)
	assert.Zero(t, site.visited("/wp-json/wp/v2/comments"))
}

// TestCollectExportDataFailsOnCoreCollections: posts and pages are the export.
// Unlike comments, media or products, a refusal there cannot be reported as an
// empty collection — that would hand the caller a plausible-looking export of a
// site it never read.
func TestCollectExportDataFailsOnCoreCollections(t *testing.T) {
	for _, route := range []string{"posts", "pages"} {
		t.Run(route, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				switch {
				case strings.HasSuffix(r.URL.Path, "/wp/v2/"+route):
					w.WriteHeader(http.StatusInternalServerError)
				case r.URL.Path == "/wp-json" || r.URL.Path == "/wp-json/":
					_, _ = w.Write([]byte(`{"name":"Test","url":"https://x.test"}`))
				default:
					_, _ = w.Write([]byte(`[]`))
				}
			}))
			t.Cleanup(server.Close)

			client, err := api.NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
			require.NoError(t, err)

			data, err := collectExportData(client, config.DefaultConfig())
			require.Error(t, err)
			assert.Contains(t, err.Error(), route)
			assert.Nil(t, data)
		})
	}
}

// TestCollectExportDataSurvivesClosedComments: a site that keeps its comment
// route shut is not a failed export. Every other collection still lands.
func TestCollectExportDataSurvivesClosedComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/wp/v2/comments"):
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"rest_comment_disabled"}`))
		case r.URL.Path == "/wp-json" || r.URL.Path == "/wp-json/":
			_, _ = w.Write([]byte(`{"name":"Test","description":"A site","url":"https://x.test"}`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)

	client, err := api.NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	data, err := collectExportData(client, config.DefaultConfig())
	require.NoError(t, err)

	assert.Empty(t, data.Comments)
	assert.Equal(t, "Test", data.Site.Name)
}
