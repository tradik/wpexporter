package localdb

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/wpconfig"
)

func TestParseEndpoint(t *testing.T) {
	saved := socketExists
	assert.True(t, saved("/"))
	assert.False(t, saved("/no/such/socket"))
	sockets := map[string]bool{}
	socketExists = func(p string) bool { return sockets[p] }
	t.Cleanup(func() { socketExists = saved })
	known := func(host string) error {
		if host == "db.internal" {
			return nil
		}
		return errors.New("no such host")
	}

	cases := []struct {
		host string
		want endpoint
	}{
		{"db.internal", endpoint{"tcp", "db.internal:3306"}},
		{"db.internal:3307", endpoint{"tcp", "db.internal:3307"}},
		{"10.0.0.5", endpoint{"tcp", "10.0.0.5:3306"}},
		{"[::1]:3310", endpoint{"tcp", "[::1]:3310"}},
		{"[::1]", endpoint{"tcp", "[::1]:3306"}},
		{"::1", endpoint{"tcp", "[::1]:3306"}},
		{"localhost:/run/mysqld/mysqld.sock", endpoint{"unix", "/run/mysqld/mysqld.sock"}},
		{":/tmp/custom.sock", endpoint{"unix", "/tmp/custom.sock"}},
		{"localhost", endpoint{"tcp", "127.0.0.1:3306"}},
		{"", endpoint{"tcp", "127.0.0.1:3306"}},
		{"localhost:3308", endpoint{"tcp", "127.0.0.1:3308"}},
	}
	for _, tc := range cases {
		got, err := parseEndpoint(tc.host, known)
		require.NoError(t, err, tc.host)
		assert.Equal(t, tc.want, got, tc.host)
	}

	// PHP's "localhost" means the socket: the first one present wins.
	sockets["/var/lib/mysql/mysql.sock"] = true
	got, err := parseEndpoint("localhost", known)
	require.NoError(t, err)
	assert.Equal(t, endpoint{"unix", "/var/lib/mysql/mysql.sock"}, got)

	for host, want := range map[string]string{
		"db":            "does not resolve",
		"db.internal:0": "port",
		"db.internal:x": "port",
		"[::1]:99999":   "port",
		"[::1":          "does not resolve", // unclosed: the whole text is the host
	} {
		_, err := parseEndpoint(host, known)
		assert.ErrorContains(t, err, want, host)
	}
}

func TestDefaultHelpers(t *testing.T) {
	assert.NoError(t, lookupHost("localhost"))
	assert.Error(t, lookupHost("no-such-host.invalid"))
}

func TestOpenUnreachable(t *testing.T) {
	s := &wpconfig.Settings{DBName: "wp", DBUser: "wp", DBPassword: "hunter2", DBHost: "127.0.0.1:1",
		DBCharset: "utf8mb4", TablePrefix: "wp_"}
	_, err := Open(context.Background(), s, Options{Timeout: time.Second})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read database wp on 127.0.0.1:1 as wp")
	assert.NotContains(t, err.Error(), "hunter2")

	s.DBHost = "no-such-host.invalid"
	_, err = Open(context.Background(), s, Options{})
	assert.ErrorContains(t, err, "does not resolve")
}

func TestOpenConnectorFails(t *testing.T) {
	newConnector = func(*mysql.Config) (driver.Connector, error) {
		return nil, errors.New("bad config for hunter2")
	}
	t.Cleanup(func() { newConnector = mysql.NewConnector })

	s := &wpconfig.Settings{DBPassword: "hunter2", DBHost: "127.0.0.1", TablePrefix: "wp_"}
	_, err := Open(context.Background(), s, Options{})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
}

func TestRedact(t *testing.T) {
	assert.NoError(t, redact(nil, "x"))
	plain := errors.New("plain")
	assert.Same(t, plain, redact(plain, ""))
	assert.Same(t, plain, redact(plain, "secret"))
	assert.Equal(t, "a [redacted] b", redact(errors.New("a secret b"), "secret").Error())
	quietLogger{}.Print("never shown")
}

// mockConnector hands database/sql the sqlmock driver, so Open's success path
// runs without a server.
type mockConnector struct {
	d   driver.Driver
	dsn string
}

func (c mockConnector) Connect(context.Context) (driver.Conn, error) { return c.d.Open(c.dsn) }
func (c mockConnector) Driver() driver.Driver                        { return c.d }

func TestOpenSucceeds(t *testing.T) {
	db, mock, err := sqlmock.NewWithDSN("localdb-open-test")
	require.NoError(t, err)
	mock.ExpectExec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("START TRANSACTION READ ONLY, WITH CONSISTENT SNAPSHOT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK").WillReturnResult(sqlmock.NewResult(0, 0))
	newConnector = func(cfg *mysql.Config) (driver.Connector, error) {
		assert.Empty(t, cfg.Params, "no session variables: charset is not one")
		assert.Contains(t, cfg.FormatDSN(), "charset=utf8mb4")
		assert.False(t, cfg.ParseTime)
		assert.False(t, cfg.MultiStatements)
		assert.False(t, cfg.AllowAllFiles)
		return mockConnector{d: db.Driver(), dsn: "localdb-open-test"}, nil
	}
	t.Cleanup(func() { newConnector = mysql.NewConnector })

	src, err := Open(context.Background(), settings(), Options{Timeout: time.Second})
	require.NoError(t, err)
	require.NoError(t, src.Close())
}
