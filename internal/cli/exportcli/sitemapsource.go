package exportcli

// The sitemap as the whole source, not a patch (#68).
//
// `--from-sitemap` recovered posts from the feed, which is the right answer for
// a site whose `/wp/v2/posts` answers 500 (#40) and no answer at all for a
// WordPress older than the content API. That site has no REST routes in either
// spelling, its content is in pages rather than posts, and its feed lists a
// handful of recent items where its sitemap lists everything.
//
// 1.8.15 read that sitemap, printed the addresses it found, and exported none
// of them: a README, a metadata.json of zeroes, and eleven years of content
// left on the server. The addresses answer 200 to anyone who asks, so this asks
// them.
//
// It runs only where there is nothing to lose: with `--from-sitemap` on, for
// the addresses no exported document already covers. A site whose API answered
// is untouched, because every address it published is already accounted for.

import (
	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// documentFetcher reads one published address. The crawler satisfies it; a test
// does not need a network to say what this function decides.
type documentFetcher interface {
	FetchDocument(pageURL string) (models.WordPressPost, bool)
}

// recoverPagesFromSitemap builds documents for the addresses the sitemap lists
// and the export does not carry.
//
// It returns the number recovered, so the run can state it: pages that came
// from the site's rendered HTML are thinner than pages from the API, and a
// consumer that needs to know says so by reading the count rather than by
// guessing.
func recoverPagesFromSitemap(
	cfg *config.Config,
	fetcher documentFetcher,
	inventory api.Inventory,
	data *models.ExportData,
) int {
	if !cfg.FromSitemap || cfg.NoPages || len(inventory.SitemapURLs) == 0 {
		return 0
	}

	covered := exportedAddresses(data)

	var recovered []models.WordPressPost

	for _, address := range inventory.SitemapURLs {
		if covered[normalizePath(address)] {
			continue
		}

		if budgetSpent(cfg, len(recovered)) {
			break
		}

		document, ok := fetcher.FetchDocument(address)
		if !ok {
			continue
		}

		recovered = append(recovered, document)
	}

	if len(recovered) == 0 {
		return 0
	}

	data.Pages = append(data.Pages, recovered...)
	data.Stats.TotalPages = len(data.Pages)
	data.Stats.RecoveredPages = len(recovered)

	logf("Recovered %d page(s) by reading what the site publishes — there was no content API "+
		"to read them from (--from-sitemap).\n", len(recovered))
	logln("  They carry title, address, SEO metadata and the rendered body, and no IDs, terms,")
	logln("  authors or dates: a published page is what the site shows a reader, not its database.")

	return len(recovered)
}

// budgetSpent reports whether a limit has already been reached, so a preview of
// five pages does not fetch a thousand.
func budgetSpent(cfg *config.Config, recovered int) bool {
	if cfg.Limit <= 0 {
		return false
	}

	return recovered >= cfg.Limit
}

// exportedAddresses is every address this export already carries, normalized so
// a trailing slash or a scheme does not make one address look like two.
func exportedAddresses(data *models.ExportData) map[string]bool {
	covered := make(map[string]bool)

	add := func(posts []models.WordPressPost) {
		for _, post := range posts {
			if post.Link != "" {
				covered[normalizePath(post.Link)] = true
			}
		}
	}

	add(data.Posts)
	add(data.Pages)

	for _, set := range data.CustomTypes {
		add(set.Posts)
	}

	for _, product := range data.Products {
		if product.Permalink != "" {
			covered[normalizePath(product.Permalink)] = true
		}
	}

	return covered
}
