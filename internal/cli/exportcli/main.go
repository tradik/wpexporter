package exportcli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/cache"
	"github.com/tradik/wpexporter/internal/cli"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/internal/seo"
	"github.com/tradik/wpexporter/internal/version"
	"github.com/tradik/wpexporter/pkg/models"
)

// quietMode is set when --quiet flag is used to suppress output
var quietMode bool

// logf prints a formatted message unless quiet mode is enabled
func logf(format string, args ...interface{}) {
	if !quietMode {
		fmt.Printf(format, args...)
	}
}

// logln prints a line unless quiet mode is enabled
func logln(args ...interface{}) {
	if !quietMode {
		fmt.Println(args...)
	}
}

var (
	cfgFile           string
	url               string
	output            string
	format            string
	bruteForce        bool
	maxID             int
	scanRange         string
	maxMediaMB        int
	downloadMedia     bool
	noMedia           bool
	relevantMediaOnly bool
	concurrent        int
	verbose           bool
	createZip         bool
	noFiles           bool
	noPosts           bool
	noPages           bool
	noProducts        bool
	noCustomTypes     bool
	customTypes       string
	// skipUnaddressableTypes is opt-in: it drops content, and the shape it keys
	// on is normal for a whole site left on plain permalinks (#78).
	skipUnaddressableTypes bool
	noUsers                bool
	pathFilter             string
	assistedCrawl          bool
	authUser               string
	authPass               string
	authToken              string
	rateLimit              int
	retries                int
	userAgent              string
	limit                  int
	limitPerType           string
	limitPosts             int
	limitPages             int
	limitMedia             int
	limitProducts          int
	resume                 bool
	timeout                int
	crawlContent           bool
	skipEmptyContent       bool
	flatHTML               bool
	basicHTML              bool
	ssgSections            bool
	keepOriginalURLs       bool
	mediaPathStyle         string
	linkStyle              string
	frontmatterStyle       string
	reportA11y             bool
	extractMeta            string
	noTags                 bool
	noMenus                bool
	noComments             bool
	noInventoryCheck       bool
	fromSitemap            bool
	quiet                  bool
	noIDs                  bool
	excludeTags            string
	excludeMediaTypes      string
	preserveClasses        string
	preserveIDs            string
	preserveStyling        string
	boilerplateClasses     string
	builderClasses         string
	crawlContentMode       string
	contentSelectors       string
	postLoopMarkers        string
	maxSitemapDocuments    int
	readMorePhrases        string
	cacheEnabled           bool
	cacheTTL               string
	cacheDir               string
	cacheClear             bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "wpexportjson",
	Short:   "WordPress content export tool",
	Version: version.String(),
	Long: `WordPress content export tool

Usage: wpexportjson export --url <site-url> [flags]

Export Flags:
  -u, --url string            WordPress site URL (required)
  -o, --output string         Output directory (default: export/{domain}.{date})
  -f, --format string         Format: json|markdown|shopify|magento|wordpress|drupal|
                              wix|squarespace|webflow|weebly|prestashop|ghost|strapi|contentful
  -c, --concurrent int        Concurrent downloads (default 5)
      --timeout int           HTTP timeout in seconds (default 30)
  -q, --quiet                 Suppress all output, only return exit code

Authentication:
      --auth-user string      Username for Basic Auth
      --auth-pass string      Password for Basic Auth
      --auth-token string     Bearer token

Content Filters:
      --no-posts              Skip blog posts
      --no-pages              Skip pages
      --no-products           Skip WooCommerce products
      --no-custom-types       Skip theme/plugin post types (Services, Portfolio, …)
      --custom-types string   Export only these custom types (comma-separated slugs)
      --skip-unaddressable-types
                              Drop custom types whose entries have no address of their own
      --no-users              Skip users
      --no-tags               Skip tags
      --no-menus              Skip navigation menus
      --no-comments           Skip reader comments
      --no-inventory-check    Skip the sitemap/feed completeness check
      --from-sitemap          Recover posts from the feed when the REST API serves none
      --no-media              Skip media downloads
      --path-filter string    Filter by URL path (e.g., /fr/art/)
      --skip-empty-content    Skip posts/pages with empty content
      --flat-html             Convert HTML to Markdown (Bricks Builder support)
      --basic-html            Clean HTML to basic elements (tables, lists - for Shopify)
      --keep-original-urls    Preserve WordPress URLs (don't convert to local paths)
      --media-path-style      Rewritten media paths: root (default, /media/...) or relative
      --link-style            link/canonical_url/hreflangs: absolute (default) or root
      --frontmatter-style     structured values: nested (default) or flat (JSON strings)
      --report-a11y           Write a11y-report.md (WCAG 2.2 contrast + missing alt text)
      --preserve-styling      What a conversion keeps when it cannot express a class:
                              auto (default) | none | all
      --boilerplate-classes   Classes this theme stamps on everything (comma-separated)
      --builder-classes       Class prefixes marking this site's page-builder markup
      --crawl-content-mode    Which pages --crawl-content re-reads: auto|empty|always
      --content-selector      Where this theme keeps the page: tag, .class, #id
      --post-loop-markers     This theme's listing elements (shortcode/block/class names)
      --max-sitemap-documents Read at most N sitemap-index children (0 = all)
      --read-more-phrases     The read-more text this theme appends, if it cannot be learned

Reaching the site:
      --origin URL            Send the site's own requests to this web server
                              (scheme://host[:port]) instead of via DNS/Cloudflare
      --origin-insecure       Skip certificate checks on --origin connections only

Advanced:
      --brute-force           Enable brute force ID discovery
      --max-id int            Max ID for brute force (default 10000)
      --scan-range START-END  Rescan a specific inclusive ID range (e.g. 100-200)
      --max-media-mb int      Per-file media download cap in MB (0 = default 2048)
      --assisted-crawl        Crawl URLs for SEO metadata
      --crawl-content         Take the rendered page where the stored body is not the page
      --relevant-media-only   Download only featured/content images
      --resume                Resume from checkpoint
      --rate-limit int        Delay between requests in ms
      --retries int           Retries for a 5xx, 429 or dropped connection (default 3)
      --user-agent string     Identify as something else (bot protection matches the default)
      --limit int             Export at most N documents in total, newest first
      --limit-per-type spec   At most N of each kind, or kind=N pairs: 5,media=10
      --limit-posts int       At most N posts (same for --limit-pages/-media/-products)
      --zip                   Create ZIP archive
      --no-files              Remove files after ZIP (requires --zip)

Docs: https://github.com/tradik/wpexporter`,
}

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export WordPress content",
	Long: `Export all content from a WordPress site including posts, pages, 
media, categories, tags, and users. Supports brute force discovery 
and multiple export formats.`,
	RunE: runExport,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.wpexportjson/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Export command flags
	exportCmd.Flags().StringVarP(&url, "url", "u", "", "WordPress site URL (required)")
	exportCmd.Flags().StringVarP(&output, "output", "o", "", "output directory or file (default: export/{domain-name}.{date}{time})")
	exportCmd.Flags().StringVarP(&format, "format", "f", "json", "export format (json|markdown|shopify|magento)")
	exportCmd.Flags().BoolVar(&bruteForce, "brute-force", false, "enable brute force ID discovery")
	exportCmd.Flags().IntVar(&maxID, "max-id", 10000, "maximum ID for brute force")
	exportCmd.Flags().StringVar(&scanRange, "scan-range", "",
		"rescan a specific inclusive ID range for posts/pages/media, e.g. --scan-range 100-200")
	exportCmd.Flags().IntVar(&maxMediaMB, "max-media-mb", 0,
		"per-file media download cap in MB (0 = built-in default of 2048)")
	exportCmd.Flags().BoolVar(&downloadMedia, "download-media", true, "download images and videos")
	exportCmd.Flags().BoolVar(&noMedia, "no-media", false, "disable media downloads (alias for --download-media=false)")
	exportCmd.Flags().BoolVar(&relevantMediaOnly, "relevant-media-only", false, "only download featured images and images embedded in content")
	exportCmd.Flags().StringVar(&excludeMediaTypes, "exclude-media-types", "",
		"media types to skip (comma-separated: images,videos,audio,documents,archives)")
	exportCmd.Flags().IntVarP(&concurrent, "concurrent", "c", 5, "concurrent downloads")
	exportCmd.Flags().BoolVar(&createZip, "zip", false, "create ZIP archive of export")
	exportCmd.Flags().BoolVar(&noFiles, "no-files", false, "remove export files after creating ZIP (requires --zip)")
	exportCmd.Flags().BoolVar(&noPosts, "no-posts", false, "skip exporting blog posts")
	exportCmd.Flags().BoolVar(&noPages, "no-pages", false, "skip exporting pages")
	exportCmd.Flags().BoolVar(&noProducts, "no-products", false, "skip exporting WooCommerce products")
	exportCmd.Flags().BoolVar(&noCustomTypes, "no-custom-types", false,
		"skip the custom post types a theme or plugin registered (Services, Portfolio, …)")
	exportCmd.Flags().StringVar(&customTypes, "custom-types", "",
		"export only these custom post types (comma-separated slugs, e.g. cpt_services,cpt_portfolio)")
	exportCmd.Flags().BoolVar(&skipUnaddressableTypes, "skip-unaddressable-types", false,
		"drop custom post types whose entries have no address of their own (a plugin's data store, "+
			"published at /?type=123). Off by default: a site on plain permalinks publishes everything that way")
	exportCmd.Flags().BoolVar(&noUsers, "no-users", false, "skip exporting users")
	exportCmd.Flags().StringVar(&authUser, "auth-user", "", "username for Basic Auth")
	exportCmd.Flags().StringVar(&authPass, "auth-pass", "", "password for Basic Auth")
	exportCmd.Flags().StringVar(&authToken, "auth-token", "", "Bearer token for authentication")
	exportCmd.Flags().IntVar(&rateLimit, "rate-limit", 0, "delay between API requests in milliseconds (0 = no limit)")
	exportCmd.Flags().IntVar(&limit, "limit", 0,
		"export at most N documents in total, newest first (0 = no limit); the walk stops when the "+
			"budget is spent, so a preview of a site does not download the site")
	exportCmd.Flags().StringVar(&limitPerType, "limit-per-type", "",
		"at most N of each kind, or kind=N pairs — \"5\", \"posts=5,media=10\" or \"5,media=10\"; "+
			"a kind is a collection name or a custom type's slug")
	exportCmd.Flags().IntVar(&limitPosts, "limit-posts", 0, "export at most N posts (0 = no limit)")
	exportCmd.Flags().IntVar(&limitPages, "limit-pages", 0, "export at most N pages (0 = no limit)")
	exportCmd.Flags().IntVar(&limitMedia, "limit-media", 0, "export at most N media items (0 = no limit)")
	exportCmd.Flags().IntVar(&limitProducts, "limit-products", 0, "export at most N products (0 = no limit)")
	exportCmd.Flags().StringVar(&userAgent, "user-agent", "",
		"the User-Agent to send; bot protection matches on the default, and a browser's string "+
			"is the remedy that most often works against a 403 from a wall")
	exportCmd.Flags().IntVar(&retries, "retries", 3,
		"attempts for a request the site answers with 5xx or 429, or drops (0 = no retry)")
	exportCmd.Flags().BoolVar(&resume, "resume", false, "resume from checkpoint if previous export was interrupted")
	exportCmd.Flags().IntVar(&timeout, "timeout", 30, "HTTP request timeout in seconds")
	exportCmd.Flags().StringVar(&pathFilter, "path-filter", "", "filter posts/pages by URL path pattern (e.g., /fr/arts/)")
	exportCmd.Flags().BoolVar(&assistedCrawl, "assisted-crawl", false, "crawl URLs for SEO metadata")
	exportCmd.Flags().StringVar(&excludeTags, "exclude-tags", "", "SEO tags to exclude (comma-separated: title,meta:description,og:title)")
	exportCmd.Flags().BoolVar(&crawlContent, "crawl-content", false, "take the rendered page where the stored body is empty or is page-builder markup")
	exportCmd.Flags().BoolVar(&skipEmptyContent, "skip-empty-content", false, "skip posts/pages with empty content")
	exportCmd.Flags().BoolVar(&flatHTML, "flat-html", false, "convert HTML to Markdown (Bricks Builder support)")
	exportCmd.Flags().BoolVar(&basicHTML, "basic-html", false, "clean HTML to basic elements (tables, lists, links - for Shopify)")
	exportCmd.Flags().BoolVar(&ssgSections, "ssg-sections", false,
		"markdown: emit ## Excerpt/## Content sections and omit the duplicate body H1 (for ssg)")
	exportCmd.Flags().StringVar(&preserveClasses, "preserve-classes", "",
		"CSS classes whose elements travel as HTML (comma-separated, wildcards allowed: trx_addons_inline_*)")
	exportCmd.Flags().StringVar(&preserveIDs, "preserve-ids", "",
		"element IDs whose elements travel as HTML (comma-separated, wildcards allowed)")
	exportCmd.Flags().StringVar(&preserveStyling, "preserve-styling", export.StylingAuto,
		"what a conversion holds on to when it cannot express a class: auto|none|all")
	exportCmd.Flags().StringVar(&boilerplateClasses, "boilerplate-classes", "",
		"classes this theme stamps on everything and that mean nothing (comma-separated, wildcards)")
	exportCmd.Flags().StringVar(&builderClasses, "builder-classes", "",
		"class prefixes marking this site's page-builder markup (comma-separated, wildcards)")
	exportCmd.Flags().StringVar(&crawlContentMode, "crawl-content-mode", seo.CrawlAuto,
		"which pages --crawl-content re-reads: auto|empty|always")
	exportCmd.Flags().StringVar(&contentSelectors, "content-selector", "",
		"where this theme keeps the page: tag, .class, #id or tag.class (comma-separated, tried first)")
	exportCmd.Flags().IntVar(&maxSitemapDocuments, "max-sitemap-documents", 0,
		"read at most N child documents of a sitemap index (0 = all; the run names what it skipped)")
	exportCmd.Flags().StringVar(&postLoopMarkers, "post-loop-markers", "",
		"this theme's listing elements — shortcode, block or class names (comma-separated)")
	exportCmd.Flags().StringVar(&readMorePhrases, "read-more-phrases", "",
		"the read-more text this theme appends, when too few excerpts exist to learn it (comma-separated)")
	exportCmd.Flags().BoolVar(&keepOriginalURLs, "keep-original-urls", false,
		"preserve original WordPress URLs in content (don't convert to local paths)")
	exportCmd.Flags().StringVar(&mediaPathStyle, "media-path-style", "root",
		"form of rewritten media paths: root (/media/...) or relative (media/...)")
	exportCmd.Flags().StringVar(&linkStyle, "link-style", "absolute",
		"form of link/canonical_url/hreflangs: absolute (source URL) or root (root-relative path); ssg defaults to root")
	exportCmd.Flags().StringVar(&frontmatterStyle, "frontmatter-style", "nested",
		"form of the structured frontmatter values (meta, hreflangs): nested (YAML structure) or "+
			"flat (one JSON string each, so they survive a store that holds only string lists)")
	exportCmd.Flags().BoolVar(&reportA11y, "report-a11y", false,
		"write a11y-report.md flagging WCAG 2.2 contrast and missing alt-text issues")
	exportCmd.Flags().StringVar(&extractMeta, "extract-meta", "all",
		"which meta tags to keep beyond the named SEO fields: all, none, or a comma-separated allow-list")
	exportCmd.Flags().BoolVar(&noTags, "no-tags", false, "skip exporting tags")
	exportCmd.Flags().BoolVar(&noMenus, "no-menus", false, "skip exporting navigation menus")
	exportCmd.Flags().BoolVar(&noComments, "no-comments", false, "skip exporting reader comments")
	exportCmd.Flags().BoolVar(&noInventoryCheck, "no-inventory-check", false,
		"skip reading the site's sitemap and feed to report what the export did not cover")
	exportCmd.Flags().BoolVar(&fromSitemap, "from-sitemap", false,
		"when the REST API serves no posts, recover what the site's feed still publishes")
	exportCmd.Flags().BoolVar(&noIDs, "no-ids", false, "exclude numeric IDs from frontmatter (keep only names)")
	exportCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress all output, only return exit code")

	// Cache flags
	exportCmd.Flags().BoolVar(&cacheEnabled, "cache", false, "enable caching of API responses and crawl data")
	exportCmd.Flags().StringVar(&cacheTTL, "cache-ttl", "24h", "cache TTL (e.g., 24h, 1h, 0 for unlimited)")
	exportCmd.Flags().StringVar(&cacheDir, "cache-dir", "", "cache directory (default: ~/.wpexporter/cache)")
	exportCmd.Flags().BoolVar(&cacheClear, "cache-clear", false, "clear cache before export")

	// Mark required flags
	if err := exportCmd.MarkFlagRequired("url"); err != nil {
		panic(fmt.Sprintf("Failed to mark url flag as required: %v", err))
	}

	rootCmd.AddCommand(exportCmd)
}

