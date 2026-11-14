package exportcli

// Local mode (FEAT-001): the site read from its own database, on its own
// server, with the rendered pages crawled from its own web server.
//
// The database is the inventory and the structure — every published document,
// term, author, comment, menu and attachment. It is not the rendered page:
// post_content is raw material (shortcodes, block comments, no paragraphs, and
// for a page builder often a stale copy). So each document's body comes from
// the crawl through the origin, and only where that gives nothing from the
// cleaned stored body; how many stayed raw is reported.

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/internal/localdb"
	"github.com/tradik/wpexporter/internal/origin"
	"github.com/tradik/wpexporter/internal/seo"
	"github.com/tradik/wpexporter/internal/wpconfig"
)

// openDatabase and detectOrigin are localdb.Open and origin.Detect; tests
// substitute a mock database and a web server that is not on port 80.
var (
	openDatabase = localdb.Open
	detectOrigin = origin.Detect
)

// exportLocal runs an export from the database.
func exportLocal(cmd *cobra.Command, cfg *config.Config) error {
	if err := checkLocalFlags(cmd, cfg); err != nil {
		return err
	}

	// One budget for the database's walks and the REST client's products.
	limits := api.NewLimits(cfg.Limit, cfg.LimitPerType, cfg.LimitByType)
	src, err := openLocal(cfg, limits)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	// There are no API responses to cache here; the crawl through the origin
	// is what --cache keeps, as it does on the REST path.
	fileCache, err := initializeCache(cfg, nil)
	if err != nil {
		return err
	}
	if fileCache != nil {
		defer func() { _ = fileCache.Close() }()
	}

	if err := checkOutput(cfg); err != nil {
		return err
	}

	originUp := connectOrigin(cfg)
	applyLocalDefaults(cmd, cfg, originUp)

	up, err := src.Uploads()
	if err != nil {
		return err
	}
	cfg.UploadsURL, cfg.UploadsDir = up.BaseURL, up.Dir

	r := &exportRun{cfg: cfg, src: src, exporter: export.NewExporter(cfg), fileCache: fileCache, limits: limits}
	r.announce()
	return r.runLocal(src, originUp)
}

// runLocal runs the local stages.
func (r *exportRun) runLocal(src *localdb.Source, originUp bool) error {
	r.limitMedia()
	if err := r.fetchLocal(src, originUp); err != nil {
		return err
	}
	if originUp {
		r.resolveLinks()
	}

	raw := rawBodies(&r.c)
	r.shapeDocuments()
	r.noteRawContent(raw, originUp)

	if r.cfg.DownloadMedia {
		logln("Reading the media library...")
		media, err := r.src.GetMedia()
		if err := noteIncomplete(&r.c.gaps, "media", err); err != nil {
			return err
		}
		r.c.media = media
		logf("Found %d media items (files are read from %s)\n", len(media), r.cfg.UploadsDir)
		r.filterRelevantMedia()
	} else {
		logln("Skipping media (--no-media)")
	}
	r.convertContent()

	// The stated totals are the database's own counts of what a limit cut
	// short. There is no sitemap check: the database is the inventory.
	r.stated = localStated{src: src, client: r.apiClient, limits: r.limits}
	r.c.notices = append(r.c.notices, src.Notes()...)

	exportData := r.assemble()
	zipPath, err := r.write(exportData)
	if err != nil {
		return err
	}
	r.summarize(exportData, zipPath)
	return nil
}

// openLocal reads wp-config.php and opens the database. A run given --local
// without --url takes the site's address from the database.
func openLocal(cfg *config.Config, limits *api.Limits) (*localdb.Source, error) {
	logf("Reading WordPress settings from %s...\n", cfg.LocalPath)
	settings, err := wpconfig.Load(cfg.LocalPath, wpconfig.Overrides{
		DBHost: cfg.DBHost, DBName: cfg.DBName, DBUser: cfg.DBUser, TablePrefix: cfg.DBPrefix,
	}, os.Getenv)
	if err != nil {
		return nil, fmt.Errorf("wp-config.php: %w", err)
	}

	src, err := openDatabase(context.Background(), settings, localdb.Options{
		BlogID: cfg.BlogID, Timeout: seconds(cfg.Timeout), Limits: limits,
	})
	if err != nil {
		return nil, err
	}
	logf("Connected to database %s (read only, consistent snapshot)\n", settings.DBName)

	if cfg.URL == "" {
		home, err := src.Home()
		if err != nil || home == "" {
			_ = src.Close()
			return nil, fmt.Errorf("the site's address could not be read from the database; give --url: %v", err)
		}
		cfg.URL = home
	}
	return src, nil
}

// connectOrigin settles where the site's web server is: --origin as given, or
// found on this machine. It reports whether one answers.
func connectOrigin(cfg *config.Config) bool {
	if cfg.Origin != "" {
		logf("Reaching %s at %s (--origin)\n", cfg.URL, cfg.Origin)
		return true
	}
	if o, ok := detectOrigin(cfg); ok {
		cfg.Origin = o.String()
		logf("Found the site's web server at %s\n", cfg.Origin)
		return true
	}
	logln("No web server answered for the site on 127.0.0.1 (ports 443 and 80): the export comes " +
		"from the database alone, without crawled pages or SEO. Point --origin at the site's web server to add them.")
	return false
}

// applyLocalDefaults turns on the crawl local mode relies on — every page's
// rendered body and SEO, read through the origin — unless the operator said
// otherwise. Without an origin there is nothing to crawl.
func applyLocalDefaults(cmd *cobra.Command, cfg *config.Config, originUp bool) {
	if !originUp {
		return
	}
	if !cmd.Flags().Changed("assisted-crawl") {
		cfg.AssistedCrawl = true
	}
	if !cmd.Flags().Changed("crawl-content") {
		cfg.CrawlContent = true
	}
	// The stored body is never the rendered page, so every page is re-read —
	// the existing "always" mode of --crawl-content-mode, not a new mechanism.
	if !cmd.Flags().Changed("crawl-content-mode") && cfg.CrawlContentMode == "" {
		cfg.CrawlContentMode = seo.CrawlAlways
	}
}
