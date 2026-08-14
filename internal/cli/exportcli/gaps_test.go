package exportcli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
)

// TestNoteIncompleteRecordsAGap: a collection that broke off mid-walk is
// reported and survived. The records already fetched are the caller's, and the
// export carries on (#37).
func TestNoteIncompleteRecordsAGap(t *testing.T) {
	var gaps []string

	partial := &api.PartialError{
		Endpoint: "posts",
		Page:     3,
		Fetched:  200,
		Err:      fmt.Errorf("API returned status 500"),
	}

	require.NoError(t, noteIncomplete(&gaps, "posts", partial))
	require.Len(t, gaps, 1)
	assert.Contains(t, gaps[0], "stopped at page 3 after 200 records")
	assert.Contains(t, gaps[0], "status 500")
}

// TestNoteIncompleteKeepsRealFailuresFatal: everything that is not a hole in a
// collection still ends the export. Reporting an unreachable host as a gap
// would hand back a plausible-looking export of a site nobody read.
func TestNoteIncompleteKeepsRealFailuresFatal(t *testing.T) {
	var gaps []string

	err := noteIncomplete(&gaps, "pages", errors.New("no such host"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get pages")
	assert.Empty(t, gaps)
}

// TestNoteIncompleteIgnoresSuccess: the common path records nothing.
func TestNoteIncompleteIgnoresSuccess(t *testing.T) {
	var gaps []string

	require.NoError(t, noteIncomplete(&gaps, "posts", nil))
	assert.Empty(t, gaps)
}
