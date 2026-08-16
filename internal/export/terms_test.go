package export

// Archives that survive the migration (#45). On one measured site 48 tag
// archives and 9 category archives 404'd, because the export wrote display
// names and the generator made slugs out of them — "hand made pasta" against
// WordPress's own `hand-made-pasta-3`.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/pkg/models"
)

// nestedTaxonomy is the tree from the issue: recipes → pasta-rice, and a tag
// whose slug carries the suffix WordPress adds to a name used before.
func nestedTaxonomy() *models.ExportData {
	post := models.WordPressPost{
		ID: 1, Slug: "focaccia", Link: "https://x.test/blog/focaccia/",
		Categories: []int{20}, Tags: []int{30},
	}
	post.Title.Rendered = "Focaccia"
	post.Content.Rendered = "<p>Dough.</p>"

	return &models.ExportData{
		Posts: []models.WordPressPost{post},
		Categories: []models.WordPressCategory{
			{ID: 10, Name: "Recipes", Slug: "recipes"},
			{ID: 20, Name: "Pasta & Rice", Slug: "pasta-rice", Parent: 10},
		},
		Tags: []models.WordPressTag{
			{ID: 30, Name: "hand made pasta", Slug: "hand-made-pasta-3"},
		},
	}
}

// TestTermPathsFollowTheParentChain: the address the site publishes, rebuilt
// from what the REST API already states.
func TestTermPathsFollowTheParentChain(t *testing.T) {
	data := nestedTaxonomy()
	index := buildCategoryIndex(data.Categories)

	assert.Equal(t, "recipes", index[10].Path)
	assert.Equal(t, "recipes/pasta-rice", index[20].Path)
	assert.Equal(t, "pasta-rice", index[20].Slug, "the leaf keeps its own slug too")
}

// TestTermPathSurvivesABrokenTree: a parent that is missing, or a loop somebody
// created by editing the database, must not cost the term its own slug or hang
// the export.
func TestTermPathSurvivesABrokenTree(t *testing.T) {
	orphan := buildCategoryIndex([]models.WordPressCategory{
		{ID: 5, Name: "Orphan", Slug: "orphan", Parent: 999},
	})
	assert.Equal(t, "orphan", orphan[5].Path)

	looped := buildCategoryIndex([]models.WordPressCategory{
		{ID: 1, Name: "A", Slug: "a", Parent: 2},
		{ID: 2, Name: "B", Slug: "b", Parent: 1},
	})
	assert.Contains(t, looped[1].Path, "a", "a loop is bounded rather than followed")
}

// TestMarkdownCarriesTheArchiveAddress: names stay where they were, and the
// addresses arrive beside them.
func TestMarkdownCarriesTheArchiveAddress(t *testing.T) {
	exporter, output := newMarkdownExporter(t)
	data := nestedTaxonomy()
	exporter.buildLookupMaps(data)

	body := exporter.generateMarkdownContent(data.Posts[0], "post")

	assert.Contains(t, body, `- "Pasta & Rice"`, "the display name a consumer reads today")
	assert.Contains(t, body, "category_slugs:\n  - \"pasta-rice\"")
	assert.Contains(t, body, "category_paths:\n  - \"recipes/pasta-rice\"")
	assert.Contains(t, body, "tag_slugs:\n  - \"hand-made-pasta-3\"",
		"the slug WordPress actually publishes, suffix and all")

	_ = output
}

// TestFlatTaxonomyWritesNoPaths: a site with no nesting does not carry a second
// copy of its slugs.
func TestFlatTaxonomyWritesNoPaths(t *testing.T) {
	exporter, _ := newMarkdownExporter(t)

	post := models.WordPressPost{ID: 2, Slug: "flat", Categories: []int{7}}
	post.Title.Rendered = "Flat"
	data := &models.ExportData{
		Posts:      []models.WordPressPost{post},
		Categories: []models.WordPressCategory{{ID: 7, Name: "News", Slug: "news"}},
	}
	exporter.buildLookupMaps(data)

	body := exporter.generateMarkdownContent(post, "post")
	assert.Contains(t, body, `category_slugs:`)
	assert.NotContains(t, body, "category_paths:")
}

// TestSSGCarriesTheArchiveAddress: the generator format is where this matters
// most, because it builds the archives itself.
func TestSSGCarriesTheArchiveAddress(t *testing.T) {
	exporter, output := newMarkdownExporter(t)
	exporter.config.Format = "ssg"

	data := nestedTaxonomy()
	exporter.buildLookupMaps(data)

	require.NoError(t, os.MkdirAll(filepath.Join(output, "posts"), 0750))
	document := exporter.generateSSGContent(data.Posts[0], "post")

	assert.Contains(t, document, `category: "Pasta & Rice"`)
	assert.Contains(t, document, `category_slug: "pasta-rice"`)
	assert.Contains(t, document, `category_path: "recipes/pasta-rice"`)
	assert.Contains(t, document, `tag_slugs:`)
}

// TestUnknownTermsAreSkipped: a post referring to a term the export does not
// carry — filtered out, or deleted between two requests — writes no address
// rather than an empty one.
func TestUnknownTermsAreSkipped(t *testing.T) {
	index := buildCategoryIndex([]models.WordPressCategory{{ID: 1, Name: "Known", Slug: "known"}})

	resolved := index.identities([]int{1, 404})
	require.Len(t, resolved, 1)
	assert.Equal(t, "known", resolved[0].Slug)

	assert.Empty(t, index.identities(nil))
	assert.Nil(t, pathsOf(resolved), "a flat list has no paths worth repeating")
}
