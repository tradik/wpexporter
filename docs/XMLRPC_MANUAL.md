# WordPress XML-RPC Export Manual

## Overview

The `wpxmlrpc` tool provides an alternative method for exporting WordPress content using the XML-RPC protocol. This is particularly useful when the REST API is disabled or when you need to access content that requires authentication.

## Prerequisites

- WordPress site with XML-RPC enabled
- Valid WordPress username and password
- Administrative or editor privileges on the WordPress site

## Installation

### From Source
```bash
git clone https://github.com/tradik/wpexportjson.git
cd wpexportjson
make build
```

The XML-RPC client will be built as `build/wpxmlrpc`.

### Using Go Install
```bash
go install github.com/tradik/wpexportjson/cmd/wpxmlrpc@latest
```

## Basic Usage

### Command Structure
```bash
wpxmlrpc export --url <wordpress-url> --username <username> --password <password> [options]
```

### Required Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `--url` | WordPress site URL | `https://example.com` |
| `--username` | WordPress username | `admin` |
| `--password` | WordPress password | `your-password` |

### Optional Parameters

| Parameter | Short | Description | Default |
|-----------|-------|-------------|---------|
| `--output` | `-o` | Output directory or file | `./xmlrpc-export` |
| `--format` | `-f` | Export format (json/markdown) | `json` |
| `--verbose` | `-v` | Enable verbose output | `false` |
| `--config` | | Configuration file path | - |

## Examples

### Basic Export
```bash
wpxmlrpc export --url https://myblog.com --username admin --password mypassword
```

### Export to Specific Directory
```bash
wpxmlrpc export \
  --url https://myblog.com \
  --username admin \
  --password mypassword \
  --output ./my-blog-backup
```

### Export as Markdown
```bash
wpxmlrpc export \
  --url https://myblog.com \
  --username admin \
  --password mypassword \
  --format markdown \
  --output ./markdown-export
```

### Using Configuration File
Create a `config.yaml` file:
```yaml
url: "https://myblog.com"
output: "./xmlrpc-export"
format: "json"
verbose: true
```

Then run:
```bash
wpxmlrpc export --username admin --password mypassword --config config.yaml
```

## Security Considerations

### Password Security
- **Never hardcode passwords** in scripts or configuration files
- Use environment variables for sensitive data:
  ```bash
  export WP_USERNAME="admin"
  export WP_PASSWORD="mypassword"
  wpxmlrpc export --url https://myblog.com --username $WP_USERNAME --password $WP_PASSWORD
  ```

### HTTPS Requirement
- Always use HTTPS URLs when possible to encrypt credentials in transit
- Avoid using XML-RPC over unencrypted HTTP connections

### Application Passwords (WordPress 5.6+)
For enhanced security, use WordPress Application Passwords:

1. Go to your WordPress admin → Users → Your Profile
2. Scroll to "Application Passwords"
3. Create a new application password
4. Use the generated password instead of your regular password

## XML-RPC Methods Used

The tool uses the following WordPress XML-RPC methods:

| Method | Purpose | Authentication Required |
|--------|---------|------------------------|
| `wp.getOptions` | Site information and connection test | Yes |
| `wp.getPosts` | Retrieve posts | Yes |
| `wp.getPages` | Retrieve pages | Yes |
| `wp.getMediaLibrary` | Retrieve media files | Yes |
| `wp.getTerms` | Retrieve categories and tags | Yes |
| `wp.getUsers` | Retrieve users | Yes |

## Troubleshooting

### XML-RPC Disabled
If you get an error about XML-RPC being disabled:

1. **Check if XML-RPC is enabled:**
   ```bash
   curl -X POST https://yoursite.com/xmlrpc.php \
     -H "Content-Type: text/xml" \
     -d '<?xml version="1.0"?><methodCall><methodName>system.listMethods</methodName></methodCall>'
   ```

