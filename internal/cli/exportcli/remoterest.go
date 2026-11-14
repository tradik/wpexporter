package exportcli

import (
	"errors"
	"fmt"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// fetchRemoteRest reads what the REST export reads after the documents and the
// media: taxonomy, people, menus, comments, and what brute force and a range
// rescan add.
func (r *exportRun) fetchRemoteRest() error {
	cfg, apiClient, c := r.cfg, r.apiClient, &r.c

	logln("Fetching categories...")
	// Taxonomy is not the export: a site whose categories route is missing —
	// a WordPress older than the one that introduced wp/v2, a plugin that hid
	// it — still has its posts and pages, and used to lose them all here before
	// --from-sitemap could even be reached (#68).
	var err error
	c.categories, err = r.src.GetCategories()
	if err := noteIncomplete(&c.gaps, "categories", err); err != nil {
		return err
	}
	logf("Found %d categories\n", len(c.categories))

	if !cfg.NoTags {
		logln("Fetching tags...")
		c.tags, err = r.src.GetTags()
		if err := noteIncomplete(&c.gaps, "tags", err); err != nil {
			return err
		}
		logf("Found %d tags\n", len(c.tags))
	} else {
		logln("Skipping tags (--no-tags)")
	}

	if !cfg.NoUsers {
		logln("Fetching users...")
		c.users, err = r.src.GetUsers()
		if err != nil {
			// Don't fail on users fetch error, just warn and continue
			logf("Warning: could not fetch users: %v\n", err)
			c.users = []models.WordPressUser{}
		} else {
			logf("Found %d users\n", len(c.users))
		}
	} else {
		logln("Skipping users (--no-users)")
	}

	c.menus = fetchMenus(r.src, cfg)
	c.comments = fetchComments(r.src, cfg)

	if err := r.discover(); err != nil {
		return err
	}

	// Asked again because a run told to skip posts and pages learns the shape of
	// the site's API later than one that was not; noteSiteAPI says a thing once.
	noteSiteAPI(apiClient, &c.notices)

	fallBackToFeed(apiClient, cfg)
	return nil
}

// menuReader is what fetchMenus reads menus from: the REST client, or the
// database.
type menuReader interface {
	GetMenus() ([]models.WordPressMenu, error)
}

// fetchMenus reads the navigation menus. Menu structure is the one part of a
// site that cannot be reconstructed from the content afterwards.
func fetchMenus(src menuReader, cfg *config.Config) []models.WordPressMenu {
	if cfg.NoMenus {
		logln("Skipping menus (--no-menus)")
		return nil
	}

	logln("Fetching menus...")
	menus, err := src.GetMenus()

	switch {
	case errors.Is(err, api.ErrMenusNotAccessible):
		// WordPress gates menus behind edit_theme_options, so a public API
		// still refuses them. Say what would make it work rather than just
		// reporting a permission error.
		logln("  Menus are not readable on this site — WordPress requires " +
			"authentication for them. Pass --auth-user/--auth-pass or --auth-token to include them.")
		return nil
	case err != nil:
		logf("Warning: could not fetch menus: %v\n", err)
		return nil
	default:
		logf("Found %d menus\n", len(menus))
		return menus
	}
}

// discover runs brute force discovery and the targeted range rescan.
func (r *exportRun) discover() error {
	cfg, c := r.cfg, &r.c

	// Perform brute force scanning if enabled
	if cfg.BruteForce {
		logln("\nPerforming brute force content discovery...")
		scanResult, err := r.scanner.ScanForContent(c.posts, c.pages, c.media)
		if err != nil {
			return fmt.Errorf("brute force scan failed: %w", err)
		}

		// Merge brute force results
		c.posts = append(c.posts, scanResult.Posts...)
		c.pages = append(c.pages, scanResult.Pages...)
		c.media = append(c.media, scanResult.Media...)
		c.bruteForceFound = scanResult.Found
	}

	// Targeted range rescan (--scan-range) — pull a specific inclusive ID range
	// for posts/pages/media and merge any items not already fetched.
	if cfg.ScanRange != "" {
		logf("\nRescanning ID range %s...\n", cfg.ScanRange)
		n, err := appendScanRange(r.scanner, cfg, &c.posts, &c.pages, &c.media)
		if err != nil {
			return fmt.Errorf("scan-range failed: %w", err)
		}
		c.bruteForceFound += n
		logf("Range rescan found %d new items\n", n)
	}
	return nil
}
