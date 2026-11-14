package wpconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnvironmentValues(t *testing.T) {
	src := `<?php
define('DB_NAME', getenv('WP_DB'));
define('DB_USER', getenv_docker('WORDPRESS_DB_USER', 'example user'));
define('DB_PASSWORD', $_ENV['WP_PW']);
define('DB_HOST', $_SERVER["WP_HOST"]);
$table_prefix = 'wp_';
`
	s := parseOK(t, src, map[string]string{"WP_DB": "envdb", "WP_PW": "envpw", "WP_HOST": "db:3306"})
	assert.Equal(t, "envdb", s.DBName)
	assert.Equal(t, "example user", s.DBUser, "getenv_docker falls back to its default")
	assert.Equal(t, "envpw", s.DBPassword)
	assert.Equal(t, "db:3306", s.DBHost)

	s = parseOK(t, src, map[string]string{"WP_DB": "a", "WORDPRESS_DB_USER": "fromenv", "WP_PW": "b", "WP_HOST": "c"})
	assert.Equal(t, "fromenv", s.DBUser)
}

func TestParseEnvironmentMissing(t *testing.T) {
	src := strings.Replace(base, "'pw'", "getenv('SECRET_PW')", 1)
	_, err := parse([]byte(src), "/w", "/w/wp-config.php", Overrides{}, env(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SECRET_PW")
	assert.Contains(t, err.Error(), PasswordEnv)

	src = strings.Replace(base, "'wpuser'", "$_ENV['THE_USER']", 1)
	_, err = parse([]byte(src), "/w", "/w/wp-config.php", Overrides{}, env(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "THE_USER")
	assert.Contains(t, err.Error(), "--db-user")
}

// The official Docker image reads secrets from <NAME>_FILE first. That file is
// not ours to read, and the default after it would be a wrong guess.
func TestParseDockerSecretFileIsNotGuessed(t *testing.T) {
	src := strings.Replace(base, "'pw'", "getenv_docker('WORDPRESS_DB_PASSWORD', 'example password')", 1)
	_, err := parse([]byte(src), "/w", "/w/wp-config.php", Overrides{},
		env(map[string]string{"WORDPRESS_DB_PASSWORD_FILE": "/run/secrets/db"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WORDPRESS_DB_PASSWORD_FILE")
}

func TestParseComputedValuesAreRefused(t *testing.T) {
	for _, expr := range []string{
		"my_secret()",
		"require __DIR__ . '/secret.php'",
		"$password",
		`"pw-$suffix"`,
		`"\x41"`,
		"FOO_UNDEFINED",
		"1 + 2",
		"getenv()",
		"getenv('A', 'B', 'C')",
		"getenv_docker('A')",
		"$_ENV[$key]",
		"'unterminated",
		"$_ENV['X'",
		"getenv('X'",
	} {
		src := strings.Replace(base, "'pw'", expr, 1)
		_, err := parse([]byte(src), "/w", "/w/wp-config.php", Overrides{}, env(nil))
		require.Error(t, err, expr)
		assert.Contains(t, err.Error(), "DB_PASSWORD", expr)
		assert.Contains(t, err.Error(), PasswordEnv, expr)
	}
}

func TestParseOverridesWin(t *testing.T) {
	src := strings.Replace(base, "'pw'", "my_secret()", 1)
	ov := Overrides{DBHost: "127.0.0.1:3307", DBName: "n", DBUser: "u", DBPassword: "fromenv", TablePrefix: "x_"}
	s, err := parse([]byte(src), "/w", "/w/wp-config.php", ov, env(nil))
	require.NoError(t, err)
	assert.Equal(t, Settings{
		DBName: "n", DBUser: "u", DBPassword: "fromenv", DBHost: "127.0.0.1:3307",
		DBCharset: "utf8mb4", TablePrefix: "x_", ABSPath: "/w", File: "/w/wp-config.php",
	}, *s)
}

func TestParseRequiredMissing(t *testing.T) {
	for name, flag := range map[string]string{
		"DB_NAME": "--db-name", "DB_USER": "--db-user", "DB_HOST": "--db-host", "DB_PASSWORD": PasswordEnv,
	} {
		src := regexpDefine(name).ReplaceAllString(base, "")
		_, err := parse([]byte(src), "/w", "/w/wp-config.php", Overrides{}, env(nil))
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), name)
		assert.Contains(t, err.Error(), flag)
	}
}
