package seo

import (
	"regexp"
	"sort"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

var (
	// metaTagPattern matches a complete meta tag.
	metaTagPattern = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	// metaAttrPattern matches one quoted attribute inside a tag.
	metaAttrPattern = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("[^"]*"|'[^']*')`)
	// jsonLDPattern captures the body of an application/ld+json script block.
	jsonLDPattern = regexp.MustCompile(`(?is)<script[^>]+type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
)

// analyticsPatterns maps a tracking identifier's shape to the field it fills.
// Matching the identifier itself rather than the surrounding script survives
// however the snippet was minified, inlined or wrapped by a plugin.
var analyticsPatterns = []struct {
	name    string
	pattern *regexp.Regexp
	field   func(*models.Analytics) *[]string
}{
	{"GA4", regexp.MustCompile(`\bG-[A-Z0-9]{6,12}\b`),
		func(a *models.Analytics) *[]string { return &a.GA4 }},
	{"Universal Analytics", regexp.MustCompile(`\bUA-\d{4,12}-\d{1,4}\b`),
		func(a *models.Analytics) *[]string { return &a.UniversalAnalytics }},
	{"Google Tag Manager", regexp.MustCompile(`\bGTM-[A-Z0-9]{4,10}\b`),
		func(a *models.Analytics) *[]string { return &a.GoogleTagManager }},
	{"Google Ads", regexp.MustCompile(`\bAW-\d{6,12}\b`),
		func(a *models.Analytics) *[]string { return &a.GoogleAdsConversion }},
	{"Meta Pixel", regexp.MustCompile(`(?is)fbq\s*\(\s*['"]init['"]\s*,\s*['"](\d{10,20})['"]`),
		func(a *models.Analytics) *[]string { return &a.MetaPixel }},
	{"Hotjar", regexp.MustCompile(`(?is)hjid\s*:\s*(\d{5,12})`),
		func(a *models.Analytics) *[]string { return &a.HotjarSiteID }},
	{"Microsoft Clarity", regexp.MustCompile(`(?is)clarity\.ms/tag/([a-z0-9]{6,15})`),
		func(a *models.Analytics) *[]string { return &a.ClarityProjectID }},
	{"LinkedIn", regexp.MustCompile(`(?is)_linkedin_partner_id\s*=\s*["'](\d{4,12})["']`),
		func(a *models.Analytics) *[]string { return &a.LinkedInPartnerID }},
	{"TikTok", regexp.MustCompile(`(?is)ttq\.load\s*\(\s*['"]([A-Z0-9]{15,25})['"]`),
		func(a *models.Analytics) *[]string { return &a.TikTokPixel }},
}

// namedSEOFields maps a meta name/property to the SEOData field it fills, so
// each is promoted to a first-class key instead of living in the catch-all map.
var namedSEOFields = map[string]func(*models.SEOData) *string{
	"description":            func(s *models.SEOData) *string { return &s.MetaDescription },
	"keywords":               func(s *models.SEOData) *string { return &s.MetaKeywords },
	"robots":                 func(s *models.SEOData) *string { return &s.Robots },
	"og:title":               func(s *models.SEOData) *string { return &s.OGTitle },
	"og:description":         func(s *models.SEOData) *string { return &s.OGDescription },
	"og:image":               func(s *models.SEOData) *string { return &s.OGImage },
	"og:type":                func(s *models.SEOData) *string { return &s.OGType },
	"og:url":                 func(s *models.SEOData) *string { return &s.OGURL },
	"og:site_name":           func(s *models.SEOData) *string { return &s.OGSiteName },
	"og:locale":              func(s *models.SEOData) *string { return &s.OGLocale },
	"twitter:card":           func(s *models.SEOData) *string { return &s.TwitterCard },
	"twitter:title":          func(s *models.SEOData) *string { return &s.TwitterTitle },
	"twitter:description":    func(s *models.SEOData) *string { return &s.TwitterDesc },
	"twitter:image":          func(s *models.SEOData) *string { return &s.TwitterImage },
	"twitter:site":           func(s *models.SEOData) *string { return &s.TwitterSite },
	"article:published_time": func(s *models.SEOData) *string { return &s.ArticlePublished },
	"article:modified_time":  func(s *models.SEOData) *string { return &s.ArticleModified },
	"article:author":         func(s *models.SEOData) *string { return &s.ArticleAuthor },
	"article:section":        func(s *models.SEOData) *string { return &s.ArticleSection },
}

// metaTag is one parsed meta element.
type metaTag struct {
	key     string
	content string
}

// parseMetaTags reads every meta element in the page, keyed by its name,
// property or http-equiv attribute.
func parseMetaTags(html string) []metaTag {
	var tags []metaTag

	for _, element := range metaTagPattern.FindAllString(html, -1) {
		attrs := parseTagAttributes(element)

		key := firstNonEmpty(attrs["name"], attrs["property"], attrs["itemprop"], attrs["http-equiv"])
		if key == "" {
			continue
		}

		content, ok := attrs["content"]
		if !ok || strings.TrimSpace(content) == "" {
			continue
		}

		tags = append(tags, metaTag{key: strings.ToLower(strings.TrimSpace(key)), content: content})
	}

	return tags
}

// parseTagAttributes extracts a tag's quoted attributes, lower-casing names and
// decoding entity-encoded values.
func parseTagAttributes(element string) map[string]string {
	attrs := make(map[string]string)

	for _, match := range metaAttrPattern.FindAllStringSubmatch(element, -1) {
		quoted := match[2]
		value := quoted[1 : len(quoted)-1]

		name := strings.ToLower(match[1])
		if _, exists := attrs[name]; exists {
			continue // first occurrence wins, as browsers do
		}

		attrs[name] = decodeEntities(value)
	}

	return attrs
}

// applyMetaTags fills the named SEO fields and collects the rest.
//
// keep decides which non-named tags are retained; a nil keep retains all of
// them, because a tag nobody anticipated is exactly the one worth keeping.
func applyMetaTags(seo *models.SEOData, tags []metaTag, keep func(string) bool) {
	for _, tag := range tags {
		if field, named := namedSEOFields[tag.key]; named {
			target := field(seo)
			if *target == "" {
				*target = tag.content
			}

			continue
		}

		if keep != nil && !keep(tag.key) {
			continue
		}

		if seo.Meta == nil {
			seo.Meta = make(map[string]string)
		}
		if _, exists := seo.Meta[tag.key]; !exists {
			seo.Meta[tag.key] = tag.content
		}
	}
}

// applyPageMeta fills seo from every meta tag on the page, honoring both the
// --exclude-tags list and the --extract-meta policy.
func (c *Crawler) applyPageMeta(seo *models.SEOData, html string, excluded map[string]bool) {
	tags := parseMetaTags(html)

	kept := make([]metaTag, 0, len(tags))
	for _, tag := range tags {
		// --exclude-tags names OG tags as "og:image" and plain ones as
		// "meta:description", so check both spellings.
		if excluded[tag.key] || excluded["meta:"+tag.key] {
			continue
		}

		kept = append(kept, tag)
	}

	applyMetaTags(seo, kept, c.metaKeepFunc())
}

// metaKeepFunc returns the predicate deciding which unnamed meta tags survive,
// per the --extract-meta policy. Nil means keep everything.
func (c *Crawler) metaKeepFunc() func(string) bool {
	policy := strings.TrimSpace(strings.ToLower(c.config.ExtractMeta))

	switch policy {
	case "", "all":
		return nil
	case "none":
		return func(string) bool { return false }
	default:
		allowed := make(map[string]bool)
		for _, name := range strings.Split(policy, ",") {
			if name = strings.TrimSpace(name); name != "" {
				allowed[name] = true
			}
		}

		return func(key string) bool { return allowed[key] }
	}
}

// extractJSONLD returns the raw structured-data blocks declared on the page.
func extractJSONLD(html string) []string {
	var blocks []string

	for _, match := range jsonLDPattern.FindAllStringSubmatch(html, -1) {
		block := strings.TrimSpace(match[1])
		if block != "" {
			blocks = append(blocks, block)
		}
	}

	return blocks
}

// extractAnalytics finds tracking identifiers anywhere in the page.
func extractAnalytics(html string) models.Analytics {
	var analytics models.Analytics

	for _, detector := range analyticsPatterns {
		target := detector.field(&analytics)

		for _, match := range detector.pattern.FindAllStringSubmatch(html, -1) {
			// A capturing pattern yields the bare ID in group 1; a bare-ID
			// pattern has no groups and the whole match is the ID.
			id := match[0]
			if len(match) > 1 && match[1] != "" {
				id = match[1]
			}

			*target = appendUnique(*target, id)
		}
	}

	return analytics
}

// mergeAnalytics folds one page's findings into the site-wide set.
func mergeAnalytics(into *models.Analytics, found models.Analytics) {
	for _, detector := range analyticsPatterns {
		target := detector.field(into)
		for _, id := range *detector.field(&found) {
			*target = appendUnique(*target, id)
		}
	}
}

// isEmptyAnalytics reports whether nothing was detected.
func isEmptyAnalytics(a models.Analytics) bool {
	for _, detector := range analyticsPatterns {
		if len(*detector.field(&a)) > 0 {
			return false
		}
	}

	return true
}

// appendUnique adds a value if it is not already present, keeping the slice
// sorted so exports are reproducible.
func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	values = append(values, value)
	sort.Strings(values)

	return values
}

// firstNonEmpty returns the first non-blank string.
func firstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}

	return ""
}
