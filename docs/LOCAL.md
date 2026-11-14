# Local mode and `--origin`

Exporting a site over its public address goes through whatever stands in front
of it. Behind Cloudflare that is usually the end of the story: the REST API
answers with a challenge page, the crawl is rate-limited or blocked, and the
export comes back thin or empty. Two options, added in 1.9.0, take the proxy
out of the path.

| You have | Use | What it does |
|---|---|---|
| A shell on any machine that can reach the site's web server directly | `--origin` | The ordinary REST export, with every request for the site sent straight to its web server |
| A shell on the WordPress server itself | `--local` | Reads `wp-config.php` and the database (read only), and crawls the rendered pages through the web server |

## `--origin`: the REST export without the proxy

```bash
wpexporter export --url https://example.com --origin https://127.0.0.1 -f ssg
```

`--origin scheme://host[:port]` is where requests for the site really go. The
request still names the site — the `Host` header and the TLS server name stay
`example.com`, only the address dialled changes — so the web server answers
exactly as it would for a visitor, and the certificate is checked against the
site's name. The site's `www.` twin is routed the same way; every other host (a
CDN, an embed) is fetched as usual.

| Situation | Flag |
|---|---|
| Web server on this machine, TLS | `--origin https://127.0.0.1` |
| WordPress behind `cloudflared`, plain HTTP on a local port | `--origin http://127.0.0.1:8080` |
| The origin uses a Cloudflare Origin CA certificate | add `--origin-insecure` |

`--origin-insecure` skips certificate verification **only** on connections to
the origin; every other host is verified. When the origin speaks plain HTTP for
an `https` site, `X-Forwarded-Proto: https` is sent, without which WordPress
redirects every request back to `https` forever. Credentials (`--auth-*`) are
never sent to a foreign host after a redirect.

## `--local`: the site from its own database

```bash
# on the server, next to WordPress
wpexporter export --local /var/www/html -f ssg

# WordPress behind a cloudflared tunnel, origin on a local port
wpexporter export --local /var/www/html --origin http://127.0.0.1:8080 -f ssg
```

What happens:

1. **`wp-config.php` is read, never executed.** It is looked for in the
   directory given, then the one above (as WordPress itself does). Comments are
   ignored; `define()` values and `$table_prefix` are understood as plain
   strings, concatenations, `getenv()`, `getenv_docker()` (the official Docker
   image), `$_ENV[]`, `$_SERVER[]`, `__DIR__` and `ABSPATH`. A value computed any
   other way is an error that names the flag to give instead — nothing is
   guessed.
2. **The database is opened read only**, inside one `READ ONLY` transaction on a
   consistent snapshot. Nothing is written and nothing is locked.
3. **The site's web server is found**: `--origin` if given, otherwise
   `https://127.0.0.1:443` then `http://127.0.0.1:80`, asked for the site by
   name. If one answers, `--assisted-crawl` and `--crawl-content` are switched on
   (unless you set them), with `--crawl-content-mode always`.
4. **Every published document, term, author, approved comment, menu and media
   item** is read from the database.
5. **Each document's address is asked of the site.** The database gives the
   permalink structure; rewrite slugs of custom types and plugins such as WPML
   live in PHP. WordPress answers `/?p=ID`, `/?page_id=ID` and
   `/?post_type=T&p=ID` with a redirect to the canonical address, and that
   address is used.
6. **Each page's body is the rendered page**, crawled through the web server.
   Where the crawl gives nothing, the stored body is cleaned (block comments
   removed, paragraphs added) and the report says how many documents that was,
   and how many still contain shortcodes.
7. **Media is copied from the uploads directory** on disk, with the same
   `--max-media-mb` limit as a download. A path that resolves outside the
   uploads directory — `..`, or a symbolic link pointing elsewhere — is refused.

Without a web server the export still runs, from the database alone, and says
so in the summary and in `metadata.json`.

### Flags

| Flag | Config key | Meaning |
|---|---|---|
| `--local <path>` | `local_path` | The WordPress directory (ABSPATH). `--url` becomes optional: the site's `home` option is used |
| `--origin <url>` | `origin` | The site's web server (see above) |
| `--origin-insecure` | `origin_insecure` | Skip certificate checks on origin connections only |
| `--db-host`, `--db-name`, `--db-user`, `--db-prefix` | `db_host` … | Override what `wp-config.php` says |
| env `WPEXPORTER_DB_PASSWORD` | — | The database password, when `wp-config.php` cannot give it. There is deliberately no flag (a flag is visible in `ps`) and no config key |
| `--blog-id <n>` | `blog_id` | Multisite: which site to read (default: the main one) |

