package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseVersionSet pins what an operator can ask for, and what each choice
// leaves the server able to answer.
func TestParseVersionSet(t *testing.T) {
	tests := []struct {
		name       string
		spec       string
		wantModern []string
		wantLegacy bool
	}{
		{name: "empty is both eras", spec: "", wantModern: modernVersions, wantLegacy: true},
		{name: "all is both eras", spec: "all", wantModern: modernVersions, wantLegacy: true},
		{name: "modern drops the handshake", spec: "modern", wantModern: modernVersions},
		{name: "legacy drops per-request versioning", spec: "legacy", wantLegacy: true},
		{name: "case and space are forgiven", spec: "  LEGACY ", wantLegacy: true},
		{
			name:       "one modern revision",
			spec:       Version20260728,
			wantModern: []string{Version20260728},
		},
		{name: "one legacy revision", spec: Version20241105, wantLegacy: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			set, err := ParseVersionSet(tc.spec)
			require.NoError(t, err)

			assert.Equal(t, tc.wantModern, set.Modern())
			assert.Equal(t, len(tc.wantModern) > 0, set.SpeaksModern())
			assert.Equal(t, tc.wantLegacy, set.SpeaksLegacy())
		})
	}
}

// TestParseVersionSetRejectsUnknown: a typo in --protocol has to stop the
// server rather than quietly leave it speaking something else.
func TestParseVersionSetRejectsUnknown(t *testing.T) {
	_, err := ParseVersionSet("1999-01-01")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1999-01-01")
	// The message has to say what would have worked.
	assert.Contains(t, err.Error(), Version20260728)
	assert.Contains(t, err.Error(), Version20241105)
}

// TestPinnedLegacyRevisionOffersNothingElse: pinning to 2024-11-05 is for a
// client that understands only that one, so nothing newer is offered either.
func TestPinnedLegacyRevisionOffersNothingElse(t *testing.T) {
	set, err := ParseVersionSet(Version20241105)
	require.NoError(t, err)

	assert.False(t, set.SpeaksModern())
	assert.Equal(t, Version20241105, set.NegotiateLegacy(Version20250618))
	assert.Equal(t, Version20241105, set.Describe())
}

// TestNegotiateLegacy: a legacy client cannot ask twice, so it is answered with
// what it wanted when that is implemented and with the newest revision on offer
// when it is not.
func TestNegotiateLegacy(t *testing.T) {
	all := AllVersions()

	assert.Equal(t, Version20241105, all.NegotiateLegacy(Version20241105))
	assert.Equal(t, Version20250618, all.NegotiateLegacy(Version20250618))
	assert.Equal(t, Version20251125, all.NegotiateLegacy("1999-01-01"))
	assert.Equal(t, Version20251125, all.NegotiateLegacy(""))

	modernOnly, err := ParseVersionSet(ProtocolModern)
	require.NoError(t, err)
	assert.Empty(t, modernOnly.NegotiateLegacy(Version20241105))
}

// TestSupportsModern: only the modern era answers per-request versions, so a
// legacy revision named in _meta is not one of them.
func TestSupportsModern(t *testing.T) {
	all := AllVersions()

	assert.True(t, all.SupportsModern(Version20260728))
	assert.False(t, all.SupportsModern(Version20241105))
	assert.False(t, all.SupportsModern("1999-01-01"))
}

func TestDescribeAndKnownVersions(t *testing.T) {
	assert.Equal(t, "none", VersionSet{}.Describe())
	assert.Equal(t,
		"2026-07-28, 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05",
		AllVersions().Describe())

	known := KnownVersions()
	assert.Equal(t, Version20260728, known[0], "newest first")
	assert.Len(t, known, len(modernVersions)+len(legacyVersions))
}
