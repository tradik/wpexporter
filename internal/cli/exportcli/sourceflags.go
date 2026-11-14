package exportcli

// Where the export reads the site from (FEAT-001).
//
// --origin sends the site's own requests to its web server rather than to
// whatever its DNS name points at — Cloudflare, usually, which is what blocks
// the REST API and the crawl. It works with or without local mode.

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/origin"
)

var (
	originAddr     string
	originInsecure bool
)

func init() {
	registerSourceFlags(exportCmd)
}

// registerSourceFlags adds the source-selection flags to cmd.
func registerSourceFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&originAddr, "origin", "",
		"send the site's own requests to this web server (scheme://host[:port]), bypassing DNS/Cloudflare")
	cmd.Flags().BoolVar(&originInsecure, "origin-insecure", false,
		"do not verify the certificate on connections to --origin (only those)")
}

// applySourceFlagOverrides copies the source flags the operator gave onto cfg.
func applySourceFlagOverrides(cmd *cobra.Command, cfg *config.Config) {
	if cmd.Flags().Changed("origin") {
		cfg.Origin = originAddr
	}
	if cmd.Flags().Changed("origin-insecure") {
		cfg.OriginInsecure = originInsecure
	}
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
	return nil
}
