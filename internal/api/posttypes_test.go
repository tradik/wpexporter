package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
)

// typesResponse is the shape /wp/v2/types answers with: an object keyed by type
// slug, mixing WordPress internals, plugin bookkeeping and the theme's own
// content types.
const typesResponse = `{
  "post":              {"slug":"post","name":"Posts","rest_base":"posts"},
  "page":              {"slug":"page","name":"Pages","rest_base":"pages"},
  "attachment":        {"slug":"attachment","name":"Media","rest_base":"media"},
  "wp_template":       {"slug":"wp_template","name":"Templates","rest_base":"templates"},
  "wp_font_face":      {"slug":"wp_font_face","name":"Font Faces","rest_base":"font-families/(?P<font_family_id>[\\d]+)/font-faces"},
  "elementor_library": {"slug":"elementor_library","name":"My Templates","rest_base":"elementor_library"},
  "rank_math_schema":  {"slug":"rank_math_schema","name":"Schemas","rest_base":"rank_math_schema"},
  "cpt_services":      {"slug":"cpt_services","name":"Services","rest_base":"cpt_services","has_archive":true,"taxonomies":["cpt_services_group"]},
  "cpt_portfolio":     {"slug":"cpt_portfolio","name":"Portfolio","rest_base":"cpt_portfolio"},
  "no_rest_base":      {"slug":"no_rest_base","name":"Hidden"}
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5})
	if err != nil {
		t.Fatal(err)
	}
	return client, server
}

func TestGetPostTypes(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wp-json/wp/v2/types" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(typesResponse))
	})

	types, err := client.GetPostTypes()
	if err != nil {
		t.Fatal(err)
	}
	// Everything with a rest_base comes back, built-ins included; filtering is
	// the caller's decision.
	if len(types) != 9 {
		t.Fatalf("expected 9 types, got %d: %+v", len(types), types)
	}
	// Sorted by slug, so a report and a re-run agree.
	if types[0].Slug != "attachment" {
		t.Errorf("types should be sorted by slug, got %q first", types[0].Slug)
	}
	for _, tp := range types {
		if tp.Slug == "cpt_services" && (tp.Name != "Services" || !tp.HasArchive) {
			t.Errorf("cpt_services not parsed: %+v", tp)
		}
		if tp.Slug == "no_rest_base" {
			t.Error("a type with no rest_base is not fetchable and must be dropped")
		}
	}
}

func TestGetPostTypes_Errors(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	if _, err := client.GetPostTypes(); err == nil {
		t.Error("a non-200 response must be an error")
	}

	client, _ = newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})
	if _, err := client.GetPostTypes(); err == nil {
		t.Error("an unparseable response must be an error")
	}
}

func TestCustomPostTypes(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(typesResponse))
	})
	types, err := client.GetPostTypes()
	if err != nil {
		t.Fatal(err)
	}

	custom := CustomPostTypes(types)

	got := map[string]bool{}
	for _, tp := range custom {
		got[tp.Slug] = true
	}
	for _, want := range []string{"cpt_services", "cpt_portfolio"} {
		if !got[want] {
			t.Errorf("%s is site content and must be kept: %+v", want, custom)
		}
	}
	for _, unwanted := range []string{
		"post", "page", "attachment", // handled by their own exporters
		"wp_template", "wp_font_face", // WordPress internals
		"elementor_library", "rank_math_schema", // plugin bookkeeping
	} {
		if got[unwanted] {
			t.Errorf("%s is not site content and must be dropped: %+v", unwanted, custom)
		}
	}
}

func TestGetCustomPosts(t *testing.T) {
	calls := 0
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`[{"id":1,"slug":"wms","title":{"rendered":"WMS"}}]`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})

	posts, err := client.GetCustomPosts("cpt_services")
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Slug != "wms" {
		t.Fatalf("unexpected entries: %+v", posts)
	}
	if calls < 2 {
		t.Errorf("pagination should continue until an empty page, got %d calls", calls)
	}
}

func TestGetCustomPosts_Error(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if _, err := client.GetCustomPosts("cpt_services"); err == nil {
		t.Error("a failing collection must be an error, not an empty export")
	}
}

func TestIsBookkeepingType(t *testing.T) {
	for slug, want := range map[string]bool{
		"elementor_snippet": true,
		"acf-field":         true,
		"jet-engine":        true,
		// Page construction, not page content: a theme's saved layouts are
		// fragments a visitor met inside other pages (#28).
		"cpt_layouts":  true,
		"theme_popups": true,
		"cpt_services": false,
		"portfolio":    false,
	} {
		if got := isBookkeepingType(slug); got != want {
			t.Errorf("isBookkeepingType(%q) = %v, want %v", slug, got, want)
		}
	}
}
