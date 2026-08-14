package exportcli

// Fetching the site's reader comments (#35).
//
// Comments are the one part of a site its owner did not write and cannot
// rewrite: names, dates, threads and opinions left by readers over years. They
// ship by default, like posts and pages, and a site that keeps its comments
// switched off degrades to a warning rather than a failed export.

import (
	"errors"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// fetchComments retrieves every comment the site publishes. Any failure is a
// warning, never fatal: the rest of the export is still worth having.
func fetchComments(client *api.Client, cfg *config.Config) []models.WordPressComment {
	if cfg.NoComments {
		logln("Skipping comments (--no-comments)")
		return nil
	}

	logln("Fetching comments...")

	comments, err := client.GetComments()

	switch {
	case errors.Is(err, api.ErrCommentsDisabled):
		// Nothing to fetch and nothing to fix: the site turned commenting off.
		// Saying "pass credentials" here would send the operator after data
		// that does not exist.
		logln("  This site has comments disabled — there are none to export.")
		return nil
	case errors.Is(err, api.ErrCommentsNotAccessible):
		// Some installations disable the comments route entirely, or gate it
		// behind authentication. Say which it is instead of reporting a bare
		// permission error.
		logln("  Comments are not readable on this site — the REST route is " +
			"disabled or gated. Pass --auth-user/--auth-pass or --auth-token to include them.")
		return nil
	case err != nil:
		logf("Warning: could not fetch comments: %v\n", err)
		return nil
	}

	logf("Found %d comments\n", len(comments))

	return comments
}
