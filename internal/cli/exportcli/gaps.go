package exportcli

// Incomplete collections (#37).
//
// A page of results that will not come after the retries used to end the
// export, discarding everything already fetched. Sites are flaky — a shared
// host answers 500 under load and the same URL works seconds later — so a
// single blip could make a large site unexportable. A gap is now reported and
// the run carries on: an export missing a hundred posts and saying so is worth
// more than no export.

import (
	"fmt"

	"github.com/tradik/wpexporter/internal/api"
)

// noteIncomplete records a partial read and lets the export continue, or
// returns the error when the failure was not a partial read.
//
// The distinction is the whole point: a page that could not be fetched leaves a
// hole in a collection, while anything else — an unreachable host, an
// unparseable response from the first request — is a broken export rather than
// an incomplete one, and pretending otherwise would hand back a plausible
// looking export of a site nobody read.
func noteIncomplete(gaps *[]string, collection string, err error) error {
	if err == nil {
		return nil
	}

	description, partial := api.Gap(err)
	if !partial {
		return fmt.Errorf("failed to get %s: %w", collection, err)
	}

	*gaps = append(*gaps, description)
	logf("Warning: %s are incomplete — %s\n", collection, description)

	return nil
}
