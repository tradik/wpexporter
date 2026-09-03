# WordPress Export JSON

[![Go Version](https://img.shields.io/badge/Go-1.27.0+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT%20%2F%20BSD--3--Clause-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-ghcr.io-2496ED?style=flat&logo=docker)](https://github.com/tradik/wpexporter/pkgs/container/wpexporter)
[![Documentation](https://img.shields.io/badge/docs-wpexporter.tradik.com-0050A6?style=flat&logo=readthedocs&logoColor=white)](https://wpexporter.tradik.com/)
[![Docs Site](https://github.com/tradik/wpexporter/actions/workflows/docs-site.yml/badge.svg)](https://github.com/tradik/wpexporter/actions/workflows/docs-site.yml)
[![CI/CD](https://github.com/tradik/wpexporter/actions/workflows/ci.yml/badge.svg)](https://github.com/tradik/wpexporter/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/tradik/wpexporter?include_prereleases)](https://github.com/tradik/wpexporter/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/tradik/wpexporter)](https://goreportcard.com/report/github.com/tradik/wpexporter)
[![GitHub Stars](https://img.shields.io/github/stars/tradik/wpexporter?style=social)](https://github.com/tradik/wpexporter/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/tradik/wpexporter?style=social)](https://github.com/tradik/wpexporter/network/members)
[![GitHub Issues](https://img.shields.io/github/issues/tradik/wpexporter)](https://github.com/tradik/wpexporter/issues)
[![Homebrew](https://img.shields.io/badge/Homebrew-tradik%2Ftap-FBB040?style=flat&logo=homebrew)](https://github.com/tradik/homebrew-tap)
[![Snap](https://img.shields.io/badge/Snap_Store-wpexporter-82BEA0?style=flat&logo=snapcraft)](https://snapcraft.io/wpexporter)

> **Repository:** [github.com/tradik/wpexporter](https://github.com/tradik/wpexporter)
> **Documentation:** [wpexporter.tradik.com](https://wpexporter.tradik.com/) — the guides in
> [docs/](docs/), published from this repository on every push.

A comprehensive WordPress content export toolkit.

**`wpexporter`** is the single entry point — one command covering the whole toolkit:

```bash
wpexporter export --url https://example.com -f ssg   # REST API export
wpexporter xmlrpc export --url ... --username ...    # XML-RPC export
wpexporter mcp                                        # MCP server for AI assistants
```

The three tools are also installed as standalone binaries, which behave identically:

| Binary | Role | Umbrella equivalent |
|---|---|---|
| **wpexportjson** | REST API exporter with brute force content discovery | `wpexporter export` |
| **wpxmlrpc** | XML-RPC exporter for authenticated access | `wpexporter xmlrpc` |
| **wpmcp** | MCP (Model Context Protocol) server for AI assistants | `wpexporter mcp` |

**Export to 14+ popular platforms** including e-commerce systems ([Shopify](https://www.shopify.com/), [Magento](https://business.adobe.com/products/magento/magento-commerce.html), [PrestaShop](https://www.prestashop.com/)), traditional CMS platforms ([WordPress](https://wordpress.org/), [Drupal](https://www.drupal.org/), [Wix](https://www.wix.com/), [Squarespace](https://www.squarespace.com/), [Webflow](https://webflow.com/), [Weebly](https://www.weebly.com/)), and headless CMS solutions ([Ghost](https://ghost.org/), [Strapi](https://strapi.io/), [Contentful](https://www.contentful.com/)), plus JSON and Markdown formats with full media download support.

## Features

### wpexporter (REST API Client)
- 🔍 **Complete Content Discovery**: Scans WordPress REST API for posts, pages, and media
- 🚀 **Brute Force Mode**: Attempts to discover unlisted content by ID enumeration
- 📁 **Multiple Export Formats**: JSON, Markdown, SSG, Shopify, Magento, WordPress, Drupal, Wix, Squarespace, Webflow, Weebly, PrestaShop, Ghost, Strapi, and Contentful
- 🏗️ **Static Site Generator Output**: `-f ssg` writes a drop-in content source — URL-mirroring paths, single-spelled front matter, cleaned body HTML
- ♿ **Accessibility Report**: `--report-a11y` flags WCAG 2.2 contrast and missing alt-text issues before you publish
- 🛒 **E-commerce Integration**: Export to [Shopify](https://www.shopify.com/), [Magento](https://business.adobe.com/products/magento/magento-commerce.html), [PrestaShop](https://www.prestashop.com/) CSV formats
- 🌐 **CMS Migration**: Export to [WordPress](https://wordpress.org/), [Drupal](https://www.drupal.org/), [Wix](https://www.wix.com/), [Squarespace](https://www.squarespace.com/), [Webflow](https://webflow.com/), [Weebly](https://www.weebly.com/)
- 📝 **Headless CMS Support**: Export to [Ghost](https://ghost.org/), [Strapi](https://strapi.io/), [Contentful](https://www.contentful.com/) JSON formats
- 🛍️ **WooCommerce Support**: Detects and exports WooCommerce products automatically
- 🧩 **Custom Post Types**: Discovers the types a theme or plugin registered (Services, Portfolio, Team, …) and exports their entries alongside pages — no flag needed
- 🎨 **Theme Palette**: `--assisted-crawl` records the site's own colours (primary/secondary/accent/text/background/link) from its CSS custom properties
- 💬 **Reader Comments**: Every approved comment, threaded and addressed by page URL, in `comments.json` — skip with `--no-comments`
- 🧹 **Content Filtering**: Control what to export with `--no-posts`, `--no-pages`, `--no-products`, `--no-custom-types` and `--no-comments` flags
- 🖼️ **Media Download**: Downloads images and videos with content
- ⚡ **Concurrent Processing**: Fast parallel downloads and processing
- 📊 **Progress Tracking**: Real-time progress bars and status updates
- 🛠️ **Configurable**: Flexible configuration options via CLI or config file
- 🔀 **Both REST API Spellings**: Reads a site that serves only `/?rest_route=` — plain permalinks, or a plugin hiding `/wp-json/` — and says so in the report; a site answering normally spends no extra request on the question
- 🕰️ **Pre-4.7 WordPress**: An install older than the content API is named as such and read from its sitemap — each address fetched and written as a page — instead of exporting as an empty site
- ⚡ **Built with Go 1.27**: `encoding/json` v2 roughly halves decoding time — the largest single cost in an export — and the conversion passes compile their patterns once instead of once per document
- 🔓 **Shop Without Keys**: Products come from WooCommerce's public storefront API — prices, images, categories and stock — so a migration does not stop while somebody hunts for consumer keys
- 🏠 **Front Page and Blog Identified**: `show_on_front`, `front_page` and `posts_page` are recorded in `metadata.json`, read from the site's settings or from its own rendered markup — never guessed from a slug
- 🛒 **The Catalog on Disk**: `markdown` and `ssg` write every product, with price, SKU, stock and images in front matter, at the address the shop's own navigation links to
- 🎨 **Theme Styling Survives**: A heading carrying a class Markdown cannot express travels as HTML, so the migrated page keeps the color its stylesheet gives it — `--preserve-styling auto|none|all`
- 🌍 **Language-Independent**: The theme's "continue reading" is learned from its own repetition rather than matched against a list of English phrases, and every text threshold counts characters instead of bytes — so a Japanese or Polish site is judged by the same rule as an English one
- 🔊 **No Silent Truncation**: The sitemap index is read to the end by default, and a post type set aside as theme bookkeeping is named in the report with the flag that overrides it
- 🧭 **Finds Things by Asking**: The sitemap comes from `robots.txt` when it is not where anyone would guess, the feed from the page's own `<link rel="alternate">`, and the REST API from whichever of its two spellings answers
- 🎛️ **Sites Differ, So the Rules Bend**: `--builder-classes`, `--boilerplate-classes`, `--content-selector` and `--crawl-content-mode` let one site's theme, builder and markup be named instead of guessed
- 🌐 **No Authentication**: Works with public WordPress REST API

### wpxmlrpc (XML-RPC Client)
- 🔐 **Authenticated Access**: Access private content with WordPress credentials
- 📜 **Legacy Support**: Works with older WordPress versions
- 🔒 **Secure Authentication**: Username/password or application passwords
- 📊 **Complete Export**: Posts, pages, media, categories, tags, and users
- 📁 **Multiple Formats**: JSON and Markdown export support
- 🛡️ **XML-RPC Protocol**: Direct WordPress XML-RPC API integration

### wpmcp (MCP Server)
- 🤖 **AI Integration**: Enables Claude and other AI assistants to interact with WordPress
- 🔧 **8 Tools**: list_formats, get_site_info, list_posts, list_pages, export_site, get_post, list_categories, list_media
- 📡 **JSON-RPC 2.0**: MCP over stdio, both eras — the current per-request-versioned revision (`2026-07-28`) and the `initialize` handshake (`2025-11-25` and earlier), chosen per request
- 🎯 **Pinnable**: `--protocol modern|legacy|<revision>` when a client has to be held to one
- 🔐 **Authentication Support**: Basic Auth and Bearer token support
- ⚡ **Fast Response**: Optimized for quick AI assistant interactions

## Documentation

Every guide lives in [docs/](docs/) and is published at
**[wpexporter.tradik.com](https://wpexporter.tradik.com/)** on each push.

| Guide | What it covers |
|---|---|
| [Installation](docs/INSTALL.md) | Homebrew, Snap, Docker, `go install`, from source |
| [Quick start](docs/QUICKSTART.md) | The first export, over REST and over XML-RPC |
| [Command line reference](docs/CLI.md) | Every flag, the config file, resume/checkpoint |
| [Export formats](docs/FORMATS.md) | All fifteen formats and what each one writes |
| [E-commerce formats](docs/FORMATS-ECOMMERCE.md) | Shopify, Magento, PrestaShop |
| [CMS and headless formats](docs/FORMATS-CMS.md) | WordPress WXR, Drupal, Ghost, Strapi, Contentful |
| [Website builder formats](docs/FORMATS-BUILDERS.md) | Wix, Squarespace, Webflow, Weebly |
| [Static site generator format](docs/SSG-FORMAT.md) | `-f ssg`: layout, front matter, content cleanup |
| [Media and URL rewriting](docs/MEDIA.md) | Downloads, path styles, link styles, size variants |
| [SEO metadata extraction](docs/SEO.md) | `--assisted-crawl`, site marketing metadata |
| [HTML to Markdown](docs/MARKDOWN.md) | `--flat-html`, custom rules, Gutenberg blocks |
| [Page builder rules](docs/PAGE-BUILDERS.md) | Bricks, Elementor, Divi, Oxygen, GenerateBlocks |
| [Navigation menus](docs/MENUS.md) | Menu export and why it needs credentials |
| [Reader comments](docs/COMMENTS.md) | `comments.json`, addressed by page URL |
| [Accessibility report](docs/ACCESSIBILITY.md) | `--report-a11y`, WCAG 2.2 checks |
| [MCP server](docs/MCP.md) | Driving the exporter from Claude and other assistants |
| [XML-RPC manual](docs/XMLRPC_MANUAL.md) | The authenticated path, for sites with REST disabled |
| [Architecture](docs/ARCHITECTURE.md) | How the pieces fit together |
| [Development](docs/DEVELOPMENT.md) | Building, testing, the source tree |

## Install

```bash
brew install tradik/tap/wpexporter    # macOS / Linux
sudo snap install wpexporter          # Linux
docker pull ghcr.io/tradik/wpexporter:latest
```

Other routes — `go install`, release tarballs, building from source — are in the
[installation guide](docs/INSTALL.md).

## First export

```bash
# Everything the REST API lists, as a static-site content source
wpexporter export --url https://example.com -f ssg

# Private content, over XML-RPC
wpexporter xmlrpc export --url https://example.com --username admin --password '…'
```

More recipes in the [quick start](docs/QUICKSTART.md); every flag is in the
[command line reference](docs/CLI.md).

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is dual-licensed under the **MIT License** and **BSD 3-Clause License**.
See the [LICENSE](LICENSE) file for full license texts and choose the one that best fits your use case.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for a list of changes and version history.
