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

The sitemap index is read to the end. It used to stop at twenty child
documents, which is a number this tool invented: WordPress writes one child per
2,000 URLs, so a shop with 60,000 products was quietly told it published 40,000
addresses. `--max-sitemap-documents N` sets a bound for an operator who would
rather not spend the requests, and the run then names the documents it skipped.

A post type whose slug contains `layout`, `template`, `block`, `section`,
`popup` or `widget` is read as a theme's saved fragments rather than as content.
That is right for a builder and wrong for a magazine whose type is called
`section`, so every type set aside is **named in the report** — and
`--custom-types <slug>` insists, whatever the rule thinks of the slug.

That rule reads slugs, so it misses a plugin's data store whose slug looks like
content — Modula's `modula-gallery`, for one. Such a type is registered without
a rewrite rule, so WordPress publishes its entries at `/?modula-gallery=1289`:
they are the plugin's records, never a page a visitor reaches.
`--skip-unaddressable-types` drops a type whose **every** entry is published
that way, and names what it dropped.

It is off by default and stays that way. A WordPress left on **plain
permalinks** publishes every type at a query-string address, and there the flag
would take the site's real content — which is why this is the operator's call
rather than a rule. One entry with a real permalink is enough to keep the type:
a half-configured type is still the site's, and dropping it would lose the
entries that were addressable.

