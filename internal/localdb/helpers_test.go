package localdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
	"github.com/tradik/wpexporter/internal/wpconfig"
)

// settings is the canned site's wp-config.php.
func settings() *wpconfig.Settings {
	return &wpconfig.Settings{DBName: "wp", DBUser: "wp", DBPassword: "hunter2", DBHost: "localhost",
		DBCharset: "utf8mb4", TablePrefix: "wp_", ABSPath: "/var/www/html", File: "/var/www/html/wp-config.php"}
}

// openSite opens the canned site and checks, at the end of the test, that no
// statement broke a rule.
func openSite(t *testing.T) (*Source, *localdbtest.Recorder) {
	t.Helper()
	db, mock, rec := localdbtest.New(t)
	localdbtest.ExpectSnapshot(mock)
	localdbtest.ExpectSite(mock)

	src, err := New(context.Background(), db, settings(), Options{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, src.Close())
		require.Empty(t, rec.Violations())
	})
	return src, rec
}
