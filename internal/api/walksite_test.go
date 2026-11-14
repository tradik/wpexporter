package api

// The stub site the pagination tests walk: one listing of records {"id": N},
// paginated as configured — or not at all.

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// listing is how the stub site paginates.
type listing struct {
	// records is the collection's size; pages past it are refused or empty.
	records int
	// ignorePage serves the first page for every ?page=.
	ignorePage bool
	// headers sends X-WP-Total and X-WP-TotalPages.
	headers bool
	// claimPages overrides X-WP-TotalPages, to state fewer pages than served.
	claimPages int
	// refuseEnd answers 400 rest_post_invalid_page_number past the end,
	// as WordPress does; otherwise it answers [].
	refuseEnd bool
}

// walkSite serves every collection the client walks as records {"id": N},
// and counts the requests made for each path.
func walkSite(t *testing.T, l listing) (*Client, func(path string) int) {
	t.Helper()
	var mu sync.Mutex
	asked := map[string]int{}

	url := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked[r.URL.Path]++
		mu.Unlock()
		l.serve(w, r)
	})

	cfg := config.DefaultConfig()
	cfg.URL = url
	cfg.Timeout = 5
	cfg.Retries = 0
	client, err := NewClient(cfg)
	require.NoError(t, err)

	return client, func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return asked[path]
	}
}

// serve answers one page request the way the listing paginates.
func (l listing) serve(w http.ResponseWriter, r *http.Request) {
	page, perPage := l.requested(r)
	if l.headers {
		w.Header().Set("X-WP-Total", strconv.Itoa(l.records))
		w.Header().Set("X-WP-TotalPages", strconv.Itoa(l.pages(perPage)))
	}

	start := (page - 1) * perPage
	if start >= l.records {
		l.pastTheEnd(w)
		return
	}
	ids := make([]string, 0, perPage)
	for id := start + 1; id <= min(start+perPage, l.records); id++ {
		ids = append(ids, fmt.Sprintf(`{"id":%d}`, id))
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("[" + strings.Join(ids, ",") + "]"))
}

// requested reads ?page= and ?per_page=, as a site that ignores the first one
// would.
func (l listing) requested(r *http.Request) (page, perPage int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 || l.ignorePage {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	return page, perPage
}

// pages is the page count the listing states.
func (l listing) pages(perPage int) int {
	if l.claimPages > 0 {
		return l.claimPages
	}
	return (l.records + perPage - 1) / perPage
}

// pastTheEnd answers a page past the last: refused as WordPress does, or empty.
func (l listing) pastTheEnd(w http.ResponseWriter) {
	if l.refuseEnd {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"rest_post_invalid_page_number"}`))
		return
	}
	_, _ = w.Write([]byte(`[]`))
}