// getDirSize calculates the total size of a directory
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// getFileSize returns the size of a file
func getFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// formatFileSize formats bytes into human-readable size
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// promptPassword prompts the user to enter a password securely (hidden input)
func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(syscall.Stdin)) //nolint:unconvert // Required for Windows compatibility (syscall.Stdin is uintptr on Windows)
	fmt.Println()                                          // Print newline after password input
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return string(password), nil
}

// configFileExists checks if a configuration file exists in standard locations
func configFileExists() bool {
	return cli.ConfigFileExists("wpexportjson")
}

// applyFlagOverrides applies command line flag values to the configuration
func applyFlagOverrides(cmd *cobra.Command, cfg *config.Config) error {
	if cmd.Flags().Changed("url") {
		cfg.URL = url
	}
	if cmd.Flags().Changed("output") {
		cfg.Output = output
	}
	if cmd.Flags().Changed("format") {
		cfg.Format = format
	}
	if cmd.Flags().Changed("brute-force") {
		cfg.BruteForce = bruteForce
	}
	if cmd.Flags().Changed("max-id") {
		cfg.MaxID = maxID
	}
	if cmd.Flags().Changed("download-media") {
		cfg.DownloadMedia = downloadMedia
	}
	if cmd.Flags().Changed("no-media") && noMedia {
		cfg.DownloadMedia = false
	}
	if cmd.Flags().Changed("relevant-media-only") {
		cfg.RelevantMediaOnly = relevantMediaOnly
	}
	if cmd.Flags().Changed("concurrent") {
		cfg.Concurrent = concurrent
	}
	if cmd.Flags().Changed("verbose") {
		cfg.Verbose = verbose
	}
	if cmd.Flags().Changed("zip") {
		cfg.CreateZip = createZip
	}
	if cmd.Flags().Changed("no-files") {
		cfg.NoFiles = noFiles
	}
	if cmd.Flags().Changed("no-posts") {
		cfg.NoPosts = noPosts
	}
	if cmd.Flags().Changed("no-pages") {
		cfg.NoPages = noPages
	}
	if cmd.Flags().Changed("no-products") {
		cfg.NoProducts = noProducts
	}
	if cmd.Flags().Changed("skip-unaddressable-types") {
		cfg.SkipUnaddressableTypes = skipUnaddressableTypes
	}

	if cmd.Flags().Changed("no-custom-types") {
		cfg.NoCustomTypes = noCustomTypes
	}
	if cmd.Flags().Changed("custom-types") {
		cfg.CustomTypes = splitCommaList(customTypes)
	}
	if cmd.Flags().Changed("no-users") {
		cfg.NoUsers = noUsers
	}
	if cmd.Flags().Changed("auth-user") {
		cfg.AuthUser = authUser
	}
	if cmd.Flags().Changed("auth-pass") {
		cfg.AuthPass = authPass
	}
	if cmd.Flags().Changed("auth-token") {
		cfg.AuthToken = authToken
	}

	// Prompt for password if auth-user is provided but auth-pass is not
	if cfg.AuthUser != "" && cfg.AuthPass == "" && cfg.AuthToken == "" {
		password, err := promptPassword("Enter password for " + cfg.AuthUser + ": ")
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		cfg.AuthPass = password
	}

	if cmd.Flags().Changed("path-filter") {
		if strings.HasPrefix(pathFilter, "-") {
			return fmt.Errorf("invalid --path-filter value '%s': looks like a flag", pathFilter)
		}
		cfg.PathFilter = pathFilter
	}
	if cmd.Flags().Changed("scan-range") {
		cfg.ScanRange = scanRange
	}
	if cmd.Flags().Changed("max-media-mb") {
		cfg.MaxMediaBytes = int64(maxMediaMB) << 20
	}
	if cmd.Flags().Changed("assisted-crawl") {
		cfg.AssistedCrawl = assistedCrawl
	}
	if cmd.Flags().Changed("exclude-tags") && excludeTags != "" {
		cfg.ExcludeTags = strings.Split(excludeTags, ",")
		for i := range cfg.ExcludeTags {
			cfg.ExcludeTags[i] = strings.TrimSpace(cfg.ExcludeTags[i])
		}
	}
	if cmd.Flags().Changed("exclude-media-types") && excludeMediaTypes != "" {
		cfg.ExcludeMediaTypes = strings.Split(excludeMediaTypes, ",")
		for i := range cfg.ExcludeMediaTypes {
			cfg.ExcludeMediaTypes[i] = strings.TrimSpace(cfg.ExcludeMediaTypes[i])
		}
	}
	if cmd.Flags().Changed("preserve-styling") {
		cfg.PreserveStyling = preserveStyling
	}
	if err := checkMode("preserve-styling", cfg.PreserveStyling, export.StylingModes); err != nil {
		return err
	}
	if err := checkMode("crawl-content-mode", cfg.CrawlContentMode, seo.CrawlModes); err != nil {
		return err
	}
	if cmd.Flags().Changed("boilerplate-classes") {
		cfg.BoilerplateClasses = splitList(boilerplateClasses)
	}
	if cmd.Flags().Changed("builder-classes") {
		cfg.BuilderClasses = splitList(builderClasses)
	}
	if cmd.Flags().Changed("crawl-content-mode") {
		cfg.CrawlContentMode = crawlContentMode
	}
	if cmd.Flags().Changed("content-selector") {
		cfg.ContentSelectors = splitList(contentSelectors)
	}
	if cmd.Flags().Changed("max-sitemap-documents") {
		cfg.MaxSitemapDocuments = maxSitemapDocuments
	}
	if cmd.Flags().Changed("post-loop-markers") {
		cfg.PostLoopMarkers = splitList(postLoopMarkers)
	}
	if cmd.Flags().Changed("read-more-phrases") {
		cfg.ReadMorePhrases = splitList(readMorePhrases)
	}
	if cmd.Flags().Changed("preserve-classes") && preserveClasses != "" {
		cfg.PreserveClasses = strings.Split(preserveClasses, ",")
		for i := range cfg.PreserveClasses {
			cfg.PreserveClasses[i] = strings.TrimSpace(cfg.PreserveClasses[i])
		}
	}
	if cmd.Flags().Changed("preserve-ids") && preserveIDs != "" {
		cfg.PreserveIDs = strings.Split(preserveIDs, ",")
		for i := range cfg.PreserveIDs {
			cfg.PreserveIDs[i] = strings.TrimSpace(cfg.PreserveIDs[i])
		}
	}
	if cmd.Flags().Changed("rate-limit") {
		cfg.RateLimit = rateLimit
	}
	if cmd.Flags().Changed("retries") {
		cfg.Retries = retries
	}
	if cmd.Flags().Changed("user-agent") && userAgent != "" {
		cfg.UserAgent = userAgent
	}
	if cmd.Flags().Changed("limit") {
		cfg.Limit = limit
	}
	shortcuts := map[string]int{}
	for name, value := range map[string]*int{
		"posts": &limitPosts, "pages": &limitPages,
		"media": &limitMedia, "products": &limitProducts,
	} {
		if cmd.Flags().Changed("limit-" + name) {
			shortcuts[name] = *value
		}
	}

	if cmd.Flags().Changed("limit-per-type") || len(shortcuts) > 0 {
		conflicts, err := applyPerTypeLimits(cfg, limitPerType, shortcuts)
		if err != nil {
			return err
		}

		for _, conflict := range conflicts {
			logf("Note: %s\n", conflict)
		}
	}
	if cmd.Flags().Changed("resume") {
		cfg.Resume = resume
	}
	if cmd.Flags().Changed("timeout") {
		cfg.Timeout = timeout
	}
	if cmd.Flags().Changed("crawl-content") {
		cfg.CrawlContent = crawlContent
	}
	if cmd.Flags().Changed("skip-empty-content") {
		cfg.SkipEmptyContent = skipEmptyContent
	}
	if cmd.Flags().Changed("flat-html") {
		cfg.FlatHTML = flatHTML
	}
	if cmd.Flags().Changed("basic-html") {
		cfg.BasicHTML = basicHTML
	}
	if cmd.Flags().Changed("ssg-sections") {
		cfg.SSGSections = ssgSections
	}
	if cmd.Flags().Changed("keep-original-urls") {
		cfg.KeepOriginalURLs = keepOriginalURLs
	}
	if cmd.Flags().Changed("media-path-style") {
		cfg.MediaPathStyle = mediaPathStyle
	}
	if cmd.Flags().Changed("link-style") {
		cfg.LinkStyle = linkStyle
	}
	if cmd.Flags().Changed("frontmatter-style") {
		cfg.FrontmatterStyle = frontmatterStyle
	}
	if cmd.Flags().Changed("report-a11y") {
		cfg.ReportA11y = reportA11y
	}
	if cmd.Flags().Changed("extract-meta") {
		cfg.ExtractMeta = extractMeta
	}
	if cmd.Flags().Changed("no-tags") {
		cfg.NoTags = noTags
	}
	if cmd.Flags().Changed("no-menus") {
		cfg.NoMenus = noMenus
	}
	if cmd.Flags().Changed("no-comments") {
		cfg.NoComments = noComments
	}
	if cmd.Flags().Changed("no-inventory-check") {
		cfg.NoInventoryCheck = noInventoryCheck
	}
	if cmd.Flags().Changed("from-sitemap") {
		cfg.FromSitemap = fromSitemap
	}
	if cmd.Flags().Changed("no-ids") {
		cfg.NoIDs = noIDs
	}
	if cmd.Flags().Changed("quiet") || cmd.Flags().Changed("q") {
		cfg.Quiet = quiet
	}
	if cmd.Flags().Changed("cache") {
		cfg.Cache = cacheEnabled
	}
	if cmd.Flags().Changed("cache-ttl") {
		cfg.CacheTTL = cacheTTL
	}
	if cmd.Flags().Changed("cache-dir") {
		cfg.CacheDir = cacheDir
	}
	if cmd.Flags().Changed("cache-clear") {
		cfg.CacheClear = cacheClear
	}

	applySourceFlagOverrides(cmd, cfg)

	return nil
}

