package exportcli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
)

// newCommentsServer answers one page of comments, then nothing.
func newCommentsServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		if r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server
}

func newCommentsAPIClient(t *testing.T, url string) *api.Client {
	t.Helper()

	client, err := api.NewClient(&config.Config{URL: url, Timeout: 5})
	if err != nil {
		t.Fatal(err)
	}

	return client
}

// TestFetchComments: the comments a site publishes come back as content (#35).
func TestFetchComments(t *testing.T) {
	server := newCommentsServer(t, http.StatusOK,
		`[{"id":11,"post":7,"author_name":"Jan","content":{"rendered":"<p>x</p>"}}]`)

	comments := fetchComments(newCommentsAPIClient(t, server.URL), &config.Config{})

	if len(comments) != 1 || comments[0].Author != "Jan" {
		t.Fatalf("expected the site's one comment, got %+v", comments)
	}
}

// TestFetchCommentsSkipped: --no-comments means no request at all.
func TestFetchCommentsSkipped(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested = true
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	if comments := fetchComments(newCommentsAPIClient(t, server.URL),
		&config.Config{NoComments: true}); comments != nil {
		t.Fatalf("expected no comments, got %+v", comments)
	}
	if requested {
		t.Fatal("--no-comments must not hit the API")
	}
}

// TestFetchCommentsGatedRouteDegrades: a site that does not serve comments
// costs a warning, never the export.
func TestFetchCommentsGatedRouteDegrades(t *testing.T) {
	server := newCommentsServer(t, http.StatusUnauthorized, "")

	if comments := fetchComments(newCommentsAPIClient(t, server.URL), &config.Config{}); comments != nil {
		t.Fatalf("expected no comments from a gated route, got %+v", comments)
	}
}

// TestFetchCommentsServerErrorDegrades: an outright failure is a warning too —
// the pages and posts already fetched are still worth writing.
func TestFetchCommentsServerErrorDegrades(t *testing.T) {
	server := newCommentsServer(t, http.StatusInternalServerError, "")

	if comments := fetchComments(newCommentsAPIClient(t, server.URL), &config.Config{}); comments != nil {
		t.Fatalf("expected no comments after a server error, got %+v", comments)
	}
}

// TestFetchCommentsDisabledSiteSaysSo: a site with commenting switched off must
// not be answered with "pass credentials" — there is nothing to authenticate
// for (#35).
func TestFetchCommentsDisabledSiteSaysSo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"rest_comment_disabled","message":"Comments are disabled."}`))
	}))
	t.Cleanup(server.Close)

	if comments := fetchComments(newCommentsAPIClient(t, server.URL), &config.Config{}); comments != nil {
		t.Fatalf("a disabled comment system yields nothing, got %+v", comments)
	}
}
