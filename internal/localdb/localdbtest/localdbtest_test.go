package localdbtest

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	assert.NoError(t, Check("SELECT ID, user_nicename FROM wp_users"))
	assert.ErrorContains(t, Check("SELECT user_email FROM wp_users"), "user_email")
	assert.ErrorContains(t, Check("SELECT meta_value FROM wp_usermeta WHERE meta_key = 'wp_capabilities'"), "wp_capabilities")
	assert.ErrorContains(t, Check("LOCK TABLES wp_posts READ"), "LOCK")
	assert.ErrorContains(t, Check("update wp_options set x = 1"), "update")
}

// The matcher refuses a forbidden statement even when an expectation would
// match it, and records it once.
func TestMatcherRecordsViolations(t *testing.T) {
	db, mock, rec := New(t)
	mock.ExpectQuery("FROM wp_users").WillReturnRows(sqlmock.NewRows([]string{"x"}))
	for range 2 {
		_, err := db.Query("SELECT user_pass FROM wp_users")
		require.Error(t, err)
	}
	assert.Equal(t, []string{"SELECT user_pass FROM wp_users"}, rec.Violations())
	assert.Equal(t, []string{"SELECT user_pass FROM wp_users"}, rec.Statements())

	_, err := db.Query("SELECT 1 FROM elsewhere")
	assert.Error(t, err, "a statement no expectation contains is not matched")
}

// fatal captures Fatalf instead of ending the test.
type fatal struct {
	testing.TB
	msg string
}

func (f *fatal) Helper() {
	// Nothing to mark: this fake only records the failure.
}
func (f *fatal) Fatalf(format string, _ ...any) { f.msg = format }

func TestNewReportsMockFailure(t *testing.T) {
	saved := openMock
	openMock = func(sqlmock.QueryMatcher) (*sql.DB, sqlmock.Sqlmock, error) { return nil, nil, errors.New("boom") }
	t.Cleanup(func() { openMock = saved })
	f := &fatal{TB: t}
	db, mock, rec := New(f)
	assert.Nil(t, db)
	assert.Nil(t, mock)
	assert.Nil(t, rec)
	assert.Contains(t, f.msg, "sqlmock")
}
