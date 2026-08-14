# MCP server

`wpmcp` speaks the Model Context Protocol over stdio, so Claude and other assistants can inspect and export a WordPress site without a shell between them. Eight tools cover site information, listings, a single post and a full export in any supported format.

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
| `get_site_info` | Get WordPress site information |
| `list_posts` | List posts with optional path filtering |
| `list_pages` | List pages from a site |
| `export_site` | Full site export to any format |
| `get_post` | Get a specific post by ID |
| `list_categories` | List all categories |
| `list_media` | List media files |
