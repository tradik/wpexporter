package wpconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// env returns a getenv over a fixed map.
func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

const base = `<?php
define( 'DB_NAME', 'wordpress' );
define( 'DB_USER', 'wpuser' );
define( 'DB_PASSWORD', 'pw' );
define( 'DB_HOST', 'localhost' );
$table_prefix = 'wp_';
`

func parseOK(t *testing.T, src string, vars map[string]string) *Settings {
	t.Helper()
	s, err := parse([]byte(src), "/var/www/html", "/var/www/html/wp-config.php", Overrides{}, env(vars))
	require.NoError(t, err)
	return s
}

func TestParseDefineVariants(t *testing.T) {
	cases := []struct {
		name, line, want string
	}{
		{"single quotes", `define('DB_PASSWORD', 'abc');`, "abc"},
		{"double quotes", `define("DB_PASSWORD", "abc");`, "abc"},
		{"spaces everywhere", `define( "DB_PASSWORD" , 'abc' ) ;`, "abc"},
		{"upper-case keyword", `DEFINE('DB_PASSWORD','abc');`, "abc"},
		{"escaped apostrophe", `define('DB_PASSWORD', 'it\'s');`, "it's"},
		{"escaped backslash", `define('DB_PASSWORD', 'a\\b');`, `a\b`},
		{"other backslash kept", `define('DB_PASSWORD', 'a\nb');`, `a\nb`},
		{"dollar in single quotes", `define('DB_PASSWORD', 'pa$word');`, "pa$word"},
		{"semicolon and paren", `define('DB_PASSWORD', 'a;b)c');`, "a;b)c"},
		{"quote of other kind", `define('DB_PASSWORD', "it's");`, "it's"},
		{"double-quoted escapes", `define('DB_PASSWORD', "a\"b\\c\$d\te");`, "a\"b\\c$d\te"},
		{"double-quoted newline", `define('DB_PASSWORD', "a\nb\rc\vd\fe\ef");`, "a\nb\rc\vd\fe\x1bf"},
		{"double-quoted unknown escape", `define('DB_PASSWORD', "a\qb");`, `a\qb`},
		{"comment markers inside", `define('DB_PASSWORD', 'a//b#c/*d*/');`, "a//b#c/*d*/"},
		{"concatenation", `define('DB_PASSWORD', 'ab' . "cd");`, "abcd"},
		{"empty", `define('DB_PASSWORD', '');`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.Replace(base, "define( 'DB_PASSWORD', 'pw' );", tc.line, 1)
			assert.Equal(t, tc.want, parseOK(t, src, nil).DBPassword)
		})
	}
}

func TestParseCommentedDefinesDoNotWin(t *testing.T) {
	src := `<?php
// define('DB_NAME', 'line');
# define('DB_NAME', 'hash');
/* define('DB_NAME', 'block');
   define('DB_NAME', 'block2'); */
define('DB_NAME', 'real'); // define('DB_NAME', 'trailing');
define('DB_USER', 'u'); define('DB_PASSWORD', 'p'); define('DB_HOST', 'h');
$table_prefix = "wp_";
`
	assert.Equal(t, "real", parseOK(t, src, nil).DBName)
}

func TestParseFirstDefineWins(t *testing.T) {
	src := base + "define('DB_NAME', 'second');\n"
	assert.Equal(t, "wordpress", parseOK(t, src, nil).DBName)
}

func TestParseTablePrefix(t *testing.T) {
	s := parseOK(t, strings.Replace(base, "$table_prefix = 'wp_';", `$table_prefix  =  "site2_" ;`, 1), nil)
	assert.Equal(t, "site2_", s.TablePrefix)

	s = parseOK(t, strings.Replace(base, "$table_prefix = 'wp_';",
		`$table_prefix = getenv_docker('WORDPRESS_TABLE_PREFIX', 'dock_');`, 1), nil)
	assert.Equal(t, "dock_", s.TablePrefix)

	// A comparison is not an assignment.
	s = parseOK(t, base+"if ($table_prefix == 'x') {}\n", nil)
	assert.Equal(t, "wp_", s.TablePrefix)
}

