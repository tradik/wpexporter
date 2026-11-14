package exportcli

// Where the export reads the site from (FEAT-001).
//
// --origin sends the site's own requests to its web server rather than to
// whatever its DNS name points at — Cloudflare, usually, which is what blocks
// the REST API and the crawl. It works with or without local mode.
//
// --local reads the site from its own WordPress directory and database, on the
// server. The database password has no flag: a flag is visible to every user
// of the machine in the process list.

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/origin"
)

var (
	originAddr     string
	originInsecure bool
	localPath      string
	dbHost         string
	dbName         string
	dbUser         string
	dbPrefix       string
	blogID         int
)

func init() {
	registerSourceFlags(exportCmd)
	// A run needs an address or a WordPress directory (--local, which reads the
	// address from the database). Without either it stops at parsing, as a run
	// without --url always has. Declared here because files' init functions run
	// in name order, and --local does not exist before this one.
	exportCmd.MarkFlagsOneRequired("url", "local")
}

// registerSourceFlags adds the source-selection flags to cmd.
func registerSourceFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&originAddr, "origin", "",
		"send the site's own requests to this web server (scheme://host[:port]), bypassing DNS/Cloudflare")
	f.BoolVar(&originInsecure, "origin-insecure", false,
		"do not verify the certificate on connections to --origin (only those)")
	f.StringVar(&localPath, "local", "",
		"read the site from this WordPress directory and its database (run on the server)")
	f.StringVar(&dbHost, "db-host", "", "database host, overriding wp-config.php (--local)")
	f.StringVar(&dbName, "db-name", "", "database name, overriding wp-config.php (--local)")
	f.StringVar(&dbUser, "db-user", "", "database user, overriding wp-config.php (--local; password: WPEXPORTER_DB_PASSWORD)")
	f.StringVar(&dbPrefix, "db-prefix", "", "table prefix, overriding wp-config.php (--local)")
	f.IntVar(&blogID, "blog-id", 0, "multisite: which site to read (--local; default the main site)")
}

// applySourceFlagOverrides copies the source flags the operator gave onto cfg.
func applySourceFlagOverrides(cmd *cobra.Command, cfg *config.Config) {
	set := func(name string, apply func()) {
		if cmd.Flags().Changed(name) {
			apply()
		}
	}
	set("origin", func() { cfg.Origin = originAddr })
	set("origin-insecure", func() { cfg.OriginInsecure = originInsecure })
	set("local", func() { cfg.LocalPath = localPath })
	set("db-host", func() { cfg.DBHost = dbHost })
	set("db-name", func() { cfg.DBName = dbName })
	set("db-user", func() { cfg.DBUser = dbUser })
	set("db-prefix", func() { cfg.DBPrefix = dbPrefix })
	set("blog-id", func() { cfg.BlogID = blogID })
}

// validateSource refuses a source configuration that cannot work, before any
// request is made.
func validateSource(cfg *config.Config) error {
	o, err := origin.Parse(cfg.Origin)
	if err != nil {
		return err
	}
	if cfg.OriginInsecure && !o.Enabled() {
		return fmt.Errorf("--origin-insecure applies to --origin connections only; give --origin as well")
	}
	if cfg.LocalPath == "" && (cfg.DBHost != "" || cfg.DBName != "" || cfg.DBUser != "" || cfg.DBPrefix != "" || cfg.BlogID != 0) {
		return fmt.Errorf("--db-host, --db-name, --db-user, --db-prefix and --blog-id apply to --local only")
	}
	return nil
}

// notThroughREST is why REST credentials mean nothing in local mode.
const notThroughREST = "nothing is read through the REST API"

// localConflicts are the options local mode cannot honor, each with why.
var localConflicts = []struct {
	flag   string
	set    func(cmd *cobra.Command, cfg *config.Config) bool
	reason string
}{
	{"brute-force", func(_ *cobra.Command, c *config.Config) bool { return c.BruteForce }, "the database lists every ID already"},
	{"scan-range", func(_ *cobra.Command, c *config.Config) bool { return c.ScanRange != "" }, "the database lists every ID already"},
	{"max-id", func(cmd *cobra.Command, _ *config.Config) bool { return cmd.Flags().Changed("max-id") }, "it bounds --brute-force"},
	{"resume", func(_ *cobra.Command, c *config.Config) bool { return c.Resume }, "a database snapshot is read in one go"},
	{"from-sitemap", func(_ *cobra.Command, c *config.Config) bool { return c.FromSitemap }, "the database is the inventory"},
	{"auth-user", func(_ *cobra.Command, c *config.Config) bool { return c.AuthUser != "" }, notThroughREST},
	{"auth-pass", func(_ *cobra.Command, c *config.Config) bool { return c.AuthPass != "" }, notThroughREST},
	{"auth-token", func(_ *cobra.Command, c *config.Config) bool { return c.AuthToken != "" }, notThroughREST},
}

// checkLocalFlags names the first option given that local mode cannot honor.
func checkLocalFlags(cmd *cobra.Command, cfg *config.Config) error {
	for _, c := range localConflicts {
		if c.set(cmd, cfg) {
			return fmt.Errorf("--local cannot be combined with --%s: %s", c.flag, c.reason)
		}
	}
	return localSnapError()
}
