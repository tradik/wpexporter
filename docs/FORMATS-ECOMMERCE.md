# E-commerce export formats

Posts and pages become products: Shopify and Magento take comma-delimited CSV, PrestaShop takes semicolon-delimited CSV, and each importer wants its own column names. Media URLs stay absolute in all three — the target platform imports the files from the live site.

## Shopify Export Format

The Shopify export format generates CSV files compatible with Shopify's product import system. This allows you to migrate WordPress content (posts, pages) to Shopify as products.

### Output Files

When exporting to Shopify format, the following files are generated:

| File | Description |
|------|-------------|
| `shopify_posts.csv` | WordPress posts exported as Shopify products |
| `shopify_pages.csv` | WordPress pages exported as Shopify products |
| `shopify_products.csv` | Combined posts and pages as products |
| `shopify_metadata.csv` | Site metadata and export statistics |

### CSV Column Mapping

WordPress content is mapped to Shopify product fields as follows:

| WordPress Field | Shopify Field |
|-----------------|---------------|
| Post Slug | Handle |
| Post Title | Title |
| Post Content (HTML) | Body (HTML) |
| Author Name | Vendor |
| First Category | Type |
| Tags | Tags (comma-separated) |
| Post Status | Published (TRUE/FALSE) |
| Featured Image | Image Src |
| Post Excerpt | SEO Description |
| Post ID | Variant SKU (format: WP-{id}) |

**Note:** The Body (HTML) field includes a styled metadata header with post details (ID, slug, dates, status, author, categories, tags, and hreflang links when available via `--assisted-crawl`).

### Usage Example

```bash
# Export WordPress content to Shopify format
wpexportjson export --url https://your-wordpress-site.com -f shopify

# Export to Shopify and create ZIP for easy upload
wpexportjson export --url https://your-wordpress-site.com -f shopify --zip
```

### Importing to Shopify

1. Log in to your Shopify Admin
2. Go to **Products** > **Import**
3. Click **Add file** and select `shopify_products.csv`
4. Review the import preview
5. Click **Import products**

> **Note**: The exported CSV follows Shopify's official product CSV format. For best results, review the [Shopify CSV import documentation](https://help.shopify.com/en/manual/products/import-export/using-csv).

## Magento Export Format

The Magento export format generates CSV files compatible with Magento 2's product import system. This allows you to migrate WordPress content (posts, pages) to Magento as simple products.

### Output Files

When exporting to Magento format, the following files are generated:

| File | Description |
|------|-------------|
| `magento_posts.csv` | WordPress posts exported as Magento products |
| `magento_pages.csv` | WordPress pages exported as Magento products |
| `magento_products.csv` | Combined posts and pages as products |
| `magento_metadata.csv` | Site metadata and export statistics |

### CSV Column Mapping

WordPress content is mapped to Magento product fields as follows:

| WordPress Field | Magento Field |
|-----------------|---------------|
| Post Slug (uppercase) | sku |
| Post Title | name |
| Post Content (HTML) | description |
| Post Excerpt | short_description |
| Categories | categories (Default Category/Name format) |
| Tags | meta_keywords |
| Post Slug | url_key |
| Post Title | meta_title |
| Post Excerpt | meta_description |
| Featured Image | base_image, small_image, thumbnail_image |
| Post Status | product_online (1=enabled, 0=disabled) |

### Usage Example

```bash
# Export WordPress content to Magento format
wpexportjson export --url https://your-wordpress-site.com -f magento

# Export to Magento and create ZIP for easy upload
wpexportjson export --url https://your-wordpress-site.com -f magento --zip
```

### Importing to Magento 2

1. Log in to your Magento 2 Admin Panel
2. Go to **System** > **Data Transfer** > **Import**
3. Select **Products** as Entity Type
4. Choose **Add/Update** as Import Behavior
5. Upload `magento_products.csv`
6. Click **Check Data** to validate
7. Click **Import** to complete

> **Note**: Before importing, ensure image files are uploaded to `/pub/media/import/` on your Magento server. For best results, review the [Magento 2 CSV import documentation](https://experienceleague.adobe.com/docs/commerce-admin/systems/data-transfer/import/data-import.html).

## PrestaShop Export Format

The [PrestaShop](https://www.prestashop.com/) export format generates semicolon-delimited CSV files compatible with PrestaShop's import system. Posts and pages are converted to products.

### Output Files

| File | Description |
|------|-------------|
| `prestashop_products.csv` | Products (from posts/pages) |
| `prestashop_posts.csv` | Blog posts |
| `prestashop_pages.csv` | CMS pages |
| `prestashop_categories.csv` | Product categories |
| `prestashop_metadata.csv` | Export metadata |
| `prestashop_export.json` | Complete JSON backup |

### PrestaShop Content Mapping

| WordPress Source | PrestaShop Destination |
|------------------|----------------------|
| Post Title | Product name |
| Post Content | Product description |
| Post Excerpt | Short description |
| Categories | Product categories |
| Tags | Tags |
| Featured Image | Product image |
| Post ID | Reference (WP-{id}) |

### Usage Example

```bash
wpexportjson export --url https://your-wordpress-site.com -f prestashop
```
