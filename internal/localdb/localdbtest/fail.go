package localdbtest

import (
	"database/sql/driver"
	"errors"

	"github.com/DATA-DOG/go-sqlmock"
)

// Password is a password the failure errors carry, so a test can prove it
// never reaches the operator.
const Password = "hunter2"

// repeats is how many times a failure is registered: a source does not cache
// a failed read, so a later consumer asks again and must fail again.
const repeats = 20

// Fail makes the statement containing fragment (with args, when given) fail
// with an error that quotes Password. Register it before ExpectSite: in
// unordered mode the first matching expectation wins.
func Fail(mock sqlmock.Sqlmock, fragment string, args ...driver.Value) {
	for range repeats {
		expect(mock, fragment, args).WillReturnError(errors.New("access denied for user 'wp' using password " + Password))
	}
}

// Garble makes the statement containing fragment return a row of the wrong
// shape, so scanning it fails.
func Garble(mock sqlmock.Sqlmock, fragment string, args ...driver.Value) {
	for range repeats {
		expect(mock, fragment, args).WillReturnRows(sqlmock.NewRows([]string{"x"}).AddRow("not the right shape"))
	}
}

// expect registers a query expectation, with arguments when given.
func expect(mock sqlmock.Sqlmock, fragment string, args []driver.Value) *sqlmock.ExpectedQuery {
	e := mock.ExpectQuery(fragment)
	if len(args) > 0 {
		e = e.WithArgs(args...)
	}
	return e
}