// initializeCache creates and configures the file cache if enabled
func initializeCache(cfg *config.Config, apiClient *api.Client) (*cache.FileCache, error) {
	if !cfg.Cache {
		return nil, nil
	}

	// Determine cache directory
	cacheDirectory := cfg.CacheDir
	if cacheDirectory == "" {
		cacheDirectory = cache.DefaultCacheDir()
	}

	// Parse TTL
	ttl, err := cache.ParseTTL(cfg.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid cache TTL: %w", err)
	}

	// Create cache
	fileCache, err := cache.NewFileCache(cacheDirectory, ttl, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Clear cache if requested
	if cfg.CacheClear {
		if err := fileCache.Clear(); err != nil {
			_ = fileCache.Close()
			return nil, fmt.Errorf("failed to clear cache: %w", err)
		}
		logln("Cache cleared")
	}

	// Set cache on API client
	apiClient.SetCache(fileCache)

	logf("Cache enabled (TTL: %s, dir: %s)\n", cfg.CacheTTL, fileCache.GetCacheDir())

	return fileCache, nil
}

// postIDSet returns the set of post/page IDs present in posts.
func postIDSet(posts []models.WordPressPost) map[int]bool {
	seen := make(map[int]bool, len(posts))
	for _, p := range posts {
		seen[p.ID] = true
	}
	return seen
}

// scanPostRange rescans an ID range for a post-like content type ("posts"/"pages").
func scanPostRange(sc contentScanner, kind string, start, end int) []models.WordPressPost {
	res, err := sc.ScanSpecificRange(kind, start, end)
	if err != nil {
		return nil
	}
	items, _ := res.([]models.WordPressPost)
	return items
}

// appendScanRange rescans the configured inclusive ID range for posts, pages and
// media, appending items not already present (deduplicated by ID). Returns the
// number of newly-added items.
func appendScanRange(sc contentScanner, cfg *config.Config,
	posts, pages *[]models.WordPressPost, media *[]models.WordPressMedia) (int, error) {
	start, end, ok, err := config.ParseScanRange(cfg.ScanRange)
	if err != nil || !ok {
		return 0, err
	}

	found := 0

	postSeen := postIDSet(*posts)
	for _, p := range scanPostRange(sc, "posts", start, end) {
		if !postSeen[p.ID] {
			postSeen[p.ID] = true
			*posts = append(*posts, p)
			found++
		}
	}

	pageSeen := postIDSet(*pages)
	for _, p := range scanPostRange(sc, "pages", start, end) {
		if !pageSeen[p.ID] {
			pageSeen[p.ID] = true
			*pages = append(*pages, p)
			found++
		}
	}

	mediaSeen := make(map[int]bool, len(*media))
	for _, m := range *media {
		mediaSeen[m.ID] = true
	}
	if res, err := sc.ScanSpecificRange("media", start, end); err == nil {
		if items, ok := res.([]models.WordPressMedia); ok {
			for _, m := range items {
				if !mediaSeen[m.ID] {
					mediaSeen[m.ID] = true
					*media = append(*media, m)
					found++
				}
			}
		}
	}

	return found, nil
}

// homePageURL returns the address to read site-level marketing metadata from:
// the site's declared home URL, falling back to the configured URL.
func homePageURL(site *models.SiteInfo, cfg *config.Config) string {
	if site != nil {
		if site.HomeURL != "" {
			return site.HomeURL
		}
		if site.URL != "" {
			return site.URL
		}
	}

	return cfg.URL
}

// snapPrivateTmpError reports that a snap-confined run cannot usefully write to
// /tmp, or nil when the combination does not apply.
//
// A strictly confined snap gets a private mount namespace for /tmp, so an export
// written there lands in /tmp/snap-private-tmp/snap.<name>/tmp/... — root-owned and
// invisible from the user's shell. Without this check the export reports success
// and the output simply is not where it says it is (issue #19).
func snapPrivateTmpError(outputPath string) error {
	snapName := os.Getenv("SNAP_NAME")
	if os.Getenv("SNAP") == "" && snapName == "" {
		return nil // not running inside a snap
	}

	abs, err := filepath.Abs(outputPath)
	if err != nil {
		return nil // unresolvable path is the permission check's problem, not ours
	}
	if abs != "/tmp" && !strings.HasPrefix(abs, "/tmp/") {
		return nil
	}

	if snapName == "" {
		snapName = "wpexporter"
	}

	return fmt.Errorf(
		"output path %q is unusable from a snap: /tmp is private to the snap, so the export would land in "+
			"/tmp/snap-private-tmp/snap.%s/tmp and be invisible outside it.\n"+
			"Export somewhere under your home directory instead, for example -o ~/export", abs, snapName)
}

// checkOutputPermissions verifies we can write to the output directory
func checkOutputPermissions(outputPath string) error {
	// Get the parent directory
	parentDir := filepath.Dir(outputPath)

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return fmt.Errorf("cannot create output directory '%s': %w", parentDir, err)
	}

	// Try to create a temporary file to verify write permissions
	testFile := filepath.Join(parentDir, ".wpexporter_permission_test")
	f, err := os.Create(testFile) // #nosec G304 -- testFile is constructed from validated config output
	if err != nil {
		return fmt.Errorf("no write permission for output directory '%s': %w", parentDir, err)
	}
	_ = f.Close()
	_ = os.Remove(testFile)

	return nil
}

