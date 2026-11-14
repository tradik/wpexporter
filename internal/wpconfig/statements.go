package wpconfig

import (
	"strings"
)

// statement is one define() — or the $table_prefix assignment — as written:
// the constant's name and the unevaluated expression giving its value.
type statement struct {
	name string
	expr string
}

// findDefines returns every define('NAME', expr) call in code, in file order.
// Calls whose first argument is not a plain string literal, method calls
// (->define, ::define) and identifiers that merely contain the word are not
// defines and are skipped.
func findDefines(code string) []statement {
	var out []statement
	// Byte-for-byte: strings.ToLower rewrites invalid UTF-8 into longer
	// sequences, after which its indices no longer point into code (found by
	// FuzzParse).
	lower := asciiLower(code)

	for from := 0; ; {
		at := strings.Index(lower[from:], "define")
		if at < 0 {
			return out
		}
		at += from
		from = at + len("define")

		if !standsAlone(code, at) {
			continue
		}
		open := skipSpace(code, from)
		if open >= len(code) || code[open] != '(' {
			continue
		}
		args, next, ok := scanArgs(code, open)
		if !ok {
			return out
		}
		if st, ok := defineCall(args); ok {
			out = append(out, st)
			from = next
		}
	}
}

// standsAlone reports whether the word at is the define function itself, not
// part of an identifier, a method (->define, ::define) or a variable.
func standsAlone(code string, at int) bool {
	if at == 0 {
		return true
	}
	c := code[at-1]
	return !isIdent(c) && c != '>' && c != ':' && c != '$'
}

// defineCall reads a define()'s arguments: a literal name and the expression.
// define() once took a third, case-insensitivity argument; the value is still
// the second.
func defineCall(args string) (statement, bool) {
	parts := splitTopLevel(args)
	if len(parts) < 2 {
		return statement{}, false
	}
	name, ok := literalName(strings.TrimSpace(parts[0]))
	if !ok {
		return statement{}, false
	}
	return statement{name: name, expr: strings.TrimSpace(parts[1])}, true
}

// literalName returns the content of a quoted constant name such as 'DB_NAME'.
func literalName(s string) (string, bool) {
	if len(s) < 3 || (s[0] != '\'' && s[0] != '"') || s[len(s)-1] != s[0] {
		return "", false
	}
	name := s[1 : len(s)-1]
	for i := 0; i < len(name); i++ {
		if !isIdent(name[i]) {
			return "", false
		}
	}
	return name, true
}

// findTablePrefix returns the expression assigned to $table_prefix, if any.
// The first plain assignment wins; a comparison (==) is not one.
func findTablePrefix(code string) (string, bool) {
	const variable = "$table_prefix"
	for from := 0; ; {
		at := strings.Index(code[from:], variable)
		if at < 0 {
			return "", false
		}
		at += from
		from = at + len(variable)

		if from < len(code) && isIdent(code[from]) {
			continue
		}
		eq := skipSpace(code, from)
		if eq >= len(code) || code[eq] != '=' || (eq+1 < len(code) && code[eq+1] == '=') {
			continue
		}
		end := statementEnd(code, eq+1)
		return strings.TrimSpace(code[eq+1 : end]), true
	}
}

// statementEnd returns the index of the semicolon ending the statement that
// starts at from (outside strings), or len(code).
func statementEnd(code string, from int) int {
	for i := from; i < len(code); i++ {
		switch code[i] {
		case '\'', '"':
			i = skipString(code, i) - 1
		case ';':
			return i
		}
	}
	return len(code)
}

// asciiLower lower-cases ASCII letters only, so the result has exactly the
// byte offsets of s.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// skipSpace returns the index of the first non-space byte at or after i.
func skipSpace(code string, i int) int {
	for i < len(code) && strings.IndexByte(" \t\r\n", code[i]) >= 0 {
		i++
	}
	return i
}
