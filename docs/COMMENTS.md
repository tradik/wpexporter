# Reader comments

Comments are the one part of a site its owner did not write: names, dates, threads and opinions readers left over years. They ship by default, addressed by page URL rather than by a post ID that means nothing on the other side of a migration.

Comments are the one part of a site its owner did not write: names, dates, threads and
opinions readers left over years. They ship by default, from `/wp/v2/comments`, which a
public WordPress serves **without authentication** — and serves approved comments only,
which is exactly what a migration wants (pending and spam rows are moderation state, not
content).

They leave the export as one `comments.json` beside `metadata.json`:

```json
{
  "total": 128,
  "pages": 31,
  "exported_at": "2026-08-14T18:20:11Z",
  "comments": [
    {
      "id": 4711,
      "post": 812,
      "parent": 0,
      "post_url": "/blog/wms-implementation-pitfalls/",
      "author": "Jan Kowalski",
      "author_url": "https://example.org",
      "author_avatar": "https://secure.gravatar.com/avatar/…?s=96",
      "date": "2024-03-01T10:00:00Z",
      "date_gmt": "2024-03-01T09:00:00Z",
      "content": "<p>Świetny tekst — u nas WMS wszedł dokładnie tak.</p>",
      "status": "approved",
      "type": "comment",
      "link": "/blog/wms-implementation-pitfalls/#comment-4711"
    }
  ]
}
```

Two things make the file portable:

- **`post_url`, not just `post`.** A WordPress post ID means nothing on the other side of a
  migration; the page address does. It takes the same form as the post's own `link`, so
  `--link-style root` yields `/blog/…/` and the default yields the absolute URL. A comment
  whose post was not exported (excluded by `--no-posts`, a path filter, or left in draft)
  falls back to its own permalink with the `#comment-N` anchor trimmed off.
- **Creation order.** Comments are sorted by id, so a reply never precedes the comment it
  answers when a target system replays them into a table with a parent reference.

A site with the REST route switched off or gated **prints a note and carries on** —
`--no-comments` skips the attempt entirely. The two cases read differently, because
their remedies do: a site that turned commenting off answers `403
rest_comment_disabled` and is reported as having no comments, while a *gated* route is
the one worth `--auth-user`/`--auth-token`. An export with no comments writes no file: an
empty `comments.json` would claim the site has none, when the truth may be that they were
never requested.
