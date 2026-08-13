package exportcli

// Fetching the site's custom post types (#28).
//
// Posts and pages are only the types WordPress ships with. A theme's Services,
// Portfolio, Team or Testimonials entries are published content with their own
// URLs, and an export that fetched only /posts and /pages dropped them without
// a word — the migrated site simply arrived missing whole sections.
//
// Discovery is one request, and what comes back is reported honestly: which
// types were found, how many entries each holds, and which were skipped.

import (
	"strings"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// fetchCustomTypes discovers the site's custom post types and fetches every
// entry of each. A discovery or fetch failure is a warning, never fatal: the
// rest of the export is still worth having.
func fetchCustomTypes(client *api.Client, cfg *config.Config) []models.CustomTypeSet {
	if cfg.NoCustomTypes {
		logln("Skipping custom post types (--no-custom-types)")
		return nil
	}

	types, err := client.GetPostTypes()
	if err != nil {
		logf("Warning: could not list post types: %v\n", err)
		return nil
	}
	custom := selectCustomTypes(api.CustomPostTypes(types), cfg.CustomTypes)
	if len(custom) == 0 {
		logln("No custom post types found")
		return nil
	}

	logf("Found %d custom post type(s): %s\n", len(custom), strings.Join(typeSlugs(custom), ", "))

	var sets []models.CustomTypeSet
	for _, t := range custom {
		posts, fetchErr := client.GetCustomPosts(t.RestBase)
		if fetchErr != nil {
			logf("Warning: could not fetch %s: %v\n", t.Slug, fetchErr)
			continue
		}
		if len(posts) == 0 {
			// A registered type with nothing in it is not a finding worth a
			// directory; say so and move on.
			logf("  %s: empty\n", t.Slug)
			continue
		}
		logf("  %s: %d entries\n", t.Slug, len(posts))
		sets = append(sets, models.CustomTypeSet{
			Slug: t.Slug, Name: t.Name, RestBase: t.RestBase, Posts: posts,
		})
	}
	return sets
}

// splitCommaList parses a comma-separated flag value, dropping empty entries so
// a trailing comma is not read as a request for a type named "".
func splitCommaList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// selectCustomTypes narrows the discovered types to an explicit --custom-types
// selection. An empty selection keeps everything that was discovered.
func selectCustomTypes(discovered []api.PostType, wanted []string) []api.PostType {
	if len(wanted) == 0 {
		return discovered
	}
	keep := make(map[string]bool, len(wanted))
	for _, name := range wanted {
		if name = strings.TrimSpace(name); name != "" {
			keep[strings.ToLower(name)] = true
		}
	}
	var out []api.PostType
	for _, t := range discovered {
		if keep[strings.ToLower(t.Slug)] || keep[strings.ToLower(t.RestBase)] {
			out = append(out, t)
		}
	}
	return out
}

// typeSlugs lists the slugs for a one-line report.
func typeSlugs(types []api.PostType) []string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, t.Slug)
	}
	return out
}

// countCustomPosts totals the entries across every fetched type, for the stats
// block.
func countCustomPosts(sets []models.CustomTypeSet) int {
	total := 0
	for _, set := range sets {
		total += len(set.Posts)
	}
	return total
}

// enrichCustomTypes runs one of the crawler's enrichment passes over every
// custom type's entries. They are ordinary published documents: a Services
// entry needs its meta description and its builder-rendered body exactly as a
// page does, and skipping them would export them stripped of both.
func enrichCustomTypes(sets []models.CustomTypeSet, enrich func([]models.WordPressPost) []models.WordPressPost) {
	for i := range sets {
		if len(sets[i].Posts) > 0 {
			sets[i].Posts = enrich(sets[i].Posts)
		}
	}
}
