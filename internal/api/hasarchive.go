package api

// `has_archive` is three things, not two (#53).
//
// WordPress returns `false` for a type with no archive, `true` for one whose
// archive lives under its own slug, and **the slug itself as a string** for one
// registered with an explicit archive:
//
//	{"product": {"has_archive": "shop"}, "mec-events": {"has_archive": true}}
//
// Declared as a bool, the whole /wp/v2/types document failed to decode on the
// first such type — and with it went every other type on the site. One product
// archive therefore cost a migration 56 events, 5 products and three more types,
// with one warning line between them and a "completed" summary. It is also why
// the pagination fix in #43 did not help that site: the walk it repaired was
// never reached, because discovery had already failed.
//
// The slug is kept rather than discarded. It is the address the archive is
// published at, which a migration needs anyway to avoid 404ing it.

import "encoding/json"

// hasArchive is WordPress's answer about a type's archive, in whichever of the
// three forms it chose.
type hasArchive struct {
	// Enabled reports whether the type publishes an archive at all.
	Enabled bool
	// Slug is the address it lives under, empty when the type uses its own.
	Slug string
}

// UnmarshalJSON reads the boolean form, then the slug form, and treats anything
// else — a null, an object, a number from some plugin — as "no archive" rather
// than as an error: one unreadable field must never cost the rest of the
// document, which is the whole lesson of this issue.
func (h *hasArchive) UnmarshalJSON(data []byte) error {
	var enabled bool
	if err := json.Unmarshal(data, &enabled); err == nil {
		h.Enabled = enabled
		h.Slug = ""

		return nil
	}

	var slug string
	if err := json.Unmarshal(data, &slug); err == nil {
		h.Slug = slug
		h.Enabled = slug != ""

		return nil
	}

	h.Enabled = false
	h.Slug = ""

	return nil
}
