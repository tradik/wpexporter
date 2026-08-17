package exportcli

// Flags that take one of a few words.
//
// A mode flag is a promise that the run does one of several things, and a typo
// in one is the worst kind of silence: `--preserve-styling nome` would read as
// an unknown value, fall to a default, and produce an export the operator
// believes is something else. So an unrecognized word ends the run, naming what
// was accepted — that is the whole of the value this file adds.

import (
	"fmt"
	"strings"
)

// checkMode returns an error naming the accepted answers when the value given
// is not one of them. An empty value is the default and always allowed.
func checkMode(flag, value string, accepted []string) error {
	if value == "" {
		return nil
	}

	for _, allowed := range accepted {
		if value == allowed {
			return nil
		}
	}

	return fmt.Errorf("--%s: unknown value %q (expected %s)",
		flag, value, strings.Join(accepted, ", "))
}
