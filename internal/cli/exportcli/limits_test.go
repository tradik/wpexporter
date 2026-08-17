package exportcli

// Shaping a preview (#62): a map as well as a number, shortcuts for the common
// kinds, and one stated rule for the kind an operator names twice.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// TestPerTypeAcceptsEveryFormItPromises: a bare number, kind=N pairs, and both
// together — the three the help offers.
func TestPerTypeAcceptsEveryFormItPromises(t *testing.T) {
	fallback, byType, err := parsePerType("5")
	require.NoError(t, err)
	assert.Equal(t, 5, fallback)
	assert.Empty(t, byType)

	fallback, byType, err = parsePerType("posts=5,media=10")
	require.NoError(t, err)
	assert.Zero(t, fallback, "no bare number means everything else is unbounded")
	assert.Equal(t, map[string]int{"posts": 5, "media": 10}, byType)

	fallback, byType, err = parsePerType("5,media=10")
	require.NoError(t, err)
	assert.Equal(t, 5, fallback)
	assert.Equal(t, map[string]int{"media": 10}, byType)

	// A custom type is named by its slug, since there can be any number of them.
	_, byType, err = parsePerType("services=3, mec-events = 7 ")
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"services": 3, "mec-events": 7}, byType)
}

// TestPerTypeRefusesWhatItCannotRead: a typo must be refused before an export
// runs, not silently read as "no limit" — which would download the site the
// operator was trying not to download.
func TestPerTypeRefusesWhatItCannotRead(t *testing.T) {
	for _, raw := range []string{"five", "posts=many", "posts=-2", "-3"} {
		_, _, err := parsePerType(raw)
		assert.Error(t, err, "raw %q", raw)
	}
}

// TestShortcutBeatsTheMapAndSaysSo: two ways of saying the same thing is two
// places that can disagree. The rule is stated once, and reported when it bites.
func TestShortcutBeatsTheMapAndSaysSo(t *testing.T) {
	cfg := config.DefaultConfig()

	conflicts, err := applyPerTypeLimits(cfg, "5,media=10", map[string]int{"media": 4})
	require.NoError(t, err)

	assert.Equal(t, 4, cfg.LimitByType["media"], "the dedicated flag wins")
	assert.Equal(t, 5, cfg.LimitPerType, "the bare number still covers every other kind")
	require.Len(t, conflicts, 1)
	assert.Contains(t, conflicts[0], "--limit-media 4 overrides --limit-per-type media=10")
}

// TestAgreementIsNotAConflict: saying the same number twice is not worth a line
// of output.
func TestAgreementIsNotAConflict(t *testing.T) {
	cfg := config.DefaultConfig()

	conflicts, err := applyPerTypeLimits(cfg, "media=10", map[string]int{"media": 10})
	require.NoError(t, err)

	assert.Empty(t, conflicts)
	assert.Equal(t, 10, cfg.LimitByType["media"])
}

// TestShortcutsAloneNeedNoMap: the common case is two flags and no syntax.
func TestShortcutsAloneNeedNoMap(t *testing.T) {
	cfg := config.DefaultConfig()

	conflicts, err := applyPerTypeLimits(cfg, "", map[string]int{"posts": 5, "media": 10})
	require.NoError(t, err)

	assert.Empty(t, conflicts)
	assert.Equal(t, map[string]int{"posts": 5, "media": 10}, cfg.LimitByType)
	assert.Zero(t, cfg.LimitPerType)
	assert.True(t, limitsActive(cfg))
}

// TestLimitsActive: what decides that media follows the documents rather than
// the library.
func TestLimitsActive(t *testing.T) {
	assert.False(t, limitsActive(config.DefaultConfig()))

	total := config.DefaultConfig()
	total.Limit = 5
	assert.True(t, limitsActive(total))

	named := config.DefaultConfig()
	named.LimitByType = map[string]int{"media": 10}
	assert.True(t, limitsActive(named))
}

// TestNegativeLimitsAreRefused: validation catches what a flag can be given, so
// an export does not start on a nonsense budget.
func TestNegativeLimitsAreRefused(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.URL = "https://x.test"
	cfg.Output = t.TempDir()
	cfg.LimitByType = map[string]int{"media": -1}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "media")
}

// TestCountLineNamesACollectionTheBudgetNeverReached: `Products: 0` under a
// --limit reads as "this shop has no products", which is what sent the reporter
// of #65 hunting for a route bug that did not exist. The budget was spent on
// pages before the catalog was asked for, and the line said nothing about it.
func TestCountLineNamesACollectionTheBudgetNeverReached(t *testing.T) {
	assert.Equal(t, "Products: 0 (none within --limit)", countLine("Products", 0, 0, true))
	assert.Equal(t, "Products: 6 (limited from 282)", countLine("Products", 6, 282, true))

	// Without a limit, zero means zero and saying more would be an invention.
	assert.Equal(t, "Products: 0", countLine("Products", 0, 0, false))
	assert.Equal(t, "Media: 12", countLine("Media", 12, 12, true))
}
