// Package cli holds what the toolkit's entry points share.
package cli

import (
	"fmt"
	"os"
)

// Main runs a command tree and applies the toolkit's exit convention: the error
// goes to stderr, the status is 1.
//
// Every binary's main() is this one call, so the convention is defined once
// rather than copied into each.
func Main(execute func() error) {
	if err := execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
