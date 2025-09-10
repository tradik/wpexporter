package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/bruteforce"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/pkg/models"
)

var (
	cfgFile       string
	url           string
	output        string
	format        string
	bruteForce    bool
	maxID         int
	downloadMedia bool
	concurrent    int
	verbose       bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wpexportjson",
	Short: "WordPress content export tool",
	Long: `A powerful WordPress content export tool that scans WordPress WP API 
to download all content, images, and videos from a website. Features brute 
force content discovery and exports to JSON or Markdown format.`,
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
	exportCmd.Flags().StringVarP(&format, "format", "f", "json", "export format (json|markdown)")
	exportCmd.Flags().BoolVar(&bruteForce, "brute-force", false, "enable brute force ID discovery")
	exportCmd.Flags().IntVar(&maxID, "max-id", 10000, "maximum ID for brute force")
	exportCmd.Flags().BoolVar(&downloadMedia, "download-media", true, "download images and videos")
	exportCmd.Flags().IntVarP(&concurrent, "concurrent", "c", 5, "concurrent downloads")

	// Mark required flags
	if err := exportCmd.MarkFlagRequired("url"); err != nil {
		panic(fmt.Sprintf("Failed to mark url flag as required: %v", err))
	}

	rootCmd.AddCommand(exportCmd)
}

func initConfig() {
	// Configuration will be loaded in runExport
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
	if cmd.Flags().Changed("concurrent") {
		cfg.Concurrent = concurrent
	}
	if cmd.Flags().Changed("verbose") {
		cfg.Verbose = verbose
	}

	// Generate default output path if not specified
	if err := cfg.GenerateDefaultOutput(); err != nil {
		return fmt.Errorf("failed to generate default output path: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
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

	// Get all content via API
	fmt.Println("Fetching posts...")
	posts, err := apiClient.GetPosts()
	if err != nil {
		return fmt.Errorf("failed to get posts: %w", err)
	}
	fmt.Printf("Found %d posts\n", len(posts))

	fmt.Println("Fetching pages...")
	pages, err := apiClient.GetPages()
	if err != nil {
		return fmt.Errorf("failed to get pages: %w", err)
	}
	fmt.Printf("Found %d pages\n", len(pages))

	fmt.Println("Fetching media...")
	media, err := apiClient.GetMedia()
	if err != nil {
		return fmt.Errorf("failed to get media: %w", err)
	}
	fmt.Printf("Found %d media items\n", len(media))

	fmt.Println("Fetching categories...")
	categories, err := apiClient.GetCategories()
	if err != nil {
		return fmt.Errorf("failed to get categories: %w", err)
	}
	fmt.Printf("Found %d categories\n", len(categories))

	fmt.Println("Fetching tags...")
	tags, err := apiClient.GetTags()
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}
	fmt.Printf("Found %d tags\n", len(tags))

	fmt.Println("Fetching users...")
	users, err := apiClient.GetUsers()
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}
	fmt.Printf("Found %d users\n", len(users))

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
		Media:      media,
		Categories: categories,
		Tags:       tags,
		Users:      users,
		Stats: models.ExportStats{
			TotalPosts:      len(posts),
			TotalPages:      len(pages),
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

	// Print summary
	duration := time.Since(startTime)
	fmt.Printf("\n=== Export Summary ===\n")
	fmt.Printf("Site: %s\n", siteInfo.Name)
	fmt.Printf("Posts: %d\n", len(posts))
	fmt.Printf("Pages: %d\n", len(pages))
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
	fmt.Printf("Output: %s\n", cfg.Output)

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
