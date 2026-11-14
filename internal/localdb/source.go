// Package localdb reads a WordPress site's content straight from its MySQL or
// MariaDB database, read-only, into the same models the REST client fills.
//
// It is the source behind local mode (FEAT-001): run on the server, next to the
// site, where the REST API is not in the way — Cloudflare, a security plugin,
// a disabled route. Everything runs inside one READ ONLY transaction on a
// consistent snapshot; nothing is written and nothing is locked.
//
// What leaves the database is what a visitor could see: published documents
// without a password, approved comments, the authors of those documents. The
// queries never select credentials or private contact data (see guard), and
// the only text interpolated into SQL is the validated table prefix.
package localdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/wpconfig"
	"github.com/tradik/wpexporter/pkg/models"
)

// Source is one site's database, opened on a read-only snapshot. It answers
// the same questions as the REST client, with the same signatures.
type Source struct {
	db       *sql.DB
	conn     *sql.Conn
	q        *querier
	end      func() error
	settings *wpconfig.Settings
	opts     Options

	options    map[string]string
	docs       map[string][]models.WordPressPost
	categories []models.WordPressCategory
	tags       []models.WordPressTag
	types      []api.PostType
	counts     map[string]int
	products   int
	notes      []string
	// stated is each limited collection's size in the database, for the
	// summary's "limited from".
	stated map[string]int
}

// New starts the read-only snapshot on db and returns the source. ctx governs
// the snapshot for the source's whole life: every read is sent under it, and
// canceling it ends them. Open is the usual way in; New exists so tests can
// hand in a mock database.
func New(ctx context.Context, db *sql.DB, s *wpconfig.Settings, opts Options) (*Source, error) {
	if !wpconfig.ValidPrefix(s.TablePrefix) {
		return nil, fmt.Errorf("table prefix %q is not valid", s.TablePrefix)
	}
	if opts.BlogID > 1 && !s.Multisite {
		return nil, fmt.Errorf("--blog-id %d: wp-config.php does not enable MULTISITE", opts.BlogID)
	}

	// One dedicated connection: a transaction belongs to a session.
	conn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, redact(err, s.DBPassword)
	}
	if err := startSnapshot(ctx, conn); err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, redact(err, s.DBPassword)
	}

	return &Source{
		db:       db,
		conn:     conn,
		end:      snapshotEnder(ctx, conn),
		q:        &querier{run: boundQuery(ctx, conn), prefix: s.TablePrefix, blogID: opts.BlogID},
		settings: s,
		opts:     opts,
		docs:     map[string][]models.WordPressPost{},
		counts:   map[string]int{},
		stated:   map[string]int{},
	}, nil
}

// Close ends the snapshot and releases the connection.
func (s *Source) Close() error {
	err := s.end()
	_ = s.conn.Close()
	_ = s.db.Close()
	return s.fail(err)
}

// startSnapshot opens the read-only snapshot on conn.
func startSnapshot(ctx context.Context, conn *sql.Conn) error {
	for _, stmt := range []string{snapshotIsolation, beginSnapshot} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// boundQuery sends queries on conn under ctx.
func boundQuery(ctx context.Context, conn *sql.Conn) func(string, ...any) (*sql.Rows, error) {
	return func(query string, args ...any) (*sql.Rows, error) {
		return conn.QueryContext(ctx, query, args...)
	}
}

// snapshotEnder returns what ends the snapshot: a ROLLBACK under ctx. Once ctx
// is done no statement can be sent under it, and none is tried: Close then
// closes the connection and the database, which ends the session on the
// server and the transaction with it.
func snapshotEnder(ctx context.Context, conn *sql.Conn) func() error {
	return func() error {
		if ctx.Err() != nil {
			return nil
		}
		_, err := conn.ExecContext(ctx, endSnapshot)
		return err
	}
}

// Notes are facts about the site learned while reading it, worded for the
// operator: settings that could not be worked out, and similar.
func (s *Source) Notes() []string {
	return append(append([]string(nil), s.settings.Notes...), s.notes...)
}

// fail cleans an error for the operator: the database password never appears
// in it, whatever the driver put there.
func (s *Source) fail(err error) error {
	return redact(err, s.settings.DBPassword)
}
