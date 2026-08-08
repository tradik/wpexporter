package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ParseScanRange parses a targeted ID range in "START-END" form into inclusive
// bounds. ok is false (with no error) when s is empty, meaning the feature is off.
func ParseScanRange(s string) (start, end int, ok bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false, nil
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("scan range must be START-END, got %q", s)
	}
	start, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid scan range start %q: %w", parts[0], err)
	}
	end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid scan range end %q: %w", parts[1], err)
	}
	if start < 1 || end < start {
		return 0, 0, false, fmt.Errorf("scan range must satisfy 1 <= start <= end, got %d-%d", start, end)
	}
	return start, end, true, nil
}

// Config represents the application configuration
type Config struct {
	URL           string `mapstructure:"url" json:"url"`
	Output        string `mapstructure:"output" json:"output"`
	Format        string `mapstructure:"format" json:"format"`
	BruteForce    bool   `mapstructure:"brute_force" json:"brute_force"`
	MaxID         int    `mapstructure:"max_id" json:"max_id"`
	ScanRange     string `mapstructure:"scan_range" json:"scan_range"` // Targeted ID range to rescan, e.g. "100-200"
	DownloadMedia bool   `mapstructure:"download_media" json:"download_media"`
	// MaxMediaBytes is the per-file media download cap in bytes (0 = built-in default).
	MaxMediaBytes     int64  `mapstructure:"max_media_bytes" json:"max_media_bytes"`
	RelevantMediaOnly bool   `mapstructure:"relevant_media_only" json:"relevant_media_only"`
	Concurrent        int    `mapstructure:"concurrent" json:"concurrent"`
	Timeout           int    `mapstructure:"timeout" json:"timeout"`
	Retries           int    `mapstructure:"retries" json:"retries"`
	UserAgent         string `mapstructure:"user_agent" json:"user_agent"`
	Verbose           bool   `mapstructure:"verbose" json:"verbose"`
	CreateZip         bool   `mapstructure:"create_zip" json:"create_zip"`
	NoFiles           bool   `mapstructure:"no_files" json:"no_files"`
	NoPosts           bool   `mapstructure:"no_posts" json:"no_posts"`
	NoPages           bool   `mapstructure:"no_pages" json:"no_pages"`
	NoProducts        bool   `mapstructure:"no_products" json:"no_products"`
	NoUsers           bool   `mapstructure:"no_users" json:"no_users"`
	PathFilter        string `mapstructure:"path_filter" json:"path_filter"`
	AssistedCrawl     bool   `mapstructure:"assisted_crawl" json:"assisted_crawl"`
	AuthUser          string `mapstructure:"auth_user" json:"auth_user"`
	AuthPass          string `mapstructure:"auth_pass" json:"auth_pass"`
	AuthToken         string `mapstructure:"auth_token" json:"auth_token"`
	RateLimit         int    `mapstructure:"rate_limit" json:"rate_limit"`                 // Milliseconds delay between API requests
	Resume            bool   `mapstructure:"resume" json:"resume"`                         // Resume from checkpoint if available
	CrawlContent      bool   `mapstructure:"crawl_content" json:"crawl_content"`           // Crawl empty content pages
	SkipEmptyContent  bool   `mapstructure:"skip_empty_content" json:"skip_empty_content"` // Skip posts/pages with empty content
	FlatHTML          bool   `mapstructure:"flat_html" json:"flat_html"`                   // Convert HTML to Markdown
	BasicHTML         bool   `mapstructure:"basic_html" json:"basic_html"`                 // Clean HTML to basic elements
	KeepOriginalURLs  bool   `mapstructure:"keep_original_urls" json:"keep_original_urls"` // Don't convert media URLs to local paths
	// MediaPathStyle selects the form of rewritten media paths in exported content:
	// "root" (/media/...) resolves from any URL depth, "relative" (media/...) only at the site root.
	MediaPathStyle string `mapstructure:"media_path_style" json:"media_path_style"`
	// LinkStyle selects the form of same-host address fields (link, canonical_url,
	// hreflangs): "absolute" keeps the source URL, "root" emits a root-relative path
	// so the content is not pinned to the retired host. Empty means the format's
	// default — see EffectiveLinkStyle.
	LinkStyle string `mapstructure:"link_style" json:"link_style"`
	// ReportA11y writes an accessibility report alongside the export.
	ReportA11y        bool           `mapstructure:"report_a11y" json:"report_a11y"`
	NoTags            bool           `mapstructure:"no_tags" json:"no_tags"`                                   // Skip exporting tags
	Quiet             bool           `mapstructure:"quiet" json:"quiet"`                                       // Suppress all output
	NoIDs             bool           `mapstructure:"no_ids" json:"no_ids"`                                     // Exclude numeric IDs from frontmatter
	ExcludeTags       []string       `mapstructure:"exclude_tags" json:"exclude_tags,omitempty"`               // SEO tags to exclude from extraction
	ExcludeMediaTypes []string       `mapstructure:"exclude_media_types" json:"exclude_media_types,omitempty"` // Media types to exclude from download
	PreserveClasses   []string       `mapstructure:"preserve_classes" json:"preserve_classes,omitempty"`       // Classes to preserve
	PreserveIDs       []string       `mapstructure:"preserve_ids" json:"preserve_ids,omitempty"`               // IDs to preserve
	FlatHTMLRules     []FlatHTMLRule `mapstructure:"flat_html_rules" json:"flat_html_rules,omitempty"`
	Cache             bool           `mapstructure:"cache" json:"cache"`             // Enable caching of API responses and crawl data
	CacheTTL          string         `mapstructure:"cache_ttl" json:"cache_ttl"`     // Cache TTL (e.g., "24h", "0" for unlimited)
	CacheDir          string         `mapstructure:"cache_dir" json:"cache_dir"`     // Cache directory path
	CacheClear        bool           `mapstructure:"cache_clear" json:"cache_clear"` // Clear cache before export
}

