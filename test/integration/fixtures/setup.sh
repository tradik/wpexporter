#!/bin/sh
# Fills the integration WordPress with what local mode must find — and with
# what it must leave out (a draft, a private page, a password-protected post).
# Writes the IDs to /out/ids.env for the Go test.
set -eu
cd /var/www/html

wp core install --url=http://example.test --title="Integration Site" --admin_user=itadmin \
  --admin_password=it-admin-password --admin_email=itadmin@example.test --skip-email
mkdir -p wp-content/mu-plugins
cp /fixtures/portfolio.php wp-content/mu-plugins/portfolio.php
wp rewrite structure '/%year%/%monthnum%/%postname%/'
wp rewrite flush

wp term create category News --slug=news >/dev/null
POST=$(wp post create --porcelain --post_status=publish --post_title="Hello local" --post_name=hello-local \
  --post_content='<!-- wp:paragraph --><p>Stored body with [gallery ids="1"] in it.</p><!-- /wp:paragraph -->')
wp post term set "$POST" category news
ABOUT=$(wp post create --porcelain --post_type=page --post_status=publish --post_title=About --post_name=about \
  --post_content='<p>About us.</p>')
TEAM=$(wp post create --porcelain --post_type=page --post_status=publish --post_title=Team --post_name=team \
  --post_parent="$ABOUT" --post_content='<p>The team.</p>')
WORK=$(wp post create --porcelain --post_type=portfolio --post_status=publish --post_title=Bridge \
  --post_name=bridge --post_content='<p>A bridge.</p>')
DRAFT=$(wp post create --porcelain --post_status=draft --post_title="Secret draft")
PRIVATE=$(wp post create --porcelain --post_type=page --post_status=private --post_title="Private page")
LOCKED=$(wp post create --porcelain --post_status=publish --post_password=pw --post_title="Locked post")

# A 1x1 PNG, attached to the post as its featured image.
echo 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==' \
  | base64 -d > /tmp/pixel.png
MEDIA=$(wp media import /tmp/pixel.png --post_id="$POST" --featured_image --porcelain)

wp comment create --comment_post_ID="$POST" --comment_content="Nice post" --comment_author=Reader \
  --comment_author_email=reader@example.test --comment_author_IP=203.0.113.9 --comment_approved=1 >/dev/null

wp menu create Main >/dev/null
wp menu item add-post main "$TEAM" >/dev/null
wp menu item add-custom main Docs https://docs.example >/dev/null
wp menu item add-post main "$PRIVATE" >/dev/null

cat > /out/ids.env <<IDS
POST=$POST
ABOUT=$ABOUT
TEAM=$TEAM
WORK=$WORK
DRAFT=$DRAFT
PRIVATE=$PRIVATE
LOCKED=$LOCKED
MEDIA=$MEDIA
IDS
echo "setup done: $(cat /out/ids.env | tr '\n' ' ')"
