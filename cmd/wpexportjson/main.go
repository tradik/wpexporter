package main

import (
	"archive/zip"
	"fmt"
	"io"
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
	createZip     bool
	noFiles       bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wpexportjson",
	Short: "WordPress content export tool",
	Long: `A powerful WordPress content export tool that scans WordPress WP API 
to download all content, images, and videos from a website. Features brute 
force content discovery and exports to JSON or Markdown format.

Examples:
  # Export to JSON (default)
  wpexportjson export --url https://example.com

  # Export to Markdown
  wpexportjson export --url https://example.com -f markdown

  # Export with custom output directory
  wpexportjson export --url https://example.com -o ./my-export

  # Export with brute force content discovery
  wpexportjson export --url https://example.com --brute-force

  # Export without downloading media
  wpexportjson export --url https://example.com --download-media=false

  # Export and create a ZIP archive
  wpexportjson export --url https://example.com --zip

  # Export to ZIP only (remove files after creating ZIP)
  wpexportjson export --url https://example.com --zip --no-files

  # Export to Markdown with ZIP archive
  wpexportjson export --url https://example.com -f markdown --zip`,
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
	exportCmd.Flags().BoolVar(&createZip, "zip", false, "create ZIP archive of export")
	exportCmd.Flags().BoolVar(&noFiles, "no-files", false, "remove export files after creating ZIP (requires --zip)")

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
	if cmd.Flags().Changed("zip") {
		cfg.CreateZip = createZip
	}
	if cmd.Flags().Changed("no-files") {
		cfg.NoFiles = noFiles
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

	return nil
}

// createZipArchive creates a ZIP archive of the specified directory
func createZipArchive(sourceDir, targetZip string) error {
	zipFile, err := os.Create(targetZip)
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
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
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
		file, err := os.Open(path)
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
