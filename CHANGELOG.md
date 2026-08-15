# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.8.7] - 2026-08-15

### Compatibility

Nothing was removed or renamed. Every new field in `metadata.json`
(`stats.uncovered`, `stats.post_loop_pages`) is omitted when empty, so a clean
export's metadata is byte-for-byte what it was; every front-matter key a
consumer reads today keeps its name, and the two new ones (`lists`,
`lists_hint`) only appear on a page that renders a post loop. `--no-inventory-check`
is the only new flag, and no existing flag changed its meaning or default.
Config files without the new key load unchanged. Pinned by tests, not by
intention.

Two changes are visible in output, both deliberate:

- an `<ol>` now exports as `1.`, `2.` instead of `- ` (that is #39), and a
  lettered, roman or reversed list keeps its HTML rather than being renumbered.
  A `<ul>` converts exactly as before;
- the export makes one or two extra requests at the end, for the site's sitemap
  and feed. `--no-inventory-check` restores the previous request pattern
  exactly.

### Fixed
- **Ordered lists exported as bullets (#39).** Every `<li>` became `- ` and the
  `<ul>`/`<ol>` around it was deleted, so an ordered list and an unordered one
  were indistinguishable in the output — on one migrated post, 2 `<ol>` and 3
  `<ul>` arrived as 5 `<ul>`. For a recipe, a tutorial or an assembly guide the
  numbers *are* the content: "step 3" is a reference, and a bulleted method
  reads as a set of unordered suggestions. Nothing downstream could recover the
  distinction, and no count-based check could notice — the same items arrive,
  with the same words.

  Lists are now converted as blocks, innermost first, so each level keeps its
  own kind and a nested list stays attached to its parent item. `<ol start="5">`
  starts at five, because the four before it are elsewhere on the page. A
  lettered, roman or reversed list keeps its HTML, which is valid in Markdown
  and says more than a number that would be wrong. Items keep their markup: the
  previous item pattern read only the text between the tags.

- **A page whose body is a post loop exported as an empty page (#41).** A site's
  `/blog/` is often a WordPress page whose body is a page-builder element —
  `[fusion_blog]`, Elementor's Posts widget, a block query — and the REST API
  serves what is stored, which is the element. The listing is produced at render
  time and never reaches the export.

  Such a page is worse than empty: it collides with the target's own listing. A
  generator told to build its archive at `/blog/` finds a migrated page already
  sitting there, the page wins, and the operator gets an empty blog with no
  error anywhere — the only way to find out was to read the built HTML. The page
  now carries `lists: posts` and `lists_hint` naming the element that was
  matched, the run reports it, and the same line is in `metadata.json` under
  `stats.post_loop_pages`. A page with real content of its own is left alone,
  marker or not.

### Added
- **The export says what it did not cover (#40).** It could only ever report
  what it fetched, so a content type the REST API does not expose was invisible:
  the run ended in a success summary and the migrated site was missing a
  section. Measured on one site, the sitemap listed 477 URLs against 155
  exported documents — 57 of them a plugin's events, which nobody was told
  about.

  After the export, the site's own sitemap (all three usual paths, following an
  index) and main feed are read, and every address they list that the export
  does not carry is reported, grouped by path and heaviest group first, then
  recorded in `metadata.json` under `stats.uncovered`. Archive views a generator
  rebuilds for itself — `/tag/`, `/category/`, a plugin's own taxonomy, date
  archives, paged listings — are not counted, because they are views of content
  the export already carries.

  Nothing is required of the site: no sitemap and no feed is a normal site, and
  the check says so in one line. A body that is a home page rather than a
  sitemap is not believed, since that is what a site without one answers with.
  `--no-inventory-check` skips the whole thing.

## [1.8.6] - 2026-08-15

A release whose reason is the Snap: 1.8.5 reached GitHub, ghcr and Homebrew and
stopped there, and the store cannot be handed a package without a tag to hang it
on. Nothing in the exporter itself changed.

### Fixed
- **The Snap could not be built for 1.8.5.** `go.mod` requires Go 1.26.6 — the
  1.8.5 decision that closed seven standard-library advisories — and no channel
  of the `go` snap ships it: `1.26/stable` and `latest/stable` are both 1.26.5.
  Go fetched 1.26.6 itself, and the line it prints while doing so landed inside
  the version string snapcraft's Go plugin parses:
  `invalid go compiler version 'go: downloading go1.26.6 (linux/arm64)'`. The
  arm64 build failed before it started and took the amd64 build down with it, so
  1.8.5 reached GitHub, ghcr and Homebrew but not the Snap Store.

  `snapcraft.yaml` no longer uses the Go plugin or the `go` snap: it fetches the
  1.26.6 toolchain from go.dev, pinned by the checksum published beside it, and
  builds with `GOTOOLCHAIN=local` so nothing can quietly substitute another one.
  The snap is now built by exactly the compiler the release notes name.

### Changed
- **The documentation site takes its colours from its own photograph.** The
  hero is `assets/wpexporter_back.jpg`, and the theme's accent ramp is that
  sunset's sky with the ink ramp warmed to match, rather than the design
  system's cool slate sitting on top of the picture. Every pairing used for text
  was measured against WCAG 2.2: body copy 18.18:1, secondary 12.81:1, muted
  7.00:1, links 6.25:1 in light; 17.59 / 14.05 / 7.03 / 10.69 in dark — none
  below what the previous palette had, and most above.

## [1.8.5] - 2026-08-14

The comments a site's readers left it. Closes #35.

### Added
- **Documentation site — <https://wpexporter.tradik.com/>.** `docs/` is now
  published as a static site, built by [SSG](https://ssg.tradik.com/) from
  `docs-site.yaml` with the bundled theme in `templates/ssgtheme` — the same
  theme and [Tradik design tokens](https://designstyles.tradik.com/) the SSG site
  itself runs on. Nothing is copied into a second location that could drift:
  `content_sources` reads `docs/` in place, so editing a guide and pushing it is
  the whole publishing workflow.

  `make site`, `make site-serve` (watch + <http://127.0.0.1:8888>) and
  `make site-check` (strict link checking, what CI runs) build it locally. The
  new **Docs Site** workflow deploys to Cloudflare Pages on a push to `main`,
  creating the Pages project and attaching the custom domain on the first run,
  and builds without deploying on a pull request. The site ships no analytics and
  therefore no consent banner; it does publish Markdown copies of every page and
  an `llms.txt`, for the agents this tool already serves over MCP.

- **Reader comments (#35).** Comments are the one part of a site its owner did
  not write and cannot rewrite — names, dates, threads and opinions left over
  years — and every export so far dropped them without a word. They are now
  fetched from `/wp/v2/comments`, which a public WordPress serves without
  authentication and which lists approved comments only (pending and spam rows
  are moderation state, not content), and written to `comments.json` beside
  `metadata.json`, with `stats.total_comments` in the metadata block.

  Each record carries **`post_url`**, not just WordPress's numeric `post`: a
  post ID means nothing on the other side of a migration, so the comment states
  the address of the page it belongs to, in the same form as that page's own
  `link` (`--link-style root` → `/blog/…/`). A comment whose post was not
  exported — excluded by `--no-posts`, a path filter, or left in draft — falls
  back to its own permalink with the `#comment-N` anchor trimmed. Records are
  sorted by id so a reply never precedes the comment it answers when a target
  system replays them into a table with a parent reference.

  `--no-comments` skips them. A site whose REST route is disabled or gated
  prints a note and the export carries on, as it does for menus; an export with
  no comments writes no file, because an empty `comments.json` would claim the
  site has none when the truth may be that they were never requested.

  The two refusals are told apart: a site that turned commenting off answers
  `403 rest_comment_disabled` and is reported as having none, while a gated
  route is the one that suggests `--auth-user`/`--auth-token`. Advising
  credentials for comments that do not exist would send an operator hunting for
  data no login can produce.

- **`Comments: N` in the export summary.** Every other collection is counted
  there; comments were fetched, written and then left out of the report, so the
  one case worth seeing — a site with comments that exported none of them —
  looked exactly like a site without any.

### Security
- **`go.mod` now requires Go 1.26.6, not `1.26`.** The 1.8.4 release note claimed
  the tool was built on 1.26.6, but only the CI workflow said so; `go 1.26` let
  any 1.26.x toolchain build it, and `govulncheck` on a 1.26.5 machine reported
  eight standard-library advisories — `net/url`, `crypto/tls`, `encoding/xml`,
  `encoding/asn1`, `net/http`, `html/template`, `net` — five of them reachable
  from code this tool runs on every export: `xml.Unmarshal` on an XML-RPC
  response, `http.Client.Do` on a crawl, `io.Copy` on a media download. The
  requirement is now stated where builds actually read it, so a local `go build`
  cannot silently produce a binary CI would never have shipped.

- **gosec pinned to v2.28.0** (was v2.22.0) and the runtime image moved to
  **Alpine 3.24** (was 3.21, two stable series behind).

- **gosec is a locked tool dependency, not an installed one.** The `security`
  job ran `go install github.com/securego/gosec/v2/cmd/gosec@v2.28.0`, which
  pins gosec itself and then re-resolves everything underneath it on every run:
  the scanner that passes today can be built from different code tomorrow, with
  no lock file to say so. It is now a `tool` directive in a separate `tools/`
  module, and both CI and `make sec` build it from there — every transitive
  version fixed and checksum-verified by `tools/go.sum`. The module is separate
  on purpose: gosec's dependency tree — gRPC, OpenTelemetry, the Google and
  Anthropic SDKs — would otherwise merge into wpexporter's own and turn up in
  every SBOM and vulnerability report taken of this tool.

### Fixed
- **A classic theme's palette was read as nothing at all (#34).**
  `marketing.colors` came from CSS custom properties, which covers block themes,
  Elementor and GeneratePress 3.x and misses everything older — most of the
  sites anyone migrates. bociany.pl (GeneratePress classic) exported no `colors`
  key at all, so the migrated site arrived in the target theme's defaults, while
  the palette sat in plain sight: `body`, `a`, `.main-navigation`, the button
  classes, and a `theme-color` meta tag already stating the brand red.

  When a page declares no theme properties, the roles are now taken from those
  rules — background and text from `body`, link from `a`, primary from the
  header or navigation rule (or `theme_color`, the brand colour by definition),
  accent from the button rule. Core's own `--wp--preset--color--*` are still
  never read: they are Gutenberg's defaults, identical on every site, so
  recording them would say something false about this one. The background and
  text pair is contrast-checked before it is emitted, and a pair that cannot be
  a page's real body colours is dropped rather than guessed at.

  The WCAG colour arithmetic moved into `internal/wcag`, shared with the
  accessibility report: two copies of a luminance formula are two chances to
  disagree about whether a site passes.

- **A page that shared a slug overwrote another page (#38).** Pages were written
  as `pages/<slug>.md`, but WordPress page URLs are hierarchical and a slug is
  unique only within its branch: on bociany.pl a child of
  `/zerowisko-i-pokarm/` and an unrelated top-level page both claimed
  `pages/znaczenie-zerowisk-bociana-bialego.md`, and whichever was written
  second won. 124 pages fetched, 111 files written, success reported. Every
  inbound link and menu entry to the losing page 404s after the migration.

  Pages now land under the path their URL states — `pages/zerowisko-i-pokarm/
  znaczenie-zerowisk.md` — which is the placement the `ssg` format already used;
  the two formats disagreeing about where a page lives *was* the defect. A child
  states its `parent` and `parent_slug` in front matter, so the tree can be
  rebuilt without re-deriving it from paths. Two documents that still want one
  file are both written, the second with its ID appended, and the rename is
  reported rather than left to be noticed. `stats.pages_written` is in
  `metadata.json`, and the summary reads `Pages: 124 fetched, 124 written`
  whenever the two differ.

- **One transient 5xx no longer ends the export (#37).** The exporter stopped at
  the first non-2xx answer and discarded everything already fetched. Sites are
  flaky: on 4sound.pl the posts route answered 500 about three times in four,
  and the same URL succeeded seconds later — so an export of thousands of
  requests could not finish at all, and nothing that had been downloaded
  survived.

  A 5xx, a 429, a request timeout and a dropped connection are now retried with
  exponential backoff and jitter, honouring the site's own `Retry-After` in
  either form the header allows. `--retries N` tunes it (default 3, `0` disables
  it). `Config.Retries` had existed all along with no flag to set it and no
  effect on a 5xx.

  A page of results that still will not come is a **gap, not the end**: the
  records already fetched are kept, the collection is reported as incomplete —
  in the console, in the export summary and in `stats.incomplete` in
  `metadata.json`, which outlives the console — and the export carries on. The
  MCP `export_site` tool draws the same line and returns the gaps in its result,
  since an assistant handed a complete-looking export has no other way to learn
  a hundred posts are missing. A partial fetch is never cached, or the next run
  would inherit the gap as the site's whole content.

- **The MCP `export_site` tool dropped comments too.** Everything above fixed the
  CLI; an agent exporting a site over MCP still lost every comment, which is the
  same #35 through a different door — and the door with no console to print a
  warning to. `export_site` now fetches them, honours a new `noComments`
  argument alongside `noPosts`/`noPages`/`noProducts`, and reports
  `stats.comments`, so a site whose comment route is closed reads as a zero
  rather than as silence. Collecting the site's data moved into
  `collectExportData`, which states in one place what is fatal (posts, pages)
  and what is optional by installation (products, media, comments).

- **`make sec` now runs the scanner CI runs.** It called whatever `gosec`
  happened to be on `PATH` — and when there was none it printed an install hint
  and exited successfully, so `make check` reported a clean security pass having
  scanned nothing. It also omitted the pipeline's `-exclude` list, so a developer
  who did have gosec installed saw four findings CI accepts. Both commands now
  build the pinned tool and share one exclusion list.

- **The guide-card template no longer writes an `<li>` outside its list.** The
  card partial opened with `<li>` and the `<ul>` lived at the call site, which is
  valid once the template is expanded and invalid to every reader that sees the
  file as it is written. The list item moved to the call site; the partial is the
  card itself. The rendered page is unchanged.

### Removed
- **The Jekyll GitHub Pages workflow.** It built the repository root with Jekyll
  and published it to GitHub Pages — a second documentation site, made of one
  rendered README, with no navigation, no search and no relation to `docs/`. Two
  sites for one project is one site too many, and the survivor is the one the
  guides are actually written for. The last deployment stays live until GitHub
  Pages is switched off in the repository settings; nothing rebuilds it.

### Changed
- **The README is now an index, and the manual lives in `docs/`.** It had grown
  to 1,800 lines — the whole documentation set in one scroll, and the only copy
  of it, which is why the new site had nothing to publish. Installation, the flag
  reference, the fifteen formats, media rewriting, SEO extraction, the Markdown
  converter, the page-builder rule sets, menus, comments, the accessibility
  report, the MCP server and the development notes are now seventeen guides under
  `docs/`, **moved verbatim** — only their headings rose a level where a section
  became a page. The README keeps its identity block, the feature list, a
  three-command install, a first export and a table of the guides. One copy of
  each subject, in the place the site reads from.

- **CI no longer runs on documentation.** A guide, a man page, a README edit or a
  change to the site template compiles nothing, yet every one of them used to run
  the full pipeline — tests, lint, gosec, the release, the Docker push and the
  Homebrew update. `ci.yml` now ignores `docs/**`, `templates/**`, `assets/**`,
  `man/**`, `**.md`, `LICENSE` and `docs-site.yaml`; those paths trigger the Docs
  Site workflow instead. `paths-ignore` skips a run only when *every* changed file
  matches, so a commit that touches Go code and a guide together still runs the
  whole pipeline.

- **Stale references fixed in `docs/XMLRPC_MANUAL.md`** — the API example imported
  `github.com/tradik/wpexportjson/internal/…` and the support link pointed at the
  old repository name; both now name `tradik/wpexporter`.

- **Dependencies upgraded**: `progressbar` 3.19.0 → 3.19.1, `fsnotify` 1.9.0 →
  1.10.1, `go-toml/v2` 2.2.4 → 2.4.3, `go.yaml.in/yaml/v3` 3.0.4 → 3.0.5,
  `golang.org/x/net` 0.57.0 → 0.58.0, `golang.org/x/text` 0.40.0 → 0.41.0. The
  direct requirements were already current.

- **GitHub Actions moved to their newest releases**, all still pinned by commit
  SHA: `checkout` v6.0.3 → v7.0.1, `setup-go` v6.5.0 → v7.0.0,
  `action-gh-release` v2.6.2 → v3.0.2 (a Node 24 runtime change; the inputs are
  unchanged), `login-action` v4.4.0 → v4.6.0, and on the Pages workflow
  `configure-pages` v5 → v6.0.0, `upload-pages-artifact` v3 → v5.0.0,
  `deploy-pages` v4 → v5.0.0. Each pin's trailing comment now names the exact
  patch release the SHA is: `codecov-action` was labelled `# v6` while pointing
  at v7.0.0, which is the failure mode a bare major comment invites.

## [1.8.4] - 2026-08-14

### Fixed
- **The site's own name, tagline and timezone were dropped from every export
  (#32).** WordPress core publishes `gmt_offset` at the REST root as a *number*
  (`"gmt_offset":2`); the reader declared it a string, so `json.Unmarshal`
  rejected the whole document and the identity fields went with it — silently,
  because an unreadable root is not treated as a failure. Every export of a
  public site therefore recorded `"name": ""` while `/wp-json/` served the name
  plainly, and the migrated site came up titled after its domain. The offset is
  now read quoted or bare, and one field of an unexpected type can no longer
  cost its siblings.
- **The name and tagline arrived HTML-encoded.** WordPress stores them escaped
  and serves them that way, so a tagline like `Fundacja Przyrodnicza &quot;pro
  Natura&quot;` reached the target verbatim — into a `<title>`, a meta
  description and a template variable, where an entity is just text. Both
  fields are decoded once, at the source.

### Security
- **Built on Go 1.26.6** (was 1.26.5), which patches seven standard-library
  advisories reachable from a tool whose whole job is parsing untrusted HTML,
  JSON and archive input.

## [1.8.3] - 2026-08-13

### Fixed
- **Media referenced by content but absent from the library was never downloaded
  (#30).** The downloader and the URL rewriter both worked from `/wp/v2/media`, and
  three kinds of file never appear there: page-builder renditions (Elementor writes
  its own crops to `uploads/elementor/thumbs/` with no attachment record),
  attachments whose record was deleted while the file is still served, and brand
  assets declared only in the document head. Content kept pointing at all three, so
  an export that reported success left the migrated site hotlinking the source host
  — and losing those images the day it was retired.

  A salvage pass now collects the same-host asset URLs the index cannot resolve —
  from content, excerpt, every SEO field and the marketing block — fetches them, and
  registers them so the ordinary rewrite reaches them. Only same-host URLs with an
  asset extension are followed: a CDN image is somebody else's file, and a page
  address is not media. Salvaged names carry a short hash of their source path,
  because Elementor repeats basenames across directories; the hash is derived from
  the path, so re-exporting does not duplicate anything. A URL that no longer
  resolves is skipped rather than failing the export, leaving it absolute as before.
  `--exclude-media-types` is honored.
- **Only `og:image` was localized among the metadata fields (#30).**
  `twitter:image`, the `meta` map (`msapplication-TileImage` and friends) and the
  JSON-LD blocks kept absolute URLs even for files that had been downloaded, as did
  the marketing block's favicon, apple-touch-icon and logo — which sit in the head
  of every page. All of them are rewritten now. Page addresses inside JSON-LD pass
  through untouched, since the rewriter only replaces what resolves to an exported
  attachment.

## [1.8.2] - 2026-08-13

Everything a migration was quietly losing: the theme's own content types, its
colours, and pages whose markup rendered as visible text. Closes #26, #27, #28.

### Added
- **Custom post types (#28)**. A WordPress site is rarely just posts and pages:
  themes and plugins register their own types — Services, Portfolio, Team,
  Testimonials — and those entries are published content with their own URLs. The
  export now discovers them from `/wp/v2/types` and fetches every entry, with the
  same SEO crawl, media localisation and link rewriting the pages get. They land
  under `pages/<type-slug>/` (markdown) or nested by their published URL (`-f ssg`),
  keeping the addresses a migration must preserve, and their WordPress type travels
  in front matter. `metadata.json` gains a `custom_types` block and
  `stats.total_custom_posts`.

  WordPress internals (templates, patterns, navigation, fonts), plugin bookkeeping
  (`elementor_*`, `rank_math_*`, `acf-*`, `jet-*`, …) and a theme's saved page
  construction (anything named `*layout*`, `*template*`, `*block*`, `*section*`,
  `*popup*`, `*widget*`) are excluded: they are markup a visitor met inside other
  pages, not documents. `--no-custom-types` turns the whole thing off;
  `--custom-types a,b` narrows it to named slugs. The export summary lists every
  type it found and how many entries each holds.
- **Theme palette (#27)**, written to `metadata.json` under `marketing.colors`
  when `--assisted-crawl` is used. Colours are read from the CSS custom properties
  the page declares — a theme's own variables first, then block-editor presets
  (`--wp--preset--color--*`), then the page builder's globals
  (`--e-global-color-*`) — and mapped to roles: `primary`, `secondary`, `accent`,
  `text`, `background`, `link`. Only literal colour values are recorded; a `var()`
  reference or a gradient is dropped rather than carried to a stylesheet where it
  would mean nothing. This is the one part of a site's look a migration can carry
  verbatim, and it was previously lost entirely.

### Fixed
- **Page-builder markup rendered as literal text (#26)**. Elementor, WPBakery and
  Divi indent their nested markup with tabs, and the converter turns `</p>` into a
  blank line — which ends the surrounding HTML block per CommonMark. Every
  following tab-indented line was then four columns deep, i.e. an indented code
  block, so visitors read `</div>` in monospace down the middle of the page. The
  converter now strips leading whitespace outside fenced code blocks, where it
  never carried meaning: this converter emits flat list markers and flat
  blockquotes, so the only indentation in its output was the source HTML's own
  pretty-printing. Indentation inside a `<pre>`-derived fence is untouched.

  Content already exported with an older version can be repaired in place with
  `ssg repair --fix` (ssg 1.8.31+), which reports the same defect during every
  build.
- **The snap reported the wrong version.** `wpexporter --version` inside the 1.8.1
  snap printed `1.8.0`. The snap build stamped `-X main.Version=…`, but 1.8.0 moved
  the build identity to `internal/version` and left the commands as thin wrappers
  that no longer declare `Version` — and the linker ignores `-X` for a symbol that
  does not exist, silently and without an error, so every snap since 1.8.0 shipped
  whatever default that package carried. The package version was always correct
  (`snap list` showed 1.8.1); only the binary's self-report was stale. Release
  tarballs and Homebrew were unaffected — they build through the Makefile, which
  already stamped the right package.

  The Docker image now stamps its binaries too; previously it passed no version
  flags at all. A test asserts that every build file stamps `internal/version` and
  that the package default matches the `VERSION` file, since neither half of this
  failure produces a build error.

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
