package api

// WordPress does not promise a JSON type for every value it publishes. The REST
// root is the case that cost a whole site's identity: core emits
// `"gmt_offset":2` as a number, older installs and some plugins quote it, and a
// plain `string` field made json.Unmarshal fail on the entire document — so the
// export recorded no site name, no tagline and no timezone, without a word
// about why (#32).

import "encoding/json"

// jsonScalar is a string that also accepts an unquoted number, and treats
// anything else (null, an object, a bool) as absent rather than as an error:
// one unreadable field must never cost the rest of the document.
type jsonScalar string

// String returns the value as WordPress would have written it quoted.
func (s jsonScalar) String() string { return string(s) }

// UnmarshalJSON reads the quoted form, then the bare-number form, then gives up
// quietly.
func (s *jsonScalar) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = jsonScalar(text)
		return nil
	}

	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		*s = jsonScalar(number.String())
		return nil
	}

	*s = ""
	return nil
}
