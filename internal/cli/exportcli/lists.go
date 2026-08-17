package exportcli

// Comma-separated lists, read once.
//
// Four flags take one — classes to preserve, classes to ignore, class prefixes
// that mark a builder, media types to skip — and each had its own loop with its
// own idea of what a stray comma or a trailing space meant.

import "strings"

// splitList reads a comma-separated flag value, trimming each entry and
// dropping the empty ones. A trailing comma is a typo, not an instruction to
// match everything.
func splitList(raw string) []string {
	var values []string

	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}

	return values
}
