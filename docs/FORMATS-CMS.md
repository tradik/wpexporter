# CMS and headless export formats

WXR for WordPress itself, and JSON shaped for the migration tool of Drupal, Ghost, Strapi and Contentful. These formats keep the content model — posts, pages, taxonomies, authors and media as separate entities rather than flattened rows.

## WordPress WXR Export Format

The WordPress export format generates a WXR (WordPress eXtended RSS) XML file that can be imported into another WordPress installation. This is the standard format used by WordPress for content migration.

### Output Files

When exporting to WordPress format, the following file is generated:

| File | Description |
|------|-------------|
| `wordpress_export.xml` | Complete WXR export with all content |

### WXR Content Mapping

| WordPress Source | WXR Element |
|------------------|-------------|
| Posts | `<item>` with `<wp:post_type>post</wp:post_type>` |
| Pages | `<item>` with `<wp:post_type>page</wp:post_type>` |
| Media/Attachments | `<item>` with `<wp:post_type>attachment</wp:post_type>` |
| Categories | `<wp:category>` |
| Tags | `<wp:tag>` |
| Authors | `<wp:author>` |
| Featured Images | `<wp:postmeta>` with `_thumbnail_id` |
| SEO Data | `<wp:postmeta>` with Yoast-compatible keys |

### Usage Example

```bash
# Export WordPress content to WXR format
wpexportjson export --url https://your-wordpress-site.com -f wordpress

# Export to WordPress WXR and create ZIP for easy transfer
wpexportjson export --url https://your-wordpress-site.com -f wordpress --zip
```

### Importing to WordPress

1. Log in to your WordPress Admin Dashboard
2. Go to **Tools** > **Import**
3. Click **Install Now** under WordPress (if not already installed)
4. Click **Run Importer**
5. Upload `wordpress_export.xml`
6. Assign authors and select whether to import attachments
7. Click **Submit** to complete

> **Note**: WXR is the official WordPress import/export format. For best results, review the [WordPress Import documentation](https://wordpress.org/documentation/article/importing-content/#wordpress).

## Drupal Export Format

The Drupal export format generates JSON files compatible with Drupal's Migrate module. This format is designed for migrating WordPress content to Drupal 8/9/10.

### Output Files

When exporting to Drupal format, the following files are generated:

| File | Description |
|------|-------------|
| `drupal_export.json` | Complete export with all content types |
| `drupal_nodes.json` | Posts and pages as Drupal nodes |
| `drupal_terms.json` | Categories and tags as taxonomy terms |
| `drupal_users.json` | Users as Drupal user accounts |
| `drupal_media.json` | Media files as Drupal media entities |

### Drupal Content Mapping

| WordPress Source | Drupal Destination |
|------------------|-------------------|
| Posts | Node type: `article` |
| Pages | Node type: `page` |
| Categories | Taxonomy vocabulary: `categories` |
| Tags | Taxonomy vocabulary: `tags` |
| Featured Image | Media entity reference (`field_image`) |
| Post Content | Body field with `full_html` format |
| Post Excerpt | Body summary field |
| SEO Data | Metatag module fields |

### Usage Example

```bash
# Export WordPress content to Drupal format
wpexportjson export --url https://your-wordpress-site.com -f drupal

# Export to Drupal and create ZIP for easy transfer
wpexportjson export --url https://your-wordpress-site.com -f drupal --zip
```

### Importing to Drupal

1. Install the **Migrate** and **Migrate Source JSON** modules
2. Upload the JSON files to your Drupal server
3. Create migration configuration files referencing the JSON sources
4. Run migrations using Drush: `drush migrate:import --all`

> **Note**: Drupal migration requires custom migration YAML configuration. The JSON structure is designed to work with `migrate_source_json` plugin. For best results, review the [Drupal Migrate documentation](https://www.drupal.org/docs/drupal-apis/migrate-api).

## Ghost Export Format

The [Ghost](https://ghost.org/) export format generates a JSON file compatible with Ghost CMS import.

### Output Files

| File | Description |
|------|-------------|
| `ghost_export.json` | Complete Ghost import format |

### Ghost Content Mapping

| WordPress Source | Ghost Destination |
|------------------|-------------------|
| Posts | Posts |
| Pages | Pages |
| Categories | Tags (with category prefix) |
| Tags | Tags |
| Users | Users |
| Featured Image | Feature image |
| SEO Data | Meta fields |

### Usage Example

```bash
wpexportjson export --url https://your-wordpress-site.com -f ghost
```

### Importing to Ghost

1. Log in to your Ghost Admin panel
2. Go to **Settings** > **Labs**
3. Find **Import content** section
4. Upload `ghost_export.json`

## Strapi Export Format

The [Strapi](https://strapi.io/) export format generates JSON files compatible with Strapi v4 headless CMS.

### Output Files

| File | Description |
|------|-------------|
| `strapi_export.json` | Complete export with all content types |
| `strapi_articles.json` | Blog articles |
| `strapi_pages.json` | Pages |
| `strapi_categories.json` | Categories |
| `strapi_tags.json` | Tags |
| `strapi_authors.json` | Authors |
| `strapi_media.json` | Media files |

### Strapi Content Mapping

| WordPress Source | Strapi Destination |
|------------------|-------------------|
| Posts | Articles (collection type) |
| Pages | Pages (collection type) |
| Categories | Categories (collection type) |
| Tags | Tags (collection type) |
| Users | Authors (collection type) |
| Media | Media library |
| SEO Data | SEO component fields |

### Usage Example

```bash
wpexportjson export --url https://your-wordpress-site.com -f strapi
```

## Contentful Export Format

The [Contentful](https://www.contentful.com/) export format generates a JSON file compatible with Contentful's import tool.

### Output Files

| File | Description |
|------|-------------|
| `contentful_export.json` | Complete Contentful import format |

### Contentful Content Mapping

| WordPress Source | Contentful Destination |
|------------------|----------------------|
| Posts | blogPost content type |
| Pages | page content type |
| Categories | category content type |
| Tags | tag content type |
| Users | author content type |
| Media | Assets |

### Content Types Created

The export includes content type definitions for:
- `blogPost` - Blog posts with title, slug, content, author, categories, tags
- `page` - Static pages with title, slug, content
- `category` - Categories with name, slug, description
- `tag` - Tags with name, slug
- `author` - Authors with name, slug, bio

### Usage Example

```bash
wpexportjson export --url https://your-wordpress-site.com -f contentful
```

### Importing to Contentful

1. Install the Contentful CLI: `npm install -g contentful-cli`
2. Log in: `contentful login`
3. Import: `contentful space import --content-file contentful_export.json`
