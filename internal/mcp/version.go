package mcp

import (
	"fmt"
	"sort"
	"strings"
)

// Protocol revisions this server speaks.
//
// MCP splits its revisions into two eras. A *modern* revision carries the
// protocol version, client identity and client capabilities in every request's
// `_meta`, has no handshake at all and requires `server/discover`. A *legacy*
// revision opens with an `initialize` exchange and keeps the negotiated version
// for the life of the connection. The two are not interchangeable: a modern
// client talking to a legacy-only server fails outright.
const (
	// Version20260728 is the first modern revision.
	Version20260728 = "2026-07-28"

	// The legacy revisions. This server exposes tools and ping only, and the
	// wire contract for `initialize`, `tools/list`, `tools/call` and `ping` is
	// identical across all four, so each is answered as itself rather than
	// being collapsed into the oldest.
	Version20251125 = "2025-11-25"
	Version20250618 = "2025-06-18"
	Version20250326 = "2025-03-26"
	Version20241105 = "2024-11-05"
)

// Version is the revision this server prefers: the newest it implements, and
// the one a modern client is answered under unless the operator narrows the
// set with --protocol.
const Version = Version20260728

// modernVersions and legacyVersions are ordered newest first, which is the
// order server/discover advertises them in and the order a narrowed selection
// resolves "latest" in.
var (
	modernVersions = []string{Version20260728}
	legacyVersions = []string{Version20251125, Version20250618, Version20250326, Version20241105}
)

// Protocol selection keywords accepted by ParseVersionSet.
const (
	// ProtocolAll is the default: both eras, so a client of either kind is
	// served and neither has to know which one it reached.
	ProtocolAll = "all"
	// ProtocolModern drops the handshake era entirely.
	ProtocolModern = "modern"
	// ProtocolLegacy drops the modern era, leaving a server that looks exactly
	// like one built before the era split — which is what a client stuck on an
	// old SDK probes for.
	ProtocolLegacy = "legacy"
)

// VersionSet is the set of protocol revisions one server instance answers for.
// The zero value speaks nothing; build one with ParseVersionSet or AllVersions.
type VersionSet struct {
	modern []string
	legacy []string
}

// AllVersions is every revision this build implements, in both eras.
func AllVersions() VersionSet {
	return VersionSet{modern: modernVersions, legacy: legacyVersions}
}

// ParseVersionSet resolves the operator's --protocol choice.
//
// An empty value and "all" mean both eras. "modern" and "legacy" keep one era
// whole. Anything else must name a single revision — "2024-11-05" pins the
// server to the one revision a client that cannot be upgraded understands, and
// nothing else is offered or accepted.
func ParseVersionSet(spec string) (VersionSet, error) {
	switch strings.ToLower(strings.TrimSpace(spec)) {
	case "", ProtocolAll:
		return AllVersions(), nil
	case ProtocolModern:
		return VersionSet{modern: modernVersions}, nil
	case ProtocolLegacy:
		return VersionSet{legacy: legacyVersions}, nil
	}

	requested := strings.TrimSpace(spec)
	if contains(modernVersions, requested) {
		return VersionSet{modern: []string{requested}}, nil
	}
	if contains(legacyVersions, requested) {
		return VersionSet{legacy: []string{requested}}, nil
	}

	return VersionSet{}, fmt.Errorf(
		"unknown MCP protocol revision %q: expected one of %s, or %s/%s/%s",
		requested, strings.Join(KnownVersions(), ", "), ProtocolAll, ProtocolModern, ProtocolLegacy)
}

// KnownVersions lists every revision this build implements, newest first, for
// help text and error messages.
func KnownVersions() []string {
	return append(append([]string{}, modernVersions...), legacyVersions...)
}

// Modern reports the modern revisions this set offers, newest first. It is what
// server/discover advertises and what an UnsupportedProtocolVersionError names:
// a client picks from it for its *next* request, which is modern-shaped, so a
// legacy revision listed here would only send it back into the same error.
func (v VersionSet) Modern() []string {
	return v.modern
}

// SpeaksModern reports whether per-request versioning is answered at all. When
// it is false the server behaves exactly like one built before the era split,
// which is the deterministic signal a dual-era client probes for.
func (v VersionSet) SpeaksModern() bool {
	return len(v.modern) > 0
}

// SpeaksLegacy reports whether the initialize handshake is answered at all.
func (v VersionSet) SpeaksLegacy() bool {
	return len(v.legacy) > 0
}

// SupportsModern reports whether one modern revision is in the set.
func (v VersionSet) SupportsModern(version string) bool {
	return contains(v.modern, version)
}

// NegotiateLegacy answers an initialize handshake: the revision the client
// asked for when this server implements it, and otherwise the newest one it
// does — a legacy client has no way to ask again, so it gets a usable answer
// rather than an error.
func (v VersionSet) NegotiateLegacy(requested string) string {
	if contains(v.legacy, requested) {
		return requested
	}

	if len(v.legacy) == 0 {
		return ""
	}

	return v.legacy[0]
}

// Describe names the set for the --version banner and for the diagnostic a
// client gets when it opens in the wrong era.
func (v VersionSet) Describe() string {
	all := append(append([]string{}, v.modern...), v.legacy...)
	if len(all) == 0 {
		return "none"
	}

	sort.Sort(sort.Reverse(sort.StringSlice(all)))

	return strings.Join(all, ", ")
}

func contains(versions []string, want string) bool {
	for _, version := range versions {
		if version == want {
			return true
		}
	}

	return false
}
