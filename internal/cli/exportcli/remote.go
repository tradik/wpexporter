package exportcli

// The stages that read a site over its REST API, in the order the export has
// always run them.

import (
	"errors"
	"fmt"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/seo"
	"github.com/tradik/wpexporter/pkg/models"
)

// fetchRemoteDocuments reads the site description, posts, pages, products and
// custom types.
func (r *exportRun) fetchRemoteDocuments() error {
	cfg, apiClient, c := r.cfg, r.apiClient, &r.c

	// Get site information
	logln("\nFetching site information...")
	siteInfo, err := r.src.GetSiteInfo()
	siteInfoGap, rootUnread := api.Gap(err)
	if err != nil && !rootUnread {
		return fmt.Errorf("failed to get site info: %w", err)
	}
	if rootUnread {
		// A closed /wp-json root leaves the identity fields empty. The export
		// goes on — the collections are read separately — but the run says the
		// site was never described rather than presenting the blanks as the
		// site's own answer (#79).
		logln("Warning: " + siteInfoGap)
	}
	c.siteInfo = siteInfo

	r.limitMedia()

	// Collections the site would not read to the end. They are reported here,
	// again in the summary and in metadata.json, and never silently dropped —
	// but they do not end the export (#37). A root document that never answered
	// is the same kind of hole, so it is carried in the same list (#79).
	if rootUnread {
		c.gaps = append(c.gaps, siteInfoGap)
	}

	// Get content via API (respecting filter flags)
	if !cfg.NoPosts {
		logln("Fetching posts...")
		if cfg.Resume {
			c.posts, err = apiClient.GetPostsWithCheckpoint(r.checkpointMgr.GetState(), r.saveCheckpoint)
		} else {
			c.posts, err = r.src.GetPosts()
		}
		if err := noteIncomplete(&c.gaps, "posts", err); err != nil {
			return err
		}
		logf("Found %d posts\n", len(c.posts))
	} else {
		logln("Skipping posts (--no-posts)")
	}

	if !cfg.NoPages {
		logln("Fetching pages...")
		if cfg.Resume {
			c.pages, err = apiClient.GetPagesWithCheckpoint(r.checkpointMgr.GetState(), r.saveCheckpoint)
		} else {
			c.pages, err = r.src.GetPages()
		}
		if err := noteIncomplete(&c.gaps, "pages", err); err != nil {
			return err
		}
		logf("Found %d pages\n", len(c.pages))
	} else {
		logln("Skipping pages (--no-pages)")
	}

	// By now something has been asked for, so the client knows which spelling
	// this site answers to — or that it answers to neither (#66, #68).
	noteSiteAPI(apiClient, &c.notices)

	r.fetchRemoteProducts()

	// The types a theme or plugin registered: Services, Portfolio, Team — content
	// the site published that posts and pages alone never covered (#28). Fetched
	// here so the SEO crawl below enriches them like everything else.
	c.customTypes = fetchCustomTypes(r.src, cfg)
	return nil
}

// limitMedia narrows the media to what the documents reference when the run
// is limited.
func (r *exportRun) limitMedia() {
	// A limited export downloads the media its documents reference, not the
	// whole library: fetching 200 MB of images for a five-page preview is the
	// thing #60 exists to stop. --relevant-media-only already has that logic,
	// so the limit switches it on rather than growing a second copy.
	if limitsActive(r.cfg) && !r.cfg.RelevantMediaOnly {
		r.cfg.RelevantMediaOnly = true
		logln("Limiting media to what the exported documents reference (--limit)")
	}
}