// FlatHTMLRule defines a custom HTML to Markdown conversion rule
// Example config.yaml:
//
//	flat_html_rules:
//	  - class: "brxe-heading"
//	    tag: "div"
//	    markdown: "## {content}\n\n"
//	  - class: "elementor-heading-title"
//	    markdown: "# {content}\n\n"
//	  - class: "my-paragraph"
//	    markdown: "{content}\n\n"
type FlatHTMLRule struct {
	Class    string `mapstructure:"class" json:"class"`       // CSS class to match (required)
	Tag      string `mapstructure:"tag" json:"tag"`           // HTML tag to match (optional, e.g., "div", "span")
	Markdown string `mapstructure:"markdown" json:"markdown"` // Markdown template ({content} placeholder)
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Output:            "", // Will be generated based on URL and date
		Format:            "json",
		BruteForce:        false,
		MaxID:             10000,
		ScanRange:         "",
		MaxMediaBytes:     0,
		DownloadMedia:     true,
		RelevantMediaOnly: false,
		Concurrent:        5,
		Timeout:           30,
		Retries:           3,
		UserAgent:         "WordPress-Export-JSON/1.0",
		Verbose:           false,
		CreateZip:         false,
		NoFiles:           false,
		NoPosts:           false,
		NoPages:           false,
		NoProducts:        false,
		NoUsers:           false,
		PathFilter:        "",
		AssistedCrawl:     false,
		RateLimit:         0,      // No rate limiting by default
		Resume:            false,  // Don't resume by default
		CrawlContent:      false,  // Don't crawl empty content by default
		SkipEmptyContent:  false,  // Don't skip empty content by default
		FlatHTML:          false,  // Don't flatten HTML by default
		NoTags:            false,  // Don't skip tags by default
		Cache:             false,  // Caching disabled by default
		CacheTTL:          "24h",  // 24 hour cache TTL by default
		CacheDir:          "",     // Will default to ~/.wpexporter/cache
		CacheClear:        false,  // Don't clear cache by default
		MediaPathStyle:    "root", // Root-relative media paths resolve from any URL depth
		LinkStyle:         "",     // Empty = per-format default (see EffectiveLinkStyle)
		ReportA11y:        false,  // Don't write an accessibility report by default
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configFile string) (*Config, error) {
	config := DefaultConfig()

	// Set up viper
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.wpexportjson")
	viper.AddConfigPath("/etc/wpexportjson")

	// Set environment variable prefix
	viper.SetEnvPrefix("WPEXPORT")
	viper.AutomaticEnv()

	// Bind environment variables
	if err := viper.BindEnv("url", "WPEXPORT_URL"); err != nil {
		return nil, fmt.Errorf("failed to bind url environment variable: %w", err)
	}
	if err := viper.BindEnv("output", "WPEXPORT_OUTPUT"); err != nil {
		return nil, fmt.Errorf("failed to bind output environment variable: %w", err)
	}
	if err := viper.BindEnv("format", "WPEXPORT_FORMAT"); err != nil {
		return nil, fmt.Errorf("failed to bind format environment variable: %w", err)
	}
	if err := viper.BindEnv("brute_force", "WPEXPORT_BRUTE_FORCE"); err != nil {
		return nil, fmt.Errorf("failed to bind brute_force environment variable: %w", err)
	}
	if err := viper.BindEnv("max_id", "WPEXPORT_MAX_ID"); err != nil {
		return nil, fmt.Errorf("failed to bind max_id environment variable: %w", err)
	}
	if err := viper.BindEnv("download_media", "WPEXPORT_DOWNLOAD_MEDIA"); err != nil {
		return nil, fmt.Errorf("failed to bind download_media environment variable: %w", err)
	}
	if err := viper.BindEnv("relevant_media_only", "WPEXPORT_RELEVANT_MEDIA_ONLY"); err != nil {
		return nil, fmt.Errorf("failed to bind relevant_media_only environment variable: %w", err)
	}
	if err := viper.BindEnv("path_filter", "WPEXPORT_PATH_FILTER"); err != nil {
		return nil, fmt.Errorf("failed to bind path_filter environment variable: %w", err)
	}
	if err := viper.BindEnv("assisted_crawl", "WPEXPORT_ASSISTED_CRAWL"); err != nil {
		return nil, fmt.Errorf("failed to bind assisted_crawl environment variable: %w", err)
	}
	if err := viper.BindEnv("concurrent", "WPEXPORT_CONCURRENT"); err != nil {
		return nil, fmt.Errorf("failed to bind concurrent environment variable: %w", err)
	}
	if err := viper.BindEnv("timeout", "WPEXPORT_TIMEOUT"); err != nil {
		return nil, fmt.Errorf("failed to bind timeout environment variable: %w", err)
	}
	if err := viper.BindEnv("retries", "WPEXPORT_RETRIES"); err != nil {
		return nil, fmt.Errorf("failed to bind retries environment variable: %w", err)
	}
	if err := viper.BindEnv("user_agent", "WPEXPORT_USER_AGENT"); err != nil {
		return nil, fmt.Errorf("failed to bind user_agent environment variable: %w", err)
	}
	if err := viper.BindEnv("verbose", "WPEXPORT_VERBOSE"); err != nil {
		return nil, fmt.Errorf("failed to bind verbose environment variable: %w", err)
	}
	if err := viper.BindEnv("rate_limit", "WPEXPORT_RATE_LIMIT"); err != nil {
		return nil, fmt.Errorf("failed to bind rate_limit environment variable: %w", err)
	}
	if err := viper.BindEnv("resume", "WPEXPORT_RESUME"); err != nil {
		return nil, fmt.Errorf("failed to bind resume environment variable: %w", err)
	}
	if err := viper.BindEnv("crawl_content", "WPEXPORT_CRAWL_CONTENT"); err != nil {
		return nil, fmt.Errorf("failed to bind crawl_content environment variable: %w", err)
	}
	if err := viper.BindEnv("skip_empty_content", "WPEXPORT_SKIP_EMPTY_CONTENT"); err != nil {
		return nil, fmt.Errorf("failed to bind skip_empty_content environment variable: %w", err)
	}
	if err := viper.BindEnv("flat_html", "WPEXPORT_FLAT_HTML"); err != nil {
		return nil, fmt.Errorf("failed to bind flat_html environment variable: %w", err)
	}
	if err := viper.BindEnv("basic_html", "WPEXPORT_BASIC_HTML"); err != nil {
		return nil, fmt.Errorf("failed to bind basic_html environment variable: %w", err)
	}
	if err := viper.BindEnv("keep_original_urls", "WPEXPORT_KEEP_ORIGINAL_URLS"); err != nil {
		return nil, fmt.Errorf("failed to bind keep_original_urls environment variable: %w", err)
	}
	if err := viper.BindEnv("no_tags", "WPEXPORT_NO_TAGS"); err != nil {
		return nil, fmt.Errorf("failed to bind no_tags environment variable: %w", err)
	}
	if err := viper.BindEnv("no_ids", "WPEXPORT_NO_IDS"); err != nil {
		return nil, fmt.Errorf("failed to bind no_ids environment variable: %w", err)
	}
	if err := viper.BindEnv("exclude_tags", "WPEXPORT_EXCLUDE_TAGS"); err != nil {
		return nil, fmt.Errorf("failed to bind exclude_tags environment variable: %w", err)
	}
	if err := viper.BindEnv("exclude_media_types", "WPEXPORT_EXCLUDE_MEDIA_TYPES"); err != nil {
		return nil, fmt.Errorf("failed to bind exclude_media_types environment variable: %w", err)
	}
	if err := viper.BindEnv("preserve_classes", "WPEXPORT_PRESERVE_CLASSES"); err != nil {
		return nil, fmt.Errorf("failed to bind preserve_classes environment variable: %w", err)
	}
	if err := viper.BindEnv("preserve_ids", "WPEXPORT_PRESERVE_IDS"); err != nil {
		return nil, fmt.Errorf("failed to bind preserve_ids environment variable: %w", err)
	}
	if err := viper.BindEnv("cache", "WPEXPORT_CACHE"); err != nil {
		return nil, fmt.Errorf("failed to bind cache environment variable: %w", err)
	}
	if err := viper.BindEnv("cache_ttl", "WPEXPORT_CACHE_TTL"); err != nil {
		return nil, fmt.Errorf("failed to bind cache_ttl environment variable: %w", err)
	}
	if err := viper.BindEnv("cache_dir", "WPEXPORT_CACHE_DIR"); err != nil {
		return nil, fmt.Errorf("failed to bind cache_dir environment variable: %w", err)
	}
	if err := viper.BindEnv("cache_clear", "WPEXPORT_CACHE_CLEAR"); err != nil {
		return nil, fmt.Errorf("failed to bind cache_clear environment variable: %w", err)
	}

	// Load config file if specified
	if configFile != "" {
		viper.SetConfigFile(configFile)
	}

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal config
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("URL is required")
	}

	validFormats := map[string]bool{
		"json": true, "markdown": true, "ssg": true, "shopify": true, "magento": true,
		"wordpress": true, "drupal": true, "wix": true, "squarespace": true,
		"webflow": true, "weebly": true, "prestashop": true, "ghost": true,
		"strapi": true, "contentful": true,
	}
	if !validFormats[c.Format] {
		return fmt.Errorf("format must be one of: json, markdown, ssg, shopify, magento, wordpress, drupal, " +
			"wix, squarespace, webflow, weebly, prestashop, ghost, strapi, contentful")
	}

	if c.MaxID <= 0 {
		return fmt.Errorf("max_id must be greater than 0")
	}

	if c.Concurrent <= 0 {
		return fmt.Errorf("concurrent must be greater than 0")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	if c.Retries < 0 {
		return fmt.Errorf("retries must be greater than or equal to 0")
	}

	if _, _, _, err := ParseScanRange(c.ScanRange); err != nil {
		return err
	}

	if c.MediaPathStyle != "" && c.MediaPathStyle != "root" && c.MediaPathStyle != "relative" {
		return fmt.Errorf("media_path_style must be one of: root, relative")
	}

	if c.LinkStyle != "" && c.LinkStyle != "absolute" && c.LinkStyle != "root" {
		return fmt.Errorf("link_style must be one of: absolute, root")
	}

	return nil
}

