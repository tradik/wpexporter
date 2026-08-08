package xmlrpccli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCmd_Properties(t *testing.T) {
	assert.Equal(t, "wpxmlrpc", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
	assert.NotEmpty(t, rootCmd.Long)
}

func TestExportCmd_Flags(t *testing.T) {
	// Verify export command has required flags
	urlFlag := exportCmd.Flags().Lookup("url")
	assert.NotNil(t, urlFlag)

	usernameFlag := exportCmd.Flags().Lookup("username")
	assert.NotNil(t, usernameFlag)

	passwordFlag := exportCmd.Flags().Lookup("password")
	assert.NotNil(t, passwordFlag)

	// Check other flags exist
	assert.NotNil(t, exportCmd.Flags().Lookup("output"))
	assert.NotNil(t, exportCmd.Flags().Lookup("format"))
}

func TestExportCmd_FlagDefaults(t *testing.T) {
	formatFlag := exportCmd.Flags().Lookup("format")
	assert.Equal(t, "json", formatFlag.DefValue)
}

func TestGlobalFlags(t *testing.T) {
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
}
