package export

// A term's identity, not only its label (#45).
//
// The export wrote a term's display name and left the target to make a slug out
// of it. Usually the two agree; when they do not, every archive the source
// published is a 404 on the migrated site and every link anyone ever made to it
// is broken. On one measured migration that was 48 tag archives and 9 category
// archives — "hand made pasta" whose slug is `hand-made-pasta-3`, because the
// site has had three terms of that name over the years, which is entirely
// normal in a site old enough to be worth migrating.
//
// Hierarchy went the same way: the source publishes
// /category/recipes/pasta-rice/ and the export recorded only the leaf, so the
// migrated site served /category/pasta-rice/ and the published address 404'd.
//
// WordPress states both — `slug` and `parent` are on every term the REST API
// serves — so both are exported. The display names stay exactly where they
// were: a consumer reading them today keeps working, and one that wants the
// address now has it.

import (
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// termIdentity is what a term needs for its archive to survive a migration.
type termIdentity struct {
	ID   int
	Name string
	Slug string
	// Path is the slug chain from the root, "recipes/pasta-rice" for a nested
	// category and just the slug for a flat one.
	Path string
}

// termIndex resolves term IDs to their identity, for one taxonomy.
type termIndex map[int]termIdentity

// buildCategoryIndex indexes the site's categories, resolving each one's parent
// chain once.
func buildCategoryIndex(categories []models.WordPressCategory) termIndex {
	byID := make(map[int]models.WordPressCategory, len(categories))
	for _, category := range categories {
		byID[category.ID] = category
	}

	index := make(termIndex, len(categories))
	for _, category := range categories {
		index[category.ID] = termIdentity{
			ID:   category.ID,
			Name: category.Name,
			Slug: category.Slug,
			Path: categoryPath(byID, category),
		}
	}

	return index
}

// buildTagIndex indexes the site's tags. Tags are flat, so a tag's path is its
// slug.
func buildTagIndex(tags []models.WordPressTag) termIndex {
	index := make(termIndex, len(tags))
	for _, tag := range tags {
		index[tag.ID] = termIdentity{ID: tag.ID, Name: tag.Name, Slug: tag.Slug, Path: tag.Slug}
	}

	return index
}

// maxTermDepth bounds the parent walk. A category tree deeper than this is a
// loop — WordPress does not prevent one from being created by direct database
// edits, and following it would not end.
const maxTermDepth = 16

// categoryPath builds the slug chain the site publishes the archive under.
func categoryPath(byID map[int]models.WordPressCategory, category models.WordPressCategory) string {
	segments := []string{category.Slug}

	parent := category.Parent
	for depth := 0; parent > 0 && depth < maxTermDepth; depth++ {
		ancestor, known := byID[parent]
		if !known || ancestor.Slug == "" {
			break
		}

		segments = append([]string{ancestor.Slug}, segments...)
		parent = ancestor.Parent
	}

	return strings.Join(segments, "/")
}

// identities resolves a post's term IDs, keeping the site's own order and
// skipping IDs the export does not know — a term filtered out, or one deleted
// between two requests.
func (index termIndex) identities(ids []int) []termIdentity {
	resolved := make([]termIdentity, 0, len(ids))

	for _, id := range ids {
		if term, known := index[id]; known && term.Slug != "" {
			resolved = append(resolved, term)
		}
	}

	return resolved
}

// slugs lists the term slugs, which is what an archive's address is made of.
func slugsOf(terms []termIdentity) []string {
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		out = append(out, term.Slug)
	}

	return out
}

// pathsOf lists the full slug chains, which is what a hierarchical archive's
// address is made of. Nil when no term is nested, so a flat site does not carry
// a second copy of its slugs.
func pathsOf(terms []termIdentity) []string {
	nested := false
	out := make([]string, 0, len(terms))

	for _, term := range terms {
		out = append(out, term.Path)
		if term.Path != term.Slug {
			nested = true
		}
	}

	if !nested {
		return nil
	}

	return out
}

// writeTermAddresses adds the slugs — and, where the taxonomy is nested, the
// full published paths — beside the names already written.
//
// Added beside rather than instead of: a consumer reading display names today
// keeps working unchanged, and one that needs the address now has it. The keys
// are singular-prefixed to match the names above them: `category_slugs` under
// `categories`.
func (e *Exporter) writeTermAddresses(builder *strings.Builder, terms []termIdentity, taxonomy string) {
	if len(terms) == 0 {
		return
	}

	writeYAMLList(builder, taxonomy+"_slugs", slugsOf(terms))
	writeYAMLList(builder, taxonomy+"_paths", pathsOf(terms))
}

// writeYAMLList writes a list of quoted values, or nothing when there are none:
// a generator reads an absent key more easily than an empty one.
func writeYAMLList(builder *strings.Builder, key string, values []string) {
	if len(values) == 0 {
		return
	}

	builder.WriteString(key + ":\n")
	for _, value := range values {
		builder.WriteString("  - \"" + value + "\"\n")
	}
}
