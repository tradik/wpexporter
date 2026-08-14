# Accessibility report

`--report-a11y` writes `a11y-report.md` next to the export. It changes nothing — it tells you what you are about to publish, measured against WCAG 2.2 contrast and non-text-content criteria.

Writes `a11y-report.md` next to the export. It changes nothing — it tells you what you are
about to publish:

```bash
wpexportjson export --url https://example.com -f ssg --report-a11y
```

| Check | Criterion |
|-------|-----------|
| Inline editor colours below a 4.5:1 contrast ratio | WCAG 2.2 SC 1.4.3 Contrast (Minimum) |
| Images with no alt text | WCAG 2.2 SC 1.1.1 Non-text Content |

Contrast is measured against the declared `background-color` where the content sets one, and
against white otherwise — which is the worst case for the bright palette the classic WordPress
editor offered. A 2010-era site typically carries a handful of these (`#ffff00` on white is
**1.07:1** against a 4.5:1 requirement). Redesigning the content is not the exporter's job, but
knowing before you publish is.
