package localdb

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
)

func TestSiteInfo(t *testing.T) {
	src, _ := openSite(t)
	info, err := src.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "Local Site", info.Name)
	assert.Equal(t, "Read from the database", info.Description)
	assert.Equal(t, "https://example.com", info.URL)
	assert.Equal(t, "https://example.com", info.HomeURL)
	assert.Equal(t, "UTC+2", info.Timezone)
	assert.Equal(t, "en_US", info.Language)
	assert.Equal(t, 1, info.StartOfWeek)
	assert.Equal(t, "page", info.ShowOnFront)
	assert.Empty(t, info.AdminEmail, "not published by the public API either")
	require.NotNil(t, info.FrontPage)
	assert.Equal(t, "https://example.com/", info.FrontPage.Link)
	require.NotNil(t, info.PostsPage)
	assert.Equal(t, "blog", info.PostsPage.Slug)

	home, err := src.Home()
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", home)
}

func TestWPHomeOverridesOption(t *testing.T) {
	db, mock, _ := localdbtest.New(t)
	localdbtest.ExpectSnapshot(mock)
	localdbtest.ExpectSite(mock)
	s := settings()
	s.Home, s.SiteURL = "https://www.example.org/", "https://www.example.org/wp"
	s.Notes = []string{"from wp-config"}
	src, err := New(context.Background(), db, s, Options{})
	require.NoError(t, err)
	defer func() { _ = src.Close() }()

	home, err := src.Home()
	require.NoError(t, err)
	assert.Equal(t, "https://www.example.org", home)
	up, err := src.Uploads()
	require.NoError(t, err)
	assert.Equal(t, "https://www.example.org/wp/wp-content/uploads", up.BaseURL)
	assert.Equal(t, []string{"from wp-config"}, src.Notes())
}

func TestTimezone(t *testing.T) {
	assert.Equal(t, "Europe/Warsaw", timezone("Europe/Warsaw", "1"))
	assert.Equal(t, "UTC", timezone("", "0"))
	assert.Equal(t, "UTC", timezone("", "x"))
	assert.Equal(t, "UTC-5:30", timezone("", "-5.5"))
	assert.Equal(t, "UTC+5:45", timezone("", "5.75"))
}

func TestNewRefusals(t *testing.T) {
	db, _, _ := localdbtest.New(t)
	s := settings()
	s.TablePrefix = "wp; DROP"
	_, err := New(context.Background(), db, s, Options{})
	assert.ErrorContains(t, err, "prefix")

	_, err = New(context.Background(), db, settings(), Options{BlogID: 2})
	assert.ErrorContains(t, err, "MULTISITE")
}

func TestNewBeginFails(t *testing.T) {
	db, mock, _ := localdbtest.New(t)
	mock.ExpectExec(localdbtest.Begin).WillReturnError(errors.New("denied for hunter2"))
	_, err := New(context.Background(), db, settings(), Options{})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
	assert.Contains(t, err.Error(), "[redacted]")
}

func TestNewConnFails(t *testing.T) {
	db, mock, _ := localdbtest.New(t)
	mock.ExpectClose()
	require.NoError(t, db.Close())
	_, err := New(context.Background(), db, settings(), Options{})
	assert.Error(t, err)
}

func TestCloseReportsRollbackError(t *testing.T) {
	db, mock, _ := localdbtest.New(t)
	mock.ExpectExec(localdbtest.Begin).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(localdbtest.End).WillReturnError(errors.New("gone"))
	src, err := New(context.Background(), db, settings(), Options{})
	require.NoError(t, err)
	assert.ErrorContains(t, src.Close(), "gone")
}

// §4: nothing but the snapshot statements and SELECTs is ever sent, and no
// statement names a forbidden column — across every collection.
func TestEveryStatementIsAReadOfAllowedColumns(t *testing.T) {
	src, rec := openSite(t)
	_, err := src.GetSiteInfo()
	require.NoError(t, err)
	for _, read := range []func() error{
		func() error { _, err := src.GetPosts(); return err },
		func() error { _, err := src.GetCustomPosts("portfolio"); return err },
		func() error { _, err := src.GetUsers(); return err },
		func() error { _, err := src.GetComments(); return err },
		func() error { _, err := src.GetMenus(); return err },
		func() error { _, err := src.GetMedia(); return err },
	} {
		require.NoError(t, read())
	}

	statements := rec.Statements()
	require.NotEmpty(t, statements)
	for _, s := range statements {
		require.NoError(t, localdbtest.Check(s), s)
		if s != localdbtest.Begin && s != localdbtest.End {
			assert.Regexp(t, `^SELECT `, s)
			assert.NotContains(t, s, "SELECT *")
		}
	}
	assert.Contains(t, statements, localdbtest.Begin)
}