2. **Enable XML-RPC in WordPress:**
   Add to your theme's `functions.php`:
   ```php
   add_filter('xmlrpc_enabled', '__return_true');
   ```

3. **Check for security plugins** that might block XML-RPC

### Authentication Errors
- Verify username and password are correct
- Check if two-factor authentication is enabled (may require app passwords)
- Ensure the user has sufficient privileges

### Connection Issues
- Verify the WordPress URL is correct
- Check if the site is behind a firewall or CDN
- Try increasing timeout in configuration

### Large Sites
For sites with many posts/pages:
- The tool automatically handles pagination
- Consider using the REST API client (`wpexportjson`) for better performance
- Monitor memory usage for very large exports

## Output Formats

### JSON Format
Exports all data as a single JSON file with the following structure:
```json
{
  "site": { ... },
  "posts": [ ... ],
  "pages": [ ... ],
  "media": [ ... ],
  "categories": [ ... ],
  "tags": [ ... ],
  "users": [ ... ],
  "stats": { ... },
  "exported_at": "2024-01-07T12:00:00Z"
}
```

### Markdown Format
Creates a directory structure:
```
output/
├── README.md           # Site information
├── posts/             # Individual post files
│   ├── 2024-01-01-post-title.md
│   └── ...
├── pages/             # Individual page files
│   ├── 2024-01-01-page-title.md
│   └── ...
├── media/             # Downloaded media files
│   ├── image1.jpg
│   └── ...
└── metadata.json      # Categories, tags, users, etc.
```

## Comparison with REST API Client

| Feature | XML-RPC (`wpxmlrpc`) | REST API (`wpexportjson`) |
|---------|---------------------|---------------------------|
| Authentication | Username/Password | Public API (no auth needed) |
| Performance | Slower | Faster |
| Brute Force | Not supported | Supported |
| Media Download | Limited | Full support |
| Compatibility | Older WordPress | WordPress 4.7+ |
| Security | Requires credentials | No credentials needed |

## Configuration File Reference

```yaml
# WordPress site URL (required via CLI)
url: "https://your-wordpress-site.com"

# Output directory or file path
output: "./xmlrpc-export"

# Export format: json or markdown
format: "json"

# Download media files (images, videos, etc.)
download_media: true

# Number of concurrent downloads
concurrent: 5

# HTTP request timeout in seconds
timeout: 30

# Number of retries for failed requests
retries: 3

# User agent string for HTTP requests
user_agent: "WordPress-XML-RPC-Export/1.0"

# Enable verbose output
verbose: false
```

## Advanced Usage

### Batch Processing
Process multiple sites:
```bash
#!/bin/bash
sites=("site1.com" "site2.com" "site3.com")
for site in "${sites[@]}"; do
  wpxmlrpc export --url "https://$site" --username admin --password "$WP_PASSWORD" --output "./exports/$site"
done
```

### Automated Backups
Create a cron job for regular backups:
```bash
# Add to crontab (crontab -e)
0 2 * * 0 /usr/local/bin/wpxmlrpc export --url https://myblog.com --username admin --password "$WP_PASSWORD" --output "/backups/$(date +\%Y-\%m-\%d)"
```

## API Reference

The XML-RPC client can be used programmatically:

```go
package main

import (
    "github.com/tradik/wpexportjson/internal/config"
    "github.com/tradik/wpexportjson/internal/xmlrpc"
)

func main() {
    cfg := config.DefaultConfig()
    cfg.URL = "https://example.com"
    
    client, err := xmlrpc.NewClient(cfg, "username", "password")
    if err != nil {
        panic(err)
    }
    
    posts, err := client.GetPosts()
    if err != nil {
        panic(err)
    }
    
    // Process posts...
}
```

## Support and Contributing

- Report issues: [GitHub Issues](https://github.com/tradik/wpexportjson/issues)
- Documentation: [Project README](../README.md)
- Contributing: See [Contributing Guidelines](../CONTRIBUTING.md)
