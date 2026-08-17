# HTML to Markdown conversion

`--flat-html` turns rendered HTML into clean Markdown, with conversion rules you can extend per site and elements you can keep intact. Gutenberg content is covered by the same pass: its blocks are HTML with comment markers, and the markers are stripped on the way out.

The `--flat-html` option converts HTML content to clean Markdown format. This is useful for:

- Sites using page builders (Bricks Builder, Elementor) that output complex HTML
- Migrating content to markdown-based systems
- Cleaning up HTML before export

## Built-in Conversions

| HTML Element | Markdown Output |
|--------------|-----------------|
| `<h1>` - `<h6>` | `#` - `######` |
| `<p>` | Plain text with line breaks |
| `<strong>`, `<b>` | `**bold**` |
| `<em>`, `<i>` | `*italic*` |
| `<a href="...">` | `[text](url)` |
| `<img src="..." alt="...">` | `![alt](src)` |
| `<ul>`, `<ol>` | `-` or `1.` lists |
| `<blockquote>` | `>` quote |
| `<code>` | `` `inline` `` |
| `<pre><code>` | ` ``` ` code block |
| `<hr>` | `---` |
| Bricks: `.brxe-heading` | `##` heading |
| Bricks: `.brxe-text` | paragraph |

## Custom Conversion Rules

You can define custom rules in your `config.yaml` for site-specific HTML classes:

```yaml
flat_html_rules:
  # Bricks Builder custom headings
  - class: "brxe-heading"
    tag: "div"
    markdown: "## {content}\n\n"

  # Elementor headings
  - class: "elementor-heading-title"
    markdown: "# {content}\n\n"

  # Custom paragraph class
  - class: "my-paragraph"
    markdown: "{content}\n\n"

  # Specific tag + class combination
  - class: "custom-quote"
    tag: "span"
    markdown: "> {content}\n\n"
```

**Rule fields:**
- `class` (required): CSS class to match
- `tag` (optional): HTML tag to match (e.g., "div", "span")
- `markdown`: Output template where `{content}` is replaced with the element's text

## Preserving HTML Elements

Use `--preserve-classes` and `--preserve-ids` to keep certain elements intact. They apply to the `markdown` and `ssg` formats as well as to `--flat-html` and `--basic-html`. This is useful for:

- Newsletter signup forms (Klaviyo, Mailchimp)
- Embedded widgets and third-party scripts
- Custom interactive elements you don't want converted
- **Elements a theme styles through a class**, which is the case Markdown cannot express at all

That last one is worth spelling out (#67). Themes of some families emit one generated class per element and a stylesheet rule to match — `trx_addons_inline_158836093` is where a heading's colour lives. Converted to `## Title`, the class has nowhere to go: the stylesheets migrate fine, but there is nothing left for them to match, and the front page's headline renders in the body colour while a headline two sections down keeps the theme's by accident, because that section colours its inner `<span>` as well.

```bash
# Keep whatever the theme colours through a generated class
wpexportjson export --url https://example.com -f markdown \
  --preserve-classes "trx_addons_inline_*"

# Keep every element that carries any class at all
wpexportjson export --url https://example.com -f markdown --preserve-classes "*"
```

A preserved element travels as the HTML it arrived as — Markdown allows raw HTML — and nothing inside it is converted either, so a `<strong>` within it stays a `<strong>`. Name nothing and the conversion is exactly what it always was: a heading with no attributes still becomes `##`.

```bash
# Preserve Klaviyo forms (with wildcard)
wpexportjson export --url https://example.com --basic-html \
  --preserve-classes "klaviyo-form-*"

# Preserve specific elements by ID
wpexportjson export --url https://example.com --flat-html \
  --preserve-ids "newsletter-form,sidebar-widget"

# Combine classes and IDs (comma-separated)
wpexportjson export --url https://example.com --basic-html \
  --preserve-classes "klaviyo-form-*,mailchimp-widget" \
  --preserve-ids "contact-form"
```

**Wildcard support:**
- `klaviyo-form-*` matches `klaviyo-form-XL7uTf`, `klaviyo-form-ABC123`, etc.
- `brxe-*-section` matches `brxe-hero-section`, `brxe-footer-section`, etc.

**Configuration file:**
```yaml
preserve_classes:
  - "klaviyo-form-*"
  - "mailchimp-widget"
preserve_ids:
  - "newsletter-form"
```
## Usage Example

```bash
# Convert HTML to Markdown with default rules
wpexportjson export --url https://example.com --flat-html -f markdown

# With custom rules from config file
wpexportjson export --url https://example.com --flat-html --config config.yaml -f markdown

# Combine with content crawling for page builder sites
wpexportjson export --url https://example.com --crawl-content --flat-html -f markdown
```

## Markdown Frontmatter Output

When using `--assisted-crawl` with markdown format, SEO fields are included in the frontmatter:

```yaml
---
id: 123
title: "Original Post Title"
seo_title: "SEO Optimized Title | Site Name"
meta_description: "A compelling description for search engines..."
og_title: "Title for Social Sharing"
og_image: "https://example.com/social-image.jpg"
lang: "en-US"
hreflangs:
  - lang: "en-US"
    href: "https://example.com/post/"
  - lang: "de-DE"
    href: "https://example.com/de/post/"
  - lang: "fr-FR"
    href: "https://example.com/fr/post/"
excerpt: "A brief summary of the post content..."
# ... other fields
---

# Post Title

The full post content follows directly after the frontmatter...
```

## Gutenberg blocks support

WordPress Gutenberg editor stores content as HTML with special comment markers. Here's how wpexporter handles Gutenberg blocks:

### ✅ Standard Export Behavior

Gutenberg blocks export automatically in all formats:

| Content Type | Export Result |
|--------------|---------------|
| Standard blocks (paragraphs, headings, lists) | ✅ Exported as HTML content |
| Core blocks (quote, code, image, gallery) | ✅ Embedded HTML preserved |
| Custom blocks (plugins, themes) | ✅ Rendered HTML output |
| Block patterns & reusable blocks | ✅ Resolved to final HTML |

**No configuration needed** for JSON, WordPress WXR, or HTML-preserving formats.

### 🔄 Markdown Conversion with `--flat-html`

When exporting to Markdown format, use `--flat-html` to convert Gutenberg HTML to clean Markdown:

```bash
# Convert Gutenberg content to Markdown
wpexportjson export --url https://example.com --flat-html -f markdown
```

Common Gutenberg CSS classes for custom rules in `config.yaml`:

```yaml
flat_html_rules:
  # Core Gutenberg blocks
  - class: "wp-block-heading"
    markdown: "## {content}\n\n"
  - class: "wp-block-paragraph"
    markdown: "{content}\n\n"
  - class: "wp-block-quote"
    markdown: "> {content}\n\n"
  - class: "wp-block-code"
    markdown: "```\n{content}\n```\n\n"
  - class: "wp-block-preformatted"
    markdown: "```\n{content}\n```\n\n"
  - class: "wp-block-list"
    markdown: "{content}\n\n"
  - class: "wp-block-image"
    markdown: "{content}\n\n"

  # Extended blocks
  - class: "wp-block-pullquote"
    markdown: "> **{content}**\n\n"
  - class: "wp-block-verse"
    markdown: "*{content}*\n\n"
  - class: "wp-block-table"
    markdown: "{content}\n\n"
```

### 📋 Block Detection

The exporter preserves Gutenberg comment markers in HTML exports:
```html
<!-- wp:paragraph -->
<p>Content here...</p>
<!-- /wp:paragraph -->
```

These markers are stripped during Markdown conversion with `--flat-html`.
