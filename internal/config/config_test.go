package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Format != "json" {
		t.Errorf("Expected format 'json', got %s", cfg.Format)
	}

	if cfg.BruteForce != false {
		t.Errorf("Expected brute_force false, got %v", cfg.BruteForce)
	}

	if cfg.MaxID != 10000 {
		t.Errorf("Expected max_id 10000, got %d", cfg.MaxID)
	}

	if cfg.DownloadMedia != true {
		t.Errorf("Expected download_media true, got %v", cfg.DownloadMedia)
	}

	if cfg.Concurrent != 5 {
		t.Errorf("Expected concurrent 5, got %d", cfg.Concurrent)
	}

	if cfg.Timeout != 30 {
		t.Errorf("Expected timeout 30, got %d", cfg.Timeout)
	}

	if cfg.Retries != 3 {
		t.Errorf("Expected retries 3, got %d", cfg.Retries)
	}

	if cfg.UserAgent != "WordPress-Export-JSON/1.0" {
		t.Errorf("Expected user agent 'WordPress-Export-JSON/1.0', got %s", cfg.UserAgent)
	}

	if cfg.Verbose != false {
		t.Errorf("Expected verbose false, got %v", cfg.Verbose)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "Valid config",
			cfg: &Config{
				URL:        "https://example.com",
				Format:     "json",
				MaxID:      100,
				Concurrent: 5,
				Timeout:    30,
				Retries:    3,
			},
			wantErr: false,
		},
		{
			name: "Empty URL",
			cfg: &Config{
				URL:        "",
				Format:     "json",
				MaxID:      100,
				Concurrent: 5,
				Timeout:    30,
				Retries:    3,
			},
			wantErr: true,
		},
		{
			name: "Invalid format",
			cfg: &Config{
				URL:        "https://example.com",
				Format:     "xml",
				MaxID:      100,
				Concurrent: 5,
				Timeout:    30,
				Retries:    3,
			},
			wantErr: true,
		},
		{
			name: "Zero MaxID",
			cfg: &Config{
				URL:        "https://example.com",
				Format:     "json",
				MaxID:      0,
				Concurrent: 5,
				Timeout:    30,
				Retries:    3,
			},
			wantErr: true,
		},
		{
			name: "Zero Concurrent",
			cfg: &Config{
				URL:        "https://example.com",
				Format:     "json",
				MaxID:      100,
				Concurrent: 0,
				Timeout:    30,
				Retries:    3,
			},
			wantErr: true,
		},
		{
			name: "Zero Timeout",
			cfg: &Config{
				URL:        "https://example.com",
				Format:     "json",
				MaxID:      100,
				Concurrent: 5,
				Timeout:    0,
				Retries:    3,
			},
			wantErr: true,
		},
		{
			name: "Negative Retries",
			cfg: &Config{
				URL:        "https://example.com",
				Format:     "json",
				MaxID:      100,
				Concurrent: 5,
				Timeout:    30,
				Retries:    -1,
			},
			wantErr: true,
		},
		{
			name: "Valid markdown format",
			cfg: &Config{
				URL:        "https://example.com",
				Format:     "markdown",
				MaxID:      100,
				Concurrent: 5,
				Timeout:    30,
				Retries:    3,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateDefaultOutput(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		wantErr  bool
		contains string
	}{
		{
			name: "Valid URL generates output",
			cfg: &Config{
				URL: "https://example.com",
			},
			wantErr:  false,
			contains: "export/example.com.",
		},
		{
			name: "URL with www removes www",
			cfg: &Config{
				URL: "https://www.example.com",
			},
			wantErr:  false,
			contains: "export/example.com.",
		},
		{
			name: "Empty URL returns error",
			cfg: &Config{
				URL: "",
			},
			wantErr: true,
		},
		{
			name: "Invalid URL returns error",
			cfg: &Config{
				URL: "ht tp://invalid url with spaces",
			},
			wantErr: true,
		},
		{
			name: "Already set output doesn't change",
			cfg: &Config{
				URL:    "https://example.com",
				Output: "custom/output",
			},
			wantErr:  false,
			contains: "custom/output",
		},
		{
			name: "URL with special characters sanitized",
			cfg: &Config{
				URL: "https://my-site_with.special.com",
			},
			wantErr:  false,
			contains: "export/my-site_with.special.com.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.GenerateDefaultOutput()
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateDefaultOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.cfg.Output == "" {
				t.Error("GenerateDefaultOutput() should set output path")
			}

			if !tt.wantErr && tt.contains != "" {
				if !filepath.IsAbs(tt.cfg.Output) && !strings.HasPrefix(tt.cfg.Output, tt.contains) {
					t.Errorf("GenerateDefaultOutput() output = %v, should contain %v", tt.cfg.Output, tt.contains)
				}
			}
		})
	}
}

func TestEnsureOutputDir(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "JSON file output creates parent directory",
			cfg: &Config{
				Output: filepath.Join(tempDir, "output", "data.json"),
				Format: "json",
			},
			wantErr: false,
		},
		{
			name: "Markdown directory output creates directory",
			cfg: &Config{
				Output: filepath.Join(tempDir, "output", "markdown"),
				Format: "markdown",
			},
			wantErr: false,
		},
		{
			name: "Nested directory creation",
			cfg: &Config{
				Output: filepath.Join(tempDir, "deep", "nested", "path", "data.json"),
				Format: "json",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.EnsureOutputDir()
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureOutputDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Check if directory was created
				dir := filepath.Dir(tt.cfg.Output)
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					t.Errorf("EnsureOutputDir() directory was not created: %s", dir)
				}
			}
		})
	}
}

