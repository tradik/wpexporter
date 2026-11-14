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
	ctx      context.Context
	db       *sql.DB
	q        *querier
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

// New starts the read-only snapshot on db and returns the source. Open is the
// usual way in; New exists so tests can hand in a mock database.
func New(ctx context.Context, db *sql.DB, s *wpconfig.Settings, opts Options) (*Source, error) {
	if !wpconfig.ValidPrefix(s.TablePrefix) {
		return nil, fmt.Errorf("table prefix %q is not valid", s.TablePrefix)
	}
	if opts.BlogID > 1 && !s.Multisite {
		return nil, fmt.Errorf("--blog-id %d: wp-config.php does not enable MULTISITE", opts.BlogID)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, redact(err, s.DBPassword)
	}
	if _, err := conn.ExecContext(ctx, beginReadOnly); err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, redact(err, s.DBPassword)
	}

	return &Source{
		ctx:      ctx,
		db:       db,
		q:        &querier{conn: conn, prefix: s.TablePrefix, blogID: opts.BlogID},
		settings: s,
		opts:     opts,
		docs:     map[string][]models.WordPressPost{},
		counts:   map[string]int{},
		stated:   map[string]int{},
	}, nil
}

// Close ends the snapshot and releases the connection.
func (s *Source) Close() error {
	_, err := s.q.conn.ExecContext(s.ctx, endReadOnly)
	_ = s.q.conn.Close()
	_ = s.db.Close()
	return s.fail(err)
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
