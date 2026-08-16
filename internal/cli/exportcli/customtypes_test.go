package exportcli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

const typesJSON = `{
  "post":          {"slug":"post","name":"Posts","rest_base":"posts"},
  "cpt_services":  {"slug":"cpt_services","name":"Services","rest_base":"cpt_services"},
  "cpt_portfolio": {"slug":"cpt_portfolio","name":"Portfolio","rest_base":"cpt_portfolio"},
  "cpt_team":      {"slug":"cpt_team","name":"Team","rest_base":"cpt_team"}
}`

// newTypesServer answers the types listing plus one entry for cpt_services and
// cpt_portfolio; cpt_team is registered but empty, like most starter themes.
func newTypesServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/wp-json/wp/v2/types":
			_, _ = w.Write([]byte(typesJSON))
		case r.URL.Query().Get("page") != "1":
			_, _ = w.Write([]byte(`[]`))
		case r.URL.Path == "/wp-json/wp/v2/cpt_services":
			_, _ = w.Write([]byte(`[{"id":1,"slug":"wms","link":"https://x.test/services/wms/"}]`))
		case r.URL.Path == "/wp-json/wp/v2/cpt_portfolio":
			_, _ = w.Write([]byte(`[{"id":2,"slug":"case-a","link":"https://x.test/portfolio/case-a/"}]`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newTypesClient(t *testing.T, url string) *api.Client {
	t.Helper()
	client, err := api.NewClient(&config.Config{URL: url, Timeout: 5})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestFetchCustomTypes(t *testing.T) {
	server := newTypesServer(t)
	client := newTypesClient(t, server.URL)

	sets := fetchCustomTypes(client, &config.Config{})

	if len(sets) != 2 {
		t.Fatalf("expected the two types that have entries, got %+v", sets)
	}
	// An empty registered type is reported but not exported as a directory.
	for _, set := range sets {
		if set.Slug == "cpt_team" {
			t.Error("an empty type should not become a set")
		}
	}
	if countCustomPosts(sets) != 2 {
		t.Errorf("entry count wrong: %d", countCustomPosts(sets))
	}
}

func TestFetchCustomTypes_Selection(t *testing.T) {
	server := newTypesServer(t)
	client := newTypesClient(t, server.URL)

	sets := fetchCustomTypes(client, &config.Config{CustomTypes: []string{"cpt_portfolio"}})

	if len(sets) != 1 || sets[0].Slug != "cpt_portfolio" {
		t.Fatalf("--custom-types should narrow the export, got %+v", sets)
	}
}

func TestFetchCustomTypes_Disabled(t *testing.T) {
	server := newTypesServer(t)
	client := newTypesClient(t, server.URL)

	if sets := fetchCustomTypes(client, &config.Config{NoCustomTypes: true}); sets != nil {
		t.Errorf("--no-custom-types must fetch nothing, got %+v", sets)
	}
}

func TestFetchCustomTypes_DiscoveryFailureIsNotFatal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	// A site that refuses the types endpoint still exports its posts and pages.
	if sets := fetchCustomTypes(newTypesClient(t, server.URL), &config.Config{}); sets != nil {
		t.Errorf("a failed discovery should yield nothing, got %+v", sets)
	}
}

func TestFetchCustomTypes_NoneRegistered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"post":{"slug":"post","name":"Posts","rest_base":"posts"}}`))
	}))
	t.Cleanup(server.Close)

	if sets := fetchCustomTypes(newTypesClient(t, server.URL), &config.Config{}); sets != nil {
		t.Errorf("a site with only built-in types should yield nothing, got %+v", sets)
	}
}

func TestSelectCustomTypes(t *testing.T) {
	discovered := []api.PostType{
		{Slug: "cpt_services", RestBase: "cpt_services"},
		{Slug: "cpt_portfolio", RestBase: "portfolio"},
	}

	if got, unmatched := selectCustomTypes(discovered, nil); len(got) != 2 || unmatched != nil {
		t.Errorf("an empty selection keeps everything, got %+v / %+v", got, unmatched)
	}
	// Either the type slug or its REST collection name identifies a type.
	got, unmatched := selectCustomTypes(discovered, []string{"CPT_SERVICES", " portfolio "})
	if len(got) != 2 || len(unmatched) != 0 {
		t.Errorf("matching should be case- and space-insensitive, got %+v / %+v", got, unmatched)
	}
	// An unmatched name is reported rather than silently exporting nothing (#43).
	got, unmatched = selectCustomTypes(discovered, []string{"absent"})
	if got != nil || len(unmatched) != 1 || unmatched[0] != "absent" {
		t.Errorf("an unmatched selection must be named, got %+v / %+v", got, unmatched)
	}
}

func TestSplitCommaList(t *testing.T) {
	got := splitCommaList(" a , ,b,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("unexpected split: %+v", got)
	}
	if splitCommaList("") != nil {
		t.Error("an empty value should select nothing")
	}
}

func TestEnrichCustomTypes(t *testing.T) {
	sets := []models.CustomTypeSet{
		{Slug: "cpt_services", Posts: []models.WordPressPost{{ID: 1}}},
		{Slug: "cpt_empty"},
	}
	calls := 0

	enrichCustomTypes(sets, func(posts []models.WordPressPost) []models.WordPressPost {
		calls++
		posts[0].Slug = "enriched"
		return posts
	})

	if calls != 1 {
		t.Errorf("only a type with entries should be crawled, got %d calls", calls)
	}
	if sets[0].Posts[0].Slug != "enriched" {
		t.Error("the enriched entries should be written back to the set")
	}
}

func TestTypeSlugs(t *testing.T) {
	got := typeSlugs([]api.PostType{{Slug: "a"}, {Slug: "b"}})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("unexpected slugs: %+v", got)
	}
}
