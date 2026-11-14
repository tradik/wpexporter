package wpconfig

import "strings"

// stripComments removes PHP comments — //, # and /* */ — from src, leaving
// string literals intact, so a commented-out define() can never be read as a
// live one. Line comments keep their newline.
func stripComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\'' || c == '"':
			end := skipString(src, i)
			b.WriteString(src[i:end])
			i = end
		case c == '#' || (c == '/' && i+1 < len(src) && src[i+1] == '/'):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return b.String()
			}
			i += end + 4
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
			i++
		}
	}

	return b.String()
}

// skipString returns the index just past the quoted string starting at
// src[start], or len(src) when it is never closed. A backslash always protects
// the next byte, which is right for scanning both quote styles.
func skipString(src string, start int) int {
	quote := src[start]
	for i := start + 1; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(src)
}

// isIdent reports whether c can be part of a PHP identifier.
func isIdent(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// scanArgs returns the text between the parenthesis at code[open] and its
// match, and the index just past the match. ok is false when it never closes.
func scanArgs(code string, open int) (args string, next int, ok bool) {
	depth := 0
	for i := open; i < len(code); i++ {
		switch code[i] {
		case '\'', '"':
			i = skipString(code, i) - 1
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return code[open+1 : i], i + 1, true
			}
		}
	}
	return "", len(code), false
}

// splitTopLevel splits s on commas that are outside strings and brackets.
func splitTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'', '"':
			i = skipString(s, i) - 1
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}