func TestGetMediaDir(t *testing.T) {
	// Get current working directory for absolute path tests
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name: "JSON file output creates media directory next to file",
			cfg: &Config{
				Output: "export/data.json",
				Format: "json",
			},
			expected: filepath.Join(cwd, "export/data_media"),
		},
		{
			name: "JSON file output with absolute path",
			cfg: &Config{
				Output: "/tmp/export/data.json",
				Format: "json",
			},
			expected: "/tmp/export/data_media",
		},
		{
			name: "Markdown directory output creates media subdirectory",
			cfg: &Config{
				Output: "export/markdown",
				Format: "markdown",
			},
			expected: filepath.Join(cwd, "export/markdown/media"),
		},
		{
			name: "JSON format but directory output",
			cfg: &Config{
				Output: "export/directory",
				Format: "json",
			},
			expected: filepath.Join(cwd, "export/directory/media"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetMediaDir()
			// GetMediaDir now always returns absolute paths
			if !filepath.IsAbs(result) {
				t.Errorf("GetMediaDir() returned non-absolute path: %v", result)
			}
			if result != tt.expected {
				t.Errorf("GetMediaDir() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeDomainName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"www.example.com", "www.example.com"},
		{"my-site.com", "my-site.com"},
		{"site_with_underscores.com", "site_with_underscores.com"},
		{"site with spaces.com", "site-with-spaces.com"},
		{"site:with:colons.com", "site-with-colons.com"},
		{"site/with/slashes.com", "site-with-slashes.com"},
		{"site\\with\\backslashes.com", "site-with-backslashes.com"},
		{"site*with*asterisks.com", "site-with-asterisks.com"},
		{"site?with?questions.com", "site-with-questions.com"},
		{"site\"with\"quotes.com", "site-with-quotes.com"},
		{"site<with>brackets.com", "site-with-brackets.com"},
		{"site|with|pipes.com", "site-with-pipes.com"},
		{"---multiple---hyphens---.com", "multiple-hyphens.com"},
		{"   spaced.com   ", "spaced.com"},
		{"", "wordpress-site"},
		{"///", "wordpress-site"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeDomainName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeDomainName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Create a test config file
	configContent := `
url: "https://test.example.com"
format: "markdown"
brute_force: true
max_id: 5000
download_media: false
concurrent: 10
timeout: 60
retries: 5
user_agent: "Custom-Agent/1.0"
verbose: true
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Test loading config from file
	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.URL != "https://test.example.com" {
		t.Errorf("LoadConfig() URL = %v, want %v", cfg.URL, "https://test.example.com")
	}

	if cfg.Format != "markdown" {
		t.Errorf("LoadConfig() Format = %v, want %v", cfg.Format, "markdown")
	}

	if cfg.BruteForce != true {
		t.Errorf("LoadConfig() BruteForce = %v, want %v", cfg.BruteForce, true)
	}

	if cfg.MaxID != 5000 {
		t.Errorf("LoadConfig() MaxID = %v, want %v", cfg.MaxID, 5000)
	}
}

func TestLoadConfigWithEnvVars(t *testing.T) {
	// Set environment variables
	err := os.Setenv("WPEXPORT_URL", "https://env.example.com")
	require.NoError(t, err)
	err = os.Setenv("WPEXPORT_FORMAT", "markdown")
	require.NoError(t, err)
	err = os.Setenv("WPEXPORT_MAX_ID", "2000")
	require.NoError(t, err)
	defer func() {
		err = os.Unsetenv("WPEXPORT_URL")
		require.NoError(t, err)
		err = os.Unsetenv("WPEXPORT_FORMAT")
		require.NoError(t, err)
		err = os.Unsetenv("WPEXPORT_MAX_ID")
		require.NoError(t, err)
	}()

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.URL != "https://env.example.com" {
		t.Errorf("LoadConfig() URL from env = %v, want %v", cfg.URL, "https://env.example.com")
	}

	if cfg.Format != "markdown" {
		t.Errorf("LoadConfig() Format from env = %v, want %v", cfg.Format, "markdown")
	}

	if cfg.MaxID != 2000 {
		t.Errorf("LoadConfig() MaxID from env = %v, want %v", cfg.MaxID, 2000)
	}
}

func TestGenerateDefaultOutputDateFormat(t *testing.T) {
	cfg := &Config{
		URL: "https://example.com",
	}

	err := cfg.GenerateDefaultOutput()
	if err != nil {
		t.Fatalf("GenerateDefaultOutput() error = %v", err)
	}

	// Check that output contains date in expected format (YYYY-MM-DD)
	now := time.Now()
	expectedDate := now.Format("2006-01-02")

	if !filepath.IsAbs(cfg.Output) {
		// Extract filename from path
		filename := filepath.Base(cfg.Output)

		// Should contain domain and date
		if !strings.HasPrefix(filename, "example.com.") {
			t.Errorf("Output filename should start with domain, got: %s", filename)
		}

		if !strings.Contains(filename, expectedDate) {
			t.Errorf("Output filename should contain date %s, got: %s", expectedDate, filename)
		}
	}
}
