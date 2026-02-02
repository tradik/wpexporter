# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.0] - 2026-01-30

### Added
- **Resume/Checkpoint**: New `--resume` flag to save progress and resume interrupted exports. Checkpoint file (`.wpexport_checkpoint.json`) is saved after each API page fetch and deleted on successful completion
- **Rate Limiting**: New `--rate-limit` flag to add delay between API requests (in milliseconds) to prevent server overload and rate limiting timeouts
- **Media Filtering**: New `--no-media` alias for `--download-media=false` and `--relevant-media-only` flag to download only featured images and images embedded in content
- **URL Path Filtering**: New `--path-filter` flag to filter posts/pages by URL path pattern (e.g., `--path-filter=/fr/arts/`)
- **SEO Metadata Extraction**: New `--assisted-crawl` flag to crawl URLs and extract SEO metadata including:
  - Page titles from `<title>` tags
  - Meta descriptions and keywords
  - Open Graph tags (og:title, og:description, og:image)
  - Canonical URLs
- SEO fields are included in JSON export and Markdown frontmatter when using `--assisted-crawl`
- **Interactive Password Prompt**: When `--auth-user` is provided without `--auth-pass`, the tool now prompts for password input securely (hidden input)
- New Makefile targets: `vet`, `sec`, `check`, `test-coverage`
- Restored and updated golangci-lint configuration
- Added comprehensive test coverage for cmd packages (wpexportjson: 26.1%, wpxmlrpc: 15.7%)

### Changed
- Updated README with new CLI options and SEO documentation
- Enhanced test coverage for new features
- Overall test coverage improved to 80.2%

### Fixed
- Clarified `--download-media` flag behavior in documentation
- Added validation for `--path-filter` to detect when value looks like a flag (prevents accidental `--path-filter --zip` confusion)
- Fixed Windows build compatibility for `term.ReadPassword` (syscall.Stdin type conversion)

## [1.2.1] - 2026-01-21

### Added
- **Authentication Support**: Export content from password-protected WordPress sites/APIs (Basic Auth and Bearer Token)
  - `--auth-user` / `--auth-pass`: Credentials for Basic Authentication
  - `--auth-token`: Bearer token for API authentication
  - Handles 401 Unauthorized responses gracefully by retrying with provided credentials

## [1.2.0] - 2026-01-21

### Added
- **Shopify CSV Export Format**: New `shopify` export format for migrating WordPress content to Shopify
  - Generates Shopify-compatible product CSV files
  - Converts WordPress posts/pages to Shopify products
  - Maps categories to product types, tags to Shopify tags
  - Includes SEO fields (title, description)
  - Supports featured images and additional content images
  - Creates separate CSV files for posts, pages, and combined products
  - Exports site metadata for reference
- **Magento 2 CSV Export Format**: New `magento` export format for migrating WordPress content to Magento
  - Generates Magento 2-compatible product CSV files (57 columns)
  - Converts WordPress posts/pages to simple products
  - Maps categories to Magento category paths (Default Category/Name)
  - Uses tags as meta keywords for SEO
  - Supports featured images and additional content images
  - Includes URL key generation for SEO-friendly URLs
  - Creates separate CSV files for posts, pages, and combined products
  - Exports site metadata for reference
- **WooCommerce Support**: Automatically detects and exports WooCommerce products (requires wc/v3 API) and maps them to Shopify/Magento product fields including price, stock, and variations
- **Content Filtering**: New flags to control export scope:
  - `--no-posts`: Skip blog posts
  - `--no-pages`: Skip pages
  - `--no-products`: Skip WooCommerce products
  - Creates separate CSV files for posts, pages, and combined products
  - Exports site metadata for reference
- `--zip` flag to create ZIP archive of export
- `--no-files` flag to remove export files after creating ZIP (requires --zip)
- Dual licensing under MIT and BSD 3-Clause (see LICENSE)

## [1.0.0] - 2025-12-05

### Added
- Initial stable release with Go 1.24
- WordPress REST API client for content discovery
- Brute force content ID enumeration
- JSON and Markdown export formats
- Media download functionality (images and videos)
- CLI interface with Cobra
- Configuration management with Viper
- Progress tracking with progress bars
- Concurrent processing support
- Comprehensive documentation and README
- Makefile with development automation
- Cross-platform build support (Linux, macOS, Windows, FreeBSD)
- GitHub Actions CI/CD pipeline with auto-versioning
- Docker support with multi-arch builds
- XML-RPC export tool (wpxmlrpc)

### Fixed
- Fixed media directory path must be absolute error

### Security
- Fixed G301 security issues: Changed directory permissions from 0755 to 0750 for better security
- Fixed G306 security issues: Changed file permissions from 0644 to 0600 for better security  
- Fixed G304 security issue: Added comprehensive path validation to prevent directory traversal attacks
- Added file path sanitization and validation in media downloader
- Enhanced security by ensuring all file operations are contained within designated directories

## [0.1.0] - 2024-01-07

### Added
- Initial release
- Basic WordPress content export functionality