// fetchRemoteProducts reads the WooCommerce catalog.
func (r *exportRun) fetchRemoteProducts() {
	cfg, apiClient, c := r.cfg, r.apiClient, &r.c
	if cfg.NoProducts {
		logln("Skipping products (--no-products)")
		return
	}

	logln("Fetching WooCommerce products...")
	var err error
	if cfg.Resume {
		c.products, err = apiClient.GetProductsWithCheckpoint(r.checkpointMgr.GetState(), r.saveCheckpoint)
	} else {
		c.products, err = apiClient.GetProducts()
	}
	switch {
	case errors.Is(err, api.ErrProductsNeedKeys):
		// The shop exists and will not talk to us without consumer keys.
		// Its products are usually public on the ordinary WordPress route,
		// which is a catalog without the commerce — and better than the
		// zero this used to report (#55).
		c.products, c.productsNotice = recoverPublicProducts(apiClient)
	case err != nil:
		// WooCommerce is optional: surface the failure but keep whatever was
		// fetched, so a transient Woo error doesn't silently abort or truncate
		// the rest of the export (GO-003).
		logf("Warning: WooCommerce products may be incomplete: %v\n", err)
	}

	if c.productsNotice != "" {
		logln(c.productsNotice)
	} else if len(c.products) > 0 {
		logf("Found %d WooCommerce products\n", len(c.products))
	} else if err == nil {
		logln("No WooCommerce products found (WooCommerce may not be installed)")
	}
}

// fetchRemoteMedia reads the media library, and the featured images its
// listing left out.
func (r *exportRun) fetchRemoteMedia() error {
	cfg, apiClient, c := r.cfg, r.apiClient, &r.c
	if !cfg.DownloadMedia {
		logln("Skipping media (--no-media)")
		return nil
	}

	logln("Fetching media...")
	var err error
	if cfg.Resume {
		c.media, err = apiClient.GetMediaWithCheckpoint(r.checkpointMgr.GetState(), r.saveCheckpoint)
	} else {
		c.media, err = r.src.GetMedia()
	}
	// A page of the media listing that will not come is a gap like any
	// other: the run keeps what it read and carries on. It used to end the
	// export and discard everything already fetched — 1251 posts and 89
	// pages, on the run that reported this (#57).
	if err := noteIncomplete(&c.gaps, "media", err); err != nil {
		return err
	}
	logf("Found %d media items\n", len(c.media))

	// Fetch missing featured images that weren't returned by paginated API
	// WordPress API may not return all media items (WPML, different post types, etc.)
	missingMedia := fetchMissingFeaturedMedia(apiClient, c.posts, c.pages, c.media, cfg.Verbose)
	if len(missingMedia) > 0 {
		c.media = append(c.media, missingMedia...)
		logf("Fetched %d additional featured images by ID\n", len(missingMedia))
	}

	r.filterRelevantMedia()
	return nil
}

// checkInventory compares what the site says it publishes with what this run
// carries, and recovers what the REST API would not serve (#40, #68).
func (r *exportRun) checkInventory(exportData *models.ExportData) {
	cfg, apiClient := r.cfg, r.apiClient
	if cfg.NoInventoryCheck {
		return
	}
	// What the site says it publishes, against what this run carries (#40). It
	// costs a request or two, runs before the metadata is written so the answer
	// lands in metadata.json as well as the console, and can only add a line to
	// the report — a site that publishes neither inventory says so and nothing
	// else changes.
	logln("\nReading the site's own inventory...")
	inventory := apiClient.FetchInventory()
	logf("Inventory: %s\n", inventory.Describe())
	noteUnreadSitemaps(apiClient, &r.c.notices)

	// A site whose /wp/v2/posts answers 500 for every request still serves
	// its feed. Recovering titles, addresses, dates and bodies from it is
	// what makes such a site exportable at all (#40).
	recoverPostsFromFeed(cfg, inventory, exportData)

	// And on a site with no content API at all, the sitemap is not a check
	// on the export — it is the export. Its addresses answer 200 to anyone
	// who asks, and 1.8.15 listed them and fetched none (#68).
	recoverPagesFromSitemap(cfg, seo.NewCrawler(cfg), inventory, exportData)

	exportData.Stats.Uncovered = checkCoverage(inventory, exportData)
}

// finishCheckpoint deletes the checkpoint of a completed resumable run.
func (r *exportRun) finishCheckpoint() {
	// Delete checkpoint on successful completion
	if r.cfg.Resume {
		if err := r.checkpointMgr.Delete(); err != nil {
			logf("Warning: failed to delete checkpoint: %v\n", err)
		}
	}
}
