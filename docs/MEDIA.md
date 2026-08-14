# Media and URL rewriting

What happens to images, documents and videos when an export downloads them: where the files land, which URL forms are recognised as the same asset, which fields get rewritten in each format, and how to take only the media the content actually uses.

When downloading media with `--download-media`, the exporter rewrites URLs in exported content to point to local files.

## 📁 File Organization

Downloaded media files are stored in a structured format, in a subfolder per media category
(`images`, `videos`, `audio`, `documents`, `archives`, `code`, `other`):

```
export/
├── posts/
│   └── my-post.md
├── pages/
│   └── about.md
└── media/
    ├── images/
    │   ├── 123_featured-image.jpg
    │   └── 124_inline-photo.png
    ├── documents/
    │   └── 125_document.pdf
    └── videos/
        └── 126_video.mp4
```

**Naming pattern:** `{media_id}_{original_filename}{extension}`

## 🔄 URL Rewriting

Every reference to a downloaded attachment is rewritten — `src`, `href`, `srcset` and any
other URL occurrence are treated identically, so the export keeps working once the source
WordPress host is retired.

| Original URL | Rewritten Path |
|--------------|----------------|
| `https://example.com/wp-content/uploads/2025/01/photo.jpg` | `/media/images/123_photo.jpg` |
| `https://example.com/wp-content/uploads/2025/01/photo-300x200.jpg` | `/media/images/123_photo-300x200.jpg` |
| `https://example.com/wp-content/uploads/2025/01/photo-150x150.jpg` | `/media/images/123_photo-150x150.jpg` |

**Files the media library does not list are salvaged.** Page-builder renditions
(`uploads/elementor/thumbs/…`), attachments whose record was deleted while the file is still
served, and brand assets declared only in the document head never appear in `/wp/v2/media` —
so without this they stayed absolute and the migrated site hotlinked the source host. Every
same-host asset URL that content, SEO metadata or the marketing block references and the
library cannot account for is fetched into `media/<kind>/` under a name prefixed with a short
hash of its source path (page builders repeat basenames across directories). A URL on a
foreign host is left alone — it is somebody else's file — and one that no longer resolves is
skipped rather than failing the export.

**Matching is scheme- and host-insensitive.** WordPress stores `post_content` with whatever URL
form was current when the post was written, while the REST API reports `source_url` in the site's
present-day form. All of these resolve to the same exported file:

| Reference form in content | Example |
|---|---|
| current form | `https://example.com/wp-content/uploads/…` |
| historic scheme | `http://example.com/wp-content/uploads/…` |
| `www` / former domain | `https://www.example.com/…`, `http://old-domain.example/…` |
| protocol-relative | `//example.com/wp-content/uploads/…` |
| root-relative | `/wp-content/uploads/…` |
| with a query string | `…/photo.jpg?ver=2` |

URLs that do not correspond to a downloaded attachment are left untouched.

## 📐 Path Style: `--media-path-style`

| Value | Emitted path | When to use |
|-------|--------------|-------------|
| `root` *(default)* | `/media/images/123_photo.jpg` | Resolves identically from any URL depth — correct for a page served at `/about/team/` |
| `relative` | `media/images/123_photo.jpg` | Only correct for content served from the site root; kept for backwards compatibility with pre-1.7.9 exports |

```bash
# Default — root-relative, works at any URL depth
wpexportjson export --url https://example.com -f markdown --download-media

# Pre-1.7.9 behaviour
wpexportjson export --url https://example.com -f markdown --media-path-style relative
```

URL rewriting applies to the `json` and `markdown` formats only, and can be disabled entirely
with `--keep-original-urls` (other formats always keep original URLs, since the target platform
imports media from the live site).

## 📋 Per-Format URL Contract

What each format does with URLs, so you know what you are getting before you run an export:

