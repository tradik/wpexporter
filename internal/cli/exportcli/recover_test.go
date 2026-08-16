package exportcli

// Recovering from the feed (#40). A site whose /wp/v2/posts answers 500 for
// every request still serves its feed; without this, such a site cannot be
// exported at all.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// feedInventory is a feed carrying two recent posts.
func feedInventory() api.Inventory {
	first := models.WordPressPost{Slug: "hello", Link: "https://x.test/blog/hello/"}
	first.Title.Rendered = "Hello"
	second := models.WordPressPost{Slug: "second", Link: "https://x.test/blog/second/"}
	second.Title.Rendered = "Second"

	return api.Inventory{
		Feed:      "https://x.test/feed/",
		FeedPosts: []models.WordPressPost{first, second},
	}
}

// TestRecoveryFillsAnEmptyExport: the case the flag exists for.
func TestRecoveryFillsAnEmptyExport(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.FromSitemap = true

	data := &models.ExportData{}
	recoverPostsFromFeed(cfg, feedInventory(), data)

	require.Len(t, data.Posts, 2)
	assert.Equal(t, "Hello", data.Posts[0].Title.Rendered)
	assert.Equal(t, 2, data.Stats.TotalPosts)
	assert.Equal(t, 2, data.Stats.RecoveredPosts, "the export states which records are thinner")
}

// TestRecoveryNeverReplacesTheAPI: REST is the better source in every respect,
// so a collection it did serve is never merged with or replaced by the feed's
// recent items.
func TestRecoveryNeverReplacesTheAPI(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.FromSitemap = true

	fetched := models.WordPressPost{ID: 9, Slug: "hello", Link: "https://x.test/blog/hello/"}
	data := &models.ExportData{Posts: []models.WordPressPost{fetched}}
	recoverPostsFromFeed(cfg, feedInventory(), data)

	require.Len(t, data.Posts, 1)
	assert.Equal(t, 9, data.Posts[0].ID)
	assert.Zero(t, data.Stats.RecoveredPosts)
}

// TestRecoveryIsAskedForNeverAssumed: without the flag nothing changes, and
// --no-posts still means no posts.
func TestRecoveryIsAskedForNeverAssumed(t *testing.T) {
	data := &models.ExportData{}
	recoverPostsFromFeed(config.DefaultConfig(), feedInventory(), data)
	assert.Empty(t, data.Posts)

	skipped := config.DefaultConfig()
	skipped.FromSitemap = true
	skipped.NoPosts = true

	data = &models.ExportData{}
	recoverPostsFromFeed(skipped, feedInventory(), data)
	assert.Empty(t, data.Posts, "--no-posts is the operator saying they do not want posts")
}

// TestRecoveryWithoutAFeed: a site that publishes neither a working API nor a
// feed cannot be helped, and says nothing rather than pretending.
func TestRecoveryWithoutAFeed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.FromSitemap = true

	data := &models.ExportData{}
	recoverPostsFromFeed(cfg, api.Inventory{}, data)

	assert.Empty(t, data.Posts)
	assert.Zero(t, data.Stats.RecoveredPosts)
}
