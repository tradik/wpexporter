# SEO metadata extraction

`--assisted-crawl` fetches the rendered page behind each post, so the titles, meta descriptions, OpenGraph tags and hreflang alternates a plugin renders — and the REST API never exposes — leave with the content. The same pass records the site's own marketing wiring into `metadata.json`.

The `--assisted-crawl` option enables extraction of SEO metadata by crawling actual page URLs. This is useful when:

- RankMath, Yoast, or other SEO plugins are installed
- SEO data is not exposed via WordPress REST API
- You need accurate `<title>` tags and meta descriptions

## Extracted SEO Fields

| Field | Source |
|-------|--------|
| `seo_title` | `<title>` tag content |
| `meta_description` | `<meta name="description">` |
| `meta_keywords` | `<meta name="keywords">` |
| `og_title` | `<meta property="og:title">` |
| `og_description` | `<meta property="og:description">` |
| `og_image` | `<meta property="og:image">` |
| `canonical_url` | `<link rel="canonical">` |
| `lang` | `<html lang="...">` or `<meta http-equiv="Content-Language">` |
| `hreflangs` | `<link rel="alternate" hreflang="...">` (all language variants) |

## Usage Example

```bash
# Export with SEO metadata extraction
wpexportjson export --url https://example.com --assisted-crawl -f markdown

# Combine with path filter for specific sections
wpexportjson export --url https://example.com --path-filter=/blog/ --assisted-crawl -f markdown

# With authentication for protected sites
wpexportjson export --url https://example.com --auth-user admin --auth-pass secret --assisted-crawl

# Exclude specific SEO tags from extraction
wpexportjson export --url https://example.com --assisted-crawl --exclude-tags 'meta:description,og:title'

# With rate limiting to prevent server overload (500ms delay between requests)
wpexportjson export --url https://example.com --rate-limit 500 -f markdown

# Resume interrupted export (checkpoint is saved automatically)
wpexportjson export --url https://example.com --resume -f markdown
```

## Site-level marketing metadata

`--assisted-crawl` also reads the home page once and records the site's marketing
wiring into `metadata.json` under `marketing`, so a migration can configure the
target instead of re-entering it by hand:

```json
{
  "marketing": {
    "verification": {
      "google-site-verification": "abc123",
      "facebook-domain-verification": "fb456"
    },
    "social_profiles": {
      "facebook": "https://facebook.com/example",
      "instagram": "https://instagram.com/example"
    },
    "og_site_name": "Example Site",
    "og_image": "https://example.com/wp-content/uploads/2024/05/social.jpg",
    "twitter_site": "@example",
    "favicon": "https://example.com/favicon-192x192.png",
    "apple_touch_icon": "https://example.com/apple-touch-icon.png",
    "theme_color": "#0f172a"
  }
}
```

Favicon, apple-touch-icon and logo are read from the document's `<link rel=...>`
tags (the largest declared favicon size wins), social profiles from `<header>` and
`<footer>` links, and relative paths are resolved to absolute URLs. Everything is
best-effort: a value the site does not declare is omitted rather than invented.
Tracking identifiers (GA4, GTM, Meta Pixel, Hotjar, Clarity, …) are recorded
separately under `analytics`.
