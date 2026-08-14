# Export formats

Fifteen formats, one crawl. The exporter reads the site once and writes whichever shape the target needs, so choosing a destination is a flag rather than a second export — and switching destinations costs nothing but the write.

| Flag | Writes | Guide |
|---|---|---|
| `-f json` *(default)* | JSON documents, media localised to `/media/…` | [Command line reference](CLI.md) |
| `-f markdown` | One Markdown file per post and page, YAML front matter | [HTML to Markdown](MARKDOWN.md) |
| `-f ssg` | A drop-in content source: URL-mirroring paths, single-spelled front matter, cleaned body HTML | [Static site generator format](SSG-FORMAT.md) |
| `-f shopify` | `shopify_posts.csv`, `shopify_pages.csv`, `shopify_products.csv`, `shopify_metadata.csv` | [E-commerce formats](FORMATS-ECOMMERCE.md) |
| `-f magento` | `magento_posts.csv`, `magento_pages.csv`, `magento_products.csv`, `magento_metadata.csv` | [E-commerce formats](FORMATS-ECOMMERCE.md) |
| `-f prestashop` | Semicolon-delimited product, post, page, category and metadata CSVs, plus a JSON backup | [E-commerce formats](FORMATS-ECOMMERCE.md) |
| `-f wordpress` | `wordpress_export.xml` — WXR, the format WordPress imports natively | [CMS and headless formats](FORMATS-CMS.md) |
| `-f drupal` | `drupal_export.json` plus per-entity node, term, user and media files | [CMS and headless formats](FORMATS-CMS.md) |
| `-f ghost` | `ghost_export.json` | [CMS and headless formats](FORMATS-CMS.md) |
| `-f strapi` | `strapi_export.json` plus per-collection article, page, category, tag, author and media files | [CMS and headless formats](FORMATS-CMS.md) |
| `-f contentful` | `contentful_export.json` | [CMS and headless formats](FORMATS-CMS.md) |
| `-f wix` | `wix_export.json` | [Website builder formats](FORMATS-BUILDERS.md) |
| `-f squarespace` | `squarespace_export.xml` — WXR, which Squarespace imports as WordPress | [Website builder formats](FORMATS-BUILDERS.md) |
| `-f webflow` | Post, page, category and author CSVs for CMS collections, plus a JSON backup | [Website builder formats](FORMATS-BUILDERS.md) |
| `-f weebly` | `weebly_export.xml` and `weebly_export.json` | [Website builder formats](FORMATS-BUILDERS.md) |

Two things hold for every platform format, and only for those: media URLs are
**left absolute**, because the target platform imports the files from the live
site, and address fields (`link`, `canonical_url`) stay absolute too. `json`,
`markdown` and `ssg` localise media instead — see
[Media and URL rewriting](MEDIA.md) for the per-format contract in full.

Adding `--zip` to any of them archives the result; `--no-files` then removes the
loose files, leaving only the archive.
