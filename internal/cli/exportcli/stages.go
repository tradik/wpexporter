package exportcli

// The stages every export shares, whatever it read the site from.

import (
	"github.com/tradik/wpexporter/internal/basichtml"
	"github.com/tradik/wpexporter/internal/filter"
	"github.com/tradik/wpexporter/internal/flathtml"
	mediafilter "github.com/tradik/wpexporter/internal/media"
	"github.com/tradik/wpexporter/internal/seo"
	"github.com/tradik/wpexporter/pkg/models"
)

// shapeDocuments applies the path filter, the crawl and --skip-empty-content.
func (r *exportRun) shapeDocuments() {
	cfg, c := r.cfg, &r.c

	// Apply path filter if specified
	if cfg.PathFilter != "" {
		pathFilter := filter.NewPathFilter(cfg.PathFilter)
		originalPosts := len(c.posts)
		originalPages := len(c.pages)
		c.posts = pathFilter.FilterPosts(c.posts)
		c.pages = pathFilter.FilterPosts(c.pages)
		logf("Path filter '%s': %d/%d posts, %d/%d pages matched\n",
			cfg.PathFilter, len(c.posts), originalPosts, len(c.pages), originalPages)
	}

	r.crawl()

	// Skip posts/pages with empty content if enabled
	if cfg.SkipEmptyContent {
		originalPosts := len(c.posts)
		originalPages := len(c.pages)
		c.posts = seo.FilterEmptyContent(c.posts)
		c.pages = seo.FilterEmptyContent(c.pages)
		logf("Skipped empty content: %d/%d posts, %d/%d pages\n",
			originalPosts-len(c.posts), originalPosts, originalPages-len(c.pages), originalPages)
	}
}

// crawl reads the rendered pages for SEO metadata and/or content.
func (r *exportRun) crawl() {
	cfg, c := r.cfg, &r.c
	var enrich func([]models.WordPressPost) []models.WordPressPost
	crawler := seo.NewCrawler(cfg)
	crawler.SetCache(r.fileCache)

	switch {
	case cfg.AssistedCrawl && cfg.CrawlContent:
		// Combined crawl - fetch each page once for both SEO and content
		logln("\nCrawling URLs for SEO metadata and content (combined)...")
		enrich = crawler.EnrichPostsWithSEOAndContent
	case cfg.AssistedCrawl:
		// SEO only
		logln("\nCrawling URLs for SEO metadata...")
		enrich = crawler.EnrichPostsWithSEO
	case cfg.CrawlContent:
		// Content only
		logln("\nCrawling pages with empty content...")
		enrich = crawler.EnrichPostsWithContent
	default:
		return
	}

	if len(c.posts) > 0 {
		c.posts = enrich(c.posts)
	}
	if len(c.pages) > 0 {
		c.pages = enrich(c.pages)
	}
	enrichCustomTypes(c.customTypes, enrich)

	if cfg.AssistedCrawl {
		c.marketing = crawler.SiteMarketing(homePageURL(c.siteInfo, cfg))
		c.analytics = crawler.Analytics()
		if cfg.CrawlContent {
			logln("SEO metadata and content extraction complete")
		} else {
			logln("SEO metadata extraction complete")
		}
	}
}

// filterRelevantMedia narrows the media to what the documents reference.
// Must happen BEFORE FlatHTML conversion because filter needs HTML <a href>
// and <img src> tags.
func (r *exportRun) filterRelevantMedia() {
	if r.cfg.RelevantMediaOnly {
		mf := mediafilter.NewFilter()
		originalMedia := len(r.c.media)
		r.c.media = mf.FilterRelevantMedia(r.c.posts, r.c.pages, r.c.media)
		logf("Filtered to %d relevant media items (from %d total)\n", len(r.c.media), originalMedia)
	}
}

// convertContent applies --flat-html or --basic-html. Must happen AFTER media
// filtering (the filter needs HTML tags).
func (r *exportRun) convertContent() {
	cfg, c := r.cfg, &r.c

	// Convert HTML to Markdown if flat-html is enabled
	if cfg.FlatHTML {
		logln("\nConverting HTML to Markdown...")
		var preserveOpts *flathtml.PreserveOptions
		if len(cfg.PreserveClasses) > 0 || len(cfg.PreserveIDs) > 0 {
			preserveOpts = &flathtml.PreserveOptions{Classes: cfg.PreserveClasses, IDs: cfg.PreserveIDs}
		}
		// Pick the narrowest constructor for the configured options.
		var converter *flathtml.Converter
		switch {
		case preserveOpts != nil:
			converter = flathtml.NewConverterWithOptions(cfg.FlatHTMLRules, preserveOpts)
		case len(cfg.FlatHTMLRules) > 0:
			converter = flathtml.NewConverterWithRules(cfg.FlatHTMLRules)
		default:
			converter = flathtml.NewConverter()
		}
		if len(c.posts) > 0 {
			c.posts = converter.ConvertPosts(c.posts)
		}
		if len(c.pages) > 0 {
			c.pages = converter.ConvertPosts(c.pages)
		}
		logln("HTML to Markdown conversion complete")
	}

	// Sanitize HTML to basic elements if basic-html is enabled
	if cfg.BasicHTML {
		logln("\nSanitizing HTML to basic elements...")
		var preserveOpts *basichtml.PreserveOptions
		if len(cfg.PreserveClasses) > 0 || len(cfg.PreserveIDs) > 0 {
			preserveOpts = &basichtml.PreserveOptions{Classes: cfg.PreserveClasses, IDs: cfg.PreserveIDs}
		}
		var sanitizer *basichtml.Sanitizer
		if preserveOpts != nil {
			sanitizer = basichtml.NewSanitizerWithOptions(preserveOpts)
		} else {
			sanitizer = basichtml.NewSanitizer()
		}
		if len(c.posts) > 0 {
			c.posts = sanitizer.SanitizePosts(c.posts)
		}
		if len(c.pages) > 0 {
			c.pages = sanitizer.SanitizePosts(c.pages)
		}
		logln("HTML sanitization complete")
	}
}

// assemble builds the export from what was collected.
func (r *exportRun) assemble() *models.ExportData {
	c := &r.c
	return &models.ExportData{
		Site: *c.siteInfo, Posts: c.posts, Pages: c.pages, Products: c.products, Media: c.media,
		Categories: c.categories, Tags: c.tags, Users: c.users, Menus: c.menus, Comments: c.comments,
		CustomTypes: c.customTypes, Analytics: c.analytics, Marketing: c.marketing,
		Stats: models.ExportStats{
			TotalPosts:       len(c.posts),
			TotalPages:       len(c.pages),
			TotalCustomPosts: countCustomPosts(c.customTypes),
			TotalProducts:    len(c.products),
			TotalMedia:       len(c.media),
			TotalCategories:  len(c.categories),
			TotalTags:        len(c.tags),
			TotalUsers:       len(c.users),
			TotalComments:    len(c.comments),
			BruteForceFound:  c.bruteForceFound,
			Incomplete:       c.gaps,
			Notices:          c.notices,
		},
	}
}
