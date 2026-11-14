package exportcli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// typeSource serves a fixed list of types and entries.
type typeSource struct {
	types   []api.PostType
	entries map[string][]models.WordPressPost
	errs    map[string]error
}

func (s typeSource) GetPostTypes() ([]api.PostType, error) { return s.types, nil }
func (s typeSource) GetCustomPosts(base string) ([]models.WordPressPost, error) {
	return s.entries[base], s.errs[base]
}

// fetchCustomTypes over any source: gaps keep what was read, failures and
// empty types are skipped, query-only types go with --skip-unaddressable-types,
// and every --custom-types name that matched nothing is explained.
func TestFetchCustomTypesReportsEverything(t *testing.T) {
	src := typeSource{
		types: []api.PostType{
			{Slug: "service", Name: "Services", RestBase: "service"},
			{Slug: "case", Name: "Cases", RestBase: "case"},
			{Slug: "broken", Name: "Broken", RestBase: "broken"},
			{Slug: "gallery", Name: "Gallery", RestBase: "gallery"},
			{Slug: "elementor_library", RestBase: "elementor_library"},
			{Slug: "product", RestBase: "product"},
			{Slug: "revision", RestBase: ""},
		},
		entries: map[string][]models.WordPressPost{
			"service": {{ID: 1, Link: "https://e.com/service/a/"}},
			"case":    {{ID: 2, Link: "https://e.com/case/b/"}},
			"gallery": {{ID: 3, Link: "https://e.com/?gallery=3"}},
		},
		errs: map[string]error{
			"case":   &api.PartialError{Endpoint: "case", Page: 2, Err: errors.New("500")},
			"broken": errors.New("refused"),
		},
	}
	cfg := config.DefaultConfig()
	cfg.SkipUnaddressableTypes = true
	cfg.CustomTypes = []string{"service", "case", "broken", "gallery", "nope", "product", "revision"}

	var sets []models.CustomTypeSet
	log := captureStdout(t, func() { sets = fetchCustomTypes(src, cfg) })

	var slugs []string
	for _, s := range sets {
		slugs = append(slugs, s.Slug)
	}
	assert.Equal(t, []string{"service", "case"}, slugs)
	for _, want := range []string{
		"case is incomplete", "could not fetch broken", "Dropped 1 post type(s) with no addresses",
		"Set aside 1 post type(s)", "registers no such type", "does not serve it over REST",
		"handles it elsewhere",
	} {
		assert.Contains(t, log, want)
	}
}
