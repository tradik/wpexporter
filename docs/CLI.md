# Command line reference

Every flag `wpexportjson export` accepts, what it defaults to, and the configuration-file form of the same settings — plus the checkpoint that makes an interrupted export resumable instead of a restart.

## Usage

### Basic Export

```bash
wpexportjson export --url https://your-wordpress-site.com
```

### Advanced Options

```bash
wpexportjson export \
  --url https://your-wordpress-site.com \
  --format markdown \
  --output ./my-export \
  --brute-force \
  --max-id 10000 \
  --download-media \
  --concurrent 10
```

### Configuration File

Create a `config.yaml` file:

```yaml
url: "https://your-wordpress-site.com"
output: "./export"
format: "json"
brute_force: true
max_id: 10000
download_media: true
concurrent: 10
```

Then run:

```bash
wpexportjson export --config config.yaml
```

## Command line options

<table>
<thead>
<tr>
<th style="white-space:nowrap">Option</th>
<th>Description</th>
<th>Default</th>
</tr>
</thead>
<tbody>
<tr><td><code>--url</code></td><td>WordPress site URL</td><td>Required</td></tr>
<tr><td><code>--output</code></td><td>Output directory or file</td><td><code>./export</code></td></tr>
<tr><td><code>--format</code></td><td>Export format (json/ markdown/ ssg/ shopify/ magento/ wordpress/ drupal/ wix/ squarespace/ webflow/ weebly/ prestashop/ ghost/ strapi/ contentful)</td><td><code>json</code></td></tr>
<tr><td><code>--brute-force</code></td><td>Enable brute force ID discovery</td><td><code>false</code></td></tr>
<tr><td><code>--max-id</code></td><td>Maximum ID for brute force</td><td><code>10000</code></td></tr>
<tr><td><code>--scan-range</code></td><td>Rescan a specific inclusive ID range for posts/pages/media, e.g. <code>100-200</code></td><td><code>""</code></td></tr>
<tr><td><code>--max-media-mb</code></td><td>Per-file media download size cap in MB (0 = built-in default of 2048)</td><td><code>0</code></td></tr>
<tr><td><code>--download-media</code></td><td>Download images and videos</td><td><code>true</code></td></tr>
<tr><td><code>--no-media</code></td><td>Disable media downloads (alias for --download-media=false)</td><td><code>false</code></td></tr>
<tr><td><code>--relevant-media-only</code></td><td>Download only featured images and media linked in content (images, PDFs, videos, etc.)</td><td><code>false</code></td></tr>
<tr><td><code>--exclude-media-types</code></td><td>Media types to skip (comma-separated: images,videos,audio,documents,archives,pdf,gif)</td><td>-</td></tr>
<tr><td><code>--media-path-style</code></td><td>Form of rewritten media paths: <code>root</code> (<code>/media/…</code>, resolves at any URL depth) or <code>relative</code> (<code>media/…</code>)</td><td><code>root</code></td></tr>
<tr><td><code>--link-style</code></td><td>Form of <code>link</code>/<code>canonical_url</code>/<code>hreflangs</code>: <code>absolute</code> (source URL) or <code>root</code> (root-relative path)</td><td><code>absolute</code><br>(<code>root</code> for <code>ssg</code>)</td></tr>
<tr><td><code>--extract-meta</code></td><td>Which meta tags to keep beyond the named SEO fields: <code>all</code>, <code>none</code>, or a comma-separated allow-list</td><td><code>all</code></td></tr>
<tr><td><code>--report-a11y</code></td><td>Write <code>a11y-report.md</code> flagging WCAG 2.2 contrast and missing alt-text issues</td><td><code>false</code></td></tr>
<tr><td><code>--concurrent</code></td><td>Concurrent downloads</td><td><code>5</code></td></tr>
<tr><td><code>--zip</code></td><td>Create ZIP archive of export</td><td><code>false</code></td></tr>
<tr><td><code>--no-files</code></td><td>Remove export files after creating ZIP (requires --zip)</td><td><code>false</code></td></tr>
<tr><td><code>--no-posts</code></td><td>Skip exporting blog posts</td><td><code>false</code></td></tr>
<tr><td><code>--no-pages</code></td><td>Skip exporting pages</td><td><code>false</code></td></tr>
<tr><td><code>--no-products</code></td><td>Skip exporting WooCommerce products</td><td><code>false</code></td></tr>
<tr><td><code>--no-custom-types</code></td><td>Skip the custom post types a theme or plugin registered</td><td><code>false</code></td></tr>
<tr><td><code>--custom-types</code></td><td>Export only these custom types (comma-separated slugs, e.g. <code>cpt_services,cpt_portfolio</code>)</td><td>-</td></tr>
<tr><td><code>--no-users</code></td><td>Skip exporting users</td><td><code>false</code></td></tr>
<tr><td><code>--no-tags</code></td><td>Skip exporting tags</td><td><code>false</code></td></tr>
<tr><td><code>--no-menus</code></td><td>Skip exporting navigation menus (they need authentication — see the navigation menus guide)</td><td><code>false</code></td></tr>
<tr><td><code>--no-comments</code></td><td>Skip exporting reader comments</td><td><code>false</code></td></tr>
<tr><td><code>--path-filter</code></td><td>Filter posts/pages by URL path pattern (e.g., /fr/arts/)</td><td>-</td></tr>
<tr><td><code>--flat-html</code></td><td>Convert HTML to Markdown (Bricks Builder, Elementor support)</td><td><code>false</code></td></tr>
<tr><td><code>--basic-html</code></td><td>Clean HTML to basic elements (tables, lists, links - for Shopify)</td><td><code>false</code></td></tr>
<tr><td><code>--ssg-sections</code></td><td>Markdown: emit <code>## Excerpt</code>/<code>## Content</code> sections and omit the duplicate body H1 (for ssg)</td><td><code>false</code></td></tr>
<tr><td><code>--preserve-classes</code></td><td>CSS classes to preserve from HTML processing (comma-separated, supports wildcards like <code>klaviyo-form-*</code>)</td><td>-</td></tr>
<tr><td><code>--preserve-ids</code></td><td>Element IDs to preserve from HTML processing (comma-separated, supports wildcards)</td><td>-</td></tr>
<tr><td><code>--assisted-crawl</code></td><td>Crawl URLs to extract SEO metadata (title, description, og tags)</td><td><code>false</code></td></tr>
<tr><td><code>--exclude-tags</code></td><td>SEO tags to exclude (comma-separated: title,meta:description,og:title,canonical,lang,hreflangs)</td><td>-</td></tr>
<tr><td><code>--crawl-content</code></td><td>Crawl pages with empty content (Bricks, Elementor page builders)</td><td><code>false</code></td></tr>
<tr><td><code>--skip-empty-content</code></td><td>Skip posts/pages with empty content from export</td><td><code>false</code></td></tr>
<tr><td><code>--auth-user</code></td><td>Username for Basic Auth (prompts for password if --auth-pass not provided)</td><td>-</td></tr>
<tr><td><code>--auth-pass</code></td><td>Password for Basic Auth</td><td>-</td></tr>
<tr><td><code>--auth-token</code></td><td>Bearer token for authentication</td><td>-</td></tr>
<tr><td><code>--rate-limit</code></td><td>Delay between API requests in milliseconds (prevents server rate limiting)</td><td><code>0</code></td></tr>
<tr><td><code>--no-inventory-check</code></td><td>Skip reading the site's sitemap and feed after the export to report what it did not cover</td><td><code>false</code></td></tr>
<tr><td><code>--frontmatter-style</code></td><td>Form of the structured front-matter values (<code>meta</code>, <code>hreflangs</code>): <code>nested</code> (YAML structure) or <code>flat</code> (one JSON string each, so they survive a store that holds only string lists — mddb and the like)</td><td><code>nested</code></td></tr>
<tr><td><code>--from-sitemap</code></td><td>When the REST API serves no posts, recover what the site's feed still publishes (title, address, date, author, body — no IDs, terms or featured images)</td><td><code>false</code></td></tr>
<tr><td><code>--retries</code></td><td>Attempts for a request the site answers with 5xx or 429, or drops. Exponential backoff with jitter, honouring <code>Retry-After</code></td><td><code>3</code></td></tr>
<tr><td><code>--resume</code></td><td>Resume from checkpoint if previous export was interrupted</td><td><code>false</code></td></tr>
<tr><td><code>--timeout</code></td><td>HTTP request timeout in seconds (increase for slow servers)</td><td><code>30</code></td></tr>
<tr><td><code>--verbose</code>, <code>-v</code></td><td>Enable verbose output</td><td><code>false</code></td></tr>
<tr><td><code>--quiet</code>, <code>-q</code></td><td>Suppress all output, only return exit code</td><td><code>false</code></td></tr>
<tr><td><code>--config</code></td><td>Configuration file path</td><td>-</td></tr>
</tbody>
</table>

## Resume / checkpoint

When exporting large sites, the `--resume` flag enables automatic checkpoint saving. If the export is interrupted (network error, server timeout, etc.), you can resume from where it left off:

```bash
# First export attempt (interrupted at 90%)
wpexportjson export --url https://large-site.com --resume -f markdown
# Error: connection timeout...

# Resume from checkpoint
wpexportjson export --url https://large-site.com --resume -f markdown
# Resuming from checkpoint: export/large-site.com.2026-02-02/.wpexport_checkpoint.json
# Checkpoint: posts=1500 (done=true), pages=42 (done=false)...
```

The checkpoint file (`.wpexport_checkpoint.json`) is automatically deleted on successful completion.
