package exportcli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// newSourceCmd returns a command carrying only the flags this file registers,
// so a test can set them without disturbing the package's real command.
func newSourceCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	registerSourceFlags(cmd)
	return cmd
}

func TestSourceFlagsRegistered(t *testing.T) {
	for _, name := range []string{"origin", "origin-insecure"} {
		assert.NotNil(t, exportCmd.Flags().Lookup(name), name)
	}
}

func TestApplySourceFlagOverrides(t *testing.T) {
	cmd := newSourceCmd()
	require.NoError(t, cmd.Flags().Set("origin", "http://127.0.0.1:8080"))
	require.NoError(t, cmd.Flags().Set("origin-insecure", "true"))

	cfg := config.DefaultConfig()
	applySourceFlagOverrides(cmd, cfg)

	assert.Equal(t, "http://127.0.0.1:8080", cfg.Origin)
	assert.True(t, cfg.OriginInsecure)
}

func TestApplySourceFlagOverridesUntouched(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Origin = "https://10.0.0.5"
	applySourceFlagOverrides(newSourceCmd(), cfg)
	assert.Equal(t, "https://10.0.0.5", cfg.Origin, "a config-file value survives when the flag is not given")
	assert.False(t, cfg.OriginInsecure)
}

func TestValidateSource(t *testing.T) {
	cfg := config.DefaultConfig()
	assert.NoError(t, validateSource(cfg))

	cfg.Origin = "http://127.0.0.1:8080"
	assert.NoError(t, validateSource(cfg))

	cfg.Origin = "127.0.0.1/blog"
	assert.ErrorContains(t, validateSource(cfg), "--origin")

	cfg.Origin = ""
	cfg.OriginInsecure = true
	assert.ErrorContains(t, validateSource(cfg), "--origin-insecure")
}

// --local stands in for --url at parse time; with neither, the command still
// refuses to run, as it always refused a run without --url. (Found by the
// integration test: url used to be a hard cobra requirement.)
func TestURLOrLocalIsRequired(t *testing.T) {
	resetExportFlags(t)
	t.Cleanup(func() { resetExportFlags(t) })

	assert.Error(t, exportCmd.ValidateFlagGroups(), "neither --url nor --local")
	require.NoError(t, exportCmd.Flags().Set("local", "/var/www/html"))
	assert.NoError(t, exportCmd.ValidateRequiredFlags())
	assert.NoError(t, exportCmd.ValidateFlagGroups())

	resetExportFlags(t)
	require.NoError(t, exportCmd.Flags().Set("url", "https://example.com"))
	assert.NoError(t, exportCmd.ValidateFlagGroups())
}
