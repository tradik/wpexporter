package exportcli

// Writing the export, and saying what was written.

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tradik/wpexporter/pkg/models"
)

// write exports the data and, when asked, zips it. It returns the ZIP's path,
// or "" when none was made.
func (r *exportRun) write(exportData *models.ExportData) (string, error) {
	cfg := r.cfg

	// Export data
	logln("\nExporting data...")
	if err := r.exporter.Export(exportData); err != nil {
		return "", fmt.Errorf("export failed: %w", err)
	}

	// Create ZIP archive if requested
	if !cfg.CreateZip {
		return "", nil
	}
	logln("Creating ZIP archive...")
	zipPath := cfg.Output + ".zip"
	if err := createZipArchive(cfg.Output, zipPath); err != nil {
		return "", fmt.Errorf("failed to create ZIP archive: %w", err)
	}
	logf("ZIP archive created: %s\n", zipPath)

	// Remove files if --no-files is set
	if cfg.NoFiles {
		logln("Removing export files...")
		if err := os.RemoveAll(cfg.Output); err != nil {
			return "", fmt.Errorf("failed to remove export files: %w", err)
		}
		logln("Export files removed")
	}
	return zipPath, nil
}

// summarize prints the export summary.
func (r *exportRun) summarize(exportData *models.ExportData, zipPath string) {
	cfg, c, stated := r.cfg, &r.c, r.stated

	// Print summary
	duration := time.Since(r.startTime)
	logf("\n=== Export Summary ===\n")
	logf("Site: %s (%s)\n", c.siteInfo.Name, c.siteInfo.URL)
	logf("%s\n", countLine("Posts", len(c.posts), stated.StatedTotal("posts"), stated.Limited()))
	// Fetched against written: a count that used to match only because nobody
	// compared them, while pages sharing a slug overwrote each other (#38).
	if written := exportData.Stats.PagesWritten; written > 0 && written != len(c.pages) {
		logf("Pages: %d fetched, %d written\n", len(c.pages), written)
	} else {
		logf("%s\n", countLine("Pages", len(c.pages), stated.StatedTotal("pages"), stated.Limited()))
	}
	// One line per custom type: "Services: 48" says more than a total, and the
	// absence of a type a user expected is the point of reporting them (#28).
	for _, set := range c.customTypes {
		logf("%s (%s): %d\n", set.Name, set.Slug, len(set.Posts))
	}
	if c.productsNotice != "" {
		logf("%s\n", c.productsNotice)
	} else {
		logf("%s\n", countLine("Products", len(c.products), stated.StatedTotal("products"), stated.Limited()))
	}
	logf("%s\n", countLine("Media", len(c.media), stated.StatedTotal("media"), stated.Limited()))
	logf("Categories: %d\n", len(c.categories))
	logf("Tags: %d\n", len(c.tags))
	logf("Users: %d\n", len(c.users))
	// Reported unconditionally: a site with comments that exports zero of them
	// is exactly the case a silent summary would hide (#35).
	logf("Comments: %d\n", len(c.comments))

	// Gaps last, where the eye stops. A summary that reports "Posts: 200"
	// without saying the walk stopped at page 3 is the silence #37 is about;
	// the same lines are in metadata.json, because a console scrolls away.
	for _, gap := range c.gaps {
		logf("Incomplete: %s\n", gap)
	}
	for _, notice := range c.notices {
		logf("Note: %s\n", notice)
	}

	// And what the site says it has, against what this run wrote (#40). The
	// export is already on disk: this can only add a sentence, never take one
	// away.
	for _, line := range exportData.Stats.Uncovered {
		logf("%s\n", line)
	}

	if cfg.BruteForce && c.bruteForceFound > 0 {
		logf("Brute force found: %d\n", c.bruteForceFound)
	}
	if cfg.DownloadMedia {
		logf("Media downloaded: %d\n", exportData.Stats.MediaDownloaded)
	}
	logf("Duration: %v\n", duration)
	r.summarizeSizes(zipPath)
}

// summarizeSizes prints the sizes and where the export is.
func (r *exportRun) summarizeSizes(zipPath string) {
	cfg := r.cfg

	// Calculate and display file sizes
	if !cfg.NoFiles {
		if totalSize, err := getDirSize(cfg.Output); err == nil {
			logf("Export size: %s\n", formatFileSize(totalSize))
		}
		// Calculate media folder size separately if it exists
		mediaDir := filepath.Join(cfg.Output, "media")
		if mediaSize, err := getDirSize(mediaDir); err == nil && mediaSize > 0 {
			logf("Media size: %s\n", formatFileSize(mediaSize))
		}
	}

	if cfg.CreateZip {
		if zipSize, err := getFileSize(zipPath); err == nil {
			logf("ZIP size: %s\n", formatFileSize(zipSize))
		}
		logf("ZIP: %s\n", zipPath)
		if !cfg.NoFiles {
			logf("Output: %s\n", cfg.Output)
		}
	} else {
		logf("Output: %s\n", cfg.Output)
	}
}
