package localdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/wpconfig"
)

// Options say which site to read and how patiently.
type Options struct {
	// BlogID is the multisite site to read; 0 or 1 is the main site.
	BlogID int
	// Timeout bounds connecting and each read.
	Timeout time.Duration
	// Limits caps the exported collections (--limit and its kin); nil reads
	// everything. It is shared with the REST client that reads products, so
	// one total budget covers both.
	Limits *api.Limits
}

// beginReadOnly opens the one transaction every query runs in: read only, on
// a consistent snapshot, so an export of a live site is one moment of it.
const beginReadOnly = "START TRANSACTION READ ONLY, WITH CONSISTENT SNAPSHOT"

// endReadOnly ends it. Nothing was written, so there is nothing to keep.
const endReadOnly = "ROLLBACK"

// quietLogger drops the driver's own log lines: they go to stderr past every
// redaction this package does.
type quietLogger struct{}

// Print implements mysql.Logger.
func (quietLogger) Print(...any) {}

// newConnector is mysql.NewConnector; swapped in tests, since nothing this
// package sets makes the real one fail.
var newConnector = mysql.NewConnector

// Open connects to the database described by s and starts the read-only
// snapshot.
func Open(ctx context.Context, s *wpconfig.Settings, opts Options) (*Source, error) {
	ep, err := parseEndpoint(s.DBHost, lookupHost)
	if err != nil {
		return nil, err
	}

	cfg := mysql.NewConfig()
	cfg.User = s.DBUser
	cfg.Passwd = s.DBPassword
	cfg.Net = ep.network
	cfg.Addr = ep.addr
	cfg.DBName = s.DBName
	cfg.ParseTime = false // 0000-00-00 dates exist; they are read as text
	cfg.Timeout = opts.Timeout
	cfg.ReadTimeout = opts.Timeout
	cfg.Logger = quietLogger{}

	// The charset WordPress connects with (SET NAMES), so a latin1 site's
	// double-encoded text comes out the way the site itself shows it. Not a
	// Params entry: the driver sends those as SET <name>=…, and the server
	// has no variable called charset (found against MariaDB 12.3).
	err = cfg.Apply(mysql.Charset(s.DBCharset, ""))
	var connector driver.Connector
	if err == nil {
		connector, err = newConnector(cfg)
	}
	if err != nil {
		return nil, redact(fmt.Errorf("database settings: %w", err), s.DBPassword)
	}
	src, err := New(ctx, sql.OpenDB(connector), s, opts)
	if err != nil {
		return nil, fmt.Errorf("cannot read database %s on %s as %s: %w", s.DBName, ep.addr, s.DBUser, err)
	}
	return src, nil
}

// redact returns err with every occurrence of secret replaced. Driver errors
// are not supposed to carry the password; this makes sure.
func redact(err error, secret string) error {
	if err == nil || secret == "" || !strings.Contains(err.Error(), secret) {
		return err
	}
	return redactedError{msg: strings.ReplaceAll(err.Error(), secret, "[redacted]")}
}

// redactedError is an error whose text has been cleaned.
type redactedError struct{ msg string }

func (e redactedError) Error() string { return e.msg }
