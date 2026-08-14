# ssgtheme — wpexporter documentation site

The theme behind <https://wpexporter.tradik.com/>. It is
[SSG](https://ssg.tradik.com/)'s own `ssgtheme`, vendored here so the site builds
from a checkout with no theme download, and adapted in the four places where a
theme has to know whose site it is.

```text
templates/ssgtheme/
├── index.html          # homepage: hero + pillars + formats + documentation cards
├── page.html           # a guide
├── post.html           # a post (unused today; kept so adding a blog is config-only)
├── category.html       # category/tag/author archives
├── partials/
│   └── chrome.html     # sc-head, sc-header, sc-footer, sc-marquee, sc-docs-nav
├── css/
│   ├── tokens.css      # design tokens only (colour, type, space, motion)
│   └── style.css       # layout and components; imports tokens.css
└── js/
    └── main.js         # progressive enhancement: menu, colour scheme, star count
```

## What differs from upstream

Everything visual — tokens, layout, components, the accessibility work — is
upstream's, unchanged. Keeping it that way is the point: a fix in SSG's theme
can be copied across without a merge. The wpexporter-specific edits are:

| File | Change |
|---|---|
| `index.html` | The whole home page: hero copy, the three pillars, the install and formats sections, and a `SoftwareApplication` JSON-LD describing wpexporter |
| `partials/chrome.html` | Brand subtitle (“WordPress export toolkit”), version chip, footer blurb, MIT licence link, and `tradik/wpexporter` as the repository fallback |
| `layouts/blog.html` | Removed — there is no blog |

## Design system

The visual language is the [Tradik design system](https://designstyles.tradik.com/).
`tokens.css` mirrors that system's published stylesheet 1:1 — token names and
values included — so a lookup there is valid here: the ink and accent ramps,
signal and surface colours, the `--color-bg-*` / `--color-fg-*` /
`--color-border-*` semantic sets, the three typefaces, the fluid type scale, the
4 px spacing grid, radius, elevation, motion and breakpoints.

Two additions are marked in the file as theme-local: the dark scheme (upstream
ships light only) and `--color-hero-wash`.

### Colour

| Role | Light | Dark | Contrast |
|---|---|---|---|
| `--color-fg-primary` | `#0F172A` | `#F8FAFC` | 17.9:1 / 16.9:1 — AAA |
| `--color-fg-secondary` | `#334155` | `#E2E8F0` | 10.4:1 / 13.4:1 — AAA |
| `--color-fg-muted` | `#64748B` | `#94A3B8` | 4.8:1 / 6.9:1 — AA |
| `--color-fg-accent` | `#0050A6` | `#99BFEB` | 7.8:1 / 9.1:1 — AA body |
| `--color-fg-danger` | `#B42318` | `#FDA29B` | 6.6:1 / 8.4:1 — AA body |

Dark mode is not a second palette: surfaces walk down the same ink ramp and links
move up the same accent ramp, because `accent-500` on a dark surface fails
contrast. Both are declared twice on purpose — once under
`@media (prefers-color-scheme: dark)` for the OS preference, once under
`:root[data-theme="dark"]` for the header toggle, so an explicit choice wins in
both directions.

### Typography

`Geist` for UI and body, `Instrument Serif` for display, `Geist Mono` for code —
the system's own faces, loaded from Google Fonts by two tags in
`partials/chrome.html`. Delete those two tags to make the theme issue **zero**
external requests: every family has a full system fallback stack (`system-ui`,
`Georgia`, `ui-monospace`), so the layout is unchanged and only the faces differ.

Sizes are the system's nine fluid `clamp()` steps; nothing in `style.css`
hard-codes a font size.

## Accessibility (WCAG 2.2)

- All body text meets AAA, all other text and UI meets AA, in both schemes.
- Interactive targets are at least 40 px tall (2.5 rem) — target size AA needs
  24 px, so there is margin to spare.
- Visible focus ring on every focusable element (`:focus-visible`, 3 px), a skip
  link, and `aria-current="page"` on the active navigation entry.
- The mobile menu and colour-scheme toggle carry `aria-expanded` /
  `aria-pressed`; Escape closes the menu and returns focus to its button.
- Nothing depends on JavaScript to be readable or navigable.
- The marquee stops under `prefers-reduced-motion` and becomes an ordinary
  scrollable row, so the platform list stays reachable without movement.

## Analytics

None. The site sets neither `variables.gtag` nor `variables.gtm_id`, so no
analytics loads and no consent banner is required. The theme still supports both
— it renders them Consent-Mode-v2-aware, defaulting every storage type to denied
— if that ever changes.

## Template contract

`sc-head` takes a dict, everything else takes the page context:

```gotemplate
{{ template "sc-head" (dict
    "Title" (printf "%s — %s" .Page.Title .Domain)
    "Desc" .Page.Excerpt
    "Canonical" (printf "/%s/" .Page.Slug)
    "OgType" "article"
    "Ctx" .) }}
{{ template "sc-header" . }}
{{ template "sc-footer" . }}
```

Files in `partials/` are parsed into the same template set as the theme root, so
these define names are callable from any role template.

## Site configuration the theme reads

Set in [`docs-site.yaml`](../../docs-site.yaml) under `variables`:

| Variable | Effect |
|---|---|
| `logo` | Brand mark beside the site name, resized at build time (header 36 px, footer 48 px tall). **Must carry its own alpha** — a logo saved on a white plate shows as a white box in dark mode. Rendered as PNG so a site without `cwebp` still builds. |
| `hero_image` | Optional homepage hero photograph, resized to 1600/900 WebP and laid under a scrim that keeps the copy at AA/AAA contrast. Unset ⇒ a plain hero. |
| `github_repo` | `owner/name` — enables the header star count. |
| `repository_url` | Full repository URL used by the footer links. |
| `version` | Renders the version chip in the header, linked to that release's tag. |
| `nav` | Header navigation: a list of `{label, url}`. |
| `docs_nav_order` | Slugs, in reading order, for the sidebar beside each guide. Anything omitted still appears, after the ordered ones. |
| `marquee` | `{title, items:[{name, icon}]}` — the “exports natively to” strip. `icon` is an SVG path on a 24×24 viewBox; an item without one renders as a name. |

Both `logo` and `hero_image` resolve inside `assets/` — see
[assets/README.md](../../assets/README.md).