Without the flag, such entries are exported at the address the export files
them at — `/modula-gallery/1289/` — because `/?modula-gallery=1289` resolves to
the site root and two of them would overwrite the front page (#78). See
[MEDIA.md](MEDIA.md#a-permalink-with-no-path).

`metadata.json`'s `site` block records **which page is the home and where the
posts went**: `show_on_front` (`page` or `posts`), and `front_page` /
`posts_page` with each page's id, slug and address (#75). They decide the shape
of anything built from the export, and every guess at them is bad — "is there a
document claiming `/`?" says nothing about the archive, and "is there a page
called `blog`?" breaks on every site that calls it `news` or `aktualnosci`. They
come from `/wp/v2/settings` where credentials reach it, and otherwise from the
`<body>` classes WordPress publishes to every visitor. A key is **absent** where
it could not be worked out, never guessed, so a consumer can tell "there is no
posts page" from "nobody looked".

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

A site serving its REST API at **`?rest_route=`** rather than `/wp-json/` is read
without being asked about. That is the fallback spelling WordPress documents,
served whenever permalinks are plain or a security plugin hides the pretty route,
and the exporter used to stop at the first 404 with a message about categories
(#66). It is discovered lazily: the pretty address is tried first, nothing is
probed until one actually 404s, and a site that answers normally spends no extra
request at all. The export is complete either way, and `stats.notices` in
`metadata.json` names the spelling that was used, because the address in the
report is not the one a reader would try by hand.

On a site with no content API, **the sitemap is the source rather than a
check**. Its addresses are fetched and written as pages, with
`stats.recovered_pages` saying how many; they carry title, address, SEO metadata
and the rendered body, and no IDs, terms, authors or dates, because a published
page is what the site shows a reader rather than what its database holds. Only
addresses no exported document already covers, only under `--from-sitemap`, and
the limit flags bound the walk (#68).

A WordPress **older than 4.7** has no `wp/v2` content routes in either spelling —
the content API arrived in that release — and answers `rest_no_route` to
everything. There is nothing to fall back to, so the run says so once, records it
in `stats.notices`, and reads the site's feed by itself rather than handing back
an empty export that looks like an empty site (#68). `--no-inventory-check`
overrules that, as it overrules everything else the inventory does.

**A shop's catalog needs no consumer keys.** Products are read from
`/wc/v3/products` when keys were given — the admin API alone sees drafts and
private products — and otherwise from `/wc/store/v1/products`, WooCommerce's
public storefront API, which carries prices with their currency, images,
categories, tags, stock and ratings without credentials (#74). `/wp/v2/product`
is the last fallback and carries the catalog page without any commerce. The run
names which of the three answered, so "no keys, and it did not matter" reads
differently from "no keys, and the prices are missing".

**A shop's catalog is written down.** `markdown` puts each product at
`products/<slug>.md`; `ssg` puts it at the path its permalink states, so the
`/produkt/<slug>/` links in the site's own navigation still resolve on the built
site. The commerce facts travel in front matter — `sku`, `price`,
`regular_price`, `sale_price`, `on_sale`, `stock_status`, `product_categories`,
`product_tags`, `images` — each omitted where the shop did not set it, and the
long description is the body. Until #65 the products were fetched, counted in
`stats.total_products` and written nowhere by either format.

**A heading keeps its own styling.** A `<h2 class="sc_item_title
trx_addons_inline_158836093">` travels as HTML rather than as `##`, because that
generated class is where the theme's color rule keys on and a Markdown heading
has nowhere to put it (#67). Boilerplate does not count: `wp-block-heading`,
`has-text-align-center`, `entry-title`, `screen-reader-text` and their kind are
what WordPress stamps on every heading everywhere and say nothing a `##` is
missing, so those convert as they always have. What counts as boilerplate is
extended per site with `--boilerplate-classes`, and how much is kept at all is
`--preserve-styling auto|none|all`: keep the headings that mean something, keep
nothing, or keep every element carrying a class — which is what a site whose
whole layout is styling needs. `--preserve-classes` and `--preserve-ids` name
elements exactly, on top of whichever mode is in force.

A post the editor **pinned to the top of the blog** carries `sticky: true`,
omitted when false. A listing sorted by date alone buries it wherever its date
falls — sixth, on the site that reported it (#51).

The **page template** WordPress drew a page with is carried as
`source_template`, absent where WordPress reports none — which is what it
reports for the default one. A theme is often two designs rather than one, and
the template is what decides which a page gets; nothing else in an export says
so. Not `template`: that names the template a *generator* should render the
document with, and a WordPress file name there would send the build looking for
one it does not have (#81). Both the `markdown` and the `ssg` front matter carry
it — see [SSG-FORMAT](SSG-FORMAT.md#the-page-template).

Emphasis is written so that it **closes**: WordPress content is full of
`<strong>text </strong>`, with the space inside the tags, and converted tag for
tag that becomes `**text **`, which in CommonMark closes nothing and prints the
asterisks to the reader. The whitespace moves outside the delimiters, and a run
with nothing but space in it is dropped (#50).

Terms carry their **addresses** as well as their names: `category_slugs`,
`category_paths` (the parent chain, when the taxonomy is nested) and `tag_slugs`
beside the existing `categories` and `tags`. A target that makes a slug out of a
display name gets it wrong wherever WordPress did not, and every archive it
publishes then 404s (#45).

An **unexpanded shortcode is removed** rather than written into the document. A
plugin that renders on the front end and not over REST leaves its source text in
`content.rendered`, and a reader of the migrated page would see
`[osm_map_v3 …]` where the site rendered a map. What was removed is reported
with counts and kept in `stats.removed_shortcodes`, so a missing calendar or
gallery is known rather than discovered (#47). A Markdown link's label and an
editorial `[sic]` are left alone.

A page whose body the API **did not serve at all** — a front page assembled from
theme sections, which live in post meta — is reported as well, and named in
`stats.empty_pages`. The export is correct and useless at the same time there;
`--assisted-crawl --crawl-content` takes the rendered page instead (#46).

That crawl reaches the pages the warning is about, which it used not to. A page
builder's body is not empty — a King Composer front page is several kilobytes of
`kc-elm` wrappers with a headline inside them — so the emptiness test passed it
over and the recommended remedy fetched five pages of twenty, none of them the
ones that needed it (#63). The body is now judged by what it amounts to: an
ordinary page carries hundreds of characters of text per container element, a
builder shell carries a handful, and a recognised class prefix (`kc-elm`,
`vc_row`, `et_pb_`, `elementor-`, `fl-builder`, `brxe-`, `oxy-`) raises that
threshold rather than being the whole rule — the next builder is on nobody's
list, and a body that is all containers and no text renders to nothing whatever
it is called. `--builder-classes` names the one this site uses,
`--content-selector` names where its theme keeps the page, and
`--crawl-content-mode auto|empty|always` decides how much is re-read at all. The
run states how many of each it found, and names the pages that were re-read and
gave nothing back. `--skip-empty-content`
is unchanged and still asks its own question: a builder page is worth crawling
and is not worth discarding.

Two things hold for every platform format, and only for those: media URLs are
**left absolute**, because the target platform imports the files from the live
site, and address fields (`link`, `canonical_url`) stay absolute too. `json`,
`markdown` and `ssg` localise media instead — see
[Media and URL rewriting](MEDIA.md) for the per-format contract in full.

Adding `--zip` to any of them archives the result; `--no-files` then removes the
loose files, leaving only the archive.
