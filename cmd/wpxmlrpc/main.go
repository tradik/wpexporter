package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/internal/xmlrpc"
	"github.com/tradik/wpexporter/pkg/models"
)

var (
	cfgFile  string
	url      string
	username string
	password string
	output   string
	format   string
	verbose  bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wpxmlrpc",
	Short: "WordPress XML-RPC content export tool",
	Long: `A WordPress XML-RPC client for exporting content using the WordPress XML-RPC API.
This tool can authenticate with WordPress sites and export all content including
posts, pages, media, categories, tags, and users using XML-RPC protocol.`,
}

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export WordPress content via XML-RPC",
	Long: `Export all content from a WordPress site using XML-RPC API.
Requires WordPress username and password for authentication.`,
	RunE: runExport,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.wpxmlrpc/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Export command flags
	exportCmd.Flags().StringVarP(&url, "url", "u", "", "WordPress site URL (required)")
	exportCmd.Flags().StringVar(&username, "username", "", "WordPress username (required)")
	exportCmd.Flags().StringVar(&password, "password", "", "WordPress password (required)")
	exportCmd.Flags().StringVarP(&output, "output", "o", "", "output directory or file (default: export/{domain-name}.{date}{time})")
	exportCmd.Flags().StringVarP(&format, "format", "f", "json", "export format (json|markdown)")

	// Mark required flags
	exportCmd.MarkFlagRequired("url")
	exportCmd.MarkFlagRequired("username")

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
		filepath.Join(os.Getenv("HOME"), ".wpxmlrpc", "config.yaml"),
		filepath.Join(os.Getenv("HOME"), ".wpxmlrpc", "config.yml"),
		"/etc/wpxmlrpc/config.yaml",
		"/etc/wpxmlrpc/config.yml",
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
	cfg.Output = "./xmlrpc-export" // Different default for XML-RPC

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

	// Create XML-RPC client
	xmlrpcClient, err := xmlrpc.NewClient(cfg, username, password)
	if err != nil {
		return fmt.Errorf("failed to create XML-RPC client: %w", err)
	}

	// Create exporter
	exporter := export.NewExporter(cfg)

	fmt.Printf("Starting WordPress XML-RPC export from: %s\n", cfg.URL)
	fmt.Printf("Output: %s (format: %s)\n", cfg.Output, cfg.Format)
	fmt.Printf("Username: %s\n", username)

	startTime := time.Now()

	// Test connection
	fmt.Println("\nTesting XML-RPC connection...")
	if err := xmlrpcClient.TestConnection(); err != nil {
		return fmt.Errorf("XML-RPC connection failed: %w", err)
	}
	fmt.Println("✓ XML-RPC connection successful")

	// Get site information
	fmt.Println("\nFetching site information...")
	siteInfo, err := xmlrpcClient.GetSiteInfo()
	if err != nil {
		return fmt.Errorf("failed to get site info: %w", err)
	}

	// Get all content via XML-RPC
	fmt.Println("Fetching posts...")
	posts, err := xmlrpcClient.GetPosts()
	if err != nil {
		return fmt.Errorf("failed to get posts: %w", err)
	}
	fmt.Printf("Found %d posts\n", len(posts))

	fmt.Println("Fetching pages...")
	pages, err := xmlrpcClient.GetPages()
	if err != nil {
		return fmt.Errorf("failed to get pages: %w", err)
	}
	fmt.Printf("Found %d pages\n", len(pages))

	fmt.Println("Fetching media...")
	media, err := xmlrpcClient.GetMedia()
	if err != nil {
		return fmt.Errorf("failed to get media: %w", err)
	}
	fmt.Printf("Found %d media items\n", len(media))

	fmt.Println("Fetching categories...")
	categories, err := xmlrpcClient.GetCategories()
	if err != nil {
		return fmt.Errorf("failed to get categories: %w", err)
	}
	fmt.Printf("Found %d categories\n", len(categories))

	fmt.Println("Fetching tags...")
	tags, err := xmlrpcClient.GetTags()
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}
	fmt.Printf("Found %d tags\n", len(tags))

	fmt.Println("Fetching users...")
	users, err := xmlrpcClient.GetUsers()
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}
	fmt.Printf("Found %d users\n", len(users))

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
		},
	}

	// Export data
	fmt.Println("\nExporting data...")
	if err := exporter.Export(exportData); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Print summary
	duration := time.Since(startTime)
	fmt.Printf("\n=== XML-RPC Export Summary ===\n")
	fmt.Printf("Site: %s\n", siteInfo.Name)
	fmt.Printf("Posts: %d\n", len(posts))
	fmt.Printf("Pages: %d\n", len(pages))
	fmt.Printf("Media: %d\n", len(media))
	fmt.Printf("Categories: %d\n", len(categories))
	fmt.Printf("Tags: %d\n", len(tags))
	fmt.Printf("Users: %d\n", len(users))
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
