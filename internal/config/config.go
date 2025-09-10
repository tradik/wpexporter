package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	URL           string `mapstructure:"url" json:"url"`
	Output        string `mapstructure:"output" json:"output"`
	Format        string `mapstructure:"format" json:"format"`
	BruteForce    bool   `mapstructure:"brute_force" json:"brute_force"`
	MaxID         int    `mapstructure:"max_id" json:"max_id"`
	DownloadMedia bool   `mapstructure:"download_media" json:"download_media"`
	Concurrent    int    `mapstructure:"concurrent" json:"concurrent"`
	Timeout       int    `mapstructure:"timeout" json:"timeout"`
	Retries       int    `mapstructure:"retries" json:"retries"`
	UserAgent     string `mapstructure:"user_agent" json:"user_agent"`
	Verbose       bool   `mapstructure:"verbose" json:"verbose"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Output:        "", // Will be generated based on URL and date
		Format:        "json",
		BruteForce:    false,
		MaxID:         10000,
		DownloadMedia: true,
		Concurrent:    5,
		Timeout:       30,
		Retries:       3,
		UserAgent:     "WordPress-Export-JSON/1.0",
		Verbose:       false,
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

	if c.Format != "json" && c.Format != "markdown" {
		return fmt.Errorf("format must be 'json' or 'markdown'")
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

	return nil
}

// EnsureOutputDir ensures the output directory exists
func (c *Config) EnsureOutputDir() error {
	if c.Format == "json" && filepath.Ext(c.Output) == ".json" {
		// If output is a JSON file, ensure parent directory exists
		dir := filepath.Dir(c.Output)
		return os.MkdirAll(dir, 0755)
	}

	// Otherwise, ensure output directory exists
	return os.MkdirAll(c.Output, 0755)
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

	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "wordpress-site"
	}

	return sanitized
}

// GetMediaDir returns the media download directory
func (c *Config) GetMediaDir() string {
	if c.Format == "json" && filepath.Ext(c.Output) == ".json" {
		// If output is a JSON file, create media directory next to it
		dir := filepath.Dir(c.Output)
		base := filepath.Base(c.Output)
		name := base[:len(base)-len(filepath.Ext(base))]
		return filepath.Join(dir, name+"_media")
	}

	// Otherwise, create media directory inside output directory
	return filepath.Join(c.Output, "media")
}
