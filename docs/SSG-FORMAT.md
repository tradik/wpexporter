# Static site generator format

`-f ssg` writes a drop-in content source for [SSG](https://ssg.tradik.com/) and other static site generators. Where `markdown` is a faithful dump of what WordPress returned, `ssg` is a content source: one name per concept, paths that mirror the site, and body HTML cleaned of the old theme's scaffolding.

A drop-in content source for [spagu/ssg](https://github.com/spagu/ssg) and other static site
generators. Where `markdown` is a faithful dump of what WordPress returned, `ssg` is a
*content source*: one name per concept, paths that mirror the site, and body HTML cleaned of
the old theme's scaffolding.

```bash
wpexportjson export --url https://example.com -f ssg -o export/site \
  --assisted-crawl --crawl-content
```

## 📂 Layout

```
export/site/
├── metadata.json                       categories / tags / users / media
├── comments.json                       reader comments, addressed by page URL
├── pages/
│   ├── about.md                        /about/
│   └── baby-water-instructor/
│       └── cost.md                     /baby-water-instructor/cost/
├── posts/
│   └── swimming/
│       └── swimming-lesson.md          posts sit at least one level below posts/
└── media/images/…
```

Pages are **nested to mirror their URL**, so the site's information architecture stays visible
in the file tree. Posts sit under their category; one with no resolvable category lands in
`posts/uncategorized/`.

## 📝 Front Matter

Single-spelled — a generator reads one name per concept, not three:

| Key | Source |
|-----|--------|
| `title` | `seo_title` if the site rendered one, else the post title |
| `slug`, `status`, `type` | as reported by WordPress |
| `date`, `modified` | RFC 3339 |
| `link` | **root-relative** by default (`--link-style absolute` to change). A permalink that carries no path — `/?modula-gallery=1289` from a type with no rewrite rule — is [replaced by the address the document is filed at](MEDIA.md#a-permalink-with-no-path), because `/` is the front page |
| `author` | resolved to a name via `metadata.json` `users[]` |
| `category` | the post's first named category |
| `description` | `meta_description`, else `og_description`, else the excerpt |
| `excerpt` | plain text, theme "Continue reading" chrome removed |
| `featured_image` | localised media path |

Empty values emit **no key at all**, so a generator sees an absent key rather than an empty
string.

## 🧹 Content Cleanup

Applied to the body of every `ssg` document:

| Transform | Why |
|-----------|-----|
| HTML entities → UTF-8 (`&#8211;` → `–`, `&hellip;` → `…`) | The file is UTF-8; the entities are noise that survives into the rendered page. `&lt;`, `&gt;`, `&amp;`, `&quot;` and `&#39;` stay encoded — decoding those would turn escaped markup into live markup |
| `alt` filled from the media library's `alt_text` | WCAG 2.2 SC 1.1.1 Non-text Content. An existing `alt` is never overwritten |
| WordPress classes dropped (`wp-image-*`, `size-*`, `align*`, `attachment-*`, `wp-block-*`) | They refer to the old theme's stylesheet. Authored classes are kept |
| `title` dropped when it merely repeats the filename | Carries no information a reader can use |
| `loading`, `decoding`, `sizes` dropped | Browser hints the generator emits itself |

The `markdown` format keeps its existing output, with two exceptions that were plainly bugs:
entities are decoded there too, and the excerpt no longer carries the "Continue reading" anchor.

## A pinned post

`sticky: true` marks a post the editor pinned to the top of the blog, and is
absent otherwise. Sort on it before the date, or the post the site owner
deliberately put first arrives wherever its date puts it (#51).

## Term addresses

`category` and the `tags` a document carries are display names. The address the
site publishes their archives under is the **slug**, and the two disagree often
enough to matter: on one migration 48 tag archives and 9 category archives 404'd
because a generator had made slugs out of names (#45). So the addresses travel
too:

```yaml
category: "Pasta & Rice"
category_slug: "pasta-rice"
category_path: "recipes/pasta-rice"   # only when the taxonomy is nested
tag_slugs:
  - "hand-made-pasta-3"
```

Use `category_path` where it is present — it is the chain WordPress published,
`/category/recipes/pasta-rice/` — and `category_slug` otherwise. The names are
unchanged and still the thing to display.

## A page that lists posts

A `/blog/` whose body is a page-builder element carries two extra keys, because
its content is generated at render time and cannot be exported (#41):

```yaml
lists: posts
lists_hint: "fusion_blog"
```

`lists_hint` names the element that was matched, so a wrong guess is visible
rather than silent. Point the generator's listing at this page's address —
in SSG, `posts_page` — instead of letting the migrated page occupy it.

## Front matter through a flat metadata store

Two values are not flat: the `meta` map and the `hreflangs` list. That is the
right shape for a generator reading these files, and the wrong one for a store
whose metadata model is key → list of strings — [mddb](https://github.com/tradik/mddb)
is one, deliberately. A pipeline through such a store loses both, and loses them
silently when the loader stringifies with Go's `%v`.

`--frontmatter-style flat` writes them as single JSON strings instead:

```yaml
meta: '{"recipe:yield":"8","twitter:label1":"Prep time"}'
hreflangs: '[{"lang":"en","href":"https://x.test/focaccia/"}]'
```

Lossless, decodable by anything that reads JSON, and stable between runs. The
default is unchanged, and `json_ld` already travelled as text, so it needs
nothing. Lists of plain strings — `categories`, `tags`, `category_slugs`,
`tag_slugs`, `category_paths` — are already what such a store holds natively.