// createZipArchive creates a ZIP archive of the specified directory
func createZipArchive(sourceDir, targetZip string) error {
	// Validate and clean paths to prevent directory traversal
	cleanSourceDir := filepath.Clean(sourceDir)
	cleanTargetZip := filepath.Clean(targetZip)

	// Get absolute paths for validation
	absSourceDir, err := filepath.Abs(cleanSourceDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute source path: %w", err)
	}
	absTargetZip, err := filepath.Abs(cleanTargetZip)
	if err != nil {
		return fmt.Errorf("failed to get absolute target path: %w", err)
	}

	// Ensure target zip is not inside source directory
	if strings.HasPrefix(absTargetZip, absSourceDir+string(filepath.Separator)) {
		return fmt.Errorf("target zip cannot be inside source directory")
	}

	// #nosec G304 -- paths are validated and cleaned above
	zipFile, err := os.Create(absTargetZip)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer func() {
		_ = zipFile.Close()
	}()

	zipWriter := zip.NewWriter(zipFile)
	defer func() {
		_ = zipWriter.Close()
	}()

	// Walk through the source directory
	err = filepath.Walk(absSourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Clean and validate path
		cleanPath := filepath.Clean(path)

		// Ensure path is within source directory (prevent directory traversal)
		if !strings.HasPrefix(cleanPath, absSourceDir) {
			return fmt.Errorf("path outside source directory: %s", path)
		}

		// Get relative path
		relPath, err := filepath.Rel(absSourceDir, cleanPath)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Validate relative path doesn't escape
		if strings.HasPrefix(relPath, "..") {
			return fmt.Errorf("invalid relative path: %s", relPath)
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Use relative path in zip
		header.Name = relPath

		// Set compression method for files
		if !info.IsDir() {
			header.Method = zip.Deflate
		} else {
			header.Name += "/"
		}

		// Create writer for this file
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a directory, we're done
		if info.IsDir() {
			return nil
		}

		// Open and copy file contents
		// #nosec G304 -- path is validated to be within source directory
		file, err := os.Open(cleanPath)
		if err != nil {
			return err
		}
		defer func() {
			_ = file.Close()
		}()

		_, err = io.Copy(writer, file)
		return err
	})

	return err
}

