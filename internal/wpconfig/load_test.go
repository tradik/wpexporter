package wpconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestLoadFromABSPATH(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "wp-config.php"), base)

	s, err := Load(root, Overrides{}, env(nil))
	require.NoError(t, err)
	assert.Equal(t, "wordpress", s.DBName)
	assert.Equal(t, filepath.Join(root, "wp-config.php"), s.File)
	assert.Equal(t, root, s.ABSPath)
}

// WordPress also reads wp-config.php one directory up, unless that directory
// is itself a WordPress (it has its own wp-settings.php).
func TestLoadFromParent(t *testing.T) {
	parent := t.TempDir()
	site := filepath.Join(parent, "public")
	require.NoError(t, os.MkdirAll(site, 0o750))
	write(t, filepath.Join(parent, "wp-config.php"), base)

	s, err := Load(site, Overrides{}, env(nil))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(parent, "wp-config.php"), s.File)

	write(t, filepath.Join(parent, "wp-settings.php"), "<?php")
	_, err = Load(site, Overrides{}, env(nil))
	assert.ErrorContains(t, err, "wp-config.php")
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(t.TempDir(), Overrides{}, env(nil))
	assert.ErrorContains(t, err, "no wp-config.php")
}

func TestLoadRelativePath(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "wp-config.php"), base)
	t.Chdir(root)

	s, err := Load(".", Overrides{}, env(nil))
	require.NoError(t, err)
	assert.Equal(t, root, s.ABSPath)
}

func TestLoadTooLarge(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "wp-config.php"), base+strings.Repeat("/* padding */\n", maxFileBytes/10))

	_, err := Load(root, Overrides{}, env(nil))
	assert.ErrorContains(t, err, "larger than")
}

func TestLoadUnreadable(t *testing.T) {
	root := t.TempDir()
	// A directory where the file should be: it exists, and cannot be read as one.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "wp-config.php"), 0o750))
	_, err := Load(root, Overrides{}, env(nil))
	assert.Error(t, err)
}

func TestLoadPasswordFromEnvironment(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "wp-config.php"), strings.Replace(base, "'pw'", "vault_lookup()", 1))

	s, err := Load(root, Overrides{}, env(map[string]string{PasswordEnv: "from-env"}))
	require.NoError(t, err)
	assert.Equal(t, "from-env", s.DBPassword)
}

// The password never appears when the settings are printed or an error is
// reported.
func TestPasswordNeverPrinted(t *testing.T) {
	const secret = "s3cr3t-'$;)"
	src := strings.Replace(base, "'pw'", `'s3cr3t-\'$;)'`, 1)
	s := parseOK(t, src, nil)
	require.Equal(t, secret, s.DBPassword)

	for _, out := range []string{fmt.Sprint(s), fmt.Sprintf("%v", *s), fmt.Sprintf("%+v", s), fmt.Sprintf("%#v", s), s.String()} {
		assert.NotContains(t, out, "s3cr3t", out)
		assert.Contains(t, out, "wordpress")
	}

	// An error about a field computed in PHP names the field, never the text.
	bad := strings.Replace(base, "'pw'", `'s3cr3t' + 1`, 1)
	_, err := parse([]byte(bad), "/w", "/w/wp-config.php", Overrides{}, env(nil))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3cr3t")
}

// FuzzParse: the parser reads files it did not write. Whatever they contain, it
// returns — it never panics or loops.
func FuzzParse(f *testing.F) {
	f.Add([]byte(base))
	f.Add([]byte(`<?php define('DB_NAME', getenv_docker('X', "d\"q")); $table_prefix = "p_"; /* x`))
	f.Add([]byte(`define(define(define('`))
	f.Add([]byte(`$table_prefix = $table_prefix = ;`))
	f.Add([]byte("define('A', \"\\"))
	f.Fuzz(func(t *testing.T, src []byte) {
		_, _ = parse(src, "/w", "/w/wp-config.php", Overrides{}, env(nil))
	})
}
