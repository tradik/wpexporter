package exportcli

// What the site publishes against what the export wrote (#40).
//
// An export can only report what it fetched, so a content type the REST API
// does not expose is invisible: the run ends in a success summary and the
// migrated site is missing a section nobody was told about. The site's own
// sitemap and feed are a second opinion — one that disagrees with a lossy
// export in a way worth printing.
//
// This never fails an export and never requires anything of the site. A site
// that publishes no sitemap and no feed says so in one line, and the run
// continues exactly as it would have.

import (
	"fmt"
	neturl "net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/pkg/models"
)

// archiveSegments are the first path segments a generator rebuilds from the
// content itself. A tag page in a sitemap is not missing content; it is a view
// of content the export already carries.
var archiveSegments = map[string]bool{
	"tag": true, "category": true, "author": true, "page": true,
	"feed": true, "comments": true, "amp": true, "search": true,
	"wp-json": true, "wp-content": true, "wp-admin": true, "wp-includes": true,
}

var (
	// yearSegmentRe matches a date archive's first segment.
	yearSegmentRe = regexp.MustCompile(`^\d{4}$`)
	// taxonomySegmentRe matches a plugin's own taxonomy archive —
	// /mec-category/, /product-tag/ — which a generator rebuilds exactly as it
	// rebuilds /category/.
	taxonomySegmentRe = regexp.MustCompile(`-(category|tag|taxonomy)$`)
)

// maxReportedSegments bounds the report. The point is to name what was missed,
// not to reprint the sitemap.
const maxReportedSegments = 8

// checkCoverage compares the site's published inventory against the addresses
// the export carries, and returns the report lines — empty when the export
// covers everything the site advertises.
func checkCoverage(inventory api.Inventory, data *models.ExportData) []string {
	if !inventory.Published() {
		return nil
	}

	exported := exportedPaths(data)

	counts := map[string]int{}
	total := 0

	for _, raw := range append(append([]string{}, inventory.SitemapURLs...), inventory.FeedURLs...) {
		path := normalizePath(raw)
		if path == "" || path == "/" {
			continue
		}
		if _, covered := exported[path]; covered {
			continue
		}

		segment := firstSegment(path)
		if isArchiveSegment(segment) {
			continue
		}

		counts["/"+segment+"/"]++
		total++
	}

	if total == 0 {
		return nil
	}

	return describeCoverage(total, counts)
}

// describeCoverage renders the report, heaviest group first: the biggest number
// is the one that means a whole content type was never exported.
func describeCoverage(total int, counts map[string]int) []string {
	type group struct {
		segment string
		count   int
	}

	groups := make([]group, 0, len(counts))
	for segment, count := range counts {
		groups = append(groups, group{segment, count})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}

		return groups[i].segment < groups[j].segment
	})

	lines := []string{fmt.Sprintf(
		"The site publishes %d URLs this export does not cover:", total)}

	for i, g := range groups {
		if i == maxReportedSegments {
			lines = append(lines, fmt.Sprintf("  …and %d more paths", len(groups)-maxReportedSegments))
			break
		}

		lines = append(lines, fmt.Sprintf("  %-28s %d", g.segment, g.count))
	}

	lines = append(lines, coverageHint(counts))

	return lines
}

// coverageHint names the remedy that actually applies. Suggesting
// --custom-types for a type the export handles elsewhere sends the operator
// somewhere the flag cannot reach, which is the shape of advice #55 was partly
// about.
func coverageHint(counts map[string]int) string {
	for _, segment := range []string{"/product/", "/shop/", "/produkt/"} {
		if counts[segment] > 0 {
			return "  /product/ is WooCommerce: it is fetched from /wc/v3, which needs consumer " +
				"keys. Without them the export falls back to what /wp/v2/product publishes — a " +
				"catalog without prices — so pass --auth-user/--auth-pass to get the rest."
		}
	}

	return "  A section missing here is usually a post type the REST API does not " +
		"expose — try --custom-types with its name, or --brute-force."
}

// exportedPaths is every address the export carries, as comparable paths.
func exportedPaths(data *models.ExportData) map[string]struct{} {
	paths := make(map[string]struct{}, len(data.Posts)+len(data.Pages))

	collect := func(posts []models.WordPressPost) {
		for i := range posts {
			if path := normalizePath(posts[i].Link); path != "" {
				paths[path] = struct{}{}
			}
		}
	}

	collect(data.Posts)
	collect(data.Pages)
	for i := range data.CustomTypes {
		collect(data.CustomTypes[i].Posts)
	}

	for i := range data.Products {
		if path := normalizePath(data.Products[i].Permalink); path != "" {
			paths[path] = struct{}{}
		}
	}

	return paths
}

// normalizePath reduces an address to the path a comparison can use: no host,
// no query, no fragment, one trailing slash. The two inventories and the export
// spell the same page differently often enough that comparing raw strings would
// report a site as missing everything it has.
func normalizePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := neturl.Parse(raw)
	if err != nil {
		return ""
	}

	path := parsed.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	return strings.ToLower(path)
}

// firstSegment is the part of a path that names its kind: /events/2026/gala/
// is an event.
func firstSegment(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return ""
	}

	if slash := strings.Index(trimmed, "/"); slash >= 0 {
		return trimmed[:slash]
	}

	return trimmed
}

// isArchiveSegment reports whether a path belongs to a view a generator builds
// for itself rather than to content an export should have carried.
func isArchiveSegment(segment string) bool {
	if segment == "" {
		return true
	}

	return archiveSegments[segment] ||
		yearSegmentRe.MatchString(segment) ||
		taxonomySegmentRe.MatchString(segment)
}
