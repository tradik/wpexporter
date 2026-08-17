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
	// --custom-types is also the answer to "your rule is wrong about my site":
	// a type named there is content whatever the bookkeeping test thinks of its
	// slug, which is how a magazine whose type is called `section` gets it.
	exportable, setAside := api.ContentTypes(types, cfg.CustomTypes)
	reportSetAsideTypes(setAside)

	custom, unmatched := selectCustomTypes(exportable, cfg.CustomTypes)
	reportUnmatchedTypes(unmatched, types, exportable)

	if len(custom) == 0 {
		logln("No custom post types found")
		return nil
	}

	logf("Found %d custom post type(s): %s\n", len(custom), strings.Join(typeSlugs(custom), ", "))

	var sets []models.CustomTypeSet
	for _, t := range custom {
		posts, fetchErr := client.GetCustomPosts(t.RestBase)
		if fetchErr != nil {
			// A gap keeps what it read: a type that served 40 of its 56 entries
			// exports the 40 and says so, rather than arriving as an empty
			// section nobody was warned about (#43).
			if description, isGap := api.Gap(fetchErr); isGap {
				logf("Warning: %s is incomplete — %s\n", t.Slug, description)
			} else {
				logf("Warning: could not fetch %s: %v\n", t.Slug, fetchErr)
			}

			if len(posts) == 0 {
				continue
			}
		}
		if len(posts) == 0 {
			// A registered type with nothing in it is not a finding worth a
			// directory; say so and move on.
			logf("  %s: empty\n", t.Slug)
			continue
		}
		logf("  %s: %d entries\n", t.Slug, len(posts))
		sets = append(sets, models.CustomTypeSet{
			Slug: t.Slug, Name: t.Name, RestBase: t.RestBase,
			// A generator cannot build a listing it does not know exists, and
			// the REST types document has said so all along (#64).
			HasArchive:  t.HasArchive,
			ArchiveLink: archiveLink(t),
			Posts:       posts,
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
// selection, and reports the names that matched nothing. An empty selection
// keeps everything that was discovered.
//
// The unmatched names are the point: a flag that silently exports nothing is
// the same shape of problem the sitemap check was written to end (#43). The
// operator has no way to tell a typo from a gated type from a broken flag
// unless the export says which it was.
func selectCustomTypes(discovered []api.PostType, wanted []string) (kept []api.PostType, unmatched []string) {
	if len(wanted) == 0 {
		return discovered, nil
	}

	// Trimmed once, up front: the flag arrives split and trimmed, but a config
	// file's value does not, and a name with a space around it must not read as
	// a type the site does not have.
	names := make([]string, 0, len(wanted))
	for _, name := range wanted {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}

	matched := make(map[string]bool, len(names))

	for _, t := range discovered {
		for _, name := range names {
			if strings.EqualFold(name, t.Slug) || strings.EqualFold(name, t.RestBase) {
				kept = append(kept, t)
				matched[strings.ToLower(name)] = true

				break
			}
		}
	}

	for _, name := range names {
		if !matched[strings.ToLower(name)] {
			unmatched = append(unmatched, name)
		}
	}

	return kept, unmatched
}

// reportUnmatchedTypes says why a requested type brought nothing: the site does
// not register it, or it registers it and the export handles it elsewhere.
//
// Naming the registered types is the difference between "try again" and "try
// what". A list is cheap here — discovery already fetched it.
func reportUnmatchedTypes(unmatched []string, registered, exportable []api.PostType) {
	for _, name := range unmatched {
		known := findRegisteredType(registered, name)

		switch {
		case known == nil:
			logf("Warning: --custom-types %s: the site registers no such type. It registers: %s\n",
				name, strings.Join(typeSlugs(exportable), ", "))
		case known.RestBase == "":
			logf("Warning: --custom-types %s: the site registers it but does not serve it over REST, "+
				"so there is nothing to fetch.\n", name)
		default:
			logf("Warning: --custom-types %s: the site registers it, but this export handles it "+
				"elsewhere — products as WooCommerce products, media as attachments, and a plugin's "+
				"own bookkeeping types not at all.\n", name)
		}
	}
}

// findRegisteredType looks a name up among every type the site declares,
// including the ones an export does not treat as content.
func findRegisteredType(registered []api.PostType, name string) *api.PostType {
	for i := range registered {
		if strings.EqualFold(name, registered[i].Slug) || strings.EqualFold(name, registered[i].RestBase) {
			return &registered[i]
		}
	}

	return nil
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

// archiveLink is the address a type publishes its listing at: the slug it
// registered explicitly, or its own, and empty when it has no archive.
//
// WordPress states both in the types document — has_archive is `true`, `false`
// or the slug itself (#53) — so no extra request is needed to answer this.
func archiveLink(t api.PostType) string {
	if !t.HasArchive {
		return ""
	}

	slug := t.ArchiveSlug
	if slug == "" {
		slug = t.Slug
	}

	return "/" + strings.Trim(slug, "/") + "/"
}

// reportSetAsideTypes names the types read as a plugin's or a theme's own
// bookkeeping rather than as content.
//
// The test is a substring match — "layout", "template", "block", "section",
// "popup", "widget" — which is right for a builder's saved fragments and wrong
// for a magazine whose content type is called `section`. Being wrong is
// survivable; being wrong in silence is what cost a site a whole type with no
// line in the report to explain where it went. Naming them also names the
// remedy: --custom-types insists.
func reportSetAsideTypes(setAside []string) {
	if len(setAside) == 0 {
		return
	}

	logf("Set aside %d post type(s) as plugin or theme bookkeeping: %s\n",
		len(setAside), strings.Join(setAside, ", "))
	logln("  If one of those is your content, name it: --custom-types <slug>")
}
