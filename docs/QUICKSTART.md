# Quick start

One command per job: point the exporter at a WordPress site, choose a format, and read the files it writes. These are the invocations worth knowing before anything else — every flag they use is spelled out in the [command line reference](CLI.md).

## REST API Export (wpexporter)
```bash
# Export all content from a WordPress site
wpexportjson export --url https://example.com --output ./export

# Export with brute force discovery
wpexportjson export --url https://example.com --brute-force --output ./export

# Export to specific format
wpexportjson export --url https://example.com --format json --output ./export.json

# Export and create ZIP archive
wpexportjson export --url https://example.com --zip

# Export to ZIP only (remove files after creating ZIP)
wpexportjson export --url https://example.com --zip --no-files

# Export to Markdown with ZIP archive
wpexportjson export --url https://example.com -f markdown --zip

# Export to Shopify-compatible CSV format
wpexportjson export --url https://example.com -f shopify

# Export to Shopify CSV with ZIP archive
wpexportjson export --url https://example.com -f shopify --zip

# Export to Magento-compatible CSV format
wpexportjson export --url https://example.com -f magento

# Export to Magento CSV with ZIP archive
wpexportjson export --url https://example.com -f magento --zip

# Export to WordPress WXR format (for WordPress import)
wpexportjson export --url https://example.com -f wordpress

# Export to Drupal-compatible JSON format
wpexportjson export --url https://example.com -f drupal

# Export to Wix-compatible JSON format
wpexportjson export --url https://example.com -f wix

# Export to Squarespace-compatible XML format
wpexportjson export --url https://example.com -f squarespace

# Export to Webflow-compatible CSV format
wpexportjson export --url https://example.com -f webflow

# Export to Weebly-compatible format (XML + JSON)
wpexportjson export --url https://example.com -f weebly

# Export to PrestaShop-compatible CSV format
wpexportjson export --url https://example.com -f prestashop

# Export to Ghost-compatible JSON format
wpexportjson export --url https://example.com -f ghost

# Export to Strapi-compatible JSON format
wpexportjson export --url https://example.com -f strapi

# Export to Contentful-compatible JSON format
wpexportjson export --url https://example.com -f contentful
```

## XML-RPC Export (wpxmlrpc)
```bash
# Export with authentication
wpxmlrpc export --url https://example.com --username admin --password mypassword --output ./xmlrpc-export

# Export to markdown format
wpxmlrpc export --url https://example.com --username admin --password mypassword --format markdown --output ./markdown-export
```
