# Website builder export formats

Wix, Squarespace, Webflow and Weebly each read a different shape: JSON, WXR-compatible XML, CMS collection CSVs, or both XML and JSON. What every one of them shares is that the importer pulls media from the live site, so the export leaves those URLs absolute.

## Wix Export Format

The [Wix](https://www.wix.com/) export format generates a JSON file containing posts, pages, categories, tags, and media that can be imported to Wix.

### Output Files

| File | Description |
|------|-------------|
| `wix_export.json` | Complete export with all content |

### Wix Content Mapping

| WordPress Source | Wix Destination |
|------------------|-----------------|
| Posts | Blog posts |
| Pages | Static pages |
| Categories | Blog categories |
| Tags | Blog tags |
| Featured Image | Cover image |
| SEO Data | SEO fields |

### Usage Example

```bash
wpexportjson export --url https://your-wordpress-site.com -f wix
```

## Squarespace Export Format

The [Squarespace](https://www.squarespace.com/) export format generates a WXR-compatible XML file that can be imported directly into Squarespace.

### Output Files

| File | Description |
|------|-------------|
| `squarespace_export.xml` | Complete WXR export for Squarespace import |

### Squarespace Content Mapping

| WordPress Source | Squarespace Destination |
|------------------|------------------------|
| Posts | Blog posts |
| Pages | Pages |
| Categories | Categories |
| Tags | Tags |
| Media | Media library items |

### Usage Example

```bash
wpexportjson export --url https://your-wordpress-site.com -f squarespace
```

### Importing to Squarespace

1. Log in to your Squarespace account
2. Go to **Settings** > **Advanced** > **Import/Export**
3. Click **Import**
4. Select **WordPress** as the source
5. Upload `squarespace_export.xml`

## Webflow Export Format

The [Webflow](https://webflow.com/) export format generates CSV files compatible with Webflow CMS import.

### Output Files

| File | Description |
|------|-------------|
| `webflow_posts.csv` | Blog posts as CMS items |
| `webflow_pages.csv` | Static pages |
| `webflow_categories.csv` | Categories |
| `webflow_authors.csv` | Authors |
| `webflow_export.json` | Complete JSON backup |

### Webflow Content Mapping

| WordPress Source | Webflow Destination |
|------------------|---------------------|
| Post Title | Name |
| Post Slug | Slug |
| Post Content | Post Body |
| Post Date | Published On |
| Author | Author reference |
| Categories | Categories (multi-reference) |
| Tags | Tags |
| SEO Data | SEO fields |

### Usage Example

```bash
wpexportjson export --url https://your-wordpress-site.com -f webflow
```

## Weebly Export Format

The [Weebly](https://www.weebly.com/) export format generates both XML and JSON files for maximum compatibility.

### Output Files

| File | Description |
|------|-------------|
| `weebly_export.xml` | WXR-compatible XML export |
| `weebly_export.json` | JSON export with posts and pages |

### Usage Example

```bash
wpexportjson export --url https://your-wordpress-site.com -f weebly
```
