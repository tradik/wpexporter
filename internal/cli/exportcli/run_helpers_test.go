package exportcli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/pkg/models"
)

// resetExportFlags returns every export flag to its default and unmarks it,
// so one end-to-end test cannot leak a flag into the next.
func resetExportFlags(t *testing.T) {
	t.Helper()
	reset := func(f *pflag.Flag) {
		require.NoError(t, f.Value.Set(f.DefValue))
		f.Changed = false
	}
	exportCmd.Flags().VisitAll(reset)
	rootCmd.PersistentFlags().VisitAll(reset)
	cfgFile = ""
}

// runWithFlags runs the export command with the given flags and returns what
// it printed.
func runWithFlags(t *testing.T, flags map[string]string) (string, error) {
	t.Helper()
	resetExportFlags(t)
	t.Cleanup(func() { resetExportFlags(t) })
	t.Chdir(t.TempDir()) // no config.yaml from the working directory
	// A test binary built by a snap-installed Go inherits SNAP*; the export
	// would then refuse a /tmp output as if it were confined (#19).
	t.Setenv("SNAP", "")
	t.Setenv("SNAP_NAME", "")
	for name, value := range flags {
		set := exportCmd.Flags().Set
		if exportCmd.Flags().Lookup(name) == nil {
			set = rootCmd.PersistentFlags().Set // --verbose, --config
		}
		require.NoError(t, set(name, value), name)
	}

	stdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	runErr := runExport(exportCmd, nil)

	_ = w.Close()
	os.Stdout = stdout
	return <-done, runErr
}

// readExport reads the export.json a -f json run wrote into dir.
func readExport(t *testing.T, dir string) models.ExportData {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "export.json")) // #nosec G304 -- test output
	require.NoError(t, err)
	var data models.ExportData
	require.NoError(t, json.Unmarshal(raw, &data))
	return data
}

// captureStdout returns what fn printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	stdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = stdout
	return <-done
}
