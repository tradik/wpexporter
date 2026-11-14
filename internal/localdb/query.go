package localdb

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// globalTables are shared by every site of a multisite network.
var globalTables = map[string]bool{"users": true, "usermeta": true}

// forbiddenColumns are never selected, by any query: credentials, private
// contact data and tracking data that an export has no business carrying.
var forbiddenColumns = []string{
	"user_pass", "user_email", "user_login", "user_activation_key",
	"comment_author_email", "comment_author_IP", "comment_agent",
}

// forbiddenPattern matches any of them as a whole word, in any case.
var forbiddenPattern = regexp.MustCompile(`(?i)\b(` + strings.Join(forbiddenColumns, "|") + `)\b`)

// tablePattern finds the {table} placeholders in a query template.
var tablePattern = regexp.MustCompile(`\{([a-z_]+)\}`)

// querier runs this package's queries on the snapshot connection, after
// checking each one is a read that touches nothing it should not.
type querier struct {
	conn   *sql.Conn
	prefix string
	blogID int
}

// table returns the real name of a WordPress table. The prefix was validated
// against ^[A-Za-z0-9_]+$ before this querier was built; it is the only text
// that ever enters a query other than through a ? placeholder.
func (q *querier) table(name string) string {
	if q.blogID > 1 && !globalTables[name] {
		return q.prefix + strconv.Itoa(q.blogID) + "_" + name
	}
	return q.prefix + name
}

// expand replaces each {table} in tmpl with its real name.
func (q *querier) expand(tmpl string) string {
	return tablePattern.ReplaceAllStringFunc(tmpl, func(m string) string {
		return q.table(m[1 : len(m)-1])
	})
}

// guard refuses a statement that is not a plain SELECT, or that names a
// forbidden column. It is the second line of defense: the tests check every
// query this package writes, and this checks them again on every run.
func guard(query string) error {
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT ") {
		return fmt.Errorf("refusing a statement that is not a SELECT")
	}
	if m := forbiddenPattern.FindString(query); m != "" {
		return fmt.Errorf("refusing a query that reads %s", m)
	}
	return nil
}

// query runs the SELECT in tmpl with args bound to its ? placeholders. An
// argument made with list stands for a whole IN (…) list.
func (q *querier) query(ctx context.Context, tmpl string, args ...any) (*sql.Rows, error) {
	stmt, flat := bindLists(q.expand(tmpl), args)
	if err := guard(stmt); err != nil {
		return nil, err
	}
	return q.conn.QueryContext(ctx, stmt, flat...) // #nosec G701 -- table names from a validated prefix; values bound as ?
}

// each runs tmpl and calls scan for every row.
func (q *querier) each(ctx context.Context, scan func(*sql.Rows) error, tmpl string, args ...any) error {
	rows, err := q.query(ctx, tmpl, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// inList is a list argument: one ? in the template becomes one ? per value.
type inList []any

// list makes an inList of values.
func list[T any](values []T) inList {
	out := make(inList, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// bindLists expands every inList argument into placeholders and flattens the
// arguments to match. An empty list becomes NULL, which IN (…) never matches.
// The templates in this package hold no ? inside a string literal.
func bindLists(stmt string, args []any) (string, []any) {
	var b strings.Builder
	flat := make([]any, 0, len(args))
	n := 0
	for i := 0; i < len(stmt); i++ {
		if stmt[i] != '?' || n >= len(args) {
			b.WriteByte(stmt[i])
			continue
		}
		arg := args[n]
		n++
		values, isList := arg.(inList)
		if !isList {
			b.WriteByte('?')
			flat = append(flat, arg)
			continue
		}
		if len(values) == 0 {
			b.WriteString("NULL")
			continue
		}
		b.WriteString(strings.TrimSuffix(strings.Repeat("?,", len(values)), ","))
		flat = append(flat, values...)
	}
	return b.String(), flat
}
