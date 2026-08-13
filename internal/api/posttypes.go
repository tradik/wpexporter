package api

// Custom post type discovery (#28).
//
// A WordPress site is rarely just posts and pages. Themes and plugins register
// their own types — Services, Portfolio, Team, Testimonials — and those entries
// hold real, published content with its own URLs. An export that fetches only
// /posts and /pages leaves them behind silently: the migrated site loses whole
// sections and nothing in the output says so.
//
// The REST API already lists every type it exposes, so discovery is a single
// request. What is NOT content is excluded by name: WordPress's own internal
// types (templates, patterns, navigation, fonts) and the editor/SEO plugin
// bookkeeping types that would otherwise arrive as pages of markup nobody wrote.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tradik/wpexporter/internal/cache"
	"github.com/tradik/wpexporter/pkg/models"
)

// builtinTypes are handled by their own exporters (posts, pages, media,
// products) or are WordPress internals that are not content at all.
var builtinTypes = map[string]bool{
	"post": true, "page": true, "attachment": true, "revision": true,
	"nav_menu_item": true, "custom_css": true, "customize_changeset": true,
	"oembed_cache": true, "user_request": true, "wp_block": true,
	"wp_template": true, "wp_template_part": true, "wp_global_styles": true,
	"wp_navigation": true, "wp_font_family": true, "wp_font_face": true,
	"product": true, "product_variation": true, "shop_order": true,
	"shop_coupon": true, "shop_order_refund": true,
}

// bookkeepingPrefixes mark types a plugin registers to store its own settings
// rather than the site's content. Their entries are serialized configuration,
// not something a visitor ever read.
var bookkeepingPrefixes = []string{
	"elementor_", "rank_math_", "aam_", "rm_", "wpforms_", "acf-", "jet-",
	"vc_", "wpcf7_", "tablepress_", "brizy_",
}

// PostType is one type the REST API exposes, reduced to what an export needs.
type PostType struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	RestBase   string `json:"rest_base"`
	HasArchive bool   `json:"has_archive"`
	Taxonomies []string
}

// GetPostTypes lists every post type the REST API exposes, including the
// built-in ones. Callers filter with CustomPostTypes.
func (c *Client) GetPostTypes() ([]PostType, error) {
	cacheKey := cache.GenerateAPIKey("types", 0)

	var cached []PostType
	if c.getFromCache(cacheKey, &cached) {
		return cached, nil
	}

	url := fmt.Sprintf("%s/types", c.baseURL)
	resp, err := c.httpClient.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to list post types: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API returned status %d for post types", resp.StatusCode())
	}

	// The endpoint answers with an object keyed by type slug, not an array.
	var raw map[string]struct {
		Slug       string   `json:"slug"`
		Name       string   `json:"name"`
		RestBase   string   `json:"rest_base"`
		HasArchive bool     `json:"has_archive"`
		Taxonomies []string `json:"taxonomies"`
	}
	if err := json.Unmarshal(resp.Body(), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse post types: %w", err)
	}

	types := make([]PostType, 0, len(raw))
	for slug, t := range raw {
		if t.Slug == "" {
			t.Slug = slug
		}
		if t.RestBase == "" {
			continue // not reachable over REST, so not fetchable
		}
		types = append(types, PostType{
			Slug: t.Slug, Name: t.Name, RestBase: t.RestBase,
			HasArchive: t.HasArchive, Taxonomies: t.Taxonomies,
		})
	}
	// Stable order so a report and a re-run agree.
	sort.Slice(types, func(i, j int) bool { return types[i].Slug < types[j].Slug })

	c.saveToCache(cacheKey, types)
	return types, nil
}

// CustomPostTypes narrows a type list to the ones that carry site content: not
// built-in, not plugin bookkeeping, and reachable over REST. A rest_base with a
// path or a regex in it (font faces are nested under a family) is not a
// collection an export can walk, so it is dropped too.
func CustomPostTypes(types []PostType) []PostType {
	var out []PostType
	for _, t := range types {
		if builtinTypes[t.Slug] || isBookkeepingType(t.Slug) {
			continue
		}
		if strings.ContainsAny(t.RestBase, "/(?<") {
			continue
		}
		out = append(out, t)
	}
	return out
}

// constructionMarkers name a type that holds page CONSTRUCTION rather than page
// content: a theme's saved layouts and templates (ThemeREX's cpt_layouts, a
// builder's reusable sections). They are markup a visitor met inside other
// pages, never a document with its own place in the site — exporting them adds
// a directory of duplicated fragments to every migration.
var constructionMarkers = []string{"layout", "template", "block", "section", "popup", "widget"}

// isBookkeepingType reports whether the slug belongs to a plugin's internal
// storage or a theme's page construction rather than to the site's content.
func isBookkeepingType(slug string) bool {
	for _, prefix := range bookkeepingPrefixes {
		if strings.HasPrefix(slug, prefix) {
			return true
		}
	}
	for _, marker := range constructionMarkers {
		if strings.Contains(slug, marker) {
			return true
		}
	}
	return false
}

// GetCustomPosts retrieves every entry of one custom post type, paginated like
// posts and pages.
func (c *Client) GetCustomPosts(restBase string) ([]models.WordPressPost, error) {
	cacheKey := cache.GenerateAPIKey("cpt:"+restBase, 0)

	var cached []models.WordPressPost
	if c.getFromCache(cacheKey, &cached) {
		return cached, nil
	}

	posts, err := c.getAllContent(restBase)
	if err != nil {
		return nil, err
	}

	c.saveToCache(cacheKey, posts)
	return posts, nil
}
