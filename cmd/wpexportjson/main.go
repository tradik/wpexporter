package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/bruteforce"
	"github.com/tradik/wpexporter/internal/checkpoint"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/internal/filter"
	"github.com/tradik/wpexporter/internal/flathtml"
	mediafilter "github.com/tradik/wpexporter/internal/media"
	"github.com/tradik/wpexporter/internal/seo"
	"github.com/tradik/wpexporter/pkg/models"
)

// Version information - set during build
var (
	Version   = "1.4.0"
	BuildDate = "unknown"
)

var (
	cfgFile           string
	url               string
	output            string
	format            string
	bruteForce        bool
	maxID             int
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
	noUsers           bool
	pathFilter        string
	assistedCrawl     bool
	authUser          string
	authPass          string
	authToken         string
	rateLimit         int
	resume            bool
	timeout           int
	crawlContent      bool
	skipEmptyContent  bool
	flatHTML          bool
	noTags            bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "wpexportjson",
	Short:   "WordPress content export tool",
	Version: Version,
	Long: `WordPress content export tool

Usage: wpexportjson export --url <site-url> [flags]

Export Flags:
  -u, --url string            WordPress site URL (required)
  -o, --output string         Output directory (default: export/{domain}.{date})
  -f, --format string         Format: json|markdown|shopify|magento|wordpress|drupal|
                              wix|squarespace|webflow|weebly|prestashop|ghost|strapi|contentful
  -c, --concurrent int        Concurrent downloads (default 5)
      --timeout int           HTTP timeout in seconds (default 30)

Authentication:
      --auth-user string      Username for Basic Auth
      --auth-pass string      Password for Basic Auth
      --auth-token string     Bearer token

Content Filters:
      --no-posts              Skip blog posts
      --no-pages              Skip pages
      --no-products           Skip WooCommerce products
      --no-users              Skip users
      --no-tags               Skip tags
      --no-media              Skip media downloads
      --path-filter string    Filter by URL path (e.g., /fr/art/)
      --skip-empty-content    Skip posts/pages with empty content
      --flat-html             Convert HTML to Markdown (Bricks Builder support)

Advanced:
      --brute-force           Enable brute force ID discovery
      --max-id int            Max ID for brute force (default 10000)
      --assisted-crawl        Crawl URLs for SEO metadata
      --crawl-content         Crawl pages with empty content (Bricks, Elementor)
      --relevant-media-only   Download only featured/content images
      --resume                Resume from checkpoint
      --rate-limit int        Delay between requests in ms
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
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.wpexportjson/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Export command flags
	exportCmd.Flags().StringVarP(&url, "url", "u", "", "WordPress site URL (required)")
	exportCmd.Flags().StringVarP(&output, "output", "o", "", "output directory or file (default: export/{domain-name}.{date}{time})")
	exportCmd.Flags().StringVarP(&format, "format", "f", "json", "export format (json|markdown|shopify|magento)")
	exportCmd.Flags().BoolVar(&bruteForce, "brute-force", false, "enable brute force ID discovery")
	exportCmd.Flags().IntVar(&maxID, "max-id", 10000, "maximum ID for brute force")
	exportCmd.Flags().BoolVar(&downloadMedia, "download-media", true, "download images and videos")
	exportCmd.Flags().BoolVar(&noMedia, "no-media", false, "disable media downloads (alias for --download-media=false)")
	exportCmd.Flags().BoolVar(&relevantMediaOnly, "relevant-media-only", false, "only download featured images and images embedded in content")
	exportCmd.Flags().IntVarP(&concurrent, "concurrent", "c", 5, "concurrent downloads")
	exportCmd.Flags().BoolVar(&createZip, "zip", false, "create ZIP archive of export")
	exportCmd.Flags().BoolVar(&noFiles, "no-files", false, "remove export files after creating ZIP (requires --zip)")
	exportCmd.Flags().BoolVar(&noPosts, "no-posts", false, "skip exporting blog posts")
	exportCmd.Flags().BoolVar(&noPages, "no-pages", false, "skip exporting pages")
	exportCmd.Flags().BoolVar(&noProducts, "no-products", false, "skip exporting WooCommerce products")
	exportCmd.Flags().BoolVar(&noUsers, "no-users", false, "skip exporting users")
	exportCmd.Flags().StringVar(&authUser, "auth-user", "", "username for Basic Auth")
	exportCmd.Flags().StringVar(&authPass, "auth-pass", "", "password for Basic Auth")
	exportCmd.Flags().StringVar(&authToken, "auth-token", "", "Bearer token for authentication")
	exportCmd.Flags().IntVar(&rateLimit, "rate-limit", 0, "delay between API requests in milliseconds (0 = no limit)")
	exportCmd.Flags().BoolVar(&resume, "resume", false, "resume from checkpoint if previous export was interrupted")
	exportCmd.Flags().IntVar(&timeout, "timeout", 30, "HTTP request timeout in seconds")
	exportCmd.Flags().StringVar(&pathFilter, "path-filter", "", "filter posts/pages by URL path pattern (e.g., /fr/arts/)")
	exportCmd.Flags().BoolVar(&assistedCrawl, "assisted-crawl", false, "crawl URLs for SEO metadata")
	exportCmd.Flags().BoolVar(&crawlContent, "crawl-content", false, "crawl pages with empty content (Bricks, Elementor)")
	exportCmd.Flags().BoolVar(&skipEmptyContent, "skip-empty-content", false, "skip posts/pages with empty content")
	exportCmd.Flags().BoolVar(&flatHTML, "flat-html", false, "convert HTML to Markdown (Bricks Builder support)")
	exportCmd.Flags().BoolVar(&noTags, "no-tags", false, "skip exporting tags")

	// Mark required flags
	if err := exportCmd.MarkFlagRequired("url"); err != nil {
		panic(fmt.Sprintf("Failed to mark url flag as required: %v", err))
	}

	rootCmd.AddCommand(exportCmd)
}

func initConfig() {
	// Configuration will be loaded in runExport
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
	configPaths := []string{
		"./config.yaml",
		"./config.yml",
		filepath.Join(os.Getenv("HOME"), ".wpexportjson", "config.yaml"),
		filepath.Join(os.Getenv("HOME"), ".wpexportjson", "config.yml"),
		"/etc/wpexportjson/config.yaml",
		"/etc/wpexportjson/config.yml",
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func runExport(cmd *cobra.Command, args []string) error {
	// Start with default configuration
	cfg := config.DefaultConfig()

	// Load configuration file if specified or found
	if cfgFile != "" || configFileExists() {
		loadedCfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg = loadedCfg
	}

	// Override config with command line flags
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
		// Validate path filter doesn't look like a flag (common user error)
		if strings.HasPrefix(pathFilter, "-") {
			return fmt.Errorf("invalid --path-filter value '%s': looks like a flag. Use --path-filter=/path/ or omit if not filtering", pathFilter)
		}
		cfg.PathFilter = pathFilter
	}
	if cmd.Flags().Changed("assisted-crawl") {
		cfg.AssistedCrawl = assistedCrawl
	}
	if cmd.Flags().Changed("rate-limit") {
		cfg.RateLimit = rateLimit
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
	if cmd.Flags().Changed("no-tags") {
		cfg.NoTags = noTags
	}

	// Validate --no-files requires --zip
	if cfg.NoFiles && !cfg.CreateZip {
		return fmt.Errorf("--no-files requires --zip flag")
	}

	// Generate default output path if not specified
	if err := cfg.GenerateDefaultOutput(); err != nil {
		return fmt.Errorf("failed to generate default output path: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Check output directory permissions before starting expensive operations
	if err := checkOutputPermissions(cfg.Output); err != nil {
		return fmt.Errorf("output directory check failed: %w", err)
	}

	// Create API client
	apiClient, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Create exporter
	exporter := export.NewExporter(cfg)

	// Create brute force scanner
	scanner := bruteforce.NewScanner(cfg, apiClient)

	// Create checkpoint manager
	checkpointMgr := checkpoint.NewManager(cfg.Output, cfg.Resume)
	var checkpointState *checkpoint.State

	// Load or create checkpoint state
	if cfg.Resume {
		var err error
		checkpointState, err = checkpointMgr.Load(cfg.URL)
		if err != nil {
			return fmt.Errorf("failed to load checkpoint: %w", err)
		}
		if checkpointMgr.Exists() {
			fmt.Printf("Resuming from checkpoint: %s\n", checkpointMgr.GetFilePath())
			fmt.Println(checkpointState.Summary())
		}
	}

	// Progress callback to save checkpoint after each page
	saveCheckpoint := func() error {
		return checkpointMgr.Save()
	}

	fmt.Printf("Starting WordPress export from: %s\n", cfg.URL)
	fmt.Printf("Output: %s (format: %s)\n", cfg.Output, cfg.Format)

	if cfg.BruteForce {
		fmt.Printf("Brute force enabled (max ID: %d)\n", cfg.MaxID)
	}

	if cfg.DownloadMedia {
		fmt.Printf("Media download enabled (concurrent: %d)\n", cfg.Concurrent)
	}

	startTime := time.Now()

	// Get site information
	fmt.Println("\nFetching site information...")
	siteInfo, err := apiClient.GetSiteInfo()
	if err != nil {
		return fmt.Errorf("failed to get site info: %w", err)
	}

	// Get content via API (respecting filter flags)
	var posts []models.WordPressPost
	if !cfg.NoPosts {
		fmt.Println("Fetching posts...")
		if cfg.Resume {
			posts, err = apiClient.GetPostsWithCheckpoint(checkpointState, saveCheckpoint)
		} else {
			posts, err = apiClient.GetPosts()
		}
		if err != nil {
			return fmt.Errorf("failed to get posts: %w", err)
		}
		fmt.Printf("Found %d posts\n", len(posts))
	} else {
		fmt.Println("Skipping posts (--no-posts)")
	}

	var pages []models.WordPressPost
	if !cfg.NoPages {
		fmt.Println("Fetching pages...")
		if cfg.Resume {
			pages, err = apiClient.GetPagesWithCheckpoint(checkpointState, saveCheckpoint)
		} else {
			pages, err = apiClient.GetPages()
		}
		if err != nil {
			return fmt.Errorf("failed to get pages: %w", err)
		}
		fmt.Printf("Found %d pages\n", len(pages))
	} else {
		fmt.Println("Skipping pages (--no-pages)")
	}

	var products []models.WooCommerceProduct
	if !cfg.NoProducts {
		fmt.Println("Fetching WooCommerce products...")
		if cfg.Resume {
			products, err = apiClient.GetProductsWithCheckpoint(checkpointState, saveCheckpoint)
		} else {
			products, err = apiClient.GetProducts()
		}
		if err != nil {
			return fmt.Errorf("failed to get products: %w", err)
		}
		if len(products) > 0 {
			fmt.Printf("Found %d WooCommerce products\n", len(products))
		} else {
			fmt.Println("No WooCommerce products found (WooCommerce may not be installed)")
		}
	} else {
		fmt.Println("Skipping products (--no-products)")
	}

	// Apply path filter if specified
	if cfg.PathFilter != "" {
		pathFilter := filter.NewPathFilter(cfg.PathFilter)
		originalPosts := len(posts)
		originalPages := len(pages)
		posts = pathFilter.FilterPosts(posts)
		pages = pathFilter.FilterPosts(pages)
		fmt.Printf("Path filter '%s': %d/%d posts, %d/%d pages matched\n",
			cfg.PathFilter, len(posts), originalPosts, len(pages), originalPages)
	}

	// Crawl URLs for SEO data if enabled
	if cfg.AssistedCrawl {
		fmt.Println("\nCrawling URLs for SEO metadata...")
		crawler := seo.NewCrawler(cfg)
		if len(posts) > 0 {
			posts = crawler.EnrichPostsWithSEO(posts)
		}
		if len(pages) > 0 {
			pages = crawler.EnrichPostsWithSEO(pages)
		}
		fmt.Println("SEO metadata extraction complete")
	}

	// Crawl content for pages with empty content (page builders like Bricks, Elementor)
	if cfg.CrawlContent {
		fmt.Println("\nCrawling pages with empty content...")
		crawler := seo.NewCrawler(cfg)
		if len(posts) > 0 {
			posts = crawler.EnrichPostsWithContent(posts)
		}
		if len(pages) > 0 {
			pages = crawler.EnrichPostsWithContent(pages)
		}
	}

	// Skip posts/pages with empty content if enabled
	if cfg.SkipEmptyContent {
		originalPosts := len(posts)
		originalPages := len(pages)
		posts = seo.FilterEmptyContent(posts)
		pages = seo.FilterEmptyContent(pages)
		fmt.Printf("Skipped empty content: %d/%d posts, %d/%d pages\n",
			originalPosts-len(posts), originalPosts, originalPages-len(pages), originalPages)
	}

	// Convert HTML to Markdown if flat-html is enabled
	if cfg.FlatHTML {
		fmt.Println("\nConverting HTML to Markdown...")
		var converter *flathtml.Converter
		if len(cfg.FlatHTMLRules) > 0 {
			converter = flathtml.NewConverterWithRules(cfg.FlatHTMLRules)
		} else {
			converter = flathtml.NewConverter()
		}
		if len(posts) > 0 {
			posts = converter.ConvertPosts(posts)
		}
		if len(pages) > 0 {
			pages = converter.ConvertPosts(pages)
		}
		fmt.Println("HTML to Markdown conversion complete")
	}

	var media []models.WordPressMedia
	if cfg.DownloadMedia {
		fmt.Println("Fetching media...")
		if cfg.Resume {
			media, err = apiClient.GetMediaWithCheckpoint(checkpointState, saveCheckpoint)
		} else {
			media, err = apiClient.GetMedia()
		}
		if err != nil {
			return fmt.Errorf("failed to get media: %w", err)
		}
		fmt.Printf("Found %d media items\n", len(media))

		// Filter to relevant media only if enabled
		if cfg.RelevantMediaOnly {
			mf := mediafilter.NewFilter()
			originalMedia := len(media)
			media = mf.FilterRelevantMedia(posts, pages, media)
			fmt.Printf("Filtered to %d relevant media items (from %d total)\n", len(media), originalMedia)
		}
	} else {
		fmt.Println("Skipping media (--no-media)")
	}

	fmt.Println("Fetching categories...")
	categories, err := apiClient.GetCategories()
	if err != nil {
		return fmt.Errorf("failed to get categories: %w", err)
	}
	fmt.Printf("Found %d categories\n", len(categories))

	var tags []models.WordPressTag
	if !cfg.NoTags {
		fmt.Println("Fetching tags...")
		tags, err = apiClient.GetTags()
		if err != nil {
			return fmt.Errorf("failed to get tags: %w", err)
		}
		fmt.Printf("Found %d tags\n", len(tags))
	} else {
		fmt.Println("Skipping tags (--no-tags)")
	}

	var users []models.WordPressUser
	if !cfg.NoUsers {
		fmt.Println("Fetching users...")
		users, err = apiClient.GetUsers()
		if err != nil {
			// Don't fail on users fetch error, just warn and continue
			fmt.Printf("Warning: could not fetch users: %v\n", err)
			users = []models.WordPressUser{}
		} else {
			fmt.Printf("Found %d users\n", len(users))
		}
	} else {
		fmt.Println("Skipping users (--no-users)")
	}

	// Perform brute force scanning if enabled
	var bruteForceFound int
	if cfg.BruteForce {
		fmt.Println("\nPerforming brute force content discovery...")
		scanResult, err := scanner.ScanForContent(posts, pages, media)
		if err != nil {
			return fmt.Errorf("brute force scan failed: %w", err)
		}

		// Merge brute force results
		posts = append(posts, scanResult.Posts...)
		pages = append(pages, scanResult.Pages...)
		media = append(media, scanResult.Media...)
		bruteForceFound = scanResult.Found
	}

	// Create export data
	exportData := &models.ExportData{
		Site:       *siteInfo,
		Posts:      posts,
		Pages:      pages,
		Products:   products,
		Media:      media,
		Categories: categories,
		Tags:       tags,
		Users:      users,
		Stats: models.ExportStats{
			TotalPosts:      len(posts),
			TotalPages:      len(pages),
			TotalProducts:   len(products),
			TotalMedia:      len(media),
			TotalCategories: len(categories),
			TotalTags:       len(tags),
			TotalUsers:      len(users),
			BruteForceFound: bruteForceFound,
		},
	}

	// Export data
	fmt.Println("\nExporting data...")
	if err := exporter.Export(exportData); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Create ZIP archive if requested
	var zipPath string
	if cfg.CreateZip {
		fmt.Println("Creating ZIP archive...")
		zipPath = cfg.Output + ".zip"
		if err := createZipArchive(cfg.Output, zipPath); err != nil {
			return fmt.Errorf("failed to create ZIP archive: %w", err)
		}
		fmt.Printf("ZIP archive created: %s\n", zipPath)

		// Remove files if --no-files is set
		if cfg.NoFiles {
			fmt.Println("Removing export files...")
			if err := os.RemoveAll(cfg.Output); err != nil {
				return fmt.Errorf("failed to remove export files: %w", err)
			}
			fmt.Println("Export files removed")
		}
	}

	// Print summary
	duration := time.Since(startTime)
	fmt.Printf("\n=== Export Summary ===\n")
	fmt.Printf("Site: %s\n", siteInfo.Name)
	fmt.Printf("Posts: %d\n", len(posts))
	fmt.Printf("Pages: %d\n", len(pages))
	fmt.Printf("Products: %d\n", len(products))
	fmt.Printf("Media: %d\n", len(media))
	fmt.Printf("Categories: %d\n", len(categories))
	fmt.Printf("Tags: %d\n", len(tags))
	fmt.Printf("Users: %d\n", len(users))

	if cfg.BruteForce && bruteForceFound > 0 {
		fmt.Printf("Brute force found: %d\n", bruteForceFound)
	}

	if cfg.DownloadMedia {
		fmt.Printf("Media downloaded: %d\n", exportData.Stats.MediaDownloaded)
	}

	fmt.Printf("Duration: %v\n", duration)

	if cfg.CreateZip {
		fmt.Printf("ZIP: %s\n", zipPath)
		if !cfg.NoFiles {
			fmt.Printf("Output: %s\n", cfg.Output)
		}
	} else {
		fmt.Printf("Output: %s\n", cfg.Output)
	}

	// Delete checkpoint on successful completion
	if cfg.Resume {
		if err := checkpointMgr.Delete(); err != nil {
			fmt.Printf("Warning: failed to delete checkpoint: %v\n", err)
		}
	}

	return nil
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
