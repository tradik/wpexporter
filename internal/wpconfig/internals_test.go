package wpconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindDefinesEdgeCases(t *testing.T) {
	got := findDefines(`define('ONE'); define('A-B', 'x'); define('OLD', 'v', true); define('LAST', 'z'`)
	assert.Equal(t, []statement{{name: "OLD", expr: "'v'"}}, got)
}

func TestFindTablePrefixEdgeCases(t *testing.T) {
	expr, ok := findTablePrefix(`$table_prefixes = 'no'; $table_prefix; $table_prefix = 'yes'`)
	assert.True(t, ok)
	assert.Equal(t, "'yes'", expr, "a statement may end at the end of the file")
}

func TestFailedDefineIsNotRetried(t *testing.T) {
	src := base + "define('WP_HOME', f());\ndefine('WP_HOME', 'https://second.example');\n"
	s := parseOK(t, src, nil)
	assert.Empty(t, s.Home, "PHP would have defined the first; the second never wins")
	assert.Len(t, s.Notes, 1)
}

func TestEvalEdgeCases(t *testing.T) {
	ev := &evaluator{absPath: "/w", file: "/w/wp-config.php", consts: map[string]value{}, getenv: env(nil)}

	v, err := ev.eval("true . 'x' . false . 'y'")
	require.NoError(t, err)
	assert.Equal(t, "1xy", v.str)

	v, err = ev.eval("__FILE__")
	require.NoError(t, err)
	assert.Equal(t, "/w/wp-config.php", v.str)

	v, err = ev.eval("'a' . 1.5")
	require.Error(t, err, "a decimal point is not a concatenation")
	assert.Empty(t, v.str)

	for _, expr := range []string{"'a' . ", "a b(1)", "f(1) (2)", "$_ENV['A']['B']"} {
		_, err := ev.eval(expr)
		assert.ErrorIs(t, err, errComputed, expr)
	}
}

func TestLoadCannotOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	root := t.TempDir()
	path := filepath.Join(root, "wp-config.php")
	require.NoError(t, os.WriteFile(path, []byte(base), 0o000))

	_, err := Load(root, Overrides{}, env(nil))
	assert.ErrorContains(t, err, "cannot open")
}

func TestLoadWithoutWorkingDirectory(t *testing.T) {
	gone := t.TempDir()
	t.Chdir(gone)
	require.NoError(t, os.Remove(gone))

	_, err := Load("relative", Overrides{}, env(nil))
	assert.ErrorContains(t, err, "--local")
}