// LocalizesURLs reports whether this export rewrites URLs for local consumption.
//
// Only the formats consumed as files do: the platform importers (shopify,
// drupal, …) pull media from the live site, so their URLs must stay original.
func (c *Config) LocalizesURLs() bool {
	if c.KeepOriginalURLs {
		return false
	}

	return c.Format == "json" || c.Format == "markdown" || c.Format == "ssg"
}

// EffectiveLinkStyle resolves the form of address fields, applying the format's
// default when none was configured.
//
// The ssg format defaults to "root": its whole purpose is rebuilding the site at
// the same paths on a new host, which is exactly the case where root-relative
// addresses preserve each URL and its search ranking. Every other format keeps
// the source URL, which a consumer needs to derive the target one.
func (c *Config) EffectiveLinkStyle() string {
	if c.LinkStyle != "" {
		return c.LinkStyle
	}

	if c.Format == "ssg" {
		return "root"
	}

	return "absolute"
}

// IsSameHost reports whether rawURL targets the same host as the configured
// WordPress URL. It is used to decide whether authentication credentials may be
// attached to a request: credentials must never be sent to a foreign host (e.g. a
// CDN serving media, or an attacker-controlled URL returned by a compromised site).
// A relative URL (no host) is treated as same-host.
func (c *Config) IsSameHost(rawURL string) bool {
	base, err := url.Parse(c.URL)
	if err != nil {
		return false
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if target.Host == "" {
		return true // relative URL resolves against the base host
	}
	return strings.EqualFold(base.Hostname(), target.Hostname())
}

// EnsureOutputDir ensures the output directory exists
func (c *Config) EnsureOutputDir() error {
	if c.Format == "json" && filepath.Ext(c.Output) == ".json" {
		// If output is a JSON file, ensure parent directory exists
		dir := filepath.Dir(c.Output)
		return os.MkdirAll(dir, 0750)
	}

	// Otherwise, ensure output directory exists
	return os.MkdirAll(c.Output, 0750)
}

// GenerateDefaultOutput generates the default output path based on URL and current date
func (c *Config) GenerateDefaultOutput() error {
	if c.Output != "" {
		return nil // Output already specified
	}

	if c.URL == "" {
		return fmt.Errorf("URL is required to generate default output path")
	}

	// Parse URL to extract domain
	parsedURL, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("invalid URL for generating output path: %w", err)
	}

	// Extract domain name and clean it
	domain := parsedURL.Hostname()
	if domain == "" {
		domain = "wordpress-site"
	}

	// Remove www. prefix if present
	domain = strings.TrimPrefix(domain, "www.")

	// Sanitize domain name for filesystem
	domain = sanitizeDomainName(domain)

	// Generate date and time string
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("150405") // HHMMSS format

	// Create default output path: export/{domain-name}.{date}{time}
	c.Output = filepath.Join("export", fmt.Sprintf("%s.%s%s", domain, dateStr, timeStr))

	return nil
}

