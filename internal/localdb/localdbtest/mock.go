package localdbtest

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// Begin and End are the two statements that open and close the snapshot.
const (
	Begin = "START TRANSACTION READ ONLY, WITH CONSISTENT SNAPSHOT"
	End   = "ROLLBACK"
)

// New returns a mock database whose matcher enforces Check on every statement,
// with expectations matched in any order.
func New(t testing.TB) (*sql.DB, sqlmock.Sqlmock, *Recorder) {
	t.Helper()
	rec := &Recorder{}
	db, mock, err := openMock(rec.Matcher())
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
		return nil, nil, nil
	}
	mock.MatchExpectationsInOrder(false)
	return db, mock, rec
}

// openMock creates the mock; swapped in this package's own tests.
var openMock = func(m sqlmock.QueryMatcher) (*sql.DB, sqlmock.Sqlmock, error) {
	return sqlmock.New(sqlmock.QueryMatcherOption(m))
}

// ExpectSnapshot expects the snapshot to be opened and, at Close, ended.
func ExpectSnapshot(mock sqlmock.Sqlmock) {
	mock.ExpectExec(Begin).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(End).WillReturnResult(sqlmock.NewResult(0, 0))
}

// rows builds a result set.
func rows(columns []string, values ...[]any) *sqlmock.Rows {
	r := sqlmock.NewRows(columns)
	for _, v := range values {
		vals := make([]driver.Value, len(v))
		for i, x := range v {
			vals[i] = x
		}
		r.AddRow(vals...)
	}
	return r
}
