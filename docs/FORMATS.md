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

`markdown` and `ssg` both write pages under the path their URL states, so a page
published at `/zerowisko/znaczenie/` becomes `pages/zerowisko/znaczenie.md`.
WordPress page addresses are hierarchical and a slug is unique only within its
branch: written flat, a child page and an unrelated top-level page sharing a
slug landed on one file and one of them was lost (#38). Two documents that still
want the same file — a site whose links are missing, so both fall back to their
slug — are both written, the second with its WordPress ID appended, and the
substitution is reported. The summary states pages written against pages fetched
whenever the two differ.

Lists keep their kind. An `<ol>` exports as `1.`, `2.`, a `<ul>` as bullets, a
nested list keeps its own kind at each level, and `<ol start="5">` starts at
five. A lettered, roman or reversed list keeps its HTML, which is valid in
Markdown: numbering it 1, 2, 3 would state something the page does not.

A page whose body is a page-builder post loop — a `/blog/` built from
`[fusion_blog]`, Elementor's Posts widget or a block query — exports with
`lists: posts` and `lists_hint` naming the element that gave it away, and the
run reports it. The REST API serves what is stored, and what is stored is the
element: the listing itself is produced at render time and cannot be exported.
Point the target's own archive at that address rather than migrating a page
over it.

After the export, the site's own **sitemap and main feed** are read — one or two
requests — and every address they list that the export does not carry is
reported and recorded in `metadata.json` under `stats.uncovered`. Archive views
a generator rebuilds itself are not counted. This is how a post type the REST
API never exposed stops being invisible. `--no-inventory-check` skips it; a site
that publishes neither document says so and nothing else changes.

When the REST API serves **no posts at all** — a site whose `/wp/v2/posts`
answers 5xx for every request still publishes its feed — `--from-sitemap`
recovers what the feed carries: title, address, date, author and body, with no
IDs, taxonomy terms or featured images, and `stats.recovered_posts` saying how
many. It is asked for rather than assumed, and never merges with or replaces a
collection the API did serve: REST is the better source in every respect, and a
feed lists recent items rather than the archive.

Two things hold for every platform format, and only for those: media URLs are
**left absolute**, because the target platform imports the files from the live
site, and address fields (`link`, `canonical_url`) stay absolute too. `json`,
`markdown` and `ssg` localise media instead — see
[Media and URL rewriting](MEDIA.md) for the per-format contract in full.

Adding `--zip` to any of them archives the result; `--no-files` then removes the
loose files, leaving only the archive.
