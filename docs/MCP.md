# MCP server

`wpmcp` speaks the Model Context Protocol over stdio, so Claude and other assistants can inspect and export a WordPress site without a shell between them. Eight tools cover site information, listings, a single post and a full export in any supported format.

It answers **both MCP eras**, deciding per request which one it is being addressed in, so no client has to be configured for it. See [Protocol revisions](#protocol-revisions) if you need to pin one.

**Claude Desktop Configuration** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "wpexporter": {
      "command": "wpmcp",
      "args": ["serve"]
    }
  }
}
```

**Claude Code Configuration** (`.claude/mcp.json`):
```json
{
  "mcpServers": {
    "wpexporter": {
      "type": "stdio",
      "command": "wpmcp",
      "args": ["serve"]
    }
  }
}
```

**Available MCP Tools:**
| Tool | Description |
|------|-------------|
| `list_formats` | List all 14 available export formats |
| `get_site_info` | Get WordPress site information, with `incomplete` when the site never described itself |
| `list_posts` | List posts with optional path filtering |
| `list_pages` | List pages from a site |
| `export_site` | Full site export to any format |
| `get_post` | Get a specific post by ID |
| `list_categories` | List all categories |
| `list_media` | List media files |

`export_site` writes the same tree the CLI does, reader comments included, and
reports the counts back to the caller — an agent has no console to read warnings
from, so `stats.comments` is where a site whose comment route is closed shows up
as a zero. `noPosts`, `noPages`, `noProducts` and `noComments` switch a
collection off, matching the `--no-…` flags in [the CLI reference](CLI.md).

## Gaps: `incomplete`

An assistant has no console. Anything a human operator would have read as a
warning has to travel in the result, or it does not exist.

A collection the site would not read to the end, and a document it never served
at all, are both **gaps**: what was fetched is kept, the run continues, and the
hole is named under `incomplete`. A key that is absent means there were none.

```json
{
  "status": "success",
  "stats": { "posts": 0, "pages": 0 },
  "incomplete": [
    "posts: stopped at page 1 after 0 records: API returned status 500"
  ]
}
```

`get_site_info` carries the same key. A WordPress whose `/wp-json` root is
locked down answers with every identity field empty, and that record is
indistinguishable from a site that genuinely has no name — an assistant reads
it as *"this site has no title"* and tells a user so. It now says which it is:

```json
{
  "name": "",
  "url": "https://example.com",
  "incomplete": ["site info: site answered 403"]
}
```

The record still comes back — a sparse root does not make a site unexportable —
and the site's own fields keep the names and the top-level position they have
always had. The gap is only reported when **nothing** described the site: a root
that 404s while `/wp/v2/settings` answers is not a hole, because the site was
described, just not by the endpoint asked first.

## Protocol revisions

MCP is split into two eras, and they are not variations on each other.

A **modern** revision (`2026-07-28` and later) has no handshake at all. Every
request carries its own protocol version, client capabilities and optional
client identity in `_meta`, the server answers each one independently, and
`server/discover` returns versions, capabilities and identity in a single call.

A **legacy** revision (`2025-11-25` and earlier) opens with an `initialize`
exchange and keeps the negotiated version for the connection.

The spec's own compatibility matrix says a modern client talking to a legacy-only
server **fails outright** — there is nothing to degrade to. `wpmcp` is therefore
*dual-era*: it reads which era a request was written in and answers under those
rules.

| Revision | Era | Notes |
|---|---|---|
| `2026-07-28` | modern | Preferred. Per-request `_meta`, `server/discover`, `resultType` on every result |
| `2025-11-25` | legacy | |
| `2025-06-18` | legacy | |
| `2025-03-26` | legacy | |
| `2024-11-05` | legacy | The first MCP revision |

The four legacy revisions are each answered as themselves rather than collapsed
into the oldest. This server exposes tools and `ping` only, and the wire contract
for `initialize`, `tools/list`, `tools/call` and `ping` is identical across all
four, so a client asking for any of them gets that one back.

### Pinning a revision

```bash
wpmcp serve                            # both eras (default)
wpmcp serve --protocol modern          # 2026-07-28 only
wpmcp serve --protocol legacy          # the initialize handshake only
wpmcp serve --protocol 2024-11-05      # one revision, and nothing else
```

Pinning is a compatibility tool, not a hardening one. It changes what a client
that opens in the other era is told:

- `--protocol legacy` makes the server **look** legacy. `server/discover` gets an
  ordinary `-32601 Method not found`, which is exactly the signal a dual-era
  client falls back on. It must not get the modern version error, or it would
  keep negotiating a version this server will never answer.
- `--protocol modern` answers `initialize` with an error **naming the revisions
  it does speak**. A legacy client has no way to ask again, so that message is
  the only diagnostic its user will see.

### Version errors

A modern request in a revision this server does not implement is answered with
`UnsupportedProtocolVersionError`, which carries the list to retry from:

```json
{
  "jsonrpc": "2.0", "id": 1,
  "error": {
    "code": -32022,
    "message": "Unsupported protocol version",
    "data": { "supported": ["2026-07-28"], "requested": "1900-01-01" }
  }
}
```

`supported` lists modern revisions only. A client picks from it for its *next*
request, which is modern-shaped, so a legacy revision named here would only send
it back into the same error.

A modern request missing a required `_meta` field — `protocolVersion` or
`clientCapabilities` — is `-32602 Invalid params`. A stateless era has no earlier
message to take the missing half from, and guessing would reintroduce exactly the
connection state the era removed.
