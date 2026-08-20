// Package rx compiles a regular expression once and hands it back thereafter.
//
// Several passes in this program build a pattern from something they only learn
// at run time — an element's tag name, an operator's class prefix, a meta tag's
// name — and called regexp.MustCompile with it. Compilation is the expensive
// half of a regular expression, and those call sites are the hot ones: once per
// element of every document, once per meta tag of every crawled page. A site
// with two thousand documents recompiled the same handful of patterns hundreds
// of thousands of times.
//
// The patterns are drawn from a small closed set in practice — the tag names
// HTML has, the classes one operator named — so caching them is bounded by the
// site rather than by its size.
package rx

import (
	"regexp"
	"sync"
)

// compiled holds one *regexp.Regexp per pattern seen.
var compiled sync.Map

// Get returns the compiled form of a pattern, compiling it the first time.
//
// It panics on a pattern that will not compile, exactly as regexp.MustCompile
// does: every pattern here is built by this program from its own literals, so a
// bad one is a bug to fix rather than input to handle.
func Get(pattern string) *regexp.Regexp {
	if cached, ok := compiled.Load(pattern); ok {
		return cached.(*regexp.Regexp)
	}

	// Compiled outside the lock and stored with LoadOrStore: two goroutines
	// racing on the same pattern do the work twice and agree on one result,
	// which is cheaper than making every reader wait for a writer.
	expression := regexp.MustCompile(pattern)

	actual, _ := compiled.LoadOrStore(pattern, expression)

	return actual.(*regexp.Regexp)
}