func TestParseTablePrefixInvalid(t *testing.T) {
	for _, bad := range []string{`'wp-'`, `'wp_; DROP'`, `''`, `get_prefix()`} {
		src := strings.Replace(base, "'wp_'", bad, 1)
		_, err := parse([]byte(src), "/w", "/w/wp-config.php", Overrides{}, env(nil))
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), "table_prefix")
	}

	_, err := parse([]byte(strings.Replace(base, "$table_prefix = 'wp_';", "", 1)),
		"/w", "/w/wp-config.php", Overrides{}, env(nil))
	assert.ErrorContains(t, err, "--db-prefix")

	// The override is validated just the same.
	_, err = parse([]byte(base), "/w", "/w/wp-config.php", Overrides{TablePrefix: "x y"}, env(nil))
	assert.ErrorContains(t, err, "table_prefix")
}

func TestParseOptionalConstants(t *testing.T) {
	src := base + `
define('DB_CHARSET', 'latin1');
define('MULTISITE', true);
define('WP_HOME', 'https://example.com');
define('WP_SITEURL', 'https://example.com/wp');
define('WP_CONTENT_DIR', '/srv/content');
define('UPLOADS', 'files');
`
	s := parseOK(t, src, nil)
	assert.Equal(t, "latin1", s.DBCharset)
	assert.True(t, s.Multisite)
	assert.Equal(t, "https://example.com", s.Home)
	assert.Equal(t, "https://example.com/wp", s.SiteURL)
	assert.Equal(t, "/srv/content", s.ContentDir)
	assert.Equal(t, "files", s.Uploads)
	assert.Empty(t, s.Notes)
}

func TestParseMultisiteSpellings(t *testing.T) {
	for expr, want := range map[string]bool{
		"true": true, "TRUE": true, "1": true, "'1'": true, "'true'": true,
		"false": false, "0": false, "''": false, "'0'": false, "null": false,
	} {
		s := parseOK(t, base+"define('MULTISITE', "+expr+");\n", nil)
		assert.Equal(t, want, s.Multisite, expr)
	}
	assert.False(t, parseOK(t, base, nil).Multisite)
}

func TestParseDefaultCharset(t *testing.T) {
	assert.Equal(t, "utf8mb4", parseOK(t, base, nil).DBCharset)
	assert.Equal(t, "utf8mb4", parseOK(t, base+"define('DB_CHARSET', '');\n", nil).DBCharset)
}

func TestParseContentDirExpressions(t *testing.T) {
	cases := map[string]string{
		"__DIR__ . '/content'":             "/var/www/html/content",
		"dirname(__FILE__) . '/content'":   "/var/www/html/content",
		"dirname( __FILE__ ) . '/content'": "/var/www/html/content",
		"ABSPATH . 'wp-content'":           "/var/www/html/wp-content",
		"'/srv' . '/x'":                    "/srv/x",
	}
	for expr, want := range cases {
		s := parseOK(t, base+"define('WP_CONTENT_DIR', "+expr+");\n", nil)
		assert.Equal(t, want, s.ContentDir, expr)
	}

	// An earlier constant may be reused.
	s := parseOK(t, base+"define('WP_CONTENT_DIR', '/srv/c');\ndefine('UPLOADS', WP_CONTENT_DIR . '/up');\n", nil)
	assert.Equal(t, "/srv/c/up", s.Uploads)
}

func TestParseContentDirUnresolvedIsANote(t *testing.T) {
	s := parseOK(t, base+"define('WP_CONTENT_DIR', realpath(__DIR__ . '/../c'));\ndefine('UPLOADS', $u);\n", nil)
	assert.Empty(t, s.ContentDir)
	assert.Empty(t, s.Uploads)
	require.Len(t, s.Notes, 2)
	assert.Contains(t, s.Notes[0], "WP_CONTENT_DIR")
	assert.Contains(t, s.Notes[1], "UPLOADS")
}

func TestParseOptionalComputedHomeIsANote(t *testing.T) {
	s := parseOK(t, base+"define('WP_HOME', 'https://' . $_SERVER['HTTP_HOST']);\n", nil)
	assert.Empty(t, s.Home)
	require.Len(t, s.Notes, 1)
	assert.Contains(t, s.Notes[0], "WP_HOME")
}

func TestParseIgnoresNonDefineCalls(t *testing.T) {
	src := base + `
if ( ! defined( 'ABSPATH' ) ) { define( 'ABSPATH', __DIR__ . '/' ); }
undefine('DB_NAME', 'x');
$x->define('DB_NAME', 'y');
define;
define(DB_NAME, 'bare');
require_once ABSPATH . 'wp-settings.php';
`
	assert.Equal(t, "wordpress", parseOK(t, src, nil).DBName)
}
