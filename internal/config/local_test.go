package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Database details never leave the process: not in an export, not in a
// checkpoint, not in a log line made from the configuration (FEAT-001 §4).
func TestDatabaseFieldsAreNeverSerialized(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DBHost, cfg.DBName, cfg.DBUser, cfg.DBPrefix = "db.internal", "secretdb", "secretuser", "pfx_"
	cfg.UploadsURL, cfg.UploadsDir = "https://example.com/u", "/srv/u"
	cfg.LocalPath, cfg.BlogID, cfg.Origin = "/var/www/html", 2, "http://127.0.0.1:8080"

	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	for _, secret := range []string{"db.internal", "secretdb", "secretuser", "pfx_", "/srv/u", "db_host", "password"} {
		assert.NotContains(t, string(raw), secret)
	}
	assert.Contains(t, string(raw), `"local_path":"/var/www/html"`)
	assert.Contains(t, string(raw), `"origin":"http://127.0.0.1:8080"`)
}

func TestValidateLocalMode(t *testing.T) {
	cfg := DefaultConfig()
	assert.ErrorContains(t, cfg.Validate(), "URL is required")

	cfg.LocalPath = "/var/www/html"
	assert.NoError(t, cfg.Validate(), "the address can come from the database")

	cfg.BlogID = -1
	assert.ErrorContains(t, cfg.Validate(), "blog_id")
}