| Format | Media URLs | Address fields (`link`, `canonical_url`) |
|--------|-----------|------------------------------------------|
| `json` | localised to `/media/…` | absolute (`--link-style root` to change) |
| `markdown` | localised to `/media/…` | absolute (`--link-style root` to change) |
| `ssg` | localised to `/media/…` | **root-relative by default** |
| `shopify`, `magento`, `wordpress`, `drupal`, `wix`, `squarespace`, `webflow`, `weebly`, `prestashop`, `ghost`, `strapi`, `contentful` | **left absolute** — the target platform imports media from the live site | absolute |

`--keep-original-urls` disables all rewriting for `json`, `markdown` and `ssg`.

## 🗂️ Which Fields Are Localised

| Field | Localised | Why |
|-------|-----------|-----|
| body content (`content.rendered`) | ✅ | assets |
| `excerpt` | ✅ | assets |
| `featured_image` | ✅ | asset |
| `og_image` | ✅ *when it resolves* | asset — but an og:image on a CDN or third-party host isn't a downloaded attachment, so it stays absolute |
| `canonical_url` | ⚙️ `--link-style` | address of the source site, not an asset |
| `link` | ⚙️ `--link-style` | as above |
| `hreflangs[].href` | ⚙️ `--link-style` | as above |

## 🔗 Address Fields: `--link-style`

`link`, `canonical_url` and `hreflangs[].href` are **addresses of the source site, not assets**, so
they are governed separately from media:

| Value | Emitted | When to use |
|-------|---------|-------------|
| `absolute` *(default)* | `https://example.com/2010/07/21/389/` | You need the original URL — to derive the target URL yourself, or because the old site stays up |
| `root` | `/2010/07/21/389/` | You are rebuilding the site **at the same paths**. Preserves each URL (and its search ranking) on the new host without pinning content to the old one |

```bash
# Rebuilding at the same paths on a new host
wpexportjson export --url https://example.com -f markdown --link-style root
```

Only **same-host** addresses are converted. An hreflang alternate or canonical pointing at a
different host keeps pointing where it points. Query strings and fragments are preserved
(`/a/?page=2#top`).

## 📷 Size Variants

WordPress generates multiple image sizes (thumbnail, medium, large, full). The exporter:

1. ✅ Downloads the **original full-size** image and every registered size variant
2. ✅ Rewrites each variant URL to **its own** exported file, preserving responsive `srcset`
3. ✅ Handles `-{width}x{height}` suffixed URLs automatically
4. ✅ **Remaps stale variants**: a registered-size change regenerates thumbnails but never
   rewrites the markup already linking to the old dimensions. A reference to a
   no-longer-generated `photo-300x199.jpg` is remapped to the closest surviving width
   (`photo-300x225.jpg`) instead of being left as a dead path. Run with `--verbose` to see
   each remap.

## 🎯 Selective Media with `--relevant-media-only`

For sites with large media libraries, use `--relevant-media-only` to download only used media:

```bash
wpexportjson export --url https://example.com --relevant-media-only -f markdown
```

**What gets downloaded:**

| Media Type | Downloaded | Condition |
|------------|------------|-----------|
| Featured images | ✅ Yes | Referenced by `featured_media` field |
| Content images | ✅ Yes | Found in `<img>` tags within content |
| Excerpt images | ✅ Yes | Found in `<img>` tags within excerpt |
| Linked PDFs/documents | ✅ Yes | Found in `<a href>` tags (pdf, docx, xlsx, etc.) |
| Linked videos | ✅ Yes | Found in `<a href>` tags (mp4, webm, avi, etc.) |
| Linked archives | ✅ Yes | Found in `<a href>` tags (zip, rar, 7z, etc.) |
| Unused library items | ❌ No | Not referenced by any post/page |

**Benefits:**
- 📉 Significantly reduces export size
- ⚡ Faster export for content-heavy sites
- 🎯 Only relevant assets are included (images, documents, videos)

## 💡 Examples

```bash
# Download all media (default)
wpexportjson export --url https://example.com -f markdown

# Download only featured images and content images
wpexportjson export --url https://example.com --relevant-media-only -f markdown

# Skip media download entirely
wpexportjson export --url https://example.com --no-media -f markdown

# Combine with path filter for targeted export
wpexportjson export --url https://example.com --path-filter=/blog/ --relevant-media-only -f markdown
```
