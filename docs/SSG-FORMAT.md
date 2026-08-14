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
| `link` | **root-relative** by default (`--link-style absolute` to change) |
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
