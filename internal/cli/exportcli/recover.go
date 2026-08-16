package exportcli

// Recovering content the REST API will not serve (#40).
//
// One measured site answers 500 for every request to /wp/v2/posts and has done
// for weeks. Its feed is served from the same WordPress and works: titles,
// addresses, dates, authors and, on most sites, whole post bodies. An export
// that reads only the REST API calls such a site unexportable; an export that
// falls back to what the site still publishes hands over most of it.
//
// The fallback is asked for, never assumed. REST is the better source in every
// respect — IDs, taxonomy terms, featured images, the full archive rather than
// the recent items — so it is never quietly replaced. `--from-sitemap` is the
// operator saying they already know the API is not answering.

import (
	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// recoverPostsFromFeed fills an empty posts collection from the site's feed.
//
// Only when the collection is empty: a feed lists recent items, so merging it
// into a set the API did serve would add a handful of thinner duplicates of
// records the export already has, addressed by the same URLs.
func recoverPostsFromFeed(cfg *config.Config, inventory api.Inventory, data *models.ExportData) {
	if !cfg.FromSitemap || cfg.NoPosts {
		return
	}

	if len(data.Posts) > 0 || len(inventory.FeedPosts) == 0 {
		return
	}

	data.Posts = inventory.FeedPosts
	data.Stats.TotalPosts = len(data.Posts)
	data.Stats.RecoveredPosts = len(data.Posts)

	logf("Recovered %d posts from the feed — the REST API served none (--from-sitemap).\n", len(data.Posts))
	logln("  They carry title, address, date, author and body, but no IDs, terms or featured images:")
	logln("  a feed is what the site publishes to readers, not its database.")
}