`DB_HOST` is read the way PHP reads it: `host`, `host:3307`, `[::1]:3306`,
`localhost:/run/mysqld/mysqld.sock`; a bare `localhost` means the socket (the
usual paths are tried, then `127.0.0.1:3306`). A name that does not resolve
from where you run — a Docker service name like `db` — is an error suggesting
`--db-host`.

Options that make no sense against a database are refused with the reason:
`--brute-force`, `--scan-range`, `--max-id`, `--resume`, `--from-sitemap` and
`--auth-*`.

### A preview: `--limit` and its kin

`--limit`, `--limit-per-type` and `--limit-posts`/`-pages`/`-media`/`-products`
mean what they mean over REST: newest first, one total budget spent across the
kinds in the order they are read (posts, pages, products, custom types,
media), and a budget per kind — a custom type named by its slug. The budget is
the SQL `LIMIT`: `--limit-posts 5` reads five rows, not the table. The summary
says what was left (`Posts: 5 (limited from 75)`), counted by the database
under the same filter as the read. Products, read over REST through the
origin, draw on the same budget. A limited export copies only the media its
documents reference, as on the REST path.

Menus, comments and users are not limited, on REST or here. The addresses
their links need still come from a full read of the published documents, so
for a strictly bounded database read add `--no-menus --no-comments`.

### `--cache`

There are no API responses to cache in local mode; what `--cache` keeps
(with `--cache-ttl`, `--cache-dir` and `--cache-clear`) is the crawl through
the origin — each rendered page's content and SEO — exactly as it does for
the REST path's crawl, plus the products read over REST.

### Docker

The official WordPress image writes `wp-config.php` with `getenv_docker()`, so
the values live in the container's environment. Run wpexporter in the
container's network and give it the site read only:

```bash
docker run --rm --network container:wordpress \
  -v /var/lib/docker/volumes/wp_html/_data:/site:ro -v "$PWD/export:/export" \
  -e WORDPRESS_DB_HOST -e WORDPRESS_DB_USER -e WORDPRESS_DB_NAME \
  -e WPEXPORTER_DB_PASSWORD="$WORDPRESS_DB_PASSWORD" \
  --user "$(id -u www-data):$(id -g www-data)" \
  ghcr.io/tradik/wpexporter wpexporter export --local /site --origin http://127.0.0.1 -o /export/site -f ssg
```

From the host instead, give the database address the host can reach with
`--db-host 127.0.0.1:3306` and the image's variables in the environment.

### Permissions

The run needs to read `wp-config.php` (often mode `640`, group `www-data`) and
the uploads directory — run as the web server's user or a member of its group,
not as root. The database user from `wp-config.php` is used as is; a dedicated
read-only user (`GRANT SELECT`) given with `--db-user` and
`WPEXPORTER_DB_PASSWORD` is better still.

The **snap** cannot use local mode: strict confinement denies it `/var/www` and
the MySQL socket, and it says so. Use the release binary, the `.deb`/`.rpm`
package or the Docker image.

### What is not exported

- Anything not published: drafts, private and password-protected documents,
  the trash, revisions; unapproved comments.
- Passwords, logins, e-mail addresses, IP addresses and browser strings — the
  columns holding them are never selected (`user_pass`, `user_email`,
  `user_login`, `user_activation_key`, `comment_author_email`,
  `comment_author_IP`, `comment_agent`). From `usermeta` only the biography is
  read; from `options` only a fixed list of display settings.
- `wp-config.php` itself, which is never copied to the output.
- **WooCommerce products** are not read from the database in this release
  (variations, HPOS and lookup tables need their own work). With a web server
  answering they come over the REST API as usual; without one, the summary says
  how many were left out.
- SEO plugin meta (Yoast, Rank Math) is not read — its values are templates
  such as `%%title%%`; the crawled page is the truth.
- Avatars (they are a hash of the e-mail address).

Local mode is deliberately not available through the MCP server: a remote agent
cannot make the server read `wp-config.php`.

## Testing against a real WordPress

`make test-integration` starts WordPress and MariaDB in Docker (ports bound to
127.0.0.1 only), fills the site with `wp-cli`, checks the three redirect forms
and runs a real `--local` export, then removes the stack and its volume. It is
not part of CI.
