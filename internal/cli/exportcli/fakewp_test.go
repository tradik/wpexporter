package exportcli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeWordPress serves the REST routes an ordinary export reads: one post, one
// page, one category, one user, one comment, one image; menus refused as a
// public site refuses them; no WooCommerce.
func fakeWordPress(t *testing.T) *httptest.Server {
	return fakeWordPressWith(t, nil)
}

// writeList answers a collection request: body on page 1, an empty list after.
func writeList(w http.ResponseWriter, r *http.Request, body string) {
	if page := r.URL.Query().Get("page"); page != "" && page != "1" {
		body = "[]" // one page of everything; a walk past it finds nothing
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-WP-Total", fmt.Sprint(strings.Count(body, `"id"`)))
	w.Header().Set("X-WP-TotalPages", "1")
	_, _ = w.Write([]byte(body))
}

// answer returns a handler writing status and body.
func answer(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// fakeWordPressWith is fakeWordPress with some routes answered by overrides,
// keyed by path; "%s" in a body stands for the server's own URL.
func fakeWordPressWith(t *testing.T, overrides map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := srv.URL
		if h, ok := overrides[r.URL.Path]; ok {
			h(w, r)
			return
		}
		list := func(body string) { writeList(w, r, body) }
		switch r.URL.Path {
		case "/wp-json/", "/wp-json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"name":"Fake Site","description":"A test","url":%q,"home":%q,"namespaces":["wp/v2"]}`, base, base)
		case "/wp-json/wp/v2/posts":
			list(fmt.Sprintf(`[{"id":1,"date":"2024-01-02T03:04:05","slug":"hello","status":"publish","type":"post",`+
				`"link":"%s/hello/","title":{"rendered":"Hello"},"content":{"rendered":"<p>Hi <img src=\"%s/wp-content/uploads/a.png\"></p>"},`+
				`"excerpt":{"rendered":""},"author":1,"featured_media":5,"categories":[3],"tags":[]}]`, base, base))
		case "/wp-json/wp/v2/pages":
			list(fmt.Sprintf(`[{"id":2,"slug":"about","status":"publish","type":"page","link":"%s/about/",`+
				`"title":{"rendered":"About"},"content":{"rendered":"<p>About us</p>"},"excerpt":{"rendered":""}}]`, base))
		case "/wp-json/wp/v2/types":
			_, _ = w.Write([]byte(`{"post":{"slug":"post","name":"Posts","rest_base":"posts"},` +
				`"page":{"slug":"page","name":"Pages","rest_base":"pages"}}`))
		case "/wp-json/wp/v2/media":
			list(fmt.Sprintf(`[{"id":5,"slug":"a","media_type":"image","mime_type":"image/png",`+
				`"source_url":"%s/wp-content/uploads/a.png","title":{"rendered":"A"}}]`, base))
		case "/wp-json/wp/v2/categories":
			list(fmt.Sprintf(`[{"id":3,"name":"News","slug":"news","link":"%s/category/news/","count":1}]`, base))
		case "/wp-json/wp/v2/tags":
			list(`[]`)
		case "/wp-json/wp/v2/users":
			list(fmt.Sprintf(`[{"id":1,"name":"Ada","slug":"ada","link":"%s/author/ada/"}]`, base))
		case "/wp-json/wp/v2/comments":
			list(`[{"id":9,"post":1,"parent":0,"author_name":"Reader","date":"2024-01-03T00:00:00",` +
				`"content":{"rendered":"<p>Nice</p>"},"status":"approved","type":"comment"}]`)
		case "/wp-json/wp/v2/menus", "/wp-json/wp/v2/settings", "/wp-json/wp/v2/menu-locations":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"rest_forbidden"}`))
		case "/wp-content/uploads/a.png":
			_, _ = w.Write([]byte("\x89PNG fake"))
		case "/", "/hello/", "/about/":
			_, _ = w.Write([]byte(`<html><head><title>Page</title><meta name="description" content="Desc"></head>` +
				`<body><main><p>Rendered body</p></main></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}
