# Navigation menus

Menu structure is the one part of a site that cannot be reconstructed from the content afterwards — nothing in a post records which menu it belonged to, in what order, or under what label. Menus are exported into `metadata.json`, and WordPress gates them behind authentication.

Menu structure is the one part of a site that **cannot be reconstructed from the content
afterwards** — nothing in a post records which menu it belonged to, in what order, or under
what label. Menus are exported into `metadata.json`:

```json
"menus": [
  {
    "id": 3, "name": "Categories", "slug": "categories", "locations": ["primary"],
    "items": [
      { "id": 41, "title": "Malta", "url": "/malta/", "parent": 0, "order": 1,
        "type": "taxonomy", "object": "category", "object_id": 5 },
      { "id": 42, "title": "About Us", "url": "/about-us", "parent": 0, "order": 2,
        "type": "post_type", "object": "page", "object_id": 7 }
    ]
  }
]
```

Item URLs follow `--link-style`, so navigation matches the exported permalinks. An item
pointing at another host keeps its absolute URL. Items are ordered by `menu_order`, which is
what the site renders by.

## ⚠️ Menus require authentication

WordPress gates `/wp/v2/menus` behind the `edit_theme_options` capability, so **a public REST
API still refuses them** regardless of how the menus are configured:

```console
$ curl -s https://example.com/wp-json/wp/v2/menus
{"code":"rest_cannot_view","message":"Sorry, you are not allowed to view menus.","data":{"status":401}}
```

Pass credentials to include them:

```bash
wpexportjson export --url https://example.com --auth-user admin --auth-pass "app password"
# or
wpexportjson export --url https://example.com --auth-token "$TOKEN"
```

Without credentials the export **prints a note and carries on** — menus are simply absent.
`--no-menus` skips the attempt entirely.
