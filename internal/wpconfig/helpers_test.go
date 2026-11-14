package wpconfig

import "regexp"

// regexpDefine matches the whole define() line for name in the test fixtures.
func regexpDefine(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^define\( '` + name + `', [^\n]*\n`)
}