// sanitizeDomainName removes invalid characters from domain name for filesystem use
func sanitizeDomainName(domain string) string {
	// Replace invalid characters with hyphens
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	sanitized := domain

	for _, char := range invalid {
		sanitized = strings.ReplaceAll(sanitized, char, "-")
	}

	// Remove multiple consecutive hyphens
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	// Handle special case: remove hyphens before domain extension
	if dotIndex := strings.LastIndex(sanitized, "."); dotIndex > 0 {
		domain := sanitized[:dotIndex]
		extension := sanitized[dotIndex:]
		domain = strings.Trim(domain, "-")
		sanitized = domain + extension
	} else {
		// Trim hyphens from start and end if no extension
		sanitized = strings.Trim(sanitized, "-")
	}

	// Final trim to ensure no leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "wordpress-site"
	}

	return sanitized
}

// GetMediaDir returns the media download directory (always absolute path)
func (c *Config) GetMediaDir() string {
	var mediaDir string

	if c.Format == "json" && filepath.Ext(c.Output) == ".json" {
		// If output is a JSON file, create media directory next to it
		dir := filepath.Dir(c.Output)
		base := filepath.Base(c.Output)
		name := base[:len(base)-len(filepath.Ext(base))]
		mediaDir = filepath.Join(dir, name+"_media")
	} else {
		// Otherwise, create media directory inside output directory
		mediaDir = filepath.Join(c.Output, "media")
	}

	// Ensure the path is absolute
	if !filepath.IsAbs(mediaDir) {
		absPath, err := filepath.Abs(mediaDir)
		if err == nil {
			return absPath
		}
	}

	return mediaDir
}
