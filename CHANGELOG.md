# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.4.0] - 2026-02-04

### Added
- **Quiet Mode**: New `--quiet` / `-q` flag suppresses all output, only returns exit code (useful for scripting and automation)
- **HTML to Markdown Conversion**: New `--flat-html` flag converts HTML content to clean Markdown format
  - Built-in support for standard HTML elements (h1-h6, p, strong, em, a, img, ul, ol, blockquote, code, pre, hr)
  - Built-in support for **Bricks Builder** CSS classes (brxe-heading, brxe-text, brxe-list, brxe-image)
  - Custom conversion rules via `flat_html_rules` in config.yaml for site-specific HTML class mappings
- **Skip Tags Export**: New `--no-tags` flag to skip exporting tags
- **Page Builder Configuration Examples**: Added comprehensive config examples in documentation for:
  - Bricks Builder
  - Elementor
  - Divi Builder
  - Oxygen Builder
  - GenerateBlocks
  - Combined multi-builder configurations

### Fixed
- Fixed `--crawl-content` HTML extraction bug where closing tags were disappearing due to non-greedy regex matching. Implemented balanced tag extraction algorithm that properly handles nested HTML elements.

### Changed
- **Optimized Crawling**: When both `--assisted-crawl` and `--crawl-content` are enabled, pages are now fetched only once to extract both SEO metadata and content (previously required two separate HTTP requests per page)

## [1.3.8] - 2026-02-04

### Added
- **Content Crawling**: New `--crawl-content` flag to extract content from pages built with page builders (Bricks, Elementor, etc.) that store content outside the standard WordPress content field
- **Skip Empty Content**: New `--skip-empty-content` flag to exclude posts/pages with empty content from export
- **Version Flag**: Added `--version` flag to display application version
- Tests for new content crawling and filtering functionality

### Changed
- Cleaned up CLI help output - removed verbose examples, added feature summary
- Updated golangci-lint complexity threshold for main export function

## [1.3.7] - 2026-02-04

### Added
- Output directory permission check before starting export (fails fast if no write permissions)

## [1.3.6] - 2026-02-04

### Fixed
- `--no-media` flag now properly skips media fetching from API (previously only skipped downloading)
- Removed redundant "(default 30)" from `--timeout` flag description

## [1.3.5] - 2026-02-02

### Added
- **8 New Export Formats** for popular CMS and e-commerce platforms:
  - **Wix**: JSON export for [Wix](https://www.wix.com/) blog migration
  - **Squarespace**: WXR-compatible XML for [Squarespace](https://www.squarespace.com/) import
  - **Webflow**: CSV files for [Webflow](https://webflow.com/) CMS collections
  - **Weebly**: XML and JSON dual export for [Weebly](https://www.weebly.com/)
  - **PrestaShop**: Semicolon-delimited CSV for [PrestaShop](https://www.prestashop.com/) products
  - **Ghost**: JSON export for [Ghost](https://ghost.org/) CMS migration
  - **Strapi**: JSON export for [Strapi](https://strapi.io/) v4 headless CMS with separate collection files
  - **Contentful**: JSON export for [Contentful](https://www.contentful.com/) with content types and assets
- Comprehensive test coverage for all new exporters (93.7% for export package)
- Platform links in documentation for all supported export formats

### Changed
- Updated `--format` flag to support 14 formats: json, markdown, shopify, magento, wordpress, drupal, wix, squarespace, webflow, weebly, prestashop, ghost, strapi, contentful
- Updated golangci-lint configuration for better code quality enforcement
- Enhanced documentation with links to all supported platforms

## [1.3.4] - 2026-02-02

### Added
- **WordPress WXR Export Format**: New `wordpress` export format generating WXR (WordPress eXtended RSS) XML files
  - Compatible with WordPress import/export system
  - Exports posts, pages, media, categories, tags, and authors
  - Includes featured images and SEO metadata as post meta
  - Full WXR 1.2 specification support
- **Drupal Export Format**: New `drupal` export format generating Drupal-compatible JSON files
  - Compatible with Drupal's Migrate module and migrate_source_json plugin
  - Exports nodes (articles and pages), taxonomy terms, users, and media
  - Generates separate JSON files for each content type for flexible migration
  - Supports Drupal 8/9/10 field structure

### Changed
- Updated `--format` flag to support additional formats: `wordpress` and `drupal`

## [1.3.3] - 2026-02-02

### Added
- **Skip Users**: New `--no-users` flag to skip exporting users
- **Timeout Flag**: New `--timeout` flag to configure HTTP request timeout in seconds (default 30)

### Fixed
- Fixed JSON parsing error for users/categories/tags when WordPress returns `meta` field as object instead of array
- Users fetching is now graceful - errors don't stop the export, just warn and continue

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