// fetchMissingFeaturedMedia fetches featured images that weren't returned by the paginated media API
// WordPress REST API may not return all media items (WPML language contexts, different post types, etc.)
func fetchMissingFeaturedMedia(
	apiClient *api.Client,
	posts []models.WordPressPost,
	pages []models.WordPressPost,
	existingMedia []models.WordPressMedia,
	verbose bool,
) []models.WordPressMedia {
	// Build set of existing media IDs
	existingIDs := make(map[int]bool)
	for _, m := range existingMedia {
		existingIDs[m.ID] = true
	}

	// Collect all featured image IDs from posts and pages
	missingIDs := make(map[int]bool)
	for _, post := range posts {
		if post.FeaturedMedia > 0 && !existingIDs[post.FeaturedMedia] {
			missingIDs[post.FeaturedMedia] = true
		}
	}
	for _, page := range pages {
		if page.FeaturedMedia > 0 && !existingIDs[page.FeaturedMedia] {
			missingIDs[page.FeaturedMedia] = true
		}
	}

	if len(missingIDs) == 0 {
		return nil
	}

	// Fetch missing media by ID
	var fetchedMedia []models.WordPressMedia
	for id := range missingIDs {
		media, err := apiClient.GetMediaByID(id)
		if err != nil {
			if verbose {
				logf("Warning: could not fetch featured image %d: %v\n", id, err)
			}
			continue
		}
		if media != nil {
			fetchedMedia = append(fetchedMedia, *media)
		}
	}

	return fetchedMedia
}

// Execute runs this tool's command tree as a standalone binary.
func Execute() error {
	return rootCmd.Execute()
}

// RootCommand returns the tool's own root, for mounting as a group under the
// umbrella command. The caller renames Use to the group it should answer to.
func RootCommand() *cobra.Command {
	return rootCmd
}

// ExportCommand returns the tool's working subcommand, for mounting directly at
// the umbrella's top level.
func ExportCommand() *cobra.Command {
	return exportCmd
}

// PersistentFlags returns the tree's global flags, bound to this package's
// variables. Mounting a subcommand without them would leave those variables
// unset.
func PersistentFlags() *pflag.FlagSet {
	return rootCmd.PersistentFlags()
}
