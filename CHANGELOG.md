# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.8.1] - 2026-08-13

Markdown/SSG export fixes reported while migrating live sites, plus site-level
marketing metadata. Closes #19, #20, #21, #22, #23, #24.

### Added
- **Site-level marketing metadata (#24)**, written to `metadata.json` under
  `marketing` when `--assisted-crawl` is used. The crawler fetches the home page once
  and records verification tokens (`google-site-verification`,
  `facebook-domain-verification`, `msvalidate.01`, `yandex-verification`, …), social
  defaults (`og:site_name`, default `og:image`, `twitter:site`), social profile links
  found in `<header>`/`<footer>` keyed by network, brand assets (favicon — largest
  declared size wins, apple-touch-icon, logo) and `theme-color`. Relative references
  resolve against the page, so the values are usable without knowing the source host.
  Best-effort throughout: an undeclared field is omitted, never invented. The same
  fetch also feeds analytics detection, so a GTM container present only on the home
  page is no longer missed.
- **`--ssg-sections` (#20)** for `-f markdown`: emits `## Excerpt` / `## Content`
  body markers — which is what ssg's parser reads — and omits the leading `# Title`
  H1 that otherwise duplicates the frontmatter title. The frontmatter `excerpt:` key
  is kept for other consumers. Default output is unchanged.

### Fixed
- **Orphaned Gutenberg block tags in markdown (#21).** The converter matched literal
  `<h2>`/`<p>`/`<ul>` only, so `<h2 class="wp-block-heading">` and friends kept their
  opening tag while their closing tag was stripped — every CommonMark renderer then
  treated the line as raw HTML and printed the `**` and `- ` markers literally. Block
  conversion is now attribute-aware. Self-contained elements (`<img>`, `<figure>`,
  `<a>`) are still passed through as complete HTML, which is valid in Markdown and
  what the SSG format and the media rewriter expect.
- **Entities in frontmatter text fields (#23).** `title.rendered` is rendered HTML and
  legitimately contains entities (`Domowe Kino &#8211; Warszawa`), which consumers put
  verbatim into `<title>`, meta descriptions and feeds. Title, author and taxonomy
  names are now flattened to plain text (excerpt already was), and the body H1 decodes
  too.
- **In-content images missed by `--relevant-media-only` (#22).** `data.Media` feeds
  both the downloader and the URL-rewriter index, so an image the filter failed to
  recognize was neither downloaded nor rewritable and kept its absolute
  `wp-content` URL. The filter now also reads `srcset` and `data-src` (Gutenberg
  figures, lazy-loading themes), and matches size/`-scaled`-insensitively, so content
  embedding `photo-1024x768.jpg` matches an attachment whose `source_url` is
  `photo-scaled.jpg`. The rewriter additionally indexes the un-`-scaled` name.
- **Snap exports to `/tmp` silently vanished (#19).** A strictly confined snap gets a
  private `/tmp`, so `-o /tmp/...` wrote a complete export into
  `/tmp/snap-private-tmp/snap.wpexporter/tmp/...` — root-owned and invisible — while
  reporting success. The run now fails before the export starts, naming the real
  destination and suggesting a path under `$HOME`.

## [1.8.0] - 2026-08-08

### Added
- **`wpexporter` command.** The project has always been called wpexporter — repository, Go
  module, Homebrew formula, Snap package, Docker image — but there was no command by that
  name. Installing it gave three differently named binaries, and the release archive was
  named after one of them, so `wpexporter` looked missing. There is now one entry point:

  | Command | Equivalent |
  |---|---|
  | `wpexporter export` | `wpexportjson export` |
  | `wpexporter xmlrpc` | `wpxmlrpc` |
  | `wpexporter mcp` | `wpmcp` |

  The three binaries remain, unchanged, for anyone scripting against them. The REST
  exporter's subcommand sits at the umbrella's top level since it is the common case; the
  other two mount as groups, because each defines its own `--config` and `--verbose` and
  hoisting them all onto one root would collide.
- **Full metadata extraction** (`--assisted-crawl`). The crawler previously read a fixed list
  of nine fields and silently discarded everything else. It now extracts:
  - **named SEO fields**: `robots`, `og:type`, `og:url`, `og:site_name`, `og:locale`,
    `twitter:card`, `twitter:title`, `twitter:description`, `twitter:image`, `twitter:site`,
    `article:published_time`, `article:modified_time`, `article:author`, `article:section`,
    alongside the existing title/description/keywords/og/canonical/lang/hreflangs
  - **every other meta tag**, into a `meta` map keyed by its `name`, `property`, `itemprop`
    or `http-equiv`. Plugins and themes put real information in tags nobody anticipated; a
    generator can ignore a key it does not recognise, but it cannot recover one the export
    dropped.
  - **`application/ld+json` blocks**, preserved raw. Rank Math and Yoast emit structured data
    there that appears in no meta tag.
  - **tracking identifiers**, collected site-wide into `metadata.json`: GA4, Universal
    Analytics, Google Tag Manager, Google Ads, Meta Pixel, Hotjar, Microsoft Clarity,
    LinkedIn and TikTok. Detection matches the identifier's shape rather than the surrounding
    snippet, so it survives however a plugin minified or wrapped it. They belong to the site
    rather than to any one post.
- **`--extract-meta`** (`all` | `none` | allow-list, config key `extract_meta`): controls
  which unnamed meta tags are kept. Defaults to `all`, because losing data is worse than
  carrying some noise.
- **Navigation menus (#16)**, exported into `metadata.json` as a `menus` array with each
  menu's name, slug, locations and ordered items (title, URL, parent, order, type, object).
  Item URLs follow `--link-style`, so navigation matches the exported permalinks; an item on
  another host keeps its absolute URL. Items are sorted by `menu_order`, which is what the
  site renders by. `--no-menus` (config key `no_menus`) skips them.

  **Correction to the issue's premise:** menus are *not* publicly readable. WordPress gates
  `/wp/v2/menus` behind `edit_theme_options`, so a public REST API answers 401 however the
  menus are configured — verified against a live site. Menus therefore need
  `--auth-user`/`--auth-pass` or `--auth-token`. Without credentials the export prints a note
  saying exactly that and carries on rather than failing.

### Fixed
- **Site information was exported empty (#15)**. Three separate faults compounded:
  - `GetSiteInfo` asked `/wp/v2/settings`, which needs authentication and returns 401 on a
    public site, then fell back to `/wp-json/wp/v2` — the *route index*, which carries no
    site fields at all. It unmarshalled cleanly into `SiteInfo` (valid JSON, no matching
    keys), so the "endpoint failed" branch never ran and every field came out blank.
  - Even when `/wp/v2/settings` was reachable, the site title was read as `name`; the
    endpoint calls it `title`, so `Name` was always empty.
  - `metadata.json` carried no `site` object at all.

  Identity now comes from the unauthenticated `/wp-json/` root (`name`, `description`,
  `url`, `home`, `timezone_string`/`gmt_offset`), with `/wp/v2/settings` overlaid where
  reachable for the fields only it has (admin email, date and time formats, start of week,
  language). `metadata.json` gains a `site` object. A transport failure is still an error;
  a 401, a 404 or an unreadable body degrades to the configured URL instead.
- **Release tarballs shipped binaries as `0644`**, so extracting one and running the binary
  gave `permission denied` until you `chmod +x` by hand. `actions/upload-artifact` zips
  internally and does not preserve the executable bit, so the build → release artifact
  round-trip dropped it. Homebrew and Snap masked this by setting the mode themselves on
  install and Docker builds from source, so only the direct-download path was affected.
  Present in v1.7.9 and v1.7.10.

### Changed
- `extractSEO` and `extractSEOAndContent` each had their own copy of the extraction block, so
  a field added to one silently missed the other. Both now call a single `populateSEO`.
- **The three command trees moved to `internal/cli/`**, leaving `cmd/*/main.go` as thin
  wrappers. `package main` cannot be imported, so the umbrella could not otherwise reuse
  them.
- **Build identity moved to `internal/version`**, and the linker now stamps that one package.
  Each command previously declared its own `main.Version`, so `-X main.Version=…` reached
  whichever binary was being built and missed the others — and `-X main.BuildTime=…` missed
  all three, since two of them called the field `BuildDate`. `--version` now reports the
  version, commit and build time consistently across every binary.
- Removed the `cobra.OnInitialize(initConfig)` scaffolding: `initConfig` was an empty
  function in every tool, and a package-level initializer would have run for all three once
  they shared a process.

## [1.7.10] - 2026-08-08

Release-plumbing fixes. No changes to export behaviour.

### Fixed
- **Binaries reported the wrong version.** `wpexportjson --version` in the 1.7.9 release
  printed `v1.7.8-7-g414285b`. The Makefile derived the version from `git describe`, and
  CI's `build` job runs *before* the `release` job creates the tag — so every release
  stamped its binaries with the **previous** tag plus a commit count, and `-X main.Version`
  overrode the correct hardcoded value. The shipped 1.7.9 binaries were the right code;
  only the version string was wrong.
- **`wpmcp` was missing from the Docker image.** The Dockerfile built and copied only
  `wpexportjson` and `wpxmlrpc`, so image users could not run the MCP server even though it
  ships in every other channel (release archives, Homebrew, Snap).

### Changed
- **The release version now comes from a `VERSION` file** rather than from incrementing the
  latest tag. Two reasons: the old scheme could only ever produce a **patch** bump, making a
  minor release impossible without hand-tagging; and an explicit version is a reviewable line
  in the diff rather than something inferred at release time. CI reads it, validates the
  `MAJOR.MINOR.PATCH` shape, and skips the release with a clear warning if that tag already
  exists (a forgotten bump).
- Version resolution is computed once in a dedicated CI job and consumed by both `build` and
  `release`, replacing the duplicated logic.
- Snap's build reads the same `VERSION` file instead of `git describe`.

## [1.7.9] - 2026-08-08

Media URL localisation fixes (issues #11, #13) and dependency security updates.

### Fixed
- **`featured_image` and `og_image` are localised too (#13)**: URL rewriting only ran
  over `Content.Rendered` and `Excerpt.Rendered`, so both front-matter image fields kept
  the original absolute `wp-content/uploads/…` URL — a static site built from the export
  lost its hero image and its Open Graph image the moment the source host was retired,
  the same failure as #11 one field over. Both now resolve through the same index as the
  body content. `og_image` is scraped rather than read from the media library, so one
  pointing at a CDN or third-party host resolves to nothing and correctly stays absolute.
- `canonical_url`, `link` and `hreflangs` are not touched by media rewriting: they are
  addresses of the source site rather than assets. Their form is controlled separately by
  the new `--link-style` flag.
- **`src`/`href` are now localised like `srcset` (#11)**: URL rewriting matched the
  REST API's `source_url` as an exact string, so only references written in the site's
  present-day URL form were replaced. WordPress stores `post_content` with whatever form
  was current when the post was written, so within a single `<img>` the dynamically
  generated `srcset` was localised while `src` — and the wrapping `href` — kept the
  original absolute `wp-content/uploads/…` URL. An export made in order to retire the
  source host therefore 404'd on every image. Matching is now scheme- and host-insensitive
  (keyed on the upload path), so historic `http://`, `www`, former-domain,
  protocol-relative, root-relative and query-string forms all resolve to the exported file.
- **Stale size variants are remapped**: a registered-size change regenerates thumbnails
  but never rewrites the markup already linking to the old dimensions. A reference to a
  no-longer-generated `photo-300x199.jpg` now resolves to the closest surviving width
  (`photo-300x225.jpg`) instead of being emitted as a dead path. `--verbose` logs each remap.
- Media paths are built with `path.Join` rather than `filepath.Join`, so exports produced
  on Windows no longer contain backslash-separated URLs.

### Added
- **`-f ssg`** (#11 proposal 2): a drop-in content source for
  [spagu/ssg](https://github.com/spagu/ssg) and other static site generators. Where
  `markdown` is a faithful dump of what WordPress returned, `ssg` is a content source:
  - pages **nested to mirror their URL** (`/a/b/` → `pages/a/b.md`), posts under their
    category (never at the top of `posts/`, which the generator requires)
  - **single-spelled front matter** — `title`, `description`, `category` rather than
    WordPress's `seo_title`/`og_title`, `meta_description`/`og_description`,
    `categories`/`category_ids`. Empty values emit no key at all
  - `author` resolved to a name, `link` root-relative by default
  - body HTML cleaned (see below)
- **Content cleanup for `ssg`**: HTML entities decoded to UTF-8; `alt` filled in from the
  media library's `alt_text` (WCAG 2.2 SC 1.1.1) without ever overwriting an existing one;
  WordPress presentation classes (`wp-image-*`, `size-*`, `align*`, `attachment-*`,
  `wp-block-*`) dropped while authored classes are kept; a `title` that merely repeats the
  filename dropped; `loading`/`decoding`/`sizes` dropped.
- **`--report-a11y`** (config key `report_a11y`): writes `a11y-report.md` next to the export,
  flagging inline colours below WCAG 2.2 SC 1.4.3's 4.5:1 minimum and images with no alt text
  (SC 1.1.1). Contrast is measured against a declared `background-color` where present and
  against white otherwise. It changes nothing about the export — any WordPress site of a
  certain age carries these, and knowing before publishing is the point.
- **`--media-path-style`** (`root` | `relative`, config key `media_path_style`): controls
  the form of rewritten media paths.
- **`--link-style`** (`absolute` | `root`, config key `link_style`): controls the form of the
  address fields `link`, `canonical_url` and `hreflangs[].href`. #11 asks for the
  root-relative form (it preserves each URL, and its search ranking, when the site is
  rebuilt at the same paths); #13 states the absolute form is correct (a consumer needs the
  original URL to derive the target one). Both are right for their case, so this is a flag
  rather than a decision. Default `absolute` keeps existing behaviour. Only same-host
  addresses are converted — an hreflang alternate or canonical on a foreign host is left
  untouched — and query strings and fragments are preserved.

### Changed
- **BREAKING (json/markdown output)**: rewritten media paths are now root-relative
  (`/media/images/123_photo.jpg`) by default. The previous relative form
  (`media/images/123_photo.jpg`) only resolved for content served from the site root — a
  page at `/about/team/` resolved it to `/about/team/media/images/…`. Pass
  `--media-path-style relative` to restore the pre-1.7.9 output.
- Each size variant now rewrites to **its own** exported file rather than collapsing to the
  full-size image, preserving responsive `srcset` behaviour.

### Security
- **GHSA-5cv4-jp36-h3mw** (medium): `golang.org/x/net` 0.49.0 → 0.57.0, fixing a denial of
  service in the HTML parser. The vulnerable path was not reachable from this codebase
  (`govulncheck` reported 0 affecting vulnerabilities), so this is hardening rather than an
  exploitable fix.
- Dependency upgrades: `golang.org/x/term` 0.39.0 → 0.45.0,
  `github.com/go-resty/resty/v2` 2.17.1 → 2.17.2, `golang.org/x/sys` 0.40.0 → 0.47.0,
  `golang.org/x/text` 0.33.0 → 0.40.0.

### Changed (markdown format)
- **HTML entities are decoded to UTF-8** in markdown body content. The exported file is
  UTF-8, so `&#8211;` and `&hellip;` were noise that survived into the rendered page. The
  five HTML-significant entities (`&lt;`, `&gt;`, `&amp;`, `&quot;`, `&#39;`) stay encoded —
  decoding those would turn escaped markup into live markup.
- **The `excerpt` no longer carries the theme's "Continue reading →" anchor.** It is
  navigation rather than content and was landing in `<meta name="description">`. Only an
  anchor that looks like read-more chrome (WordPress's `more-link` class, or recognised link
  text) is removed; an excerpt legitimately ending in a link keeps it.

### Documentation
- README documents `-f ssg` (layout, front-matter contract, content cleanup), `--report-a11y`,
  and a **per-format URL contract table** stating for every one of the 15 formats whether
  media URLs and address fields are localised (#11 proposal 3). "Media URL Mapping" rewritten:
  the matched URL forms, which fields are localised and which stay absolute, the exported
  directory layout with per-type subfolders, `--media-path-style`, and the stale-variant
  remap. Manpage, `config.example.yaml` and `docs/ARCHITECTURE.md` updated to match.

### Internal
- URL rewriting is now a `media.URLRewriter` built **once per export** rather than an
  index rebuilt for every field of every post, which was O(posts × media).

## [1.7.8] - 2026-07-09

Audit medium-severity round (SEC-002, SEC-003, GO-002, GO-003, OPS-002, OPS-003).

### Added
- **`--scan-range START-END`**: rescan a specific inclusive ID range for
  posts/pages/media and merge items not already fetched (deduped by ID). Wires up
  the previously-unreachable `Scanner.ScanSpecificRange` (GO-002).
- **`--max-media-mb`** and `config.max_media_bytes`: configurable per-file media
  download cap (SEC-002).

### Security
- **SEC-002**: bounded response/download sizes — REST responses capped at 64 MiB,
  XML-RPC at 32 MiB, and media downloads at a configurable cap (default 2 GiB) with
  oversized files dropped rather than left truncated.
- **SEC-003**: the basic-HTML sanitizer now decodes HTML entities and strips
  control/whitespace characters before checking a URL scheme against an allow-list
  (http/https/mailto/tel/ftp), defeating `javascript:`/`data:`/`vbscript:` obfuscation
  via tabs, newlines and entity-encoded colons.
- **OPS-002**: CI `GITHUB_TOKEN` is `contents: read` by default; only publishing jobs
  request the narrow write scope they need.
- **OPS-003**: all GitHub Actions in both workflows are pinned to full commit SHAs
  (mutable tags removed); the docs workflow's checkout is unified to v6.

### Fixed
- **GO-003**: WooCommerce product fetches no longer mask transport/5xx/parse failures
  as "not installed" — only 404/401/400 are treated as legitimately empty; real
  failures are surfaced (export continues with a warning since WooCommerce is optional).

### Changed
- **GO-002**: dead code connected to real functionality instead of removed —
  `mcp.NewServer` delegates to `NewServerWithIO`; the checkpoint flow uses
  `Manager.IsEnabled()`/`GetState()`; the HTML converter/sanitizer pick the narrowest
  constructor. (`Client.BruteForceContent` and `Manager.SetState` are intentionally
  left as-is: wiring them would duplicate existing behavior.)

## [1.7.7] - 2026-07-09

### Fixed
- **Snap build**: the `go.mod` `go` directive is relaxed from the exact patch `1.26.5`
  to the minor floor `go 1.26`. The go snap `1.26/stable` channel currently serves Go
  1.26.4, so requiring 1.26.5 broke the confined snap build (it cannot fetch a newer
  toolchain offline). CI still builds and tests with the exact 1.26.5 via `setup-go`;
  the `go` directive is only a minimum, so 1.26.x remains valid everywhere.

## [1.7.6] - 2026-07-08

### Changed
- **Go 1.26.5**: bumped the toolchain in `go.mod` and CI (`setup-go`) to the latest
  patch release. The `Dockerfile` (`golang:1.26-alpine`) and snap (`go/1.26/stable`)
  track the 1.26 minor and pick it up automatically.

## [1.7.5] - 2026-07-08

### Security
- **Credential leak fixed (SEC-001)**: Authentication headers (`Authorization` / HTTP
  Basic Auth) are now attached only to requests targeting the configured WordPress host.
  Media downloads and SEO crawling of URLs on a foreign host (e.g. a CDN) no longer leak
  WordPress credentials. New `Config.IsSameHost` gate.
- **CSV formula injection fixed (INT-001)**: exported CSV cells beginning with `=`, `+`,
  `-`, `@`, tab or CR are now prefixed with a quote so spreadsheet apps treat them as text.
  Applied to Shopify, Magento, PrestaShop and Webflow exporters (`csvSafe`/`csvSafeRow`).
- **HTML injection fixed (FE-001)**: the generated Shopify product metadata block now
  HTML-escapes all interpolated values and validates `href` schemes (rejecting
  `javascript:`/`data:`), preventing stored XSS in exported product descriptions.
- **CI supply-chain hardened (OPS-001)**: the `gosec` action is pinned to an immutable
  release tag instead of the mutable `@master` branch ref, and the security job now runs
  with a read-only `GITHUB_TOKEN`.

### Fixed
- **XML-RPC export now returns real data (GO-001)**: the XML-RPC client previously returned
  a fabricated "Sample Post" and empty media/term/user lists regardless of the site content.
  It now parses the actual `wp.getPosts`/`wp.getPages`/`wp.getMediaLibrary`/`wp.getTerms`/
  `wp.getUsers`/`wp.getOptions` struct responses (supports `<int>`/`<i4>`, `dateTime.iso8601`
  and untyped values).

### Changed
- **Go 1.26.4**: bumped the toolchain across `go.mod`, `Dockerfile`, CI (`setup-go`),
  the snap build channel and the README badge (was 1.25).
- Version constants in `wpexportjson` and `wpmcp` and the snap package version aligned to
  the current release (were stale at 1.6.x/1.7.1).
- Extracted repeated string literals into constants (`statusPublish`, `schemaTypeString`)
  to satisfy `goconst` and DRY.
- CI `security` job now installs `gosec` from a pinned module version (`go install
  ...@v2.22.0`) and runs it with the repo toolchain, replacing the mutable `@master`
  Docker action (fixes supply-chain pin and Go-compatibility; OPS-001).

### Fixed (CI)
- Release/homebrew/snap version calculation now selects the highest semver tag via
  `git tag --sort=-v:refname` instead of `git describe --tags --abbrev=0`, which
  returned an arbitrary tag when several tags shared a commit (v1.7.3 and v1.7.4
  pointed at the same commit) and stalled automatic version bumps.

## [1.7.1] - 2026-04-07

### Added
- **Homebrew Support**: Install via `brew install tradik/tap/wpexporter`
  - Auto-updated formula in `tradik/homebrew-tap` on each release
  - Installs `wpexportjson`, `wpxmlrpc`, `wpmcp` binaries and man pages
- **Snap Support**: Install via `sudo snap install wpexporter`
  - Snap package with strict confinement (network, home, removable-media)
  - Auto-published to Snap Store on release
- **CI/CD Pipeline Improvements**
  - Added `homebrew` job: auto-updates Homebrew formula with SHA256 hashes
  - Added `snap` job: auto-builds and publishes snap package
  - Release tarballs now include `wpmcp` binary and man pages

## [1.7.0] - 2026-03-16

### Added
- **Persistent Cache System**: New file-based caching to speed up repeated exports
  - `--cache` flag to enable caching of API responses and SEO crawl results
  - `--cache-ttl` to set cache expiration (default: 24h, use 0 for unlimited)
  - `--cache-dir` to specify custom cache directory (default: ~/.wpexporter/cache)
  - `--cache-clear` to clear cache before export
  - Caches all WordPress REST API calls: posts, pages, media, categories, tags, users
  - Caches SEO crawl results (assisted-crawl, crawl-content)
  - Site-isolated caching using URL hash (different sites don't share cache)
  - Significant performance improvement for repeated exports (media list from ~30-60s to <1s)
  - Environment variables: WPEXPORT_CACHE, WPEXPORT_CACHE_TTL, WPEXPORT_CACHE_DIR, WPEXPORT_CACHE_CLEAR
- **Preserve HTML Elements**: New flags to keep specific elements intact during HTML processing
  - `--preserve-classes` - preserve elements by CSS class (comma-separated)
  - `--preserve-ids` - preserve elements by ID (comma-separated)
  - Wildcard support: `klaviyo-form-*` matches `klaviyo-form-XL7uTf`
  - Works with both `--flat-html` and `--basic-html` options
  - Useful for newsletter forms (Klaviyo, Mailchimp), embedded widgets, custom elements
  - Environment variables: WPEXPORT_PRESERVE_CLASSES, WPEXPORT_PRESERVE_IDS
  - Configurable via config file: `preserve_classes`, `preserve_ids`

## [1.6.1] - 2026-03-11

### Added
- **Basic HTML Sanitization**: New `--basic-html` flag to clean HTML to basic elements for Shopify/ecommerce
  - Preserves: tables, lists, links, images, headers, paragraphs, basic formatting
  - Removes: Bricks Builder divs, Elementor widgets, custom classes, style/script tags
  - Strips dangerous attributes (onclick, style, data-*) while keeping safe ones (href, src, alt)
  - Mutually exclusive with `--flat-html` (use one or the other)
- **Keep Original URLs**: New `--keep-original-urls` flag to preserve WordPress URLs in content
  - Prevents conversion of media URLs to local paths (e.g., `media/images/...`)
  - Useful when exporting markdown for import to Shopify or other cloud platforms
  - Works with all formats but most useful with `--format markdown`

### Fixed
- **Missing Featured Images**: Featured images are now properly fetched even when not returned by paginated media API
  - WordPress REST API `/media` endpoint may not return all media items (WPML language contexts, restricted access)
  - Featured image IDs from posts/pages are now fetched individually using `GetMediaByID()` when missing
  - Ensures `featured_image` URL is populated in frontmatter for all posts with featured images
- **Relevant Media Over-Matching**: Fixed `--relevant-media-only` downloading too many files
  - Changed from filename-only matching to path-suffix matching (e.g., `2024/01/document.pdf`)
  - Prevents false positives when multiple files have the same name in different directories
- **Shopify Media URLs**: Fixed images not displaying in Shopify export
  - Media paths are now preserved as original WordPress URLs for Shopify format
  - Only json and markdown formats convert URLs to local paths
  - Shopify/Magento/other cloud platforms need full URLs, not relative paths

## [1.6.0] - 2026-02-11

### Changed
- **Markdown Excerpt Handling**: Excerpt moved from body section to frontmatter metadata field
  - Excerpt is now included as `excerpt: "..."` in YAML frontmatter
  - Removed separate `## Excerpt` section from content body
  - Content follows directly after frontmatter without section headers
  - Improves parser compatibility for posts with or without content sections

### Added
- **Media Subfolder Organization**: Downloaded media now organized into type-based subfolders
  - `media/images/` - All image files (jpg, png, gif, webp, svg, etc.)
  - `media/videos/` - Video files (mp4, avi, webm, mkv, mov, etc.)
  - `media/audio/` - Audio files (mp3, wav, ogg, flac, aac, etc.)
  - `media/documents/` - Documents (pdf, docx, xlsx, pptx, odt, etc.)
  - `media/archives/` - Archives (zip, rar, 7z, tar, gz, etc.)
  - `media/code/` - Code files (html, css, js, json, xml)
  - `media/other/` - Unrecognized file types
  - Content paths automatically updated to reference subfolder locations
- **Exclude Media Types Option**: New `--exclude-media-types` flag to skip specific media types from download
  - By category: `--exclude-media-types 'documents,videos,archives'`
  - By extension: `--exclude-media-types 'pdf,gif,html'`
  - By MIME type: `--exclude-media-types 'image,video'`
  - Configurable via config file: `exclude_media_types: ["documents", "pdf"]`
- **Extended Media MIME Type Support**: Added support for 80+ file types in media downloads
  - Documents: docx, doc, xlsx, xls, pptx, ppt, odt, ods, odp, epub
  - Archives: rar, 7z, tar, gz, bz2, xz
  - Video: mkv, m4v, 3gp, 3g2, ogv, mpeg
  - Audio: flac, aac, m4a, weba, wma, midi, aiff
  - Images: ico, avif, heic, heif
  - Code: html, css, js, json, xml, csv, md
- **Exclude SEO Tags Option**: New `--exclude-tags` flag to skip specific meta tags during extraction
  - Usage: `--exclude-tags 'meta:description,og:title'`
  - Supported tags: title, meta:description, meta:keywords, og:title, og:description, og:image, canonical, lang, hreflangs
  - Configurable via config file: `exclude_tags: ["meta:description", "og:title"]`
- **Duplicate Meta Tag Detection**: SEO crawler now detects and reports duplicate meta tags
  - Logs warning when duplicate tags found: `Detected duplicate tags on URL: tag1, tag2`
  - Uses first occurrence value (standard SEO behavior)
  - Detects duplicates for: title, meta:description, meta:keywords, og:title, og:description, og:image, canonical
- **Export Size Reporting**: Export summary now displays file sizes
  - Total export directory size
  - Media folder size (when media is downloaded)
  - ZIP archive size (when using `--zip`)
  - Human-readable format (B, KB, MB, GB)
- **Shopify Export Metadata**: Shopify export now includes post metadata in the HTML body content
  - ID, slug, date, modified, status, type
  - Link to original URL
  - Author name
  - Featured media ID
  - Categories and tags
  - Hreflang alternate links (when using `--assisted-crawl`)
  - Styled metadata section matching Markdown frontmatter fields
- **Hreflang Extraction**: `--assisted-crawl` now extracts hreflang alternate links
  - Captures language codes and URLs from `<link rel="alternate" hreflang="...">` tags
  - Included in both Markdown frontmatter and Shopify HTML metadata
- **Language Extraction**: `--assisted-crawl` now extracts content language
  - Captures language from `<html lang="...">` or `<meta http-equiv="Content-Language">`
  - Included in both Markdown frontmatter (`lang: "en-gb"`) and Shopify HTML metadata
- **Human-Readable Names in Frontmatter**: Markdown export now includes human-readable names
  - `categories: ["Category Name", "Another"]` with `category_ids: [152, 156]` as fallback
  - `tags: ["Tag Name"]` with `tag_ids: [42]` as fallback
  - `author: "Author Name"` with `author_id: 5` as fallback
  - Names resolved from WordPress categories, tags, and users data
  - Use `--no-ids` to exclude numeric ID fields (keep only human-readable names)
- **Featured Image in Frontmatter**: Markdown export now includes featured image URL
  - `featured_image: "https://example.com/image.jpg"` with URL from Media Library
  - `featured_image_id: 100` for numeric ID (unless `--no-ids` is used)
  - Previously only exported `featured_media: 100` (numeric ID only)
  - Consistent with author/author_id and categories/category_ids pattern
- **Relevant Media Link Extraction**: `--relevant-media-only` now extracts linked documents and videos
  - Scans `<a href>` links in content for media files (PDF, DOCX, MP4, ZIP, etc.)
  - Previously only extracted `<img src>` images from content
  - Supported file types: documents (pdf, docx, xlsx, pptx, odt, epub), videos (mp4, webm, avi, mkv), audio (mp3, wav, flac), archives (zip, rar, 7z), and images
  - Path-based matching: handles CDN URLs by comparing path suffix after `uploads/` (e.g., `2024/01/file.pdf`)
  - More precise than filename-only matching - avoids downloading unrelated files with same filename
  - Query string tolerance: `file.pdf?v=1.0` matches `file.pdf` in Media Library
  - Ensures all referenced media files are downloaded when using `--relevant-media-only`

### Fixed
- **Relevant Media Filter with FlatHTML**: Fixed `--relevant-media-only` not finding linked documents when used with `--flat-html` and `--crawl-content`
  - Media filtering now happens BEFORE HTML to Markdown conversion
  - Previously, FlatHTML converted `<a href="file.pdf">` to `[link](file.pdf)` before the filter could extract URLs
  - Now correctly finds PDFs, videos, and other linked media in crawled Bricks/Elementor content
- **Relevant Media Over-Matching**: Fixed `--relevant-media-only` downloading too many files
  - Previously matched by filename only (e.g., `document.pdf` would match ALL files named `document.pdf`)
  - Now matches by path suffix after `uploads/` (e.g., `2024/01/document.pdf`)
  - Significantly reduces ZIP file sizes by avoiding false positive matches

## [1.5.0] - 2026-02-05

### Added
- **MCP Server (wpmcp)**: New Model Context Protocol server for AI assistant integration
  - Enables Claude and other MCP-compatible AI assistants to interact with WordPress sites
  - 8 tools: `list_formats`, `get_site_info`, `list_posts`, `list_pages`, `export_site`, `get_post`, `list_categories`, `list_media`
  - JSON-RPC 2.0 protocol over stdio
  - Basic Auth and Bearer token authentication support
  - Optimized timeouts and retries for fast AI interactions
- **Makefile Updates**: Added `wpmcp` to build, install, and release targets
- **Comprehensive MCP Tests**: 51 unit tests for protocol, server, and tools

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
