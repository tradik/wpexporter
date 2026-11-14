package exportcli

import (
	"github.com/tradik/wpexporter/internal/basichtml"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/flathtml"
	"github.com/tradik/wpexporter/pkg/models"
)

// convertContent applies --flat-html or --basic-html. Must happen AFTER media
// filtering (the filter needs HTML tags).
func (r *exportRun) convertContent() {
	cfg, c := r.cfg, &r.c

	if cfg.FlatHTML {
		logln("\nConverting HTML to Markdown...")
		convertDocuments(c, newFlatConverter(cfg).ConvertPosts)
		logln("HTML to Markdown conversion complete")
	}

	if cfg.BasicHTML {
		logln("\nSanitizing HTML to basic elements...")
		convertDocuments(c, newBasicSanitizer(cfg).SanitizePosts)
		logln("HTML sanitization complete")
	}
}

// convertDocuments runs convert over the posts and the pages that exist.
func convertDocuments(c *collected, convert func([]models.WordPressPost) []models.WordPressPost) {
	if len(c.posts) > 0 {
		c.posts = convert(c.posts)
	}
	if len(c.pages) > 0 {
		c.pages = convert(c.pages)
	}
}

// preserving reports whether some elements are to travel as HTML.
func preserving(cfg *config.Config) bool {
	return len(cfg.PreserveClasses) > 0 || len(cfg.PreserveIDs) > 0
}

// newFlatConverter picks the narrowest Markdown converter for the options.
func newFlatConverter(cfg *config.Config) *flathtml.Converter {
	switch {
	case preserving(cfg):
		return flathtml.NewConverterWithOptions(cfg.FlatHTMLRules,
			&flathtml.PreserveOptions{Classes: cfg.PreserveClasses, IDs: cfg.PreserveIDs})
	case len(cfg.FlatHTMLRules) > 0:
		return flathtml.NewConverterWithRules(cfg.FlatHTMLRules)
	default:
		return flathtml.NewConverter()
	}
}

// newBasicSanitizer picks the HTML sanitizer for the options.
func newBasicSanitizer(cfg *config.Config) *basichtml.Sanitizer {
	if preserving(cfg) {
		return basichtml.NewSanitizerWithOptions(
			&basichtml.PreserveOptions{Classes: cfg.PreserveClasses, IDs: cfg.PreserveIDs})
	}
	return basichtml.NewSanitizer()
}
